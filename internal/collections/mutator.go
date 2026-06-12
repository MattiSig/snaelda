package collections

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/slugs"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
)

// Reader is the subset of the sites reader the collections module relies on
// to load the current site draft. Implementations must return a fully
// hydrated draft including collections + entries.
type Reader interface {
	LoadDraft(ctx context.Context, siteID string) (siteconfig.SiteDraft, error)
}

// Writer is the subset of the sites writer the collections module relies on
// to persist mutations. Implementations must validate the draft before
// committing.
type Writer interface {
	SaveDraft(ctx context.Context, workspaceID string, draft siteconfig.SiteDraft) error
}

// Mutator implements collection and entry CRUD on top of the SiteDraft
// load → modify → save pattern shared with the sites module.
type Mutator struct {
	reader Reader
	writer Writer
}

// NewMutator builds a Mutator wired against the provided draft reader/writer.
func NewMutator(reader Reader, writer Writer) *Mutator {
	return &Mutator{reader: reader, writer: writer}
}

// CreateCollectionInput describes a new collection. Schema may be empty;
// callers are expected to follow up with PATCH calls to populate it.
type CreateCollectionInput struct {
	Slug          string
	SingularLabel string
	PluralLabel   string
	Schema        []siteconfig.FieldDefinition
	Settings      siteconfig.CollectionSettings
}

// UpdateCollectionInput is a partial patch over a collection.
type UpdateCollectionInput struct {
	Slug          *string
	SingularLabel *string
	PluralLabel   *string
	Schema        []siteconfig.FieldDefinition
	Settings      *siteconfig.CollectionSettings
}

// CreateEntryInput describes a new entry on a collection.
type CreateEntryInput struct {
	Slug   string
	Fields map[string]any
	SEO    siteconfig.SEOConfig
	Status string
}

// UpdateEntryInput is a partial patch over an entry. Field values are merged
// with the entry's existing field map; pass an explicit nil to clear a
// field via PATCH.
type UpdateEntryInput struct {
	Slug   *string
	Fields map[string]any
	SEO    *siteconfig.SEOConfig
	Status *string
}

// ListCollections returns the current collections (in sort order) on the
// site's draft.
func (m *Mutator) ListCollections(ctx context.Context, siteID string) ([]siteconfig.Collection, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return nil, err
	}
	return draft.Collections, nil
}

// GetCollection returns a single collection by id.
func (m *Mutator) GetCollection(ctx context.Context, siteID string, collectionID string) (siteconfig.Collection, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.Collection{}, err
	}
	index := findCollection(draft.Collections, collectionID)
	if index == -1 {
		return siteconfig.Collection{}, ErrCollectionNotFound
	}
	return draft.Collections[index], nil
}

// CreateCollection appends a new collection to the site's draft and returns
// the persisted record.
func (m *Mutator) CreateCollection(ctx context.Context, workspaceID string, siteID string, input CreateCollectionInput) (siteconfig.Collection, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.Collection{}, err
	}

	singular := strings.TrimSpace(input.SingularLabel)
	plural := strings.TrimSpace(input.PluralLabel)
	if singular == "" || plural == "" {
		return siteconfig.Collection{}, ErrCollectionLabelRequired
	}

	slugValue, err := chooseCollectionSlug(input.Slug, plural, draft.Collections, "")
	if err != nil {
		return siteconfig.Collection{}, err
	}

	collectionID, err := ids.New()
	if err != nil {
		return siteconfig.Collection{}, fmt.Errorf("generate collection id: %w", err)
	}

	schema := normalizeSchema(input.Schema)

	collection := siteconfig.Collection{
		ID:            collectionID,
		Slug:          slugValue,
		SingularLabel: singular,
		PluralLabel:   plural,
		Schema:        schema,
		Settings:      input.Settings,
		SortOrder:     len(draft.Collections),
		Entries:       []siteconfig.CollectionEntry{},
	}

	draft.Collections = append(draft.Collections, collection)
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.Collection{}, err
	}
	return m.findInLoadedDraft(ctx, siteID, collectionID)
}

