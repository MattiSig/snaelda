package collections

import (
	"context"
	"errors"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func TestDiffSchemasAdditiveIsNotDestructive(t *testing.T) {
	current := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
	}
	proposed := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "summary", Label: "Summary", Type: siteconfig.FieldTypeLongText},
	}
	diff := DiffSchemas(current, proposed, nil)
	if diff.Destructive {
		t.Fatalf("expected additive change to be non-destructive: %+v", diff)
	}
	if len(diff.Changes) != 1 || diff.Changes[0].Kind != SchemaChangeAdded {
		t.Fatalf("expected single added change, got %+v", diff.Changes)
	}
}

func TestDiffSchemasRemovedFieldIsDestructive(t *testing.T) {
	current := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "summary", Label: "Summary", Type: siteconfig.FieldTypeLongText},
	}
	proposed := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
	}
	diff := DiffSchemas(current, proposed, nil)
	if !diff.Destructive {
		t.Fatalf("expected removal to be destructive")
	}
	if len(diff.Changes) != 1 || diff.Changes[0].Kind != SchemaChangeRemoved {
		t.Fatalf("expected one removed change, got %+v", diff.Changes)
	}
}

func TestDiffSchemasRetypedFieldIsDestructive(t *testing.T) {
	current := []siteconfig.FieldDefinition{
		{Key: "price", Label: "Price", Type: siteconfig.FieldTypeText},
	}
	proposed := []siteconfig.FieldDefinition{
		{Key: "price", Label: "Price", Type: siteconfig.FieldTypeNumber},
	}
	diff := DiffSchemas(current, proposed, nil)
	if !diff.Destructive {
		t.Fatalf("expected retype to be destructive")
	}
	if len(diff.Changes) != 1 || diff.Changes[0].Kind != SchemaChangeRetyped {
		t.Fatalf("expected one retyped change, got %+v", diff.Changes)
	}
}

func TestDiffSchemasRenameRequiresExplicitMapping(t *testing.T) {
	current := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText},
	}
	proposed := []siteconfig.FieldDefinition{
		{Key: "name", Label: "Name", Type: siteconfig.FieldTypeText},
	}
	// Without an explicit rename map this looks like remove+add.
	diff := DiffSchemas(current, proposed, nil)
	if !diff.Destructive {
		t.Fatalf("expected remove+add to be destructive")
	}
	gotKinds := map[SchemaChangeKind]int{}
	for _, change := range diff.Changes {
		gotKinds[change.Kind]++
	}
	if gotKinds[SchemaChangeAdded] != 1 || gotKinds[SchemaChangeRemoved] != 1 {
		t.Fatalf("expected one added + one removed, got %+v", gotKinds)
	}

	// With an explicit rename, the diff should report a single rename.
	withRename := DiffSchemas(current, proposed, map[string]string{"name": "title"})
	if !withRename.Destructive {
		t.Fatalf("expected rename to remain destructive (entry data is moved)")
	}
	if len(withRename.Changes) != 1 || withRename.Changes[0].Kind != SchemaChangeRenamed {
		t.Fatalf("expected single rename change, got %+v", withRename.Changes)
	}
}

func TestDiffSchemasOptionsNarrowedIsDestructive(t *testing.T) {
	current := []siteconfig.FieldDefinition{
		{
			Key:     "category",
			Label:   "Category",
			Type:    siteconfig.FieldTypeEnum,
			Options: []string{"residential", "commercial", "industrial"},
		},
	}
	proposed := []siteconfig.FieldDefinition{
		{
			Key:     "category",
			Label:   "Category",
			Type:    siteconfig.FieldTypeEnum,
			Options: []string{"residential", "commercial"},
		},
	}
	diff := DiffSchemas(current, proposed, nil)
	if !diff.Destructive {
		t.Fatalf("expected narrowed enum options to be destructive")
	}
}

func TestDiffSchemasLabelChangeIsNonDestructive(t *testing.T) {
	current := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText},
	}
	proposed := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Heading", Type: siteconfig.FieldTypeText},
	}
	diff := DiffSchemas(current, proposed, nil)
	if diff.Destructive {
		t.Fatalf("expected label-only change to be non-destructive: %+v", diff)
	}
	if len(diff.Changes) != 1 || diff.Changes[0].Kind != SchemaChangeModified {
		t.Fatalf("expected modified change, got %+v", diff.Changes)
	}
}

