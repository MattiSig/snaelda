package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
	Usage   *openAIUsage   `json:"usage,omitempty"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIUsage struct {
	PromptTokens        int                       `json:"prompt_tokens"`
	CompletionTokens    int                       `json:"completion_tokens"`
	TotalTokens         int                       `json:"total_tokens"`
	PromptTokensDetails *openAIPromptTokensDetail `json:"prompt_tokens_details,omitempty"`
}

type openAIPromptTokensDetail struct {
	CachedTokens int `json:"cached_tokens"`
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

// pageChangeSetRequestPayload is the trimmed payload for the page change-set
// call: the catalog lives in the cached system prefix, so the user payload
// only carries the directive, page context, and the list of allowed type
// names for inserts.
type pageChangeSetRequestPayload struct {
	SiteName          string                 `json:"siteName"`
	SiteGoal          string                 `json:"siteGoal,omitempty"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
	Page              PageChangeSetPage      `json:"page"`
	NeighborPages     []NeighborPage         `json:"neighborPages,omitempty"`
	InsertableTypes   []string               `json:"insertableTypes,omitempty"`
	Prompt            string                 `json:"prompt"`
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

// BuildPlan is the legacy single-call planner used as fallback when the
// decomposed planner is unavailable. Phase 2c moved the static block registry
// and theme catalog into the cached system prefix; the per-call payload no
// longer duplicates them.
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
	}
	userJSON, err := json.Marshal(payload)
	if err != nil {
		return generationPlan{}, fmt.Errorf("encode generation prompt payload: %w", err)
	}

	var responsePayload openAIGenerationPlanPayload
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "site_generation_plan",
		Schema: generationPlanSchema(),
		System: cachedSiteContext() + "\n\n" + themeCatalogContext() + "\n\n" + layoutBlockCatalogContext() + "\n\n" + generationPlannerSystemPrompt + languageDirective(input.PreferredLanguage),
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
		"draftSummary":        summarizeDraftForTheme(draft),
		"brand":               draft.Brand,
		"darkModeRequirement": true,
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
		System: cachedSiteContext() + "\n\n" + themeCatalogContext() + "\n\n" + themeRegenerationSystemPrompt,
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return siteconfig.ThemeSelection{}, err
	}

	return responsePayload.ThemeSelection, nil
}

// BuildOutline runs the structure-first decomposed step. The model returns
// the site name, goal, theme selection, and page list (title + slug + goal +
// SEO) — but no blocks. The next stage drafts blocks per page in parallel.
// Token-wise this is the cheapest planning call: small input, small output.
func (p *OpenAIPlanner) BuildOutline(ctx context.Context, request OutlineRequest) (OutlineResult, error) {
	if p == nil {
		return OutlineResult{}, ErrDecomposedPlannerUnavailable
	}
	userJSON, err := json.Marshal(request)
	if err != nil {
		return OutlineResult{}, fmt.Errorf("encode outline payload: %w", err)
	}
	schema := outlineSchema()
	var responsePayload OutlineResult
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "site_outline",
		Schema: schema,
		System: cachedSiteContext() + "\n\n" + themeCatalogContext() + "\n\n" + outlinePlannerSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return OutlineResult{}, err
	}
	return responsePayload, nil
}

// BuildPageLayout chooses an ordered block skeleton for one page. It receives
// the compact layout catalog, not full prop schemas, so this step can focus on
// structure and block intent.
func (p *OpenAIPlanner) BuildPageLayout(ctx context.Context, request PageLayoutRequest) (PageLayoutResult, error) {
	if p == nil {
		return PageLayoutResult{}, ErrDecomposedPlannerUnavailable
	}
	userJSON, err := json.Marshal(request)
	if err != nil {
		return PageLayoutResult{}, fmt.Errorf("encode page layout payload: %w", err)
	}

	var responsePayload PageLayoutResult
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "page_layout",
		Schema: pageLayoutSchema(defaultAllowedBlockTypes()),
		System: cachedSiteContext() + "\n\n" + layoutBlockCatalogContext() + "\n\n" + pageLayoutSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return PageLayoutResult{}, err
	}
	return responsePayload, nil
}

