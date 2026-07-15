package generation

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/slugs"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// Caps for the collections intake step. Item text beyond the cap is truncated
// by the handler; entries beyond the cap are dropped by the finisher. Both are
// generous for "type your services in a box" while keeping the draft call small.
const (
	maxSeedCollectionItemsCharacters = 4000
	maxSeedCollectionEntries         = 20
)

// draftSeedCollections turns the user's step-two intake into validated seed
// collections by running one structured draft call per confirmed collection.
// It mirrors re-spin's drop-on-failure rule: a collection whose draft call
// fails or whose finished shape does not validate is dropped (generation
// proceeds without it), never an error.
func (s *Service) draftSeedCollections(ctx context.Context, input GenerateInput) []siteconfig.Collection {
	requests := input.SeedCollectionInputs
	if len(requests) == 0 || s.seedCollectionPlanner == nil {
		return nil
	}
	if len(requests) > MaxSeedCollectionSuggestions {
		requests = requests[:MaxSeedCollectionSuggestions]
	}
	collections := make([]siteconfig.Collection, 0, len(requests))
	usedSlugs := map[string]bool{}
	for _, request := range requests {
		draft, err := s.seedCollectionPlanner.DraftSeedCollection(ctx, SeedCollectionDraftRequest{
			SitePrompt:        input.Prompt,
			SiteName:          strings.TrimSpace(input.Name),
			PreferredLanguage: strings.TrimSpace(input.PreferredLanguage),
			SingularLabel:     request.SingularLabel,
			PluralLabel:       request.PluralLabel,
			ItemsText:         request.ItemsText,
		})
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("seed collection draft failed; dropping collection",
					"pluralLabel", request.PluralLabel,
					"error", err.Error(),
				)
			}
			continue
		}
		collection, ok := finishSeedCollection(request, draft, len(collections), usedSlugs)
		if !ok {
			if s.logger != nil {
				s.logger.Warn("seed collection failed validation; dropping collection",
					"pluralLabel", request.PluralLabel,
				)
			}
			continue
		}
		collections = append(collections, collection)
	}
	return collections
}