// UpdateCollection applies the patch and re-saves.
func (m *Mutator) UpdateCollection(ctx context.Context, workspaceID string, siteID string, collectionID string, input UpdateCollectionInput) (siteconfig.Collection, error) {
	if input.Slug == nil && input.SingularLabel == nil && input.PluralLabel == nil && input.Schema == nil && input.Settings == nil {
		return siteconfig.Collection{}, ErrNoCollectionChanges
	}

	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.Collection{}, err
	}
	index := findCollection(draft.Collections, collectionID)
	if index == -1 {
		return siteconfig.Collection{}, ErrCollectionNotFound
	}

	collection := draft.Collections[index]

	if input.Slug != nil {
		slugValue, err := chooseCollectionSlug(*input.Slug, collection.PluralLabel, draft.Collections, collectionID)
		if err != nil {
			return siteconfig.Collection{}, err
		}
		collection.Slug = slugValue
	}
	if input.SingularLabel != nil {
		singular := strings.TrimSpace(*input.SingularLabel)
		if singular == "" {
			return siteconfig.Collection{}, ErrCollectionLabelRequired
		}
		collection.SingularLabel = singular
	}
	if input.PluralLabel != nil {
		plural := strings.TrimSpace(*input.PluralLabel)
		if plural == "" {
			return siteconfig.Collection{}, ErrCollectionLabelRequired
		}
		collection.PluralLabel = plural
	}
	if input.Schema != nil {
		collection.Schema = normalizeSchema(input.Schema)
	}
	if input.Settings != nil {
		collection.Settings = *input.Settings
	}

	draft.Collections[index] = collection
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.Collection{}, err
	}
	return m.findInLoadedDraft(ctx, siteID, collectionID)
}

// DeleteCollection removes a collection if no pages bind to it.
func (m *Mutator) DeleteCollection(ctx context.Context, workspaceID string, siteID string, collectionID string) error {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return err
	}
	index := findCollection(draft.Collections, collectionID)
	if index == -1 {
		return ErrCollectionNotFound
	}
	for _, page := range draft.Pages {
		if page.CollectionID == collectionID {
			return ErrCollectionInUse
		}
	}

	draft.Collections = append(draft.Collections[:index], draft.Collections[index+1:]...)
	for i := range draft.Collections {
		draft.Collections[i].SortOrder = i
	}
	return m.writer.SaveDraft(ctx, workspaceID, draft)
}

// ListEntries returns the entries on a collection, in sort order.
func (m *Mutator) ListEntries(ctx context.Context, siteID string, collectionID string) ([]siteconfig.CollectionEntry, error) {
	collection, err := m.GetCollection(ctx, siteID, collectionID)
	if err != nil {
		return nil, err
	}
	return collection.Entries, nil
}

// GetEntry returns a single entry.
func (m *Mutator) GetEntry(ctx context.Context, siteID string, collectionID string, entryID string) (siteconfig.CollectionEntry, error) {
	collection, err := m.GetCollection(ctx, siteID, collectionID)
	if err != nil {
		return siteconfig.CollectionEntry{}, err
	}
	index := findEntry(collection.Entries, entryID)
	if index == -1 {
		return siteconfig.CollectionEntry{}, ErrEntryNotFound
	}
	return collection.Entries[index], nil
}

// CreateEntry appends a new entry to a collection.
func (m *Mutator) CreateEntry(ctx context.Context, workspaceID string, siteID string, collectionID string, input CreateEntryInput) (siteconfig.CollectionEntry, error) {
	entries, err := m.CreateEntries(ctx, workspaceID, siteID, collectionID, []CreateEntryInput{input})
	if err != nil {
		return siteconfig.CollectionEntry{}, err
	}
	return entries[0], nil
}