func collectionWithSchemaAndEntries(schema []siteconfig.FieldDefinition, entries []siteconfig.CollectionEntry) siteconfig.SiteDraft {
	draft := validDraft()
	draft.Collections = []siteconfig.Collection{{
		ID:            "col_services",
		Slug:          "services",
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema:        schema,
		SortOrder:     0,
		Entries:       entries,
	}}
	return draft
}

func TestUpdateCollectionAllowsAdditiveSchema(t *testing.T) {
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
	}
	store := &memoryStore{draft: collectionWithSchemaAndEntries(schema, []siteconfig.CollectionEntry{
		{
			ID:     "entry_carpentry",
			Slug:   "carpentry",
			Status: siteconfig.EntryStatusDraft,
			Fields: map[string]any{"title": "Carpentry"},
		},
	})}
	mutator := NewMutator(store, store)

	updated, err := mutator.UpdateCollection(context.Background(), "workspace", "site_test_1", "col_services", UpdateCollectionInput{
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
			{Key: "summary", Label: "Summary", Type: siteconfig.FieldTypeLongText},
		},
	})
	if err != nil {
		t.Fatalf("expected additive change to succeed, got %v", err)
	}
	if len(updated.Schema) != 2 {
		t.Fatalf("expected updated schema with 2 fields, got %#v", updated.Schema)
	}
}

func TestUpdateCollectionRejectsDestructiveSchema(t *testing.T) {
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "summary", Label: "Summary", Type: siteconfig.FieldTypeLongText},
	}
	store := &memoryStore{draft: collectionWithSchemaAndEntries(schema, nil)}
	mutator := NewMutator(store, store)

	_, err := mutator.UpdateCollection(context.Background(), "workspace", "site_test_1", "col_services", UpdateCollectionInput{
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		},
	})
	if err == nil {
		t.Fatal("expected destructive schema change to be rejected")
	}
	if !errors.Is(err, ErrSchemaMigrationRequired) {
		t.Fatalf("expected ErrSchemaMigrationRequired, got %v", err)
	}
	var migrationErr *SchemaMigrationRequiredError
	if !errors.As(err, &migrationErr) {
		t.Fatalf("expected SchemaMigrationRequiredError, got %v", err)
	}
	if !migrationErr.Diff.Destructive {
		t.Fatalf("expected error diff to be marked destructive")
	}
	if len(migrationErr.Unmapped) == 0 {
		t.Fatalf("expected at least one unmapped change in error")
	}
}

func TestMigrateSchemaRenamesEntries(t *testing.T) {
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
	}
	entries := []siteconfig.CollectionEntry{
		{
			ID:     "entry_carpentry",
			Slug:   "carpentry",
			Status: siteconfig.EntryStatusDraft,
			Fields: map[string]any{"title": "Carpentry"},
		},
	}
	store := &memoryStore{draft: collectionWithSchemaAndEntries(schema, entries)}
	mutator := NewMutator(store, store)

	proposed := []siteconfig.FieldDefinition{
		{Key: "name", Label: "Name", Type: siteconfig.FieldTypeText, Required: true},
	}
	mappings := []FieldMapping{
		{Action: "rename", OldKey: "title", NewKey: "name"},
	}
	updated, plan, err := mutator.MigrateSchema(context.Background(), "workspace", "site_test_1", "col_services", proposed, mappings)
	if err != nil {
		t.Fatalf("expected migration to succeed, got %v", err)
	}
	if updated.SchemaVersion != 2 {
		t.Fatalf("expected schema version to bump to 2, got %d", updated.SchemaVersion)
	}
	if len(updated.Entries) != 1 {
		t.Fatalf("expected one entry to remain, got %d", len(updated.Entries))
	}
	entry := updated.Entries[0]
	if entry.Fields["name"] != "Carpentry" {
		t.Fatalf("expected renamed field value to carry forward, got %#v", entry.Fields)
	}
	if _, lingering := entry.Fields["title"]; lingering {
		t.Fatalf("expected old field key to be removed, got %#v", entry.Fields)
	}
	if plan.EntriesAffected != 1 {
		t.Fatalf("expected affected count to be 1, got %d", plan.EntriesAffected)
	}
}