// BuildPageContent runs the per-page composer step: in one structured-output
// call the model fills full props for the selected layout. It must not choose,
// reorder, add, or remove block types.
func (p *OpenAIPlanner) BuildPageContent(ctx context.Context, request PageContentRequest) (PageContentResult, error) {
	if p == nil {
		return PageContentResult{}, ErrDecomposedPlannerUnavailable
	}
	layout := append([]PageLayoutBlock(nil), request.Layout...)
	if len(layout) == 0 {
		return PageContentResult{}, fmt.Errorf("build page content: no layout blocks")
	}
	registry := siteconfig.DefaultBlockRegistry()
	seenTypes := map[string]bool{}
	blockVariants := make([]map[string]any, 0, len(layout))
	for _, layoutBlock := range layout {
		blockType := strings.TrimSpace(layoutBlock.Type)
		if blockType == "" || seenTypes[blockType] {
			continue
		}
		seenTypes[blockType] = true
		def, err := registry.Lookup(blockType, siteconfig.BlockVersionV1)
		if err != nil {
			return PageContentResult{}, fmt.Errorf("build page content: lookup %s: %w", blockType, err)
		}
		propsSchema := def.PropSchema
		if len(propsSchema) == 0 {
			propsSchema = map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			}
		}
		blockVariants = append(blockVariants, map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"type", "props"},
			"properties": map[string]any{
				"type":  map[string]any{"type": "string", "const": blockType},
				"props": propsSchema,
			},
		})
	}

	userJSON, err := json.Marshal(request)
	if err != nil {
		return PageContentResult{}, fmt.Errorf("encode page content payload: %w", err)
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"blocks"},
		"properties": map[string]any{
			"blocks": map[string]any{
				"type":     "array",
				"minItems": len(layout),
				"maxItems": len(layout),
				"items":    map[string]any{"anyOf": blockVariants},
			},
		},
	}
	var responsePayload PageContentResult
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "page_content",
		Schema: schema,
		System: cachedSiteContext() + "\n\n" + pageContentSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return PageContentResult{}, err
	}
	if err := validatePageContentMatchesLayout(responsePayload, layout); err != nil {
		return PageContentResult{}, err
	}
	return responsePayload, nil
}

func pageLayoutSchema(allowedTypes []string) map[string]any {
	if len(allowedTypes) == 0 {
		allowedTypes = []string{""}
	}
	blockSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"type", "purpose", "contentBrief", "variantHint"},
		"properties": map[string]any{
			"type": map[string]any{
				"type": "string",
				"enum": allowedTypes,
			},
			"purpose":      map[string]any{"type": "string"},
			"contentBrief": map[string]any{"type": "string"},
			"variantHint":  map[string]any{"type": "string"},
		},
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"blocks"},
		"properties": map[string]any{
			"blocks": map[string]any{
				"type":     "array",
				"minItems": 2,
				"maxItems": 9,
				"items":    blockSchema,
			},
		},
	}
}

func validatePageContentMatchesLayout(result PageContentResult, layout []PageLayoutBlock) error {
	if len(result.Blocks) != len(layout) {
		return fmt.Errorf("page content returned %d blocks for %d layout blocks", len(result.Blocks), len(layout))
	}
	for index, block := range result.Blocks {
		expected := strings.TrimSpace(layout[index].Type)
		if strings.TrimSpace(block.Type) != expected {
			return fmt.Errorf("page content block %d type mismatch: got %q want %q", index, block.Type, expected)
		}
	}
	return nil
}

func outlineSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		// OpenAI strict mode requires every key in properties to be required.
		"required": []string{"siteName", "siteGoal", "themeSelection", "pages", "assumptions"},
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
					"required":             []string{"title", "slug", "goal", "seo"},
					"properties": map[string]any{
						"title": map[string]any{"type": "string"},
						"slug":  map[string]any{"type": "string"},
						"goal":  map[string]any{"type": "string"},
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
			"assumptions": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}

// themeCatalogContext returns the JSON theme catalog as a stable suffix the
// model can rely on, positioned in the system prefix so OpenAI's auto-prompt
// caching (>1024-token identical prefixes) takes effect across calls.
func themeCatalogContext() string {
	catalog := map[string]any{
		"themeCatalog": siteconfig.DefaultThemeEditorCatalog(),
	}
	bytes, err := json.Marshal(catalog)
	if err != nil {
		return ""
	}
	return "Theme catalog (stable, prefer caching this prefix):\n" + string(bytes)
}

// cachedSiteContext is the long, stable system-message prefix shared by every
// generation-related call (outline, per-page plan, per-block copy, page
// change-set). It bundles only the stable brand direction shared by every
// generation call. Larger catalogs are appended only to the calls that need
// them so content calls do not carry every possible block in their context.
//
// The string is generated once per process via siteContextSystemOnce and held
// in memory.
var siteContextSystemOnce = sync.OnceValue(func() string {
	bundle := map[string]any{
		"brandDirection": "Snaelda is warm, crafted, ribbon-led, dependable, with a little Icelandic gravity. Dark mode should feel meaner than light mode while staying calm and readable.",
	}
	bytes, err := json.Marshal(bundle)
	if err != nil {
		return ""
	}
	return "Snaelda site context (stable, cache this prefix):\n" + string(bytes)
})

// summarizeBlockRegistryForPlan returns just enough for the model to pick
// block types: type slug, display name, category, and a one-line tagline.
// No field dumps, no enum options. Used in the cached system prefix so the
// per-call cost stays small.
func summarizeBlockRegistryForPlan() []map[string]any {
	definitions := siteconfig.DefaultBlockRegistry().Definitions()
	summary := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		summary = append(summary, map[string]any{
			"type":        definition.Type,
			"displayName": definition.DisplayName,
			"category":    string(definition.Category),
			"tagline":     definition.Tagline,
		})
	}
	return summary
}

