package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

var ErrOpenAIRefusal = errors.New("openai generation refused the request")

type OpenAIPlannerConfig struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

type OpenAIPlanner struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

type openAIChatCompletionRequest struct {
	Model          string                        `json:"model"`
	Messages       []openAIChatCompletionMessage `json:"messages"`
	ResponseFormat openAIResponseFormat          `json:"response_format"`
}

type openAIChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type       string                  `json:"type"`
	JSONSchema openAIJSONSchemaPayload `json:"json_schema"`
}

type openAIJSONSchemaPayload struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openAIChatCompletionResponse struct {
	Choices []openAIChoice `json:"choices"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIMessage struct {
	Content string `json:"content"`
	Refusal string `json:"refusal,omitempty"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

type openAIGenerationPlanPayload struct {
	SiteName       string                    `json:"siteName"`
	SiteGoal       string                    `json:"siteGoal"`
	ThemeSelection siteconfig.ThemeSelection `json:"themeSelection"`
	Pages          []generationPagePlan      `json:"pages"`
	AssetsNeeded   []string                  `json:"assetsNeeded"`
	Assumptions    []string                  `json:"assumptions"`
}

type openAIThemeSelectionPayload struct {
	ThemeSelection siteconfig.ThemeSelection `json:"themeSelection"`
}

type openAIBlockSuggestPayload struct {
	Props         map[string]any `json:"props"`
	ChangeSummary string         `json:"changeSummary"`
}

func NewOpenAIPlanner(cfg OpenAIPlannerConfig) (*OpenAIPlanner, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, nil
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("openai model is required")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 180 * time.Second}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIPlanner{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

func (p *OpenAIPlanner) BuildPlan(ctx context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
	if p == nil {
		return defaultGenerationPlanBuilder(ctx, input, feedback)
	}

	// The OpenAI structured completion runs as a single HTTP call that often
	// takes 20-60 seconds. The named phases (plan.pages / plan.theme /
	// plan.blocks) all happen inside that call, so without help the user sees
	// one step "stuck" for the entire wait and then watches everything jump.
	// We emit plan.pages immediately, then a background ticker walks through
	// plan.theme and plan.blocks while the LLM is working. emit() is
	// idempotent and won't regress the visible step, so the post-HTTP catch-up
	// is safe whether the ticker reached those steps or not.
	emit := newOrderedEmitter(feedback.ReportProgress, "plan.pages", "plan.theme", "plan.blocks")
	emit("plan.pages")

	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	go func() {
		select {
		case <-heartbeatCtx.Done():
			return
		case <-time.After(7 * time.Second):
		}
		emit("plan.theme")
		select {
		case <-heartbeatCtx.Done():
			return
		case <-time.After(9 * time.Second):
		}
		emit("plan.blocks")
	}()

	payload := map[string]any{
		"scope":             firstNonEmpty(strings.TrimSpace(input.Scope), "site"),
		"prompt":            strings.TrimSpace(input.Prompt),
		"nameHint":          strings.TrimSpace(input.NameHint),
		"slugHint":          strings.TrimSpace(input.SlugHint),
		"preferredLanguage": strings.TrimSpace(input.PreferredLanguage),
		"optionalHints":     input.OptionalHints,
		"brand":             input.Brand,
		"attempt":           feedback.Attempt,
		"validation":        feedback.ValidationIssues,
		"blocks":            summarizeBlockRegistry(),
		"themeOptions":      siteconfig.DefaultThemeEditorCatalog(),
	}
	userJSON, err := json.Marshal(payload)
	if err != nil {
		return generationPlan{}, fmt.Errorf("encode generation prompt payload: %w", err)
	}

	var responsePayload openAIGenerationPlanPayload
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "site_generation_plan",
		Schema: generationPlanSchema(),
		System: generationPlannerSystemPrompt,
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return generationPlan{}, err
	}
	stopHeartbeat()
	emit("plan.theme")
	emit("plan.blocks")

	plan := generationPlan{
		SiteName:       responsePayload.SiteName,
		SiteGoal:       responsePayload.SiteGoal,
		ThemePreset:    responsePayload.ThemeSelection.Palette,
		ThemeSelection: responsePayload.ThemeSelection,
		Theme:          siteconfig.BuildTheme(responsePayload.ThemeSelection),
		Pages:          responsePayload.Pages,
		AssetsNeeded:   responsePayload.AssetsNeeded,
		Assumptions:    responsePayload.Assumptions,
	}
	return plan, nil
}

// newOrderedEmitter wraps a progress callback so each named step is emitted at
// most once and never out of the declared order. Concurrent callers from the
// HTTP path and from the heartbeat goroutine are both safe.
func newOrderedEmitter(report func(string), order ...string) func(string) {
	if report == nil {
		return func(string) {}
	}
	rank := make(map[string]int, len(order))
	for i, step := range order {
		rank[step] = i
	}
	var (
		mu      sync.Mutex
		highest = -1
	)
	return func(step string) {
		index, ok := rank[step]
		if !ok {
			return
		}
		mu.Lock()
		if index <= highest {
			mu.Unlock()
			return
		}
		highest = index
		mu.Unlock()
		report(step)
	}
}

func (p *OpenAIPlanner) RegenerateThemeSelection(ctx context.Context, prompt string, draft siteconfig.SiteDraft) (siteconfig.ThemeSelection, error) {
	if p == nil {
		return siteconfig.ThemeSelection{}, fmt.Errorf("theme regeneration is not configured")
	}

	payload := map[string]any{
		"prompt":              strings.TrimSpace(prompt),
		"currentSelection":    siteconfig.DetectThemeSelection(draft.Theme),
		"themeOptions":        siteconfig.DefaultThemeEditorCatalog(),
		"draftSummary":        summarizeDraftForTheme(draft),
		"brand":               draft.Brand,
		"brandDirection":      "warm, crafted, friendly, a little silly, dependable",
		"darkModeRequirement": true,
		"darkModeDirection":   "warmer near-black/plum backgrounds, stronger contrast, brighter ribbon accents, calm readability",
		"brandColorRule":      "If brand.primaryColor is set, the rendered theme will override the preset's primary color with brand.primaryColor; pick a palette whose secondary/accent harmonize with it.",
	}
	userJSON, err := json.Marshal(payload)
	if err != nil {
		return siteconfig.ThemeSelection{}, fmt.Errorf("encode theme regeneration payload: %w", err)
	}

	var responsePayload openAIThemeSelectionPayload
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "theme_selection",
		Schema: themeSelectionSchema(),
		System: themeRegenerationSystemPrompt,
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return siteconfig.ThemeSelection{}, err
	}

	return responsePayload.ThemeSelection, nil
}

