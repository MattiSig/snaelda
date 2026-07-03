package sites

import "fmt"

// starterCopy holds the locale-scoped copy for the deterministic starter draft
// created when a site is scaffolded without AI generation. Keeping it keyed by
// locale means an Icelandic site never opens with English placeholder copy
// (Spec 22); adding a locale is a data change, not a code change.
type starterCopy struct {
	DefaultSubheadline string
	HeroHeadline       string // %s is the site name
	HeroCTALabel       string
	TextHeading        string
	TextBody           string
	FeaturesHeading    string
	FeaturesIntro      string
	Feature1Title      string
	Feature1Body       string
	Feature2Title      string
	Feature2Body       string
	Feature3Title      string
	Feature3Body       string
	CTAHeading         string
	CTABody            string
	CTALabel           string
	NavHome            string
}

func (c starterCopy) heroHeadline(name string) string {
	return fmt.Sprintf(c.HeroHeadline, name)
}

var starterCopyCatalog = map[string]starterCopy{
	"en": {
		DefaultSubheadline: "Start from a structured draft with real pages, editable sections, and a preview route that stays on the same site contract.",
		HeroHeadline:       "A welcoming first draft for %s",
		HeroCTALabel:       "Review the draft",
		TextHeading:        "What this starter gives you",
		TextBody:           "A single-page site scaffold with validated blocks, a saved draft, and room to tune the copy before generation and publishing are wired in.",
		FeaturesHeading:    "Ready for the next loop",
		FeaturesIntro:      "The prototype keeps each section inside the maintained registry so preview and publish can stay consistent later.",
		Feature1Title:      "Structured draft",
		Feature1Body:       "Every page and block is validated application data.",
		Feature2Title:      "Builder-friendly",
		Feature2Body:       "Site metadata can be edited without breaking the stored draft.",
		Feature3Title:      "Preview-ready",
		Feature3Body:       "The React preview reads the same draft shape the API serves.",
		CTAHeading:         "Next step",
		CTABody:            "Refine the name or slug now, then move into richer page and block editing.",
		CTALabel:           "Stay in the builder",
		NavHome:            "Home",
	},
	"is": {
		DefaultSubheadline: "Byrjaðu á skipulögðum drögum með raunverulegum síðum, breytanlegum köflum og forskoðun sem heldur sér á sama efnissamningi.",
		HeroHeadline:       "Hlýleg fyrstu drög fyrir %s",
		HeroCTALabel:       "Skoðaðu drögin",
		TextHeading:        "Hvað þessi drög gefa þér",
		TextBody:           "Einnar síðu grunnur með gildum einingum, vistuðum drögum og svigrúmi til að fínstilla textann áður en gerð og útgáfa eru tengd.",
		FeaturesHeading:    "Tilbúið fyrir næstu umferð",
		FeaturesIntro:      "Frumgerðin heldur hverjum kafla innan viðhaldnu einingaskrárinnar svo forskoðun og útgáfa haldist samræmd síðar.",
		Feature1Title:      "Skipulögð drög",
		Feature1Body:       "Hver síða og eining er staðfest gögn í kerfinu.",
		Feature2Title:      "Þægilegt í smíði",
		Feature2Body:       "Hægt er að breyta upplýsingum síðunnar án þess að skemma vistuð drög.",
		Feature3Title:      "Tilbúið til forskoðunar",
		Feature3Body:       "React-forskoðunin les sömu drög og API-ið skilar.",
		CTAHeading:         "Næsta skref",
		CTABody:            "Fínstilltu nafnið eða slóðina núna og farðu svo í ítarlegri breytingar á síðum og einingum.",
		CTALabel:           "Vertu áfram í smiðnum",
		NavHome:            "Heim",
	},
}

// starterCopyFor returns the starter copy for a normalized site locale,
// defaulting to English for unsupported values.
func starterCopyFor(locale string) starterCopy {
	if c, ok := starterCopyCatalog[resolveSiteLocale(locale)]; ok {
		return c
	}
	return starterCopyCatalog["en"]
}