// cachedSiteContext returns the shared cacheable prefix.
func cachedSiteContext() string {
	return siteContextSystemOnce()
}

// BuildClarifyingQuestions runs the small intake-form call: ask the model for
// 0-3 short, high-leverage questions that would meaningfully reshape the
// outline. Tiny payload, tiny output — designed to be the cheapest call in
// the pipeline. The model is expected to return zero questions when the
// prompt already carries enough intent.
func (p *OpenAIPlanner) BuildClarifyingQuestions(ctx context.Context, request ClarifyingQuestionsRequest) ([]ClarifyingQuestion, error) {
	if p == nil {
		return nil, ErrClarifyingPlannerUnavailable
	}
	userJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode clarifying questions payload: %w", err)
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"questions"},
		"properties": map[string]any{
			"questions": map[string]any{
				"type":     "array",
				"maxItems": MaxClarifyingQuestions,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					// OpenAI strict mode requires every property to be in
					// required. Empty string / empty array represent "not
					// applicable" for options and helper.
					"required": []string{"id", "prompt", "kind", "options", "helper"},
					"properties": map[string]any{
						"id":     map[string]any{"type": "string"},
						"prompt": map[string]any{"type": "string"},
						"kind": map[string]any{
							"type": "string",
							"enum": []string{
								ClarifyingQuestionKindSingle,
								ClarifyingQuestionKindMulti,
								ClarifyingQuestionKindText,
							},
						},
						"options": map[string]any{
							"type":     "array",
							"maxItems": 4,
							"items":    map[string]any{"type": "string"},
						},
						"helper": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	var responsePayload struct {
		Questions []ClarifyingQuestion `json:"questions"`
	}
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "clarifying_questions",
		Schema: schema,
		System: clarifyingQuestionsSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return nil, err
	}
	return responsePayload.Questions, nil
}

// PlanPageChanges runs the diff-style page reprompt: a small structured call
// that decides which blocks on a page to keep, edit, remove, or insert. It
// returns an ordered list of operations, never block copy. Per-block copy is
// drafted by SuggestBlockProps so the change-set call stays cheap and the
// rewrites can run in parallel afterwards.
func (p *OpenAIPlanner) PlanPageChanges(ctx context.Context, request PageChangeSetRequest) (PageChangeSetResponse, error) {
	if p == nil {
		return PageChangeSetResponse{}, ErrPageChangeSetUnavailable
	}
	insertableTypeNames := make([]string, 0, len(request.InsertableTypes))
	for _, item := range request.InsertableTypes {
		insertableTypeNames = append(insertableTypeNames, item.Type)
	}
	allowedInsertTypes := append([]string{}, insertableTypeNames...)
	if len(allowedInsertTypes) == 0 {
		allowedInsertTypes = []string{""}
	}
	// Trim payload: drop the full registry from the user payload (it's in the
	// cached system prefix). Send only the allowed type names for the model
	// to reference when inserting.
	payload := pageChangeSetRequestPayload{
		SiteName:          request.SiteName,
		SiteGoal:          request.SiteGoal,
		PreferredLanguage: request.PreferredLanguage,
		Brand:             request.Brand,
		Page:              request.Page,
		NeighborPages:     request.NeighborPages,
		InsertableTypes:   insertableTypeNames,
		Prompt:            request.Prompt,
	}
	userJSON, err := json.Marshal(payload)
	if err != nil {
		return PageChangeSetResponse{}, fmt.Errorf("encode page change-set payload: %w", err)
	}

	operationSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		// OpenAI strict mode requires every key in properties to be required.
		// The model returns "" for fields that don't apply to a given action.
		"required": []string{"action", "blockId", "type", "purpose", "reason"},
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					PageChangeSetActionKeep,
					PageChangeSetActionEdit,
					PageChangeSetActionRemove,
					PageChangeSetActionInsert,
				},
			},
			"blockId": map[string]any{"type": "string"},
			"type":    map[string]any{"type": "string"},
			"purpose": map[string]any{"type": "string"},
			"reason":  map[string]any{"type": "string"},
		},
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"operations", "changeSummary"},
		"properties": map[string]any{
			"operations": map[string]any{
				"type":  "array",
				"items": operationSchema,
			},
			"changeSummary": map[string]any{"type": "string"},
		},
	}

	var responsePayload PageChangeSetResponse
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "page_change_set",
		Schema: schema,
		System: cachedSiteContext() + "\n\n" + pageChangeSetSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: true,
	}, &responsePayload); err != nil {
		return PageChangeSetResponse{}, err
	}
	return responsePayload, nil
}

