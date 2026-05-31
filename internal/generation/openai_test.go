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
					"content": `{"siteName":"Ribbon & Pine","siteGoal":"Win more workshop bookings.","themeSelection":{"palette":"bright-shopfront","fontPreset":"studio-sans","sectionSpacing":"snug","radius":"pillowy","buttonStyle":"ink-solid","imageStyle":"paper-cut"},"pages":[{"title":"Home","slug":"/","goal":"Introduce the studio and get visitors to book.","blocks":[{"type":"hero","purpose":"Lead with the main offer","props":{"headline":"Book a warmer workshop site"}}],"seo":{"title":"Ribbon & Pine","description":"Workshop booking website"}}],"assetsNeeded":["hero-image"],"assumptions":["Classes are booked by inquiry."]}`,
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

func TestOpenAIPlannerRegenerateThemeSelectionParsesStructuredCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `{"themeSelection":{"palette":"after-hours","fontPreset":"editorial","sectionSpacing":"airy","radius":"soft","buttonStyle":"ribbon-fill","imageStyle":"woven-tint"}}`,
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
