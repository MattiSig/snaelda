package respin

import (
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func sampleAnalysis() AnalysisResult {
	return AnalysisResult{
		Classification: Classification{
			Vertical:   "salon",
			Services:   []string{"klipping", "litun"},
			Locale:     "is",
			Tone:       "warm and homey",
			Confidence: 0.9,
		},
		Fields: ExtractedFields{
			BusinessName: "Hárgreiðslustofan Klippt",
			Tagline:      "Fallegt hár, hlýlegt viðmót",
			About:        "Við höfum klippt Reykvíkinga í 20 ár. Komdu við.",
			Services: []ExtractService{
				{Name: "Klipping", Description: "Dömu- og herraklipping", Price: "6.900 kr."},
				{Name: "Litun", Price: "12.900 kr."},
			},
			Hours: []ExtractHours{
				{Day: "monday", Opens: "09:00", Closes: "17:00"},
				{Day: "saturday", Opens: "10:00", Closes: "14:00"},
				{Day: "sunday", Closed: true},
			},
			Contact: ContactDetails{
				Phone:   "+354 555 1234",
				Email:   "hallo@klippt.is",
				Address: "Laugavegur 1, 101 Reykjavík",
			},
			Testimonials: []Testimonial{
				{Quote: "Besta klipping í bænum!", Author: "Jóna"},
			},
		},
		TargetLocale: "is",
	}
}

func TestComposeProducesCanonicalInput(t *testing.T) {
	brand := BrandResult{
		Brand: siteconfig.BrandConfig{
			BusinessName: "Hárgreiðslustofan Klippt",
			Logo:         &siteconfig.BrandLogo{AssetID: "asset_logo", Alt: "Klippt"},
			PrimaryColor: "#7A3E48",
		},
	}
	comp := Compose(sampleAnalysis(), brand, ComposeContext{SourceURL: "https://klippt.is"})

	if comp.Degraded {
		t.Fatalf("high-confidence analysis should not be degraded")
	}
	if comp.Input.Name != "Hárgreiðslustofan Klippt" {
		t.Fatalf("Name = %q", comp.Input.Name)
	}
	if comp.Input.PreferredLanguage != "is" {
		t.Fatalf("PreferredLanguage = %q, want is", comp.Input.PreferredLanguage)
	}
	// Brand is carried verbatim (Spec 21 hard contract).
	if comp.Input.Brand.PrimaryColor != "#7A3E48" || comp.Input.Brand.Logo == nil || comp.Input.Brand.Logo.AssetID != "asset_logo" {
		t.Fatalf("brand not carried verbatim: %+v", comp.Input.Brand)
	}
	if got := comp.Input.OptionalHints["industry"]; got != "salon" {
		t.Fatalf("industry hint = %q", got)
	}
	if got := comp.Input.OptionalHints["pages"]; got != "home, services, about, contact" {
		t.Fatalf("pages hint = %q", got)
	}

	brief := comp.Input.Prompt
	for _, want := range []string{
		"Hárgreiðslustofan Klippt",
		"Klipping — Dömu- og herraklipping (6.900 kr.)",
		"Litun (12.900 kr.)",
		"Monday: 09:00–17:00",
		"Saturday: 10:00–14:00",
		"Sunday: Closed",
		"Phone: +354 555 1234",
		"Email: hallo@klippt.is",
		"Besta klipping í bænum!",
		"Source site: https://klippt.is",
		"verbatim",
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q\n---\n%s", want, brief)
		}
	}
}