// RewriteImageQuery asks the model for a sharper Pexels search query than
// what the user could type from memory, derived from the surrounding site +
// page + block context. The response is a single short string; never block
// content. The call is read-only — it never changes the draft.
func (p *OpenAIPlanner) RewriteImageQuery(ctx context.Context, request ImageQueryRequest) (string, error) {
	if p == nil {
		return "", ErrBlockSuggestUnavailable
	}
	userJSON, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode image query request: %w", err)
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"query"},
		"properties": map[string]any{
			"query": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 80,
			},
		},
	}
	var response struct {
		Query string `json:"query"`
	}
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "image_query_rewrite",
		Schema: schema,
		System: imageQueryRewriterSystemPrompt,
		User:   string(userJSON),
		Strict: true,
	}, &response); err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Query), nil
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
		"action":            request.Action,
		"tone":              request.Tone,
		"instruction":       request.Instruction,
		"blockType":         request.Block.Type,
		"blockDisplayName":  request.Definition.DisplayName,
		"currentProps":      request.Block.Props,
		"pageTitle":         request.PageTitle,
		"pageSlug":          request.PageSlug,
		"siteName":          request.SiteName,
		"siteGoal":          request.SiteGoal,
		"neighbors":         request.NeighborText,
		"preferredLanguage": request.PreferredLanguage,
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
		System: cachedSiteContext() + "\n\n" + blockSuggestSystemPrompt + languageDirective(request.PreferredLanguage),
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

// DraftCollection turns a short user prompt into a proposed collection shape
// (labels, slug, schema). The handler validates and persists the result. The
// model never sees existing entries.
func (p *OpenAIPlanner) DraftCollection(ctx context.Context, request CollectionDraftRequest) (CollectionDraftResponse, error) {
	if p == nil {
		return CollectionDraftResponse{}, ErrBlockSuggestUnavailable
	}
	userJSON, err := json.Marshal(request)
	if err != nil {
		return CollectionDraftResponse{}, fmt.Errorf("encode collection draft request: %w", err)
	}
	fieldTypeEnum := []string{
		"text", "long_text", "rich_text", "number", "boolean", "date",
		"url", "email", "phone", "location", "enum", "enum_multi",
		"asset", "asset_list",
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"singularLabel", "pluralLabel", "slug", "schema"},
		"properties": map[string]any{
			"singularLabel": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 60,
			},
			"pluralLabel": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 60,
			},
			"slug": map[string]any{
				"type":        "string",
				"pattern":     "^[a-z][a-z0-9-]*$",
				"description": "URL slug, lowercase words separated by hyphens",
			},
			"schema": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": 12,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"key", "label", "type"},
					"properties": map[string]any{
						"key": map[string]any{
							"type":        "string",
							"pattern":     "^[a-z][a-z0-9_]*$",
							"description": "snake_case field key starting with a letter",
						},
						"label":       map[string]any{"type": "string", "minLength": 1, "maxLength": 60},
						"type":        map[string]any{"type": "string", "enum": fieldTypeEnum},
						"required":    map[string]any{"type": "boolean"},
						"description": map[string]any{"type": "string", "maxLength": 200},
						"options": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string", "minLength": 1, "maxLength": 60},
						},
					},
				},
			},
		},
	}

	var response openAICollectionDraftPayload
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "collection_draft",
		Schema: schema,
		System: collectionDrafterSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: false,
	}, &response); err != nil {
		return CollectionDraftResponse{}, err
	}

	schemaOut := make([]siteconfig.FieldDefinition, 0, len(response.Schema))
	for _, field := range response.Schema {
		schemaOut = append(schemaOut, siteconfig.FieldDefinition{
			Key:         field.Key,
			Label:       field.Label,
			Type:        field.Type,
			Required:    field.Required,
			Description: field.Description,
			Options:     field.Options,
		})
	}
	return CollectionDraftResponse{
		Slug:          response.Slug,
		SingularLabel: response.SingularLabel,
		PluralLabel:   response.PluralLabel,
		Schema:        schemaOut,
	}, nil
}

// DraftEntries turns a short user prompt into starter entries for an existing
// collection. The collections handler validates and persists the result.
func (p *OpenAIPlanner) DraftEntries(ctx context.Context, request EntryDraftRequest) (EntryDraftResponse, error) {
	if p == nil {
		return EntryDraftResponse{}, ErrBlockSuggestUnavailable
	}
	userJSON, err := json.Marshal(request)
	if err != nil {
		return EntryDraftResponse{}, fmt.Errorf("encode entry draft request: %w", err)
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"entries"},
		"properties": map[string]any{
			"entries": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": 10,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"slug", "fields"},
					"properties": map[string]any{
						"slug": map[string]any{
							"type":        "string",
							"pattern":     "^[a-z][a-z0-9-]*$",
							"description": "URL slug, lowercase words separated by hyphens",
						},
						"fields": map[string]any{
							"type":                 "object",
							"additionalProperties": true,
						},
						"seo": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"title":       map[string]any{"type": "string", "maxLength": 80},
								"description": map[string]any{"type": "string", "maxLength": 180},
							},
						},
					},
				},
			},
		},
	}

	var response openAIEntryDraftPayload
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "collection_entry_draft",
		Schema: schema,
		System: collectionEntryDrafterSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: false,
	}, &response); err != nil {
		return EntryDraftResponse{}, err
	}
	out := make([]EntryDraft, 0, len(response.Entries))
	for _, entry := range response.Entries {
		out = append(out, EntryDraft{
			Slug:   entry.Slug,
			Fields: entry.Fields,
			SEO:    entry.SEO,
		})
	}
	return EntryDraftResponse{Entries: out}, nil
}

