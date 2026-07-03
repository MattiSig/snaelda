package generation

import (
	"strings"
	"testing"
)

// TestGenCatalogUIParity guarantees the Icelandic catalog defines every inline
// UI string the English source of truth defines (and no extras), so a
// deterministic Icelandic draft never falls back to an English UI string.
func TestGenCatalogUIParity(t *testing.T) {
	for key := range enGenCatalog.UI {
		if _, ok := isGenCatalog.UI[key]; !ok {
			t.Errorf("is catalog missing UI key %q", key)
		}
	}
	for key := range isGenCatalog.UI {
		if _, ok := enGenCatalog.UI[key]; !ok {
			t.Errorf("is catalog has unexpected UI key %q not in en", key)
		}
	}
}

// TestGenCatalogProfileParity checks that both catalogs cover the same
// categories and that every merged profile has all copy fields populated in the
// target language.
func TestGenCatalogProfileParity(t *testing.T) {
	categories := []string{categoryBusiness, "photography", "florist", "wellness", "creative", "craft", "food"}
	for _, locale := range []string{"en", "is"} {
		catalog := genCatalogs[locale]
		if _, ok := catalog.Profiles[categoryBusiness]; !ok {
			t.Fatalf("%s catalog missing required %q profile", locale, categoryBusiness)
		}
		for _, category := range categories {
			pc := catalog.profileCopy(category)
			fields := map[string]string{
				"CategoryLabel":   pc.CategoryLabel,
				"PrimaryCTA":      pc.PrimaryCTA,
				"ServicesTitle":   pc.ServicesTitle,
				"ServicesIntro":   pc.ServicesIntro,
				"AboutHeading":    pc.AboutHeading,
				"AboutBody":       pc.AboutBody,
				"GalleryHeading":  pc.GalleryHeading,
				"GalleryBody":     pc.GalleryBody,
				"ContactHeading":  pc.ContactHeading,
				"ContactBody":     pc.ContactBody,
				"SiteGoal":        pc.SiteGoal,
				"HomeHeadline":    pc.HomeHeadline,
				"HomeSubheadline": pc.HomeSubheadline,
				"AboutIntro":      pc.AboutIntro,
				"WorkProcess":     pc.WorkProcess,
				"FooterTagline":   pc.FooterTagline,
			}
			for name, value := range fields {
				if strings.TrimSpace(value) == "" {
					t.Errorf("%s/%s profile field %s is empty", locale, category, name)
				}
			}
			if len(pc.FeatureItems) == 0 {
				t.Errorf("%s/%s profile has no feature items", locale, category)
			}
		}
	}
}

// TestGenCatalogStructuredParity checks the pricing/testimonial/faq/team tables
// expose the same category keys across locales.
func TestGenCatalogStructuredParity(t *testing.T) {
	assertKeys := func(name string, en, is []string) {
		enSet := map[string]bool{}
		for _, k := range en {
			enSet[k] = true
		}
		isSet := map[string]bool{}
		for _, k := range is {
			isSet[k] = true
		}
		for k := range enSet {
			if !isSet[k] {
				t.Errorf("%s: is catalog missing category %q", name, k)
			}
		}
		for k := range isSet {
			if !enSet[k] {
				t.Errorf("%s: is catalog has extra category %q", name, k)
			}
		}
	}

	assertKeys("pricing", keysOf(enGenCatalog.Pricing), keysOf(isGenCatalog.Pricing))
	assertKeys("testimonials", keysOf(enGenCatalog.Testimonials), keysOf(isGenCatalog.Testimonials))
	assertKeys("faq", keysOf(enGenCatalog.FAQ), keysOf(isGenCatalog.FAQ))
	assertKeys("team", keysOf(enGenCatalog.Team), keysOf(isGenCatalog.Team))
}

func keysOf[T any](m map[string][]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestResolveGenLocale(t *testing.T) {
	cases := map[string]string{
		"is":    "is",
		"IS":    "is",
		"is-IS": "is",
		"en":    "en",
		"en-US": "en",
		"":      "en",
		"fr":    "en",
	}
	for in, want := range cases {
		if got := resolveGenLocale(in); got != want {
			t.Errorf("resolveGenLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBuildGenerationPlanIcelandic is the behavioral guard: an Icelandic plan
// must render Icelandic copy end-to-end and must not carry the old
// English-locale assumption or English fallback strings.
func TestBuildGenerationPlanIcelandic(t *testing.T) {
	plan := buildGenerationPlan("Snælda Ljósmyndir", "A calm photography studio that needs a gallery and booking flow.", "is")

	for _, assumption := range plan.Assumptions {
		if strings.Contains(assumption, "Default locale is English") {
			t.Fatalf("Icelandic plan still carries the English-locale assumption: %q", assumption)
		}
		if strings.Contains(assumption, "Contact routes use placeholder") {
			t.Fatalf("Icelandic plan leaked the English contact assumption: %q", assumption)
		}
	}

	home := plan.Pages[0]
	hero := home.Blocks[0]
	headline, _ := hero.Props["headline"].(string)
	if headline != isGenCatalog.Profiles["photography"].HomeHeadline {
		t.Fatalf("home headline = %q, want Icelandic photography headline", headline)
	}

	// The footer tagline is a category with no Icelandic override for
	// photography, so it must still resolve to the Icelandic business default.
	footer := home.Blocks[len(home.Blocks)-1]
	tagline, _ := footer.Props["tagline"].(string)
	if !strings.Contains(tagline, "einföld") {
		t.Fatalf("footer tagline is not Icelandic: %q", tagline)
	}

	// Page titles must be localized, not the English "Contact"/"Services".
	var titles []string
	for _, page := range plan.Pages {
		titles = append(titles, page.Title)
	}
	joined := strings.Join(titles, "|")
	for _, english := range []string{"Contact", "Gallery", "Services", "About"} {
		if strings.Contains(joined, english) {
			t.Fatalf("Icelandic plan leaked English page title %q in %q", english, joined)
		}
	}
}

// TestBuildGenerationPlanEnglishUnchanged confirms the English path still emits
// the original copy so the refactor is behavior-preserving for en sites.
func TestBuildGenerationPlanEnglishUnchanged(t *testing.T) {
	plan := buildGenerationPlan("North Light Studio", "A calm photography studio that needs a gallery and booking flow.", "en")
	home := plan.Pages[0]
	hero := home.Blocks[0]
	headline, _ := hero.Props["headline"].(string)
	if headline != "Natural photography for real people, places, and moments" {
		t.Fatalf("English home headline changed: %q", headline)
	}
	if plan.SiteGoal != "Turn visitors into photography inquiries and bookings." {
		t.Fatalf("English site goal changed: %q", plan.SiteGoal)
	}
}
