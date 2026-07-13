package respin

import (
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/slugs"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// Collection-worthiness thresholds. A re-spin only promotes a source's
// list-shaped content into a real collection (index + detail pages) when the list
// is big enough that per-item detail pages earn their keep; below the threshold
// the facts stay in a static block (features_grid / team_profile_cards), which
// reads better than a near-empty collection. Thresholds are soft calls to tune in
// the 50-site QA loop (Spec 21).
const (
	// servicesCollectionMinEntries promotes services outright at this count.
	servicesCollectionMinEntries = 4
	// servicesSubstanceMinEntries promotes a smaller service list when every
	// entry carries real per-item substance (a description AND a price).
	servicesSubstanceMinEntries = 2
	// peopleCollectionMinEntries promotes people only for large rosters — a
	// theatre ensemble or lab group where per-person detail pages are warranted.
	// Small founder/team rows stay in the static team_profile_cards block.
	peopleCollectionMinEntries = 6
)

// seedCollectionsResult is the outcome of the collection-worthiness decision: the
// synthesized collections (each already validated) plus flags telling the
// composer which kinds were promoted, so the brief can be slimmed to a one-line
// pointer instead of re-rendering the same facts into a static block.
type seedCollectionsResult struct {
	Collections      []siteconfig.Collection
	PromotedServices bool
	PromotedPeople   bool
}

// synthesizeSeedCollections turns the extraction's structured lists into
// deterministic seed collections (Spec 21). It never calls the model — the
// extraction already produced verbatim, target-language facts (services with
// prices, people with rewritten roles/bios), so turning them into a collection is
// a pure transform. A collection that fails to synthesize into a valid shape is
// silently dropped (that kind stays static), so this can never break generation.
func synthesizeSeedCollections(fields ExtractedFields, locale string) seedCollectionsResult {
	locale = collectionLabelLocale(locale)
	var out seedCollectionsResult

	if shouldPromoteServices(fields.Services) {
		if col, ok := buildServicesCollection(fields.Services, locale, len(out.Collections)); ok {
			out.Collections = append(out.Collections, col)
			out.PromotedServices = true
		}
	}
	if shouldPromotePeople(fields.People) {
		if col, ok := buildPeopleCollection(fields.People, locale, len(out.Collections)); ok {
			out.Collections = append(out.Collections, col)
			out.PromotedPeople = true
		}
	}
	return out
}

// shouldPromoteServices decides whether services become a collection: a large
// list (>=4), or a smaller list where every entry carries a description AND a
// price (real per-item substance worth a detail page).
func shouldPromoteServices(services []ExtractService) bool {
	if len(services) >= servicesCollectionMinEntries {
		return true
	}
	if len(services) < servicesSubstanceMinEntries {
		return false
	}
	for _, s := range services {
		if strings.TrimSpace(s.Description) == "" || strings.TrimSpace(s.Price) == "" {
			return false
		}
	}
	return true
}

// shouldPromotePeople decides whether people become a collection: only for large
// rosters. Small teams stay in the static team_profile_cards block.
func shouldPromotePeople(people []ExtractPerson) bool {
	return len(people) >= peopleCollectionMinEntries
}

func buildServicesCollection(services []ExtractService, locale string, sortOrder int) (siteconfig.Collection, bool) {
	labels := collectionLabels[locale]["services"]
	fieldLabels := collectionFieldLabels[locale]
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: fieldLabels["title"], Type: siteconfig.FieldTypeText, Required: true},
		{Key: "description", Label: fieldLabels["description"], Type: siteconfig.FieldTypeLongText},
		{Key: "price", Label: fieldLabels["price"], Type: siteconfig.FieldTypeText},
		{Key: "image", Label: fieldLabels["image"], Type: siteconfig.FieldTypeAsset},
	}

	entries := make([]siteconfig.CollectionEntry, 0, len(services))
	seenSlug := map[string]bool{}
	for _, svc := range services {
		name := strings.TrimSpace(svc.Name)
		if name == "" {
			continue
		}
		entryFields := map[string]any{"title": name}
		if desc := strings.TrimSpace(svc.Description); desc != "" {
			entryFields["description"] = desc
		}
		if price := strings.TrimSpace(svc.Price); price != "" {
			entryFields["price"] = price
		}
		entry, ok := buildEntry(name, entryFields, len(entries), seenSlug)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}

	return finishCollection(labels, "services", schema, entries, sortOrder)
}