// RewriteEntry revises one existing entry in place. The model may keep the
// slug/SEO intact or propose refinements, but it must stay within the
// collection schema.
func (p *OpenAIPlanner) RewriteEntry(ctx context.Context, request EntryRewriteRequest) (EntryRewriteResponse, error) {
	if p == nil {
		return EntryRewriteResponse{}, ErrBlockSuggestUnavailable
	}
	userJSON, err := json.Marshal(request)
	if err != nil {
		return EntryRewriteResponse{}, fmt.Errorf("encode entry rewrite request: %w", err)
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"entry", "changeSummary"},
		"properties": map[string]any{
			"changeSummary": map[string]any{"type": "string", "maxLength": 180},
			"entry": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"slug", "fields"},
				"properties": map[string]any{
					"slug": map[string]any{
						"type":        "string",
						"pattern":     "^[a-z][a-z0-9-]*$",
						"description": "URL slug, lowercase words separated by hyphens",
					},
					"fields": map[string]any{
						"type":                 "object",
						"additionalProperties": true,
					},
					"seo": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"title":       map[string]any{"type": "string", "maxLength": 80},
							"description": map[string]any{"type": "string", "maxLength": 180},
						},
					},
				},
			},
		},
	}

	var response openAIEntryRewritePayload
	if err := p.createStructuredCompletion(ctx, structuredCompletionRequest{
		Name:   "collection_entry_rewrite",
		Schema: schema,
		System: collectionEntryRewriteSystemPrompt + languageDirective(request.PreferredLanguage),
		User:   string(userJSON),
		Strict: false,
	}, &response); err != nil {
		return EntryRewriteResponse{}, err
	}

	return EntryRewriteResponse{
		Entry: EntryDraft{
			Slug:   response.Entry.Slug,
			Fields: response.Entry.Fields,
			SEO:    response.Entry.SEO,
		},
		ChangeSummary: response.ChangeSummary,
	}, nil
}

// CollectionDraftRequest mirrors the collections package's drafter contract.
// We re-declare it here so the generation package does not import collections
// (which would create a dependency loop — collections already references
// generation indirectly via the API server).
type CollectionDraftRequest struct {
	Prompt              string   `json:"prompt"`
	SiteName            string   `json:"siteName,omitempty"`
	SiteGoal            string   `json:"siteGoal,omitempty"`
	PreferredLanguage   string   `json:"preferredLanguage,omitempty"`
	ExistingCollections []string `json:"existingCollections,omitempty"`
}

// CollectionDraftResponse is the structured draft returned by the model.
type CollectionDraftResponse struct {
	Slug          string                       `json:"slug,omitempty"`
	SingularLabel string                       `json:"singularLabel"`
	PluralLabel   string                       `json:"pluralLabel"`
	Schema        []siteconfig.FieldDefinition `json:"schema"`
}

// EntryDraftRequest mirrors the collections package's entry drafter contract.
type EntryDraftRequest struct {
	Prompt            string               `json:"prompt"`
	SiteName          string               `json:"siteName,omitempty"`
	SiteGoal          string               `json:"siteGoal,omitempty"`
	PreferredLanguage string               `json:"preferredLanguage,omitempty"`
	Collection        EntryDraftCollection `json:"collection"`
	ExistingEntries   []EntryDraftExisting `json:"existingEntries,omitempty"`
}

type EntryDraftCollection struct {
	SingularLabel string                       `json:"singularLabel"`
	PluralLabel   string                       `json:"pluralLabel"`
	Slug          string                       `json:"slug"`
	Schema        []siteconfig.FieldDefinition `json:"schema"`
}

type EntryDraftExisting struct {
	Slug  string `json:"slug"`
	Title string `json:"title,omitempty"`
}

type EntryDraftResponse struct {
	Entries []EntryDraft `json:"entries"`
}

type EntryDraft struct {
	Slug   string               `json:"slug,omitempty"`
	Fields map[string]any       `json:"fields"`
	SEO    siteconfig.SEOConfig `json:"seo,omitempty"`
}