// CreateEntries appends multiple new entries to a collection and persists the
// batch in one draft write so AI-generated entry drafts either all land or
// none do.
func (m *Mutator) CreateEntries(ctx context.Context, workspaceID string, siteID string, collectionID string, inputs []CreateEntryInput) ([]siteconfig.CollectionEntry, error) {
	if len(inputs) == 0 {
		return []siteconfig.CollectionEntry{}, nil
	}
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return nil, err
	}
	collectionIndex := findCollection(draft.Collections, collectionID)
	if collectionIndex == -1 {
		return nil, ErrCollectionNotFound
	}
	collection := draft.Collections[collectionIndex]

	created := make([]siteconfig.CollectionEntry, 0, len(inputs))
	entries := append([]siteconfig.CollectionEntry(nil), collection.Entries...)
	for _, input := range inputs {
		titleHint := entryTitle(input.Fields, collection.Schema)
		slugValue, err := chooseEntrySlug(input.Slug, titleHint, entries, "")
		if err != nil {
			return nil, err
		}

		entryID, err := ids.New()
		if err != nil {
			return nil, fmt.Errorf("generate entry id: %w", err)
		}
		status := strings.TrimSpace(input.Status)
		if status == "" {
			status = siteconfig.EntryStatusDraft
		}
		fields := input.Fields
		if fields == nil {
			fields = map[string]any{}
		}

		entry := siteconfig.CollectionEntry{
			ID:        entryID,
			Slug:      slugValue,
			Fields:    fields,
			SEO:       input.SEO,
			Status:    status,
			SortOrder: len(entries),
		}
		entries = append(entries, entry)
		created = append(created, entry)
	}
	collection.Entries = entries
	draft.Collections[collectionIndex] = collection

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateEntry applies the patch and re-saves.
func (m *Mutator) UpdateEntry(ctx context.Context, workspaceID string, siteID string, collectionID string, entryID string, input UpdateEntryInput) (siteconfig.CollectionEntry, error) {
	if input.Slug == nil && input.Fields == nil && input.SEO == nil && input.Status == nil {
		return siteconfig.CollectionEntry{}, ErrNoEntryChanges
	}

	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.CollectionEntry{}, err
	}
	collectionIndex := findCollection(draft.Collections, collectionID)
	if collectionIndex == -1 {
		return siteconfig.CollectionEntry{}, ErrCollectionNotFound
	}
	collection := draft.Collections[collectionIndex]

	entryIndex := findEntry(collection.Entries, entryID)
	if entryIndex == -1 {
		return siteconfig.CollectionEntry{}, ErrEntryNotFound
	}
	entry := collection.Entries[entryIndex]

	if input.Slug != nil {
		slugValue, err := chooseEntrySlug(*input.Slug, entry.Slug, collection.Entries, entryID)
		if err != nil {
			return siteconfig.CollectionEntry{}, err
		}
		entry.Slug = slugValue
	}
	if input.Fields != nil {
		if entry.Fields == nil {
			entry.Fields = map[string]any{}
		}
		for key, value := range input.Fields {
			if value == nil {
				delete(entry.Fields, key)
				continue
			}
			entry.Fields[key] = value
		}
	}
	if input.SEO != nil {
		entry.SEO = *input.SEO
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if status == "" {
			status = siteconfig.EntryStatusDraft
		}
		entry.Status = status
	}

	collection.Entries[entryIndex] = entry
	draft.Collections[collectionIndex] = collection

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.CollectionEntry{}, err
	}
	return m.GetEntry(ctx, siteID, collectionID, entryID)
}

// DeleteEntry removes an entry from a collection.
func (m *Mutator) DeleteEntry(ctx context.Context, workspaceID string, siteID string, collectionID string, entryID string) error {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return err
	}
	collectionIndex := findCollection(draft.Collections, collectionID)
	if collectionIndex == -1 {
		return ErrCollectionNotFound
	}
	collection := draft.Collections[collectionIndex]

	entryIndex := findEntry(collection.Entries, entryID)
	if entryIndex == -1 {
		return ErrEntryNotFound
	}
	collection.Entries = append(collection.Entries[:entryIndex], collection.Entries[entryIndex+1:]...)
	for i := range collection.Entries {
		collection.Entries[i].SortOrder = i
	}
	draft.Collections[collectionIndex] = collection

	return m.writer.SaveDraft(ctx, workspaceID, draft)
}

// ReorderEntries reorders the entries on a collection to match the provided
// list of entry IDs.
func (m *Mutator) ReorderEntries(ctx context.Context, workspaceID string, siteID string, collectionID string, entryIDs []string) ([]siteconfig.CollectionEntry, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return nil, err
	}
	collectionIndex := findCollection(draft.Collections, collectionID)
	if collectionIndex == -1 {
		return nil, ErrCollectionNotFound
	}
	collection := draft.Collections[collectionIndex]

	if len(entryIDs) != len(collection.Entries) {
		return nil, ErrEntryOrderInvalid
	}
	byID := make(map[string]siteconfig.CollectionEntry, len(collection.Entries))
	for _, entry := range collection.Entries {
		byID[entry.ID] = entry
	}
	reordered := make([]siteconfig.CollectionEntry, 0, len(entryIDs))
	seen := map[string]bool{}
	for index, id := range entryIDs {
		entry, ok := byID[id]
		if !ok || seen[id] {
			return nil, ErrEntryOrderInvalid
		}
		seen[id] = true
		entry.SortOrder = index
		reordered = append(reordered, entry)
	}
	collection.Entries = reordered
	draft.Collections[collectionIndex] = collection

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return nil, err
	}
	updated, err := m.GetCollection(ctx, siteID, collectionID)
	if err != nil {
		return nil, err
	}
	return updated.Entries, nil
}

