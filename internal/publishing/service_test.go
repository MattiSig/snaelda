package publishing

import (
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func TestBuildPublishedSnapshotAddsSEOFallbacks(t *testing.T) {
	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#151215",
					"foreground": "#f6f2ec",
					"primary":    "#8fc6ff",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block_hero",
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"headline":    "Clear websites for focused teams",
							"subheadline": "Structured sites from maintained blocks.",
							"layout":      "centered",
						},
					},
				},
			},
			{
				ID:    "page_contact",
				Title: "Contact",
				Slug:  "/contact",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block_text",
						Type:    "text_section",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading": "Get in touch",
							"body":    "Send a note to plan your next launch.",
						},
					},
				},
			},
		},
	}

	snapshot := buildPublishedSnapshot(draft)
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		t.Fatalf("validate snapshot: %v", err)
	}
	if snapshot.Site.SEO.Title != "Nordic Studio" {
		t.Fatalf("expected site title fallback, got %q", snapshot.Site.SEO.Title)
	}
	if snapshot.Site.SEO.Description != "Structured sites from maintained blocks." {
		t.Fatalf("expected site description fallback, got %q", snapshot.Site.SEO.Description)
	}
	if snapshot.Pages[1].SEO.Title != "Contact | Nordic Studio" {
		t.Fatalf("expected page title fallback, got %q", snapshot.Pages[1].SEO.Title)
	}
	if snapshot.Pages[1].SEO.Description != "Send a note to plan your next launch." {
		t.Fatalf("expected page description fallback, got %q", snapshot.Pages[1].SEO.Description)
	}
}
