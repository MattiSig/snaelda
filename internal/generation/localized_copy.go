package generation

import "fmt"

// This module holds the deterministic fallback copy the generator emits when
// the AI path is unavailable or fails. Every user-visible string is keyed by
// content locale so an Icelandic site never falls back to English copy
// (Spec 22). The tables are pure data: adding a locale means adding one
// genCatalog entry with no code changes, which is the Spec 22 acceptance test.

// resolveGenLocale constrains an incoming content locale to the supported
// deterministic-copy catalogs, defaulting to English for empty or unsupported
// values. It reuses normalizeLocale so "is-IS", "IS", and "is" all resolve to
// "is".
func resolveGenLocale(value string) string {
	switch normalizeLocale(value) {
	case "is":
		return "is"
	default:
		return "en"
	}
}

// categoryBusiness is the required default profile every catalog must define;
// other categories inherit any field they leave empty from it.
const categoryBusiness = "business"

type genFeature struct {
	Title string
	Body  string
}

type genPricingPlan struct {
	Name        string
	Price       string
	Description string
	Features    []string
	CTALabel    string
}

type genTestimonial struct {
	Quote string
	Name  string
	Role  string
}

type genFAQ struct {
	Question string
	Answer   string
}

type genLink struct {
	Label string
	Href  string
}

type genTeamMember struct {
	Name  string
	Role  string
	Bio   string
	Links []genLink
}

// genProfileCopy holds every category-dependent, user-visible string used by
// the deterministic fallback generator for one (locale, category) pair. Empty
// fields inherit from the locale's business default via mergeProfileCopy.
type genProfileCopy struct {
	CategoryLabel   string
	PrimaryCTA      string
	ServicesTitle   string
	ServicesIntro   string
	FeatureItems    []genFeature
	AboutHeading    string
	AboutBody       string
	GalleryHeading  string
	GalleryBody     string
	ContactHeading  string
	ContactBody     string
	SiteGoal        string
	HomeHeadline    string // may contain a single %s placeholder for the site name
	HomeSubheadline string
	AboutIntro      string
	WorkProcess     string
	FooterTagline   string
}

// genCatalog groups all locale-scoped deterministic copy. Profiles, Pricing,
// Testimonials, FAQ, and Team are keyed by category with a required
// categoryBusiness default; UI holds the category-independent inline strings.
type genCatalog struct {
	Profiles     map[string]genProfileCopy
	Pricing      map[string][]genPricingPlan
	Testimonials map[string][]genTestimonial
	FAQ          map[string][]genFAQ
	Team         map[string][]genTeamMember
	UI           map[string]string
}

var genCatalogs = map[string]genCatalog{
	"en": enGenCatalog,
	"is": isGenCatalog,
}

// genCatalogFor returns the catalog for a resolved locale, falling back to the
// English catalog only for locales that are not registered at all (which
// resolveGenLocale already prevents for supported values).
func genCatalogFor(locale string) genCatalog {
	if catalog, ok := genCatalogs[resolveGenLocale(locale)]; ok {
		return catalog
	}
	return enGenCatalog
}

// profileCopy resolves the copy for a category, layering category-specific
// overrides on top of the locale's business default so every field is
// populated in the target language.
func (c genCatalog) profileCopy(category string) genProfileCopy {
	base := c.Profiles[categoryBusiness]
	override, ok := c.Profiles[category]
	if !ok {
		return base
	}
	return mergeProfileCopy(base, override)
}