func buildPeopleCollection(people []ExtractPerson, locale string, sortOrder int) (siteconfig.Collection, bool) {
	labels := collectionLabels[locale]["people"]
	fieldLabels := collectionFieldLabels[locale]
	schema := []siteconfig.FieldDefinition{
		{Key: "name", Label: fieldLabels["name"], Type: siteconfig.FieldTypeText, Required: true},
		{Key: "role", Label: fieldLabels["role"], Type: siteconfig.FieldTypeText},
		{Key: "bio", Label: fieldLabels["bio"], Type: siteconfig.FieldTypeLongText},
		{Key: "photo", Label: fieldLabels["photo"], Type: siteconfig.FieldTypeAsset},
	}

	entries := make([]siteconfig.CollectionEntry, 0, len(people))
	seenSlug := map[string]bool{}
	for _, p := range people {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		entryFields := map[string]any{"name": name}
		if role := strings.TrimSpace(p.Role); role != "" {
			entryFields["role"] = role
		}
		if bio := strings.TrimSpace(p.Bio); bio != "" {
			entryFields["bio"] = bio
		}
		entry, ok := buildEntry(name, entryFields, len(entries), seenSlug)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}

	return finishCollection(labels, "team", schema, entries, sortOrder)
}

// buildEntry assembles one published entry with a unique slug derived from its
// title. Entries are seeded published so the first publish does not trip the
// no_published_entries rule (Spec 21). Returns false only if an id cannot be
// minted.
func buildEntry(title string, fields map[string]any, index int, seenSlug map[string]bool) (siteconfig.CollectionEntry, bool) {
	id, err := ids.New()
	if err != nil {
		return siteconfig.CollectionEntry{}, false
	}
	slug := uniqueSlug(slugs.Generate(title), seenSlug)
	return siteconfig.CollectionEntry{
		ID:        id,
		Slug:      slug,
		Status:    siteconfig.EntryStatusPublished,
		SortOrder: index,
		Fields:    fields,
		SEO:       siteconfig.SEOConfig{Title: title},
	}, true
}

// finishCollection mints the collection id, derives the slug from the plural
// label (Icelandic transliteration comes free via slugs.Generate), attaches the
// entries, and validates the whole shape. A collection with no usable entries or
// one that fails validation is dropped (ok=false) so generation is never fed an
// invalid draft.
func finishCollection(labels [2]string, slugFallback string, schema []siteconfig.FieldDefinition, entries []siteconfig.CollectionEntry, sortOrder int) (siteconfig.Collection, bool) {
	if len(entries) == 0 {
		return siteconfig.Collection{}, false
	}
	id, err := ids.New()
	if err != nil {
		return siteconfig.Collection{}, false
	}
	collection := siteconfig.Collection{
		ID:            id,
		Slug:          slugs.GenerateWithFallback(labels[1], slugFallback),
		SingularLabel: labels[0],
		PluralLabel:   labels[1],
		Schema:        schema,
		SchemaVersion: 1,
		Settings: siteconfig.CollectionSettings{
			DefaultSort: siteconfig.CollectionSortManual,
			// Detail pages get real public URLs (/{slug}/{entry}) so each entry is
			// a browsable page, which is the whole point of promoting to a
			// collection over a static block.
			ExposeDetailURLs: true,
		},
		SortOrder: sortOrder,
		Entries:   entries,
	}
	if err := siteconfig.ValidateCollection(collection); err != nil {
		return siteconfig.Collection{}, false
	}
	return collection, true
}

// uniqueSlug returns base, suffixing "-2", "-3", … until it is unused, and
// records the chosen slug in seen.
func uniqueSlug(base string, seen map[string]bool) string {
	candidate := base
	for suffix := 2; seen[candidate]; suffix++ {
		candidate = base + "-" + itoaSmall(suffix)
	}
	seen[candidate] = true
	return candidate
}

func itoaSmall(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoaSmall(n/10) + string(rune('0'+n%10))
}

// collectionLabelLocale reduces a locale to the label table key, defaulting to
// Icelandic for the Iceland-first phase (unknown/empty → "is").
func collectionLabelLocale(locale string) string {
	locale = normalizeStageLocale(locale)
	if _, ok := collectionLabels[locale]; ok {
		return locale
	}
	return "is"
}

// collectionLabels holds {singular, plural} label pairs per locale per kind. The
// slug is derived from the plural label, so Icelandic collections land at
// transliterated URLs (/thjonusta, /starfsfolk) for free.
var collectionLabels = map[string]map[string][2]string{
	"is": {
		"services": {"Þjónusta", "Þjónusta"},
		"people":   {"Starfsmaður", "Starfsfólk"},
	},
	"en": {
		"services": {"Service", "Services"},
		"people":   {"Team member", "Team"},
	},
}

// collectionFieldLabels holds the editor-visible field labels per locale.
var collectionFieldLabels = map[string]map[string]string{
	"is": {
		"title":       "Titill",
		"description": "Lýsing",
		"price":       "Verð",
		"image":       "Mynd",
		"name":        "Nafn",
		"role":        "Hlutverk",
		"bio":         "Um",
		"photo":       "Mynd",
	},
	"en": {
		"title":       "Title",
		"description": "Description",
		"price":       "Price",
		"image":       "Image",
		"name":        "Name",
		"role":        "Role",
		"bio":         "Bio",
		"photo":       "Photo",
	},
}