func TestComposeThreadsSiteIDAndSeedAssets(t *testing.T) {
	brand := BrandResult{
		Brand:        siteconfig.BrandConfig{BusinessName: "Klippt", PrimaryColor: "#7A3E48"},
		HeroAssetIDs: []string{"hero_a", "hero_b"},
	}
	comp := Compose(sampleAnalysis(), brand, ComposeContext{SourceURL: "https://klippt.is", SiteID: "reserved-site"})

	if comp.Input.SiteID != "reserved-site" {
		t.Fatalf("SiteID = %q, want reserved-site", comp.Input.SiteID)
	}
	if len(comp.Input.SeedAssetIDs) != 2 || comp.Input.SeedAssetIDs[0] != "hero_a" || comp.Input.SeedAssetIDs[1] != "hero_b" {
		t.Fatalf("SeedAssetIDs = %#v, want the pulled hero photos", comp.Input.SeedAssetIDs)
	}
}

func TestComposeOmitsSectionsWithoutContent(t *testing.T) {
	analysis := AnalysisResult{
		Classification: Classification{Vertical: "cafe", Locale: "is", Confidence: 0.8},
		Fields:         ExtractedFields{BusinessName: "Kaffi Sól"},
		TargetLocale:   "is",
	}
	comp := Compose(analysis, BrandResult{}, ComposeContext{})

	brief := comp.Input.Prompt
	for _, absent := range []string{"Services:", "Opening hours:", "Contact:", "Testimonials", "Source site:"} {
		if strings.Contains(brief, absent) {
			t.Errorf("brief should omit %q for empty content\n---\n%s", absent, brief)
		}
	}
	// Only home page when nothing else was extracted.
	if got := comp.Input.OptionalHints["pages"]; got != "home" {
		t.Fatalf("pages hint = %q, want home", got)
	}
	// BusinessName backfilled onto the brand config even without a pulled brand.
	if comp.Input.Brand.BusinessName != "Kaffi Sól" {
		t.Fatalf("brand business name = %q", comp.Input.Brand.BusinessName)
	}
}

func TestComposeCarriesSoftDegradation(t *testing.T) {
	analysis := sampleAnalysis()
	analysis.Degraded = true
	analysis.DegradationReason = "low classification confidence; using generic block set"

	comp := Compose(analysis, BrandResult{}, ComposeContext{})
	if !comp.Degraded {
		t.Fatalf("expected degraded composition")
	}
	if comp.DegradationReason != analysis.DegradationReason {
		t.Fatalf("reason = %q", comp.DegradationReason)
	}
	// Still a full, usable input — soft degradation generates with the generic set.
	if strings.TrimSpace(comp.Input.Prompt) == "" {
		t.Fatalf("degraded composition must still carry a prompt")
	}
	if comp.PromptPrefill == "" {
		t.Fatalf("degraded composition must carry a prompt prefill")
	}
}

func TestComposePrefillReadsNaturally(t *testing.T) {
	comp := Compose(sampleAnalysis(), BrandResult{}, ComposeContext{})
	want := "A website for Hárgreiðslustofan Klippt, a salon business. Fallegt hár, hlýlegt viðmót."
	if comp.PromptPrefill != want {
		t.Fatalf("prefill = %q, want %q", comp.PromptPrefill, want)
	}
}

func TestComposeDefaultsLocaleToIcelandic(t *testing.T) {
	analysis := AnalysisResult{Fields: ExtractedFields{BusinessName: "X"}}
	comp := Compose(analysis, BrandResult{}, ComposeContext{})
	if comp.Input.PreferredLanguage != "is" {
		t.Fatalf("PreferredLanguage = %q, want is default", comp.Input.PreferredLanguage)
	}
}