type EntryRewriteRequest struct {
	Prompt            string               `json:"prompt"`
	SiteName          string               `json:"siteName,omitempty"`
	SiteGoal          string               `json:"siteGoal,omitempty"`
	PreferredLanguage string               `json:"preferredLanguage,omitempty"`
	Collection        EntryDraftCollection `json:"collection"`
	Entry             EntryDraft           `json:"entry"`
}

type EntryRewriteResponse struct {
	Entry         EntryDraft `json:"entry"`
	ChangeSummary string     `json:"changeSummary,omitempty"`
}

type openAICollectionDraftPayload struct {
	Slug          string                            `json:"slug"`
	SingularLabel string                            `json:"singularLabel"`
	PluralLabel   string                            `json:"pluralLabel"`
	Schema        []openAICollectionDraftFieldEntry `json:"schema"`
}

type openAICollectionDraftFieldEntry struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type openAIEntryDraftPayload struct {
	Entries []openAIEntryDraftEntry `json:"entries"`
}

type openAIEntryDraftEntry struct {
	Slug   string               `json:"slug"`
	Fields map[string]any       `json:"fields"`
	SEO    siteconfig.SEOConfig `json:"seo,omitempty"`
}

type openAIEntryRewritePayload struct {
	Entry         openAIEntryDraftEntry `json:"entry"`
	ChangeSummary string                `json:"changeSummary"`
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
	if completion.Usage != nil {
		cached := 0
		if completion.Usage.PromptTokensDetails != nil {
			cached = completion.Usage.PromptTokensDetails.CachedTokens
		}
		fmt.Fprintf(os.Stderr, "openai_usage call=%s prompt=%d cached=%d completion=%d total=%d\n",
			input.Name,
			completion.Usage.PromptTokens,
			cached,
			completion.Usage.CompletionTokens,
			completion.Usage.TotalTokens,
		)
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
		"required":             []string{"palette", "fontPreset", "typeScale", "sectionSpacing", "contentWidth", "radius", "buttonStyle", "imageStyle"},
		"properties": map[string]any{
			"palette": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemePaletteCalmNordic,
				siteconfig.ThemePaletteCleanLocal,
				siteconfig.ThemePaletteBrightShopfront,
				siteconfig.ThemePaletteEditorialStudio,
				siteconfig.ThemePaletteHeritageCraft,
				siteconfig.ThemePaletteAfterHours,
			}},
			"fontPreset": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeFontBalanced,
				siteconfig.ThemeFontEditorial,
				siteconfig.ThemeFontStudioSans,
				siteconfig.ThemeFontModernGrotesk,
				siteconfig.ThemeFontHumanist,
				siteconfig.ThemeFontHeritageSerif,
			}},
			"typeScale": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeTypeScaleCompact,
				siteconfig.ThemeTypeScaleBalanced,
				siteconfig.ThemeTypeScaleExpressive,
			}},
			"sectionSpacing": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeSpacingSnug,
				siteconfig.ThemeSpacingComfortable,
				siteconfig.ThemeSpacingAiry,
			}},
			"contentWidth": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeContentWidthFocused,
				siteconfig.ThemeContentWidthStandard,
				siteconfig.ThemeContentWidthWide,
			}},
			"radius": map[string]any{"type": "string", "enum": []string{
				siteconfig.ThemeRadiusSharp,
				siteconfig.ThemeRadiusCrisp,
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

const collectionDrafterSystemPrompt = `You are the collection-schema drafter for Snaelda's website builder.
Return JSON only, matching the supplied schema exactly.
Translate a short prompt into a structured "collection" — a typed list of entries the user will edit later (services, projects, menu items, team members, FAQs, etc.).

Required output:
- "singularLabel": singular noun for one entry (e.g. "Service", "Project", "Menu item"), 1-60 chars.
- "pluralLabel": plural noun (e.g. "Services", "Projects", "Menu"), 1-60 chars.
- "slug": URL slug derived from pluralLabel, lowercase words separated by hyphens (e.g. "services", "menu-items"). Must start with a letter.
- "schema": 2-8 fields that describe one entry.

Field rules:
- "key" must be snake_case starting with a letter (e.g. "title", "price", "lead_chef").
- "label" is the human-facing name.
- "type" must be one of: text, long_text, rich_text, number, boolean, date, url, email, phone, location, enum, enum_multi, asset, asset_list.
- A title-like text field should usually be the first and required. Image fields use "asset". Long descriptions use "long_text".
- For "enum" / "enum_multi" include a short "options" array (3-6 plausible values).
- Set "required": true on the 1-3 fields an entry truly needs (typically title and one core attribute).
- Keep field counts tight — avoid fields the user didn't ask for. No SEO/meta fields, no internal ids, no audit fields. Snaelda adds those automatically.

Avoid duplicating an existing collection (provided in "existingCollections" by plural label) — pick a different angle or a more specific scope if the prompt overlaps.
Plain text only. No HTML, no Markdown.`

const collectionEntryDrafterSystemPrompt = `You are the collection-entry drafter for Snaelda's website builder.
Return JSON only, matching the supplied schema exactly.
Translate a short prompt into draft entries for the provided existing collection.

Input includes:
- "collection": labels, slug, and schema for the target collection.
- "existingEntries": current entry slugs and titles, used to avoid duplicates.

Required output:
- "entries": 1-10 entries unless the user asks for a smaller number.
- Each entry must include "slug" and "fields".
- "slug" must be lowercase words separated by hyphens and must not duplicate existing entries.
- "fields" must use only keys from collection.schema. Do not invent field keys.
- Required text-like fields should be filled. Optional fields may be omitted when the prompt does not provide enough information.

Field value rules:
- text, long_text, rich_text, url, email, phone, date, enum: string values only.
- number: number values only.
- boolean: true or false only.
- enum_multi: array of strings selected from the field's options.
- location: object with "name" and optional "region", "country", "lat", "lng".
- asset, asset_list, reference: omit these fields. They need real asset or entry ids.
- For enum and enum_multi, choose only from the field's declared options.
- Dates must be YYYY-MM-DD.

Write credible starter content, but do not invent external facts like real prices, phone numbers, emails, URLs, addresses, credentials, or names of real people unless the user provided them. If a fact is missing, keep the wording generic and editable.
Plain text only. No HTML, no Markdown.`

const collectionEntryRewriteSystemPrompt = `You are the collection-entry editor for Snaelda's website builder.
Return JSON only, matching the supplied schema exactly.
Revise the provided existing entry in place according to the user's prompt.

Input includes:
- "collection": labels, slug, and schema for the target collection.
- "entry": the current entry slug, fields, and SEO metadata.

Required output:
- "entry": the revised entry payload.
- "changeSummary": one short past-tense sentence describing what changed.

Rewrite rules:
- Keep the entry recognizable unless the prompt explicitly asks for a rename or repositioning.
- "entry.slug" should usually stay the same. Only change it when the prompt clearly changes the entry's title or URL intent.
- "entry.fields" must use only keys from collection.schema. Do not invent field keys.
- Preserve existing information that still fits the new direction. Improve clarity, specificity, and structure.
- Required fields should stay filled. Optional fields may be omitted when no update is needed.
- Asset, asset_list, and reference fields should be preserved unless the user explicitly asks to remove or replace them, so if you cannot improve them with the prompt just echo the current value or omit them.
- For enum and enum_multi, choose only from the field's declared options.
- text, long_text, rich_text, url, email, phone, date, enum: string values only.
- number: number values only.
- boolean: true or false only.
- enum_multi: array of strings selected from the field's options.
- location: object with "name" and optional "region", "country", "lat", "lng".
- SEO may be refined, but keep it honest and concise.

Write plain text only. No HTML, no Markdown. Do not invent external facts the user did not provide.`

const imageQueryRewriterSystemPrompt = `You are the image-search query writer for Snaelda's website builder.
Return JSON only, matching the supplied schema exactly.
Rewrite the supplied page/block context into a single short photo-search query (under 80 characters) for the Pexels stock-photo library.
Prefer concrete nouns and a clear setting. Photography lexicon is welcome ("warm natural light", "behind the scenes", "overhead flatlay") but keep it terse.
Honor the user instruction when one is provided — the instruction trumps inferred subject.
Do not return brand names, person names, hashtags, quotation marks, or commentary. Just the query string.
If the page is image-led (florist, photographer, restaurant, hotel, cafe, salon, ceramics studio, etc.) skew the query toward atmospheric, magazine-style results.
If the page is task-oriented (pricing, contact, FAQ) prefer broader supporting imagery that matches the brand mood.`

const outlinePlannerSystemPrompt = `You are the outline planner for Snaelda's structured site generator.
Return JSON only, matching the supplied schema exactly.
Produce the SITE OUTLINE only: siteName, siteGoal, a theme selection, and the page list with title, slug, goal, and SEO meta.
Do NOT produce blocks here — block planning runs in a separate per-page step.

Page rules:
- Keep the page list tight, usually 1 to 5 pages, never more than 10.
- Use "/" for the homepage slug. Use absolute path slugs like "/about" or "/contact" for other pages.
- Each page's goal should be one sentence describing what that page must accomplish.
- SEO title and description should be honest and search-friendly.

Theme rules:
- Stay within the supplied themeCatalog enums.
- Reflect Snaelda's warm, crafted, ribbon-led brand direction.
- If brand.primaryColor is set, prefer palettes whose accents harmonise with it.

When currentOutline is provided this is a site reprompt: prefer keeping existing pages and slugs unless the new prompt explicitly asks for change. Pages dropped from the outline will be removed; pages added will be drafted fresh.`

const pageLayoutSystemPrompt = `You are the page layout planner for Snaelda's structured site generator.
Return JSON only, matching the supplied schema exactly.

Pick an ordered list of 2-8 block skeletons for ONE page.
- Choose block types ONLY from the supplied layoutBlockCatalog; never invent a type.
- The first block should set the page's tone (typically a hero or section header).
- Include purpose, contentBrief, and variantHint for each block. Keep these concise; do not write final copy.
- Use contact_form only when the page should collect visitor input. Do not use testimonials, cta_band, or text_section as a substitute for an actual form.
- Do not duplicate sections the outline assigns to a different page.
- End with footer when this page needs site-level contact details, navigation, socials, or legal copy.

When the payload includes a "prompt", treat it as the authoritative brief for this business: it may list real services, prices, hours, contact details, testimonials, and FAQs. Provide blocks that give every relevant fact from the brief a home on this page (e.g. a features/service list for services, an faq block for FAQs, a footer or contact_form for contact details). Carry those facts forward in the contentBrief so the content pass fills them verbatim instead of inventing placeholders.

The layout is structural only. Full props and copy are written in a later call.`

const pageContentSystemPrompt = `You are the page content composer for Snaelda's structured site generator.
Return JSON only, matching the supplied schema exactly.

Fill full props for ONE page's supplied layout in a single pass.
- Preserve the supplied layout exactly: same number of blocks, same order, same block types.
- Do not choose, add, remove, or reorder block types.
- Each block's props must satisfy the per-type prop schema exactly, including structural choices (variant, layout, alignment, columns). Use the layout block's variantHint when it is compatible with the schema.
- Plain text only inside copy fields: no HTML, no Markdown, no scripts, no embed code.
- Match the page goal and the brand voice. Be specific, not generic. Avoid filler.
- Do not duplicate sections the outline assigns to a different page.

The payload's "prompt" is the authoritative brief for this business. When it states real facts — business name, service names and descriptions, prices, opening hours, phone, email, address, testimonials, FAQ answers — reproduce them VERBATIM in the matching props. Never overwrite a supplied fact with a placeholder, a rounded number, or an invented substitute. Each layout block's contentBrief tells you which facts belong in that block.

Repeater items (faq items, feature items, packages, etc.): write 3-6 unless the page goal demands more.
Names, prices, hours, exact addresses: use the values the prompt + interview answers supply; invent only when they are genuinely absent, and then leave plausible placeholders the user can edit.`

const clarifyingQuestionsSystemPrompt = `You are the intake-form planner for Snaelda's site generator.
Return JSON only, matching the supplied schema exactly.
Decide which 0 to 3 short questions would meaningfully improve the generated site if answered.
Quality over quantity: ask only when an answer would reshape the output, not when you are merely curious.
If the user's prompt already specifies a detail, do not ask about it again.
If the prompt is already detailed enough to generate confidently, return zero questions.

Question rules:
- Keep each prompt short (under 90 characters) and answerable in under 5 seconds.
- Prefer "single" kind with 3-4 concrete clickable options the user can pick at a glance. Use "multi" when several options can apply at once. Use "text" only when no reasonable shortlist exists.
- Options must be specific, not generic ("Booking flow", "Contact form", "Neither" — not "Yes" / "No").
- Avoid jargon and avoid asking about visual style unless the user volunteered an opinion in the prompt.

Each question id should be a short kebab-case slug (e.g., "primary-conversion", "needs-collection", "vibe"). Helper text is optional and should clarify rare ambiguity; do not pad with marketing copy.`

const pageChangeSetSystemPrompt = `You are the structured page-edit planner for Snaelda.
Return JSON only, matching the supplied schema exactly.
Decide which blocks on an existing website page to keep, edit, remove, or insert in response to the user's reprompt directive.
Do not write block copy or props — only choose operations and short purposes. Copy is drafted separately per block.

Operation rules:
- "keep": copy the named block through unchanged. Use blockId. Use this aggressively — preserve any block the directive does not touch.
- "edit": rewrite an existing block's copy in place. Use blockId and a short purpose (one sentence) describing the new direction. Never change the block's type.
- "remove": drop an existing block. Use blockId. Use this only when the directive clearly removes the section.
- "insert": add a new block. Use type (one of the supplied insertableTypes) and a short purpose. Place the operation at the position in the operations array where the new block should appear in the final page.

Ordering: operations apply in order against the final block list. A page with three existing blocks reprompted to "add a contact section after the gallery" returns: keep(b0), keep(b1), insert(contact), keep(b2).

Prefer the smallest change set that honors the directive. If the directive is ambiguous, lean toward keep + a single edit rather than a wholesale rewrite.
Set changeSummary to one short sentence describing the diff in plain English.`

const themeRegenerationSystemPrompt = `You are the structured theme selector for Snaelda.
Return JSON only, matching the supplied schema exactly.
Choose one coherent theme selection for a small-business website draft.
Stay within the provided theme enums.
Respect the brand direction: warm, crafted, quirky, dependable, and readable.
Dark mode is required and should feel sharper and meaner than light mode while remaining calm and accessible.
Prefer meaningful visual contrast over novelty for its own sake.`
