package generation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func TestOpenAIPlannerBuildPlanParsesStructuredCompletion(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"siteName":"Ribbon & Pine","siteGoal":"Win more workshop bookings.","themeSelection":{"palette":"bright-shopfront","fontPreset":"studio-sans","typeScale":"expressive","sectionSpacing":"snug","contentWidth":"wide","radius":"pillowy","buttonStyle":"ink-solid","imageStyle":"paper-cut"},"pages":[{"title":"Home","slug":"/","goal":"Introduce the studio and get visitors to book.","blocks":[{"type":"hero","purpose":"Lead with the main offer","props":{"headline":"Book a warmer workshop site"}}],"seo":{"title":"Ribbon & Pine","description":"Workshop booking website"}}],"assetsNeeded":["hero-image"],"assumptions":["Classes are booked by inquiry."]}`,
				},
			}},
		})
	}))
	defer server.Close()

	planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{
		APIKey:  "test-key",
		Model:   "gpt-5-mini",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("new planner: %v", err)
	}

	plan, err := planner.BuildPlan(context.Background(), generationInputContext{
		NameHint: "Ribbon & Pine",
		Prompt:   "A workshop booking website for a yarn studio.",
		Scope:    "site",
	}, generationPlanFeedback{Attempt: 1})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	if requestBody["model"] != "gpt-5-mini" {
		t.Fatalf("expected model in request, got %#v", requestBody["model"])
	}
	responseFormat, ok := requestBody["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("expected response_format in request, got %#v", requestBody)
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected json schema payload, got %#v", responseFormat)
	}
	schema, ok := jsonSchema["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema body, got %#v", jsonSchema)
	}
	properties := schema["properties"].(map[string]any)
	pages := properties["pages"].(map[string]any)
	pageItems := pages["items"].(map[string]any)
	pageProps := pageItems["properties"].(map[string]any)
	blocks := pageProps["blocks"].(map[string]any)
	blockItems := blocks["items"].(map[string]any)
	anyOf, ok := blockItems["anyOf"].([]any)
	if !ok || len(anyOf) == 0 {
		t.Fatalf("expected block schemas to be specialized, got %#v", blockItems)
	}
	var heroSchema map[string]any
	for _, candidate := range anyOf {
		schemaItem, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		blockProperties, ok := schemaItem["properties"].(map[string]any)
		if !ok {
			continue
		}
		blockType, ok := blockProperties["type"].(map[string]any)
		if ok && blockType["const"] == "hero" {
			heroSchema = schemaItem
			break
		}
	}
	if heroSchema == nil {
		t.Fatalf("expected hero schema in block union, got %#v", anyOf)
	}
	heroProps := heroSchema["properties"].(map[string]any)["props"].(map[string]any)
	if heroProps["additionalProperties"] != false {
		t.Fatalf("expected hero props to reject unknown properties, got %#v", heroProps)
	}
	requiredAny, ok := heroProps["required"].([]any)
	if !ok || len(requiredAny) == 0 {
		t.Fatalf("expected hero props schema to require fields, got %#v", heroProps)
	}
	hasHeadline := false
	for _, item := range requiredAny {
		if str, ok := item.(string); ok && str == "headline" {
			hasHeadline = true
			break
		}
	}
	if !hasHeadline {
		t.Fatalf("expected hero props required to include headline, got %#v", requiredAny)
	}
	if plan.ThemeSelection.Palette != siteconfig.ThemePaletteBrightShopfront {
		t.Fatalf("expected parsed theme selection, got %#v", plan.ThemeSelection)
	}
	if plan.Theme.Tokens.Colors["background"] != "#fff3df" {
		t.Fatalf("expected theme to be built from selection, got %#v", plan.Theme.Tokens.Colors)
	}
	if len(plan.Pages) != 1 || plan.Pages[0].Slug != "/" {
		t.Fatalf("expected homepage plan, got %#v", plan.Pages)
	}
}

func TestOpenAIPlannerBuildPageLayoutParsesStructuredCompletion(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"blocks":[{"type":"hero","purpose":"Open with contact intent.","contentBrief":"Invite questions about orders and tastings.","variantHint":"standard"},{"type":"contact_form","purpose":"Collect visitor inquiries.","contentBrief":"Ask for name, email, request type, and message.","variantHint":"simple-inquiry"},{"type":"footer","purpose":"Show practical contact details.","contentBrief":"Include email, phone, and hours.","variantHint":""}]}`,
				},
			}},
		})
	}))
	defer server.Close()

	planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{
		APIKey:  "test-key",
		Model:   "gpt-5-mini",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("new planner: %v", err)
	}

	result, err := planner.BuildPageLayout(context.Background(), PageLayoutRequest{
		SiteName: "Gothenburg Roastery",
		SiteGoal: "Help visitors order beans and ask about tastings.",
		Page:     OutlinePage{Title: "Contact", Slug: "/contact", Goal: "Let visitors send an inquiry form."},
		Outline:  []OutlinePage{{Title: "Home", Slug: "/", Goal: "Introduce the roastery."}},
	})
	if err != nil {
		t.Fatalf("build page layout: %v", err)
	}

	if len(result.Blocks) != 3 || result.Blocks[1].Type != "contact_form" {
		t.Fatalf("expected parsed contact_form layout, got %#v", result.Blocks)
	}
	messages := requestBody["messages"].([]any)
	system := messages[0].(map[string]any)["content"].(string)
	if !strings.Contains(system, "layoutBlockCatalog") || !strings.Contains(system, "contact_form") {
		t.Fatalf("expected layout catalog in system prompt, got %s", system)
	}
}