// SuggestBlockProps rewrites a single block's props using the model. The
// returned props are constrained by the block definition's PropSchema, so the
// shape always matches the existing block type/version.
func (p *OpenAIPlanner) SuggestBlockProps(ctx context.Context, request BlockSuggestRequest) (BlockSuggestResponse, error) {
	if p == nil {
		return BlockSuggestResponse{}, ErrBlockSuggestUnavailable
	}
	propsSchema := request.Definition.PropSchema
	if len(propsSchema) == 0 {
		propsSchema = map[string]any{
			"type":                 "object",
			"additionalProperties": false,
		}
	}

	payload := map[string]any{
		"action":           request.Action,
		"tone":             request.Tone,
		"instruction":      request.Instruction,
		"blockType":        request.Block.Type,
		"blockDisplayName": request.Definition.DisplayName,
		"currentProps":     request.Block.Props,
		"pageTitle":        request.PageTitle,
		"pageSlug":         request.PageSlug,
		"siteName":         request.SiteName,
		"siteGoal":         request.SiteGoal,
		"neighbors":        request.NeighborText,
	}
	userJSON, err := json.Marshal(payload)
	if err != nil {
		return BlockSuggestResponse{}, fmt.Errorf("encode block suggest payload: %w", err)
	}

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"props", "changeSummary"},
		"properties": map[string]any{
			"props":         propsSchema,
			"changeSummary": map[string]any{"type": "string"},
		},
	}

	var responsePayload openAIBlockSuggestPayload
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "block_suggest",
		Schema: schema,
		System: blockSuggestSystemPrompt,
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return BlockSuggestResponse{}, err
	}
	return BlockSuggestResponse{
		Props:         responsePayload.Props,
		ChangeSummary: responsePayload.ChangeSummary,
	}, nil
}

type structuredCompletionRequest struct {
	Name   string
	Schema map[string]any
	System string
	User   string
	Strict bool
}