func TestMigrateSchemaDropsRemovedField(t *testing.T) {
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "legacy_note", Label: "Legacy note", Type: siteconfig.FieldTypeLongText},
	}
	entries := []siteconfig.CollectionEntry{
		{
			ID:     "entry_a",
			Slug:   "service-a",
			Status: siteconfig.EntryStatusDraft,
			Fields: map[string]any{"title": "Service A", "legacy_note": "Drop me"},
		},
	}
	store := &memoryStore{draft: collectionWithSchemaAndEntries(schema, entries)}
	mutator := NewMutator(store, store)

	proposed := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
	}
	mappings := []FieldMapping{
		{Action: "drop", OldKey: "legacy_note"},
	}
	updated, _, err := mutator.MigrateSchema(context.Background(), "workspace", "site_test_1", "col_services", proposed, mappings)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	entry := updated.Entries[0]
	if _, exists := entry.Fields["legacy_note"]; exists {
		t.Fatalf("expected legacy field to be dropped, got %#v", entry.Fields)
	}
	if entry.Fields["title"] != "Service A" {
		t.Fatalf("expected unrelated fields to remain, got %#v", entry.Fields)
	}
}

func TestMigrateSchemaRequiresAcknowledgementForUnmappedRemoval(t *testing.T) {
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "legacy_note", Label: "Legacy note", Type: siteconfig.FieldTypeLongText},
	}
	store := &memoryStore{draft: collectionWithSchemaAndEntries(schema, nil)}
	mutator := NewMutator(store, store)

	proposed := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
	}
	_, _, err := mutator.MigrateSchema(context.Background(), "workspace", "site_test_1", "col_services", proposed, nil)
	if err == nil {
		t.Fatal("expected migration without acknowledgement to fail")
	}
	if !errors.Is(err, ErrSchemaMigrationIncomplete) {
		t.Fatalf("expected ErrSchemaMigrationIncomplete, got %v", err)
	}
}

func TestMigrateSchemaClearsRetypedFieldValues(t *testing.T) {
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "price", Label: "Price", Type: siteconfig.FieldTypeText},
	}
	entries := []siteconfig.CollectionEntry{
		{
			ID:     "entry_a",
			Slug:   "service-a",
			Status: siteconfig.EntryStatusDraft,
			Fields: map[string]any{"title": "Service A", "price": "$120"},
		},
	}
	store := &memoryStore{draft: collectionWithSchemaAndEntries(schema, entries)}
	mutator := NewMutator(store, store)

	proposed := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "price", Label: "Price", Type: siteconfig.FieldTypeNumber},
	}
	mappings := []FieldMapping{
		{Action: "retype_clear", OldKey: "price"},
	}
	updated, _, err := mutator.MigrateSchema(context.Background(), "workspace", "site_test_1", "col_services", proposed, mappings)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	entry := updated.Entries[0]
	if _, present := entry.Fields["price"]; present {
		t.Fatalf("expected retyped field value to be cleared, got %#v", entry.Fields)
	}
	if updated.Schema[1].Type != siteconfig.FieldTypeNumber {
		t.Fatalf("expected schema to apply new type, got %#v", updated.Schema[1])
	}
}

func TestPreviewSchemaMigrationDoesNotPersist(t *testing.T) {
	schema := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		{Key: "legacy_note", Label: "Legacy note", Type: siteconfig.FieldTypeLongText},
	}
	store := &memoryStore{draft: collectionWithSchemaAndEntries(schema, nil)}
	mutator := NewMutator(store, store)

	proposed := []siteconfig.FieldDefinition{
		{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
	}
	plan, err := mutator.PreviewSchemaMigration(context.Background(), "site_test_1", "col_services", proposed, nil)
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if !plan.Diff.Destructive {
		t.Fatalf("expected preview to surface destructive diff")
	}
	if len(plan.UnmappedChanges) == 0 {
		t.Fatalf("expected preview to surface unmapped changes")
	}
	// Verify store was not mutated.
	current, err := mutator.GetCollection(context.Background(), "site_test_1", "col_services")
	if err != nil {
		t.Fatalf("get collection after preview: %v", err)
	}
	if len(current.Schema) != 2 {
		t.Fatalf("expected schema to remain unchanged after preview, got %#v", current.Schema)
	}
}
