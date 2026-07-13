package generation

import (
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func heroSubheadline(t *testing.T, block generationBlockPlan) (string, bool) {
	t.Helper()
	if block.Type != "hero" {
		t.Fatalf("expected hero block, got %q", block.Type)
	}
	value, ok := block.Props["subheadline"].(string)
	return value, ok
}

func TestPolishDropsDuplicateSupportingCopy(t *testing.T) {
	const shared = "Quick, 24/7 service."
	plan := generationPlan{
		SiteName: "The Sewer Guys",
		SiteGoal: "Help homeowners book an emergency plumber fast.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCleanLocal),
		Pages: []generationPagePlan{
			{
				Title: "Home",
				Slug:  "/",
				Blocks: []generationBlockPlan{
					{Type: "hero", Props: map[string]any{
						"headline":    "Clogged drain? We fix them 24/7.",
						"subheadline": shared,
					}},
					{Type: "footer", Props: map[string]any{
						"copyright": "The Sewer Guys",
						"tagline":   shared,
					}},
				},
			},
			{
				Title: "About",
				Slug:  "/about",
				Blocks: []generationBlockPlan{
					{Type: "hero", Props: map[string]any{
						"headline":    "About us",
						"subheadline": shared,
					}},
					{Type: "footer", Props: map[string]any{
						"copyright": "The Sewer Guys",
						"tagline":   shared,
					}},
				},
			},
		},
	}

	repaired := repairGenerationPlan(plan, "", true)

	if sub, ok := heroSubheadline(t, repaired.Pages[0].Blocks[0]); !ok || sub == "" {
		t.Fatalf("expected the first hero to keep its subheadline, got %q ok=%v", sub, ok)
	}
	if _, ok := heroSubheadline(t, repaired.Pages[1].Blocks[0]); ok {
		t.Fatalf("expected the duplicate about-hero subheadline to be dropped, got %#v", repaired.Pages[1].Blocks[0].Props)
	}
	for _, page := range repaired.Pages {
		footer := page.Blocks[len(page.Blocks)-1]
		if footer.Type != "footer" {
			t.Fatalf("expected trailing footer, got %q", footer.Type)
		}
		if tagline, ok := footer.Props["tagline"].(string); ok && tagline != "" {
			t.Fatalf("expected duplicate footer tagline dropped, got %q", tagline)
		}
	}
}

func TestPolishKeepsUniqueFooterTagline(t *testing.T) {
	plan := generationPlan{
		SiteName: "The Sewer Guys",
		SiteGoal: "Help homeowners book an emergency plumber fast.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCleanLocal),
		Pages: []generationPagePlan{
			{
				Title: "Home",
				Slug:  "/",
				Blocks: []generationBlockPlan{
					{Type: "hero", Props: map[string]any{
						"headline":    "Clogged drain? We fix them 24/7.",
						"subheadline": "Same-day dispatch across the city.",
					}},
					{Type: "footer", Props: map[string]any{
						"copyright": "The Sewer Guys",
						"tagline":   "Licensed, insured, and always on call.",
					}},
				},
			},
		},
	}

	repaired := repairGenerationPlan(plan, "", true)

	footer := repaired.Pages[0].Blocks[len(repaired.Pages[0].Blocks)-1]
	if tagline, _ := footer.Props["tagline"].(string); tagline != "Licensed, insured, and always on call." {
		t.Fatalf("expected unique footer tagline preserved, got %q", tagline)
	}
}

func TestPolishRetargetsBrokenHeroCTAs(t *testing.T) {
	plan := generationPlan{
		SiteName: "The Sewer Guys",
		SiteGoal: "Help homeowners book an emergency plumber fast.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCleanLocal),
		Pages: []generationPagePlan{
			{
				Title: "Home",
				Slug:  "/",
				Blocks: []generationBlockPlan{
					{Type: "hero", Props: map[string]any{
						"headline": "Clogged drain? We fix them 24/7.",
						"primaryCta": map[string]any{
							"label": "Book now",
							"href":  "/#contact",
						},
						"secondaryCta": map[string]any{
							"label": "Our services",
							"href":  "/services",
						},
					}},
				},
			},
			{
				Title:  "Contact",
				Slug:   "/contact",
				Blocks: []generationBlockPlan{{Type: "hero", Props: map[string]any{"headline": "Get in touch"}}},
			},
		},
	}

	repaired := repairGenerationPlan(plan, "", true)

	hero := repaired.Pages[0].Blocks[0]
	primary := hero.Props["primaryCta"].(map[string]any)
	if primary["href"] != "/contact" {
		t.Fatalf("expected broken section anchor rewritten to /contact, got %q", primary["href"])
	}
	secondary := hero.Props["secondaryCta"].(map[string]any)
	if secondary["href"] != "/contact" {
		t.Fatalf("expected missing-page link rewritten to /contact, got %q", secondary["href"])
	}
}

func TestPolishLeavesValidAndExternalCTAsUntouched(t *testing.T) {
	slugs := map[string]bool{"/": true, "/contact": true, "/about": true}

	cases := []struct {
		name    string
		href    string
		want    string
		changed bool
	}{
		{"existing page", "/about", "/about", false},
		{"home", "/", "/", false},
		{"mailto", "mailto:hi@example.com", "mailto:hi@example.com", false},
		{"tel", "tel:+3545551234", "tel:+3545551234", false},
		{"external", "https://example.com", "https://example.com", false},
		{"missing page", "/pricing", "/contact", true},
		{"section anchor", "/#contact", "/contact", true},
		{"bare anchor", "#quote", "/contact", true},
		{"path with anchor", "/about#team", "/contact", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := resolveInternalHref(tc.href, slugs, ctaFallbackHref(slugs))
			if got != tc.want || changed != tc.changed {
				t.Fatalf("resolveInternalHref(%q) = (%q, %v), want (%q, %v)", tc.href, got, changed, tc.want, tc.changed)
			}
		})
	}
}

func TestPolishSkippedOnReprompt(t *testing.T) {
	// A reprompt carries a partial plan (dropEmptyRepeaters=false); link
	// validation must not fire and rewrite a link to a page the partial plan
	// simply does not include.
	plan := generationPlan{
		SiteName: "The Sewer Guys",
		SiteGoal: "Help homeowners book an emergency plumber fast.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCleanLocal),
		Pages: []generationPagePlan{
			{
				Title: "Home",
				Slug:  "/",
				Blocks: []generationBlockPlan{
					{Type: "hero", Props: map[string]any{
						"headline": "Clogged drain? We fix them 24/7.",
						"primaryCta": map[string]any{
							"label": "Our services",
							"href":  "/services",
						},
					}},
				},
			},
		},
	}

	repaired := repairGenerationPlan(plan, "", false)

	hero := repaired.Pages[0].Blocks[0]
	primary := hero.Props["primaryCta"].(map[string]any)
	if primary["href"] != "/services" {
		t.Fatalf("expected reprompt to leave the link untouched, got %q", primary["href"])
	}
}