func (p *OpenAIPlanner) createStructuredCompletion(ctx context.Context, input structuredCompletionRequest, output any) error {
	body := openAIChatCompletionRequest{
		Model: p.model,
		Messages: []openAIChatCompletionMessage{
			{Role: "system", Content: input.System},
			{Role: "user", Content: input.User},
		},
		ResponseFormat: openAIResponseFormat{
			Type: "json_schema",
			JSONSchema: openAIJSONSchemaPayload{
				Name:   input.Name,
				Strict: input.Strict,
				Schema: input.Schema,
			},
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send openai request: %w", err)
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read openai response: %w", err)
	}

	var completion openAIChatCompletionResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return fmt.Errorf("decode openai response: %w", err)
	}
	if res.StatusCode >= http.StatusBadRequest {
		if completion.Error != nil && completion.Error.Message != "" {
			return fmt.Errorf("openai request failed: %s", completion.Error.Message)
		}
		return fmt.Errorf("openai request failed with status %d", res.StatusCode)
	}
	if len(completion.Choices) == 0 {
		return fmt.Errorf("openai response did not include a choice")
	}
	message := completion.Choices[0].Message
	if strings.TrimSpace(message.Refusal) != "" {
		return fmt.Errorf("%w: %s", ErrOpenAIRefusal, message.Refusal)
	}
	if strings.TrimSpace(message.Content) == "" {
		return fmt.Errorf("openai response did not include structured content")
	}
	if err := json.Unmarshal([]byte(message.Content), output); err != nil {
		return fmt.Errorf("decode openai structured content: %w", err)
	}
	return nil
}

func summarizeBlockRegistry() []map[string]any {
	definitions := siteconfig.DefaultBlockRegistry().Definitions()
	summary := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		summary = append(summary, map[string]any{
			"type":        definition.Type,
			"displayName": definition.DisplayName,
			"category":    definition.Category,
			"fields":      summarizeEditorFields(definition.EditorSchema),
		})
	}
	return summary
}

func summarizeEditorFields(fields []siteconfig.EditorField) []map[string]any {
	summary := make([]map[string]any, 0, len(fields))
	for _, field := range fields {
		item := map[string]any{
			"name":      field.Name,
			"control":   field.Control,
			"valueType": field.ValueType,
			"options":   field.Options,
		}
		if len(field.Fields) > 0 {
			item["fields"] = summarizeEditorFields(field.Fields)
		}
		if len(field.ItemFields) > 0 {
			item["itemFields"] = summarizeEditorFields(field.ItemFields)
		}
		summary = append(summary, item)
	}
	return summary
}

func summarizeDraftForTheme(draft siteconfig.SiteDraft) map[string]any {
	pages := make([]map[string]any, 0, len(draft.Pages))
	for _, page := range draft.Pages {
		blockTypes := make([]string, 0, len(page.Blocks))
		for _, block := range page.Blocks {
			blockTypes = append(blockTypes, block.Type)
		}
		pages = append(pages, map[string]any{
			"title":      page.Title,
			"slug":       page.Slug,
			"seo":        page.SEO,
			"blockTypes": blockTypes,
		})
	}
	return map[string]any{
		"siteName":     draft.Site.Name,
		"siteSEO":      draft.Site.SEO,
		"pages":        pages,
		"navigation":   draft.Navigation,
		"pageCount":    len(draft.Pages),
		"currentTheme": siteconfig.DetectThemeSelection(draft.Theme),
	}
}

func generationPlanSchema() map[string]any {
	blockDefinitions := siteconfig.DefaultBlockRegistry().Definitions()
	blockSchemas := make([]map[string]any, 0, len(blockDefinitions))
	for _, definition := range blockDefinitions {
		propsSchema := definition.PropSchema
		if len(propsSchema) == 0 {
			propsSchema = map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			}
		}
		blockSchemas = append(blockSchemas, map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"type", "purpose", "props"},
			"properties": map[string]any{
				"type":    map[string]any{"type": "string", "const": definition.Type},
				"purpose": map[string]any{"type": "string"},
				"props":   propsSchema,
			},
		})
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"siteName", "siteGoal", "themeSelection", "pages", "assetsNeeded", "assumptions"},
		"properties": map[string]any{
			"siteName":       map[string]any{"type": "string"},
			"siteGoal":       map[string]any{"type": "string"},
			"themeSelection": themeSelectionObjectSchema(),
			"pages": map[string]any{
				"type":     "array",
				"maxItems": siteconfig.MaxPagesPerSite,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"title", "slug", "goal", "blocks", "seo"},
					"properties": map[string]any{
						"title": map[string]any{"type": "string"},
						"slug":  map[string]any{"type": "string"},
						"goal":  map[string]any{"type": "string"},
						"blocks": map[string]any{
							"type": "array",
							"items": map[string]any{
								"anyOf": blockSchemas,
							},
						},
						"seo": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"title", "description"},
							"properties": map[string]any{
								"title":       map[string]any{"type": "string"},
								"description": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			"assetsNeeded": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
					"enum": []string{"hero-image", "supporting-image"},
				},
			},
			"assumptions": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}

func themeSelectionSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"themeSelection"},
		"properties": map[string]any{
			"themeSelection": themeSelectionObjectSchema(),
		},
	}
}

func themeSelectionObjectSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"palette", "fontPreset", "sectionSpacing", "radius", "buttonStyle", "imageStyle"},
		"properties": map[string]any{
			"palette": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemePaletteCalmNordic,
				siteconfig.ThemePalettePlayfulRibbon,
				siteconfig.ThemePaletteMeanerDark,
			}},
			"fontPreset": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeFontBalanced,
				siteconfig.ThemeFontEditorial,
				siteconfig.ThemeFontStudioSans,
			}},
			"sectionSpacing": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeSpacingSnug,
				siteconfig.ThemeSpacingComfortable,
				siteconfig.ThemeSpacingAiry,
			}},
			"radius": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeRadiusTailored,
				siteconfig.ThemeRadiusSoft,
				siteconfig.ThemeRadiusPillowy,
			}},
			"buttonStyle": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeButtonRibbonFill,
				siteconfig.ThemeButtonThreadOutline,
				siteconfig.ThemeButtonInkSolid,
			}},
			"imageStyle": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeImageSoftFrame,
				siteconfig.ThemeImageWovenTint,
				siteconfig.ThemeImagePaperCut,
			}},
		},
	}
}

const generationPlannerSystemPrompt = `You are the structured website planning engine for Snaelda.
Return JSON only, matching the supplied schema exactly.
Plan a polished small-business website draft for a solo operator or very small team.
Use only the provided block types and safe plain-text content.
Do not emit HTML, CSS, JavaScript, markdown, embed code, or unsupported block types.
Keep the page plan tight, usually 1 to 5 pages, and never exceed 10 pages.
Use "/" for the homepage slug. Use absolute path slugs like "/about" or "/contact" for other pages.
Theme choices must stay within the provided enums and should reflect Snaelda's warm, crafted, ribbon-led brand direction.

Hero block variants:
- "standard" (default): the headline-led hero with optional image, layout "centered"/"split-left"/"split-right". Use it for most pages.
- "full-page": an immersive, viewport-filling hero where the image becomes the page on first load and the headline + CTA sit over it. Pick this when the brand is image-led (photographer, restaurant, hotel, florist, gallery, salon, ceramics studio, wedding planner, tattoo artist, cafe, food, travel, fashion) or when the prompt asks for a bold, atmospheric, magazine-style opener. Always include an "image" with descriptive "alt" text when choosing "full-page", and request a "hero-image" in assetsNeeded. Keep "headline" short (3 to 7 words) and the optional "subheadline" to a single sentence so the overlay stays clean.
Default the "variant" field to "standard" whenever the prompt does not clearly call for an immersive image-led opener.

When validation feedback is provided, repair the plan instead of repeating the same mistake.`

const blockSuggestSystemPrompt = `You are the per-block AI editor for Snaelda.
Return JSON only, matching the supplied schema exactly.
Rewrite the props of a single existing website block according to the requested action.
Never change the block type, version, or shape — only rewrite the prop values inside the supplied props object.
Plain text only: no HTML, no Markdown, no scripts, no embed code.
Keep meaning and the user's underlying offer intact. Improve the copy without inventing facts.
Honor the action:
- "tighten": preserve meaning, write shorter; trim filler and double the punch per word.
- "expand": add useful detail and texture without padding; one or two extra sentences max.
- "tone": rewrite in the requested tone (friendlier, more professional, more playful, or more direct) while keeping the substance.
- "rewrite": follow the user's free-form instruction; if it conflicts with the original meaning, prefer the instruction.
For repeater fields (FAQ items, feature items, plans, etc.) keep the array length unless the action says otherwise.
For image and link fields, keep the existing values exactly unless the action requires changing them.
For enum/select fields (layout, variant, alignment, columns, etc.) keep the existing value unless the action explicitly addresses layout.
Always set "changeSummary" to one short sentence describing what changed in plain English.`

const themeRegenerationSystemPrompt = `You are the structured theme selector for Snaelda.
Return JSON only, matching the supplied schema exactly.
Choose one coherent theme selection for a small-business website draft.
Stay within the provided theme enums.
Respect the brand direction: warm, crafted, quirky, dependable, and readable.
Dark mode is required and should feel sharper and meaner than light mode while remaining calm and accessible.
Prefer meaningful visual contrast over novelty for its own sake.`
