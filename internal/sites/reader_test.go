package sites

import (
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func TestAssembleDraftFromNormalizedRows(t *testing.T) {
	rows := NormalizedDraftRows{
		Site: siteRow{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: themeRow{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#f8f7f4",
					"foreground": "#1d2520",
					"primary":    "#315c4f",
					"accent":     "#c2774b",
				},
				Typography: map[string]any{
					"heading": "Inter",
					"body":    "Inter",
				},
				Layout: map[string]any{
					"maxWidth": "1120px",
				},
				Shape: map[string]any{
					"radius": "8px",
				},
			},
		},
		Pages: []pageRow{
			{
				ID:    "page_contact",
				Title: "Contact",
				Slug:  "/contact",
				Sort:  1,
				SEO: siteconfig.SEOConfig{
					Title:       "Contact Nordic Studio",
					Description: "Start a focused site project.",
				},
				Settings: map[string]any{},
			},
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				Sort:  0,
				SEO: siteconfig.SEOConfig{
					Title:       "Nordic Studio",
					Description: "Calm design systems for focused teams.",
				},
				Settings: map[string]any{},
			},
		},
		Blocks: []blockRow{
			{
				ID:      "block_text",
				PageID:  "page_home",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Sort:    1,
				Props: map[string]any{
					"heading":   "A structured seed draft",
					"body":      "Stored as validated application data.",
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				ID:      "block_hero",
				PageID:  "page_home",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Sort:    0,
				Props: map[string]any{
					"eyebrow":     "Nordic Studio",
					"headline":    "Clear websites for focused teams",
					"subheadline": "Structured sites from maintained blocks.",
					"primaryCta": map[string]any{
						"label": "Start a project",
						"href":  "/contact",
					},
					"layout": "centered",
				},
			},
			{
				ID:      "block_cta",
				PageID:  "page_contact",
				Type:    "cta_band",
				Version: siteconfig.BlockVersionV1,
				Sort:    0,
				Props: map[string]any{
					"heading": "Ready to begin?",
					"body":    "Send a concise note.",
					"variant": "primary",
				},
				Settings: siteconfig.BlockSettings{AnchorID: "contact"},
				Hidden:   true,
			},
		},
	}

	draft := AssembleDraft(rows)
	if err := siteconfig.ValidateDraft(draft); err != nil {
		t.Fatalf("validate assembled draft: %v", err)
	}
	if draft.Pages[0].ID != "page_home" {
		t.Fatalf("expected home page first, got %q", draft.Pages[0].ID)
	}
	if draft.Pages[0].Blocks[0].ID != "block_hero" {
		t.Fatalf("expected hero block first, got %q", draft.Pages[0].Blocks[0].ID)
	}
	if draft.Navigation.Primary[1].PageID != "page_contact" {
		t.Fatalf("expected navigation to follow page order, got %#v", draft.Navigation.Primary)
	}
	if !draft.Pages[1].Blocks[0].Settings.Hidden {
		t.Fatal("expected hidden block setting to be preserved from normalized row")
	}
	if draft.Pages[1].Blocks[0].Settings.AnchorID != "contact" {
		t.Fatalf("expected anchor setting to be preserved, got %q", draft.Pages[1].Blocks[0].Settings.AnchorID)
	}
}