// finishSeedCollection deterministically completes a model draft: sanitized
// schema, minted ids, unique slugs, published entries (so the first publish
// does not trip the no_published_entries rule), detail URLs exposed, and a
// full validation pass. Returns ok=false when the result is not a collection
// worth attaching.
func finishSeedCollection(request SeedCollectionInput, draft SeedCollectionDraftResponse, sortOrder int, usedSlugs map[string]bool) (siteconfig.Collection, bool) {
	schema := sanitizeSeedSchema(draft.Schema)
	if len(schema) == 0 {
		return siteconfig.Collection{}, false
	}
	titleKey := schema[0].Key

	entries := make([]siteconfig.CollectionEntry, 0, len(draft.Entries))
	seenEntrySlug := map[string]bool{}
	for _, item := range draft.Entries {
		if len(entries) >= maxSeedCollectionEntries {
			break
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		fields := sanitizeSeedEntryFields(item.Fields, schema)
		if _, ok := fields[titleKey]; !ok {
			fields[titleKey] = title
		}
		id, err := ids.New()
		if err != nil {
			return siteconfig.Collection{}, false
		}
		entries = append(entries, siteconfig.CollectionEntry{
			ID:        id,
			Slug:      uniqueSeedSlug(slugs.GenerateWithFallback(title, "entry"), seenEntrySlug),
			Status:    siteconfig.EntryStatusPublished,
			SortOrder: len(entries),
			Fields:    fields,
			SEO:       siteconfig.SEOConfig{Title: title},
		})
	}
	if len(entries) == 0 {
		return siteconfig.Collection{}, false
	}

	id, err := ids.New()
	if err != nil {
		return siteconfig.Collection{}, false
	}
	collection := siteconfig.Collection{
		ID:            id,
		Slug:          uniqueSeedSlug(slugs.GenerateWithFallback(request.PluralLabel, "collection"), usedSlugs),
		SingularLabel: strings.TrimSpace(request.SingularLabel),
		PluralLabel:   strings.TrimSpace(request.PluralLabel),
		Schema:        schema,
		SchemaVersion: 1,
		Settings: siteconfig.CollectionSettings{
			DefaultSort: siteconfig.CollectionSortManual,
			// Detail pages get real public URLs (/{slug}/{entry}) — the point of
			// promoting the user's list to a collection over a static block.
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

// sanitizeSeedSchema keeps only well-formed fields of the types the drafter is
// allowed to use, dedupes keys, and forces the first field into the required
// text slot the entry titles rely on.
func sanitizeSeedSchema(fields []siteconfig.FieldDefinition) []siteconfig.FieldDefinition {
	out := make([]siteconfig.FieldDefinition, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		label := strings.TrimSpace(field.Label)
		if key == "" || label == "" || seen[key] {
			continue
		}
		switch field.Type {
		case siteconfig.FieldTypeText, siteconfig.FieldTypeLongText, siteconfig.FieldTypeAsset:
		default:
			continue
		}
		seen[key] = true
		out = append(out, siteconfig.FieldDefinition{
			Key:      key,
			Label:    label,
			Type:     field.Type,
			Required: field.Required,
		})
	}
	if len(out) == 0 {
		return nil
	}
	if out[0].Type != siteconfig.FieldTypeText {
		return nil
	}
	out[0].Required = true
	return out
}

// sanitizeSeedEntryFields keeps only schema-keyed, non-empty string values.
// Asset fields are always dropped — seed entries cannot reference assets that
// do not exist yet.
func sanitizeSeedEntryFields(fields map[string]any, schema []siteconfig.FieldDefinition) map[string]any {
	allowed := make(map[string]string, len(schema))
	for _, field := range schema {
		allowed[field.Key] = field.Type
	}
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		fieldType, ok := allowed[key]
		if !ok || fieldType == siteconfig.FieldTypeAsset {
			continue
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		out[key] = strings.TrimSpace(text)
	}
	return out
}

// seedCollectionsPromptDirective augments the planner-facing brief when the
// draft already carries seeded collections. Without it the outline happily
// plans a static services/pricing page that re-lists the same items with
// invented placeholder facts — the fresh-spin sibling of re-spin's slimmed
// brief (compose.go's PromotedServices pointer).
func seedCollectionsPromptDirective(collections []siteconfig.Collection) string {
	if len(collections) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nExisting collections: this site already has the following collections, each built with its own index page (already in the navigation) and per-entry detail pages:")
	for _, collection := range collections {
		titles := seedEntryTitles(collection, 6)
		fmt.Fprintf(&b, "\n- %q with %d entries (%s).", collection.PluralLabel, len(collection.Entries), strings.Join(titles, ", "))
	}
	b.WriteString("\nDo not plan pages for these collections, do not re-list their entries as static cards, pricing tables, or repeater items, and never invent placeholder facts (prices, durations) for them. Where the items are relevant, refer to the collection by its name — never write raw URL paths (like /services) in visible copy.")
	return b.String()
}

// seedEntryTitles lists up to max entry titles (the value of the schema's
// first field), appending an ellipsis marker when entries are elided.
func seedEntryTitles(collection siteconfig.Collection, max int) []string {
	if len(collection.Schema) == 0 {
		return nil
	}
	titleKey := collection.Schema[0].Key
	titles := make([]string, 0, max+1)
	for _, entry := range collection.Entries {
		if len(titles) == max {
			titles = append(titles, "…")
			break
		}
		if title, _ := entry.Fields[titleKey].(string); title != "" {
			titles = append(titles, title)
		}
	}
	return titles
}

// uniqueSeedSlug returns base, suffixing "-2", "-3", … until it is unused,
// and records the chosen slug in seen.
func uniqueSeedSlug(base string, seen map[string]bool) string {
	candidate := base
	for suffix := 2; seen[candidate]; suffix++ {
		candidate = base + "-" + strconv.Itoa(suffix)
	}
	seen[candidate] = true
	return candidate
}