func mergeProfileCopy(base, override genProfileCopy) genProfileCopy {
	merged := base
	if override.CategoryLabel != "" {
		merged.CategoryLabel = override.CategoryLabel
	}
	if override.PrimaryCTA != "" {
		merged.PrimaryCTA = override.PrimaryCTA
	}
	if override.ServicesTitle != "" {
		merged.ServicesTitle = override.ServicesTitle
	}
	if override.ServicesIntro != "" {
		merged.ServicesIntro = override.ServicesIntro
	}
	if len(override.FeatureItems) > 0 {
		merged.FeatureItems = override.FeatureItems
	}
	if override.AboutHeading != "" {
		merged.AboutHeading = override.AboutHeading
	}
	if override.AboutBody != "" {
		merged.AboutBody = override.AboutBody
	}
	if override.GalleryHeading != "" {
		merged.GalleryHeading = override.GalleryHeading
	}
	if override.GalleryBody != "" {
		merged.GalleryBody = override.GalleryBody
	}
	if override.ContactHeading != "" {
		merged.ContactHeading = override.ContactHeading
	}
	if override.ContactBody != "" {
		merged.ContactBody = override.ContactBody
	}
	if override.SiteGoal != "" {
		merged.SiteGoal = override.SiteGoal
	}
	if override.HomeHeadline != "" {
		merged.HomeHeadline = override.HomeHeadline
	}
	if override.HomeSubheadline != "" {
		merged.HomeSubheadline = override.HomeSubheadline
	}
	if override.AboutIntro != "" {
		merged.AboutIntro = override.AboutIntro
	}
	if override.WorkProcess != "" {
		merged.WorkProcess = override.WorkProcess
	}
	if override.FooterTagline != "" {
		merged.FooterTagline = override.FooterTagline
	}
	return merged
}

func categoryList[T any](table map[string][]T, category string) []T {
	if items, ok := table[category]; ok {
		return items
	}
	return table[categoryBusiness]
}

// genText returns the inline UI string for a key in the given locale. A missing
// key falls back to the English catalog so a translation gap degrades to
// English rather than an empty string; parity tests guard against that gap.
func genText(locale string, key string) string {
	catalog := genCatalogFor(locale)
	if value, ok := catalog.UI[key]; ok {
		return value
	}
	if value, ok := enGenCatalog.UI[key]; ok {
		return value
	}
	return ""
}

// homeHeadlineFor applies the site name to headlines that carry a %s
// placeholder (the business default) and returns category headlines verbatim.
func homeHeadlineFor(pc genProfileCopy, siteName string) string {
	if !containsPlaceholder(pc.HomeHeadline) {
		return pc.HomeHeadline
	}
	return fmt.Sprintf(pc.HomeHeadline, siteName)
}

func containsPlaceholder(value string) bool {
	for i := 0; i+1 < len(value); i++ {
		if value[i] == '%' && value[i+1] == 's' {
			return true
		}
	}
	return false
}

func featureMaps(items []genFeature) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"title": item.Title, "body": item.Body})
	}
	return out
}

func pricingMaps(plans []genPricingPlan, href string) []map[string]any {
	out := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		features := make([]map[string]any, 0, len(plan.Features))
		for _, feature := range plan.Features {
			features = append(features, map[string]any{"text": feature})
		}
		out = append(out, map[string]any{
			"name":        plan.Name,
			"price":       plan.Price,
			"description": plan.Description,
			"features":    toAnySlice(features),
			"cta":         map[string]any{"label": plan.CTALabel, "href": href},
		})
	}
	return out
}

func testimonialMaps(items []genTestimonial) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"quote": item.Quote, "name": item.Name, "role": item.Role})
	}
	return out
}

func faqMaps(items []genFAQ) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"question": item.Question, "answer": item.Answer})
	}
	return out
}

func teamMaps(members []genTeamMember) []map[string]any {
	out := make([]map[string]any, 0, len(members))
	for _, member := range members {
		links := make([]map[string]any, 0, len(member.Links))
		for _, link := range member.Links {
			links = append(links, map[string]any{"label": link.Label, "href": link.Href})
		}
		out = append(out, map[string]any{
			"name":  member.Name,
			"role":  member.Role,
			"bio":   member.Bio,
			"links": toAnySlice(links),
		})
	}
	return out
}