// MapWriteError converts site/draft persistence errors into the matching
// collection-domain error so handlers can map them to HTTP statuses without
// duplicating the sites error matrix.
func MapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sites.ErrNotFound) {
		return ErrCollectionNotFound
	}
	return err
}

func (m *Mutator) findInLoadedDraft(ctx context.Context, siteID string, collectionID string) (siteconfig.Collection, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.Collection{}, err
	}
	index := findCollection(draft.Collections, collectionID)
	if index == -1 {
		return siteconfig.Collection{}, ErrCollectionNotFound
	}
	return draft.Collections[index], nil
}

func findCollection(collections []siteconfig.Collection, id string) int {
	for index, collection := range collections {
		if collection.ID == id {
			return index
		}
	}
	return -1
}

func findEntry(entries []siteconfig.CollectionEntry, id string) int {
	for index, entry := range entries {
		if entry.ID == id {
			return index
		}
	}
	return -1
}

func chooseCollectionSlug(requested string, fallback string, collections []siteconfig.Collection, ignoreID string) (string, error) {
	requested = strings.TrimSpace(requested)
	base := requested
	if base == "" {
		base = fallback
	}
	exists := func(candidate string) (bool, error) {
		for _, collection := range collections {
			if collection.ID == ignoreID {
				continue
			}
			if collection.Slug == candidate {
				return true, nil
			}
		}
		return false, nil
	}
	if requested != "" && !slugs.IsValid(requested) {
		return "", ErrCollectionSlugInvalid
	}
	if requested != "" {
		taken, _ := exists(requested)
		if taken {
			return "", ErrCollectionSlugConflict
		}
		return requested, nil
	}
	candidate, err := slugs.EnsureUnique(base, exists)
	if err != nil {
		return "", err
	}
	return candidate, nil
}

func chooseEntrySlug(requested string, fallback string, entries []siteconfig.CollectionEntry, ignoreID string) (string, error) {
	requested = strings.TrimSpace(requested)
	base := requested
	if base == "" {
		base = fallback
	}
	exists := func(candidate string) (bool, error) {
		for _, entry := range entries {
			if entry.ID == ignoreID {
				continue
			}
			if entry.Slug == candidate {
				return true, nil
			}
		}
		return false, nil
	}
	if requested != "" && !slugs.IsValid(requested) {
		return "", ErrEntrySlugConflict
	}
	if requested != "" {
		taken, _ := exists(requested)
		if taken {
			return "", ErrEntrySlugConflict
		}
		return requested, nil
	}
	candidate, err := slugs.EnsureUnique(base, exists)
	if err != nil {
		return "", err
	}
	return candidate, nil
}

func entryTitle(fields map[string]any, schema []siteconfig.FieldDefinition) string {
	if fields == nil {
		return ""
	}
	for _, candidate := range []string{"title", "name", "headline"} {
		if value, ok := fields[candidate].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	for _, field := range schema {
		if field.Type != siteconfig.FieldTypeText {
			continue
		}
		if value, ok := fields[field.Key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// normalizeSchema trims field keys/labels, drops blank entries, and stabilizes
// the schema field ordering.
func normalizeSchema(schema []siteconfig.FieldDefinition) []siteconfig.FieldDefinition {
	out := make([]siteconfig.FieldDefinition, 0, len(schema))
	for _, field := range schema {
		field.Key = strings.TrimSpace(field.Key)
		field.Label = strings.TrimSpace(field.Label)
		field.Type = strings.TrimSpace(field.Type)
		if field.Key == "" || field.Type == "" {
			continue
		}
		field.Options = trimStrings(field.Options)
		out = append(out, field)
	}
	return out
}

func trimStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

// stableSortCollections is exposed for parity with the sites assembler that
// expects collections in deterministic order; reserved for callers reading
// the draft directly rather than through the mutator.
func stableSortCollections(collections []siteconfig.Collection) {
	sort.SliceStable(collections, func(i, j int) bool {
		return collections[i].SortOrder < collections[j].SortOrder
	})
}

// _ keeps stableSortCollections referenced so the compiler does not flag it
// as unused while we wire it up in a follow-up commit.
var _ = stableSortCollections