func TestOpenAIPlannerBuildPageContentUsesSelectedLayoutSchemas(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"blocks":[{"type":"hero","props":{"variant":"standard","headline":"Ask about coffee","layout":"centered"}},{"type":"contact_form","props":{"heading":"Send an inquiry","submitLabel":"Send inquiry","fields":[{"name":"name","label":"Name","type":"name","required":true},{"name":"email","label":"Email","type":"email","required":true},{"name":"message","label":"Message","type":"message","required":true}]}},{"type":"footer","props":{"copyright":"Gothenburg Roastery"}}]}`,
				},
			}},
		})
	}))
	defer server.Close()

	planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{
		APIKey:  "test-key",
		Model:   "gpt-5-mini",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("new planner: %v", err)
	}

	layout := []PageLayoutBlock{
		{Type: "hero", Purpose: "Open the page.", ContentBrief: "Contact opener.", VariantHint: "standard"},
		{Type: "contact_form", Purpose: "Collect inquiries.", ContentBrief: "Simple form.", VariantHint: "simple-inquiry"},
		{Type: "footer", Purpose: "Close with details.", ContentBrief: "Practical contact.", VariantHint: ""},
	}
	result, err := planner.BuildPageContent(context.Background(), PageContentRequest{
		SiteName: "Gothenburg Roastery",
		Page:     OutlinePage{Title: "Contact", Slug: "/contact", Goal: "Let visitors send an inquiry form."},
		Layout:   layout,
	})
	if err != nil {
		t.Fatalf("build page content: %v", err)
	}
	if len(result.Blocks) != 3 || result.Blocks[1].Type != "contact_form" {
		t.Fatalf("expected content for selected layout, got %#v", result.Blocks)
	}

	responseFormat := requestBody["response_format"].(map[string]any)
	jsonSchema := responseFormat["json_schema"].(map[string]any)
	schema := jsonSchema["schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	blocks := properties["blocks"].(map[string]any)
	if blocks["minItems"] != float64(3) || blocks["maxItems"] != float64(3) {
		t.Fatalf("expected exact layout length in schema, got %#v", blocks)
	}
	items := blocks["items"].(map[string]any)
	anyOf := items["anyOf"].([]any)
	types := map[string]bool{}
	for _, candidate := range anyOf {
		props := candidate.(map[string]any)["properties"].(map[string]any)
		blockType := props["type"].(map[string]any)["const"].(string)
		types[blockType] = true
	}
	if len(types) != 3 || !types["hero"] || !types["contact_form"] || !types["footer"] {
		t.Fatalf("expected schema to include only selected block types, got %#v", types)
	}

	messages := requestBody["messages"].([]any)
	system := messages[0].(map[string]any)["content"].(string)
	if strings.Contains(system, "layoutBlockCatalog") {
		t.Fatalf("did not expect layout catalog in page content prompt")
	}
}

func TestOpenAIPlannerBuildPageContentRejectsLayoutMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"blocks":[{"type":"contact_form","props":{"heading":"Wrong first","submitLabel":"Send","fields":[{"name":"email","label":"Email","type":"email","required":true}]}},{"type":"hero","props":{"headline":"Wrong order"}}]}`,
				},
			}},
		})
	}))
	defer server.Close()

	planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{
		APIKey:  "test-key",
		Model:   "gpt-5-mini",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("new planner: %v", err)
	}

	_, err = planner.BuildPageContent(context.Background(), PageContentRequest{
		SiteName: "Gothenburg Roastery",
		Page:     OutlinePage{Title: "Contact", Slug: "/contact", Goal: "Let visitors send an inquiry form."},
		Layout: []PageLayoutBlock{
			{Type: "hero", Purpose: "Open.", ContentBrief: "Hero.", VariantHint: ""},
			{Type: "contact_form", Purpose: "Collect.", ContentBrief: "Form.", VariantHint: ""},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "type mismatch") {
		t.Fatalf("expected layout mismatch error, got %v", err)
	}
}

func TestOpenAIPlannerRegenerateThemeSelectionParsesStructuredCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"themeSelection":{"palette":"after-hours","fontPreset":"editorial","typeScale":"expressive","sectionSpacing":"airy","contentWidth":"standard","radius":"soft","buttonStyle":"ribbon-fill","imageStyle":"woven-tint"}}`,
				},
			}},
		})
	}))
	defer server.Close()

	planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{
		APIKey:  "test-key",
		Model:   "gpt-5-mini",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("new planner: %v", err)
	}

	selection, err := planner.RegenerateThemeSelection(context.Background(), "A workshop site", siteconfig.SiteDraft{
		Site:  siteconfig.DraftSite{Name: "Ribbon & Pine"},
		Theme: siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
		},
		Pages: []siteconfig.PageDraft{{
			ID:     "page_home",
			Title:  "Home",
			Slug:   "/",
			Blocks: []siteconfig.BlockInstance{},
		}},
	})
	if err != nil {
		t.Fatalf("regenerate theme selection: %v", err)
	}

	if selection.Palette != siteconfig.ThemePaletteAfterHours || selection.SectionSpacing != siteconfig.ThemeSpacingAiry {
		t.Fatalf("expected parsed selection, got %#v", selection)
	}
}

func TestOpenAIPlannerReturnsRefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"refusal": "I can't help with that.",
				},
			}},
		})
	}))
	defer server.Close()

	planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{
		APIKey:  "test-key",
		Model:   "gpt-5-mini",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("new planner: %v", err)
	}

	_, err = planner.BuildPlan(context.Background(), generationInputContext{
		NameHint: "Ribbon & Pine",
		Prompt:   "A workshop booking website for a yarn studio.",
	}, generationPlanFeedback{})
	if err == nil || !strings.Contains(err.Error(), ErrOpenAIRefusal.Error()) {
		t.Fatalf("expected refusal error, got %v", err)
	}
}
