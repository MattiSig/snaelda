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
		httpClient = &http.Client{Timeout: 45 * time.Second}
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

	payload := map[string]any{
		"scope":        firstNonEmpty(strings.TrimSpace(input.Scope), "site"),
		"prompt":       strings.TrimSpace(input.Prompt),
		"nameHint":     strings.TrimSpace(input.NameHint),
		"slugHint":     strings.TrimSpace(input.SlugHint),
		"attempt":      feedback.Attempt,
		"validation":   feedback.ValidationIssues,
		"blocks":       summarizeBlockRegistry(),
		"themeOptions": siteconfig.DefaultThemeEditorCatalog(),
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
	}, &responsePayload); err != nil {
		return generationPlan{}, err
	}

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

func (p *OpenAIPlanner) RegenerateThemeSelection(ctx context.Context, prompt string, draft siteconfig.SiteDraft) (siteconfig.ThemeSelection, error) {
	if p == nil {
		return siteconfig.ThemeSelection{}, fmt.Errorf("theme regeneration is not configured")
	}

	payload := map[string]any{
		"prompt":              strings.TrimSpace(prompt),
		"currentSelection":    siteconfig.DetectThemeSelection(draft.Theme),
		"themeOptions":        siteconfig.DefaultThemeEditorCatalog(),
		"draftSummary":        summarizeDraftForTheme(draft),
		"brandDirection":      "warm, crafted, friendly, a little silly, dependable",
		"darkModeRequirement": true,
		"darkModeDirection":   "warmer near-black/plum backgrounds, stronger contrast, brighter ribbon accents, calm readability",
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
	}, &responsePayload); err != nil {
		return siteconfig.ThemeSelection{}, err
	}

	return responsePayload.ThemeSelection, nil
}

type structuredCompletionRequest struct {
	Name   string
	Schema map[string]any
	System string
	User   string
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
				Strict: true,
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
								"type":                 "object",
								"additionalProperties": false,
								"required":             []string{"type", "purpose", "props"},
								"properties": map[string]any{
									"type":    map[string]any{"type": "string"},
									"purpose": map[string]any{"type": "string"},
									"props": map[string]any{
										"type":                 "object",
										"additionalProperties": true,
									},
								},
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
When validation feedback is provided, repair the plan instead of repeating the same mistake.`

const themeRegenerationSystemPrompt = `You are the structured theme selector for Snaelda.
Return JSON only, matching the supplied schema exactly.
Choose one coherent theme selection for a small-business website draft.
Stay within the provided theme enums.
Respect the brand direction: warm, crafted, quirky, dependable, and readable.
Dark mode is required and should feel sharper and meaner than light mode while remaining calm and accessible.
Prefer meaningful visual contrast over novelty for its own sake.`
