package generation

import (
	"context"
	"encoding/json"
	"fmt"
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
					"content": `{"blocks":{"block_0":{"type":"hero","props":{"variant":"standard","headline":"Ask about coffee","layout":"centered"}},"block_1":{"type":"contact_form","props":{"heading":"Send an inquiry","submitLabel":"Send inquiry","fields":[{"name":"name","label":"Name","type":"name","required":true},{"name":"email","label":"Email","type":"email","required":true},{"name":"message","label":"Message","type":"message","required":true}]}},"block_2":{"type":"footer","props":{"copyright":"Gothenburg Roastery"}}}}`,
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
	if blocks["type"] != "object" {
		t.Fatalf("expected blocks to be a position-keyed object, got %#v", blocks)
	}
	required := blocks["required"].([]any)
	if len(required) != 3 {
		t.Fatalf("expected one required key per layout position, got %#v", required)
	}
	blockProps := blocks["properties"].(map[string]any)
	types := map[string]bool{}
	for index, wantType := range []string{"hero", "contact_form", "footer"} {
		key := fmt.Sprintf("block_%d", index)
		slot, ok := blockProps[key].(map[string]any)
		if !ok {
			t.Fatalf("expected schema to pin %s, got %#v", key, blockProps)
		}
		slotProps := slot["properties"].(map[string]any)
		gotType := slotProps["type"].(map[string]any)["const"].(string)
		if gotType != wantType {
			t.Fatalf("expected %s pinned to %q, got %q", key, wantType, gotType)
		}
		types[gotType] = true
		// Each slot references its type's prop schema in $defs.
		ref := slotProps["props"].(map[string]any)["$ref"].(string)
		if ref != "#/$defs/props_"+wantType {
			t.Fatalf("expected %s props to $ref props_%s, got %q", key, wantType, ref)
		}
	}
	if len(types) != 3 || !types["hero"] || !types["contact_form"] || !types["footer"] {
		t.Fatalf("expected schema to pin only selected block types, got %#v", types)
	}
	defs := schema["$defs"].(map[string]any)
	for _, wantType := range []string{"hero", "contact_form", "footer"} {
		if _, ok := defs["props_"+wantType]; !ok {
			t.Fatalf("expected $defs to define props_%s, got %#v", wantType, defs)
		}
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
					"content": `{"blocks":{"block_0":{"type":"contact_form","props":{"heading":"Wrong first","submitLabel":"Send","fields":[{"name":"email","label":"Email","type":"email","required":true}]}},"block_1":{"type":"hero","props":{"headline":"Wrong order"}}}}`,
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

// captureSystemPrompt spins up a fake OpenAI-compatible endpoint that records
// the system message of the structured completion and replies with a minimal
// valid page-content payload, so tests can assert what prompt the model saw.
func captureSystemPrompt(t *testing.T, request PageContentRequest) string {
	t.Helper()
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"blocks":{"block_0":{"type":"hero","props":{"variant":"standard","headline":"Halló","layout":"centered"}}}}`,
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
	request.Layout = []PageLayoutBlock{{Type: "hero", Purpose: "Open.", ContentBrief: "Opener.", VariantHint: "standard"}}
	if _, err := planner.BuildPageContent(context.Background(), request); err != nil {
		t.Fatalf("build page content: %v", err)
	}
	messages := requestBody["messages"].([]any)
	return messages[0].(map[string]any)["content"].(string)
}

func TestLanguageDirectiveThreadsIntoIcelandicPrompt(t *testing.T) {
	system := captureSystemPrompt(t, PageContentRequest{
		SiteName:          "Snælda Hárstofa",
		SiteGoal:          "Bóka tíma.",
		PreferredLanguage: "is",
		Page:              OutlinePage{Title: "Forsíða", Slug: "/", Goal: "Kynna stofuna."},
	})
	for _, marker := range []string{"íslenska", "LANGUAGE CONTRACT", "thjonusta"} {
		if !strings.Contains(system, marker) {
			t.Fatalf("expected Icelandic directive marker %q in system prompt, got: %s", marker, system)
		}
	}
}

func TestLanguageDirectiveAbsentForEnglishPrompt(t *testing.T) {
	system := captureSystemPrompt(t, PageContentRequest{
		SiteName:          "Downtown Barbers",
		SiteGoal:          "Book appointments.",
		PreferredLanguage: "en",
		Page:              OutlinePage{Title: "Home", Slug: "/", Goal: "Introduce the shop."},
	})
	if strings.Contains(system, "LANGUAGE CONTRACT") || strings.Contains(system, "íslenska") {
		t.Fatalf("expected no language directive for English site, got: %s", system)
	}
}

// captureUserMessage records the user-role message of the next structured
// completion so tests can assert the brief the model actually saw.
func captureUserMessage(t *testing.T, respond func() string) (*string, http.Handler) {
	t.Helper()
	var captured string
	out := &captured
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		messages := requestBody["messages"].([]any)
		for _, raw := range messages {
			msg := raw.(map[string]any)
			if msg["role"] == "user" {
				*out = msg["content"].(string)
			}
		}
		_, _ = w.Write([]byte(respond()))
	})
	return out, handler
}

// TestRespinBriefReachesPerPageCalls guards the re-spin fidelity contract: the
// composed brief (verbatim services, prices, contact) must reach both per-page
// calls so the composer reproduces facts instead of inventing placeholders.
func TestRespinBriefReachesPerPageCalls(t *testing.T) {
	const brief = "Emergency plumber. Call 555-0142. Services: drain cleaning $99, water heater repair."

	t.Run("layout", func(t *testing.T) {
		user, handler := captureUserMessage(t, func() string {
			return `{"choices":[{"message":{"content":"{\"blocks\":[{\"type\":\"hero\",\"purpose\":\"Open.\",\"contentBrief\":\"Opener.\",\"variantHint\":\"standard\"},{\"type\":\"footer\",\"purpose\":\"Close.\",\"contentBrief\":\"Contact.\",\"variantHint\":\"\"}]}"}}]}`
		})
		server := httptest.NewServer(handler)
		defer server.Close()
		planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{APIKey: "test-key", Model: "gpt-5-mini", BaseURL: server.URL})
		if err != nil {
			t.Fatalf("new planner: %v", err)
		}
		if _, err := planner.BuildPageLayout(context.Background(), PageLayoutRequest{
			SiteName: "My Sewer Guys",
			Prompt:   brief,
			Page:     OutlinePage{Title: "Home", Slug: "/", Goal: "Introduce."},
		}); err != nil {
			t.Fatalf("build page layout: %v", err)
		}
		if !strings.Contains(*user, "555-0142") || !strings.Contains(*user, "drain cleaning $99") {
			t.Fatalf("expected brief facts in layout user payload, got: %s", *user)
		}
	})

	t.Run("content", func(t *testing.T) {
		user, handler := captureUserMessage(t, func() string {
			return `{"choices":[{"message":{"content":"{\"blocks\":{\"block_0\":{\"type\":\"hero\",\"props\":{\"variant\":\"standard\",\"headline\":\"Fast plumbing\",\"layout\":\"centered\"}}}}"}}]}`
		})
		server := httptest.NewServer(handler)
		defer server.Close()
		planner, err := NewOpenAIPlanner(OpenAIPlannerConfig{APIKey: "test-key", Model: "gpt-5-mini", BaseURL: server.URL})
		if err != nil {
			t.Fatalf("new planner: %v", err)
		}
		if _, err := planner.BuildPageContent(context.Background(), PageContentRequest{
			SiteName: "My Sewer Guys",
			Prompt:   brief,
			Page:     OutlinePage{Title: "Home", Slug: "/", Goal: "Introduce."},
			Layout:   []PageLayoutBlock{{Type: "hero", Purpose: "Open.", ContentBrief: "Opener.", VariantHint: "standard"}},
		}); err != nil {
			t.Fatalf("build page content: %v", err)
		}
		if !strings.Contains(*user, "555-0142") || !strings.Contains(*user, "drain cleaning $99") {
			t.Fatalf("expected brief facts in content user payload, got: %s", *user)
		}
	})
}