func TestComposeDegradedFromSalvage(t *testing.T) {
	comp := ComposeDegraded(
		"thin content; degrade to prompt flow",
		Salvage{BusinessName: "Bakarí Braut", Vertical: "Bakery", Snippet: "Ferskt brauð daglega. Opið alla daga."},
		BrandResult{Brand: siteconfig.BrandConfig{PrimaryColor: "#123456"}},
		ComposeContext{SourceURL: "https://braut.is"},
	)

	if !comp.Degraded {
		t.Fatalf("ComposeDegraded must be degraded")
	}
	if comp.DegradationReason != "thin content; degrade to prompt flow" {
		t.Fatalf("reason = %q", comp.DegradationReason)
	}
	if comp.Input.Name != "Bakarí Braut" {
		t.Fatalf("name = %q", comp.Input.Name)
	}
	if comp.Input.PreferredLanguage != "is" {
		t.Fatalf("locale = %q", comp.Input.PreferredLanguage)
	}
	if comp.Input.OptionalHints["industry"] != "bakery" {
		t.Fatalf("industry hint = %q", comp.Input.OptionalHints["industry"])
	}
	// Brand carried verbatim even on the degraded path.
	if comp.Input.Brand.PrimaryColor != "#123456" {
		t.Fatalf("brand color = %q", comp.Input.Brand.PrimaryColor)
	}
	want := "A website for Bakarí Braut, a Bakery business. Ferskt brauð daglega."
	if comp.PromptPrefill != want {
		t.Fatalf("prefill = %q, want %q", comp.PromptPrefill, want)
	}
	// Degraded input's prompt is the prefill (a thin instance of the contract).
	if comp.Input.Prompt != comp.PromptPrefill {
		t.Fatalf("degraded input prompt should equal prefill")
	}
}

func TestComposeDegradedEmptySalvage(t *testing.T) {
	comp := ComposeDegraded("fetch failed", Salvage{}, BrandResult{}, ComposeContext{})
	if comp.Input.PreferredLanguage != "is" {
		t.Fatalf("locale = %q", comp.Input.PreferredLanguage)
	}
	if comp.PromptPrefill != "A website for my business." {
		t.Fatalf("prefill = %q", comp.PromptPrefill)
	}
	if comp.Input.OptionalHints != nil {
		t.Fatalf("no hints expected for empty salvage, got %v", comp.Input.OptionalHints)
	}
}

func TestResolvePages(t *testing.T) {
	pages := resolvePages(ExtractedFields{
		Services: []ExtractService{{Name: "x"}},
		Contact:  ContactDetails{Email: "a@b.c"},
	})
	if strings.Join(pages, ",") != "home,services,contact" {
		t.Fatalf("pages = %v", pages)
	}
}

func TestResolvePagesIncludesFAQ(t *testing.T) {
	pages := resolvePages(ExtractedFields{
		About: "About us.",
		FAQs:  []ExtractFAQ{{Question: "Do you offer warranties?", Answer: "Yes."}},
	})
	if strings.Join(pages, ",") != "home,about,faq" {
		t.Fatalf("pages = %v, want home,about,faq", pages)
	}
}

func TestComposeBriefRendersWidenedFields(t *testing.T) {
	analysis := sampleAnalysis()
	analysis.Fields.FAQs = []ExtractFAQ{{Question: "Are you available 24/7?", Answer: "Yes, always."}}
	analysis.Fields.Offers = []ExtractOffer{{Title: "Spring Maintenance Special", Description: "10% off in March"}}
	analysis.Fields.ServiceAreas = []string{"Kenosha", "Racine"}
	analysis.Fields.ClientTypes = []string{"homeowners", "restaurants"}

	brief := composeBrief(analysis, ComposeContext{SourceURL: "https://klippt.is"})
	for _, want := range []string{
		"Are you available 24/7?",
		"Yes, always.",
		"Spring Maintenance Special — 10% off in March",
		"Service areas (keep place names verbatim): Kenosha, Racine",
		"Who they serve: homeowners, restaurants",
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q\n---\n%s", want, brief)
		}
	}
}

func TestRenderHoursCalendarOrder(t *testing.T) {
	out := renderHours([]ExtractHours{
		{Day: "sunday", Closed: true},
		{Day: "monday", Opens: "09:00", Closes: "17:00"},
	})
	mondayIdx := strings.Index(out, "Monday")
	sundayIdx := strings.Index(out, "Sunday")
	if mondayIdx < 0 || sundayIdx < 0 || mondayIdx > sundayIdx {
		t.Fatalf("hours not in calendar order: %q", out)
	}
}
