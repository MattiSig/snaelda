package collections

import (
	"context"
	"errors"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type memoryStore struct {
	draft siteconfig.SiteDraft
}

func (m *memoryStore) LoadDraft(_ context.Context, _ string) (siteconfig.SiteDraft, error) {
	return cloneDraft(m.draft), nil
}

func (m *memoryStore) SaveDraft(_ context.Context, _ string, draft siteconfig.SiteDraft) error {
	if err := siteconfig.ValidateDraft(draft); err != nil {
		return err
	}
	m.draft = cloneDraft(draft)
	return nil
}

func cloneDraft(in siteconfig.SiteDraft) siteconfig.SiteDraft {
	out := in
	out.Pages = append([]siteconfig.PageDraft(nil), in.Pages...)
	out.Collections = append([]siteconfig.Collection(nil), in.Collections...)
	for i := range out.Collections {
		out.Collections[i].Entries = append([]siteconfig.CollectionEntry(nil), out.Collections[i].Entries...)
	}
	return out
}

func validDraft() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:     "site_test_1",
			Name:   "Test Studio",
			Slug:   "test-studio",
			Status: "draft",
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#101010",
					"foreground": "#fafafa",
					"primary":    "#22d3ee",
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
							"headline": "Welcome",
							"layout":   "centered",
						},
					},
				},
			},
		},
	}
}

func TestCreateCollectionAssignsSlugAndID(t *testing.T) {
	store := &memoryStore{draft: validDraft()}
	mutator := NewMutator(store, store)

	collection, err := mutator.CreateCollection(context.Background(), "workspace", "site_test_1", CreateCollectionInput{
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
			{Key: "summary", Label: "Summary", Type: siteconfig.FieldTypeLongText},
		},
	})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if collection.ID == "" {
		t.Fatal("expected generated id")
	}
	if collection.Slug != "services" {
		t.Fatalf("expected slug derived from plural label, got %q", collection.Slug)
	}
	if len(collection.Schema) != 2 {
		t.Fatalf("expected schema persisted, got %d fields", len(collection.Schema))
	}

	collections, err := mutator.ListCollections(context.Background(), "site_test_1")
	if err != nil {
		t.Fatalf("list collections: %v", err)
	}
	if len(collections) != 1 {
		t.Fatalf("expected one collection, got %d", len(collections))
	}
}

func TestCreateEntryValidatesAgainstSchema(t *testing.T) {
	store := &memoryStore{draft: validDraft()}
	mutator := NewMutator(store, store)

	collection, err := mutator.CreateCollection(context.Background(), "workspace", "site_test_1", CreateCollectionInput{
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		},
	})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}

	// Missing required title field must surface as a validation error.
	_, err = mutator.CreateEntry(context.Background(), "workspace", "site_test_1", collection.ID, CreateEntryInput{
		Fields: map[string]any{},
	})
	var validation siteconfig.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected validation error, got %v", err)
	}

	// Valid entry persists.
	entry, err := mutator.CreateEntry(context.Background(), "workspace", "site_test_1", collection.ID, CreateEntryInput{
		Slug: "carpentry",
		Fields: map[string]any{
			"title": "Carpentry",
		},
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if entry.Slug != "carpentry" {
		t.Fatalf("expected slug carpentry, got %q", entry.Slug)
	}
	if entry.Fields["title"] != "Carpentry" {
		t.Fatalf("expected field persisted, got %#v", entry.Fields)
	}
}

func TestDeleteCollectionInUseRefused(t *testing.T) {
	store := &memoryStore{draft: validDraft()}
	mutator := NewMutator(store, store)

	collection, err := mutator.CreateCollection(context.Background(), "workspace", "site_test_1", CreateCollectionInput{
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		},
	})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}

	// Add a page that references the collection.
	draft := store.draft
	draft.Pages = append(draft.Pages, siteconfig.PageDraft{
		ID:           "page_services",
		Title:        "Services",
		Slug:         "/services",
		Type:         siteconfig.PageTypeCollectionIndex,
		CollectionID: collection.ID,
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      "block_index",
				Type:    "collection_index",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "All services",
				},
			},
		},
	})
	if err := siteconfig.ValidateDraft(draft); err != nil {
		t.Fatalf("seed fixture invalid: %v", err)
	}
	store.draft = draft

	err = mutator.DeleteCollection(context.Background(), "workspace", "site_test_1", collection.ID)
	if !errors.Is(err, ErrCollectionInUse) {
		t.Fatalf("expected ErrCollectionInUse, got %v", err)
	}
}

func TestCreateEntriesPersistsBatchAtomically(t *testing.T) {
	store := &memoryStore{draft: validDraft()}
	mutator := NewMutator(store, store)

	collection, err := mutator.CreateCollection(context.Background(), "workspace", "site_test_1", CreateCollectionInput{
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Title", Type: siteconfig.FieldTypeText, Required: true},
		},
	})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}

	_, err = mutator.CreateEntries(context.Background(), "workspace", "site_test_1", collection.ID, []CreateEntryInput{
		{
			Fields: map[string]any{
				"title": "Carpentry",
			},
		},
		{
			Fields: map[string]any{},
		},
	})
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}

	savedCollection, err := mutator.GetCollection(context.Background(), "site_test_1", collection.ID)
	if err != nil {
		t.Fatalf("get collection: %v", err)
	}
	if len(savedCollection.Entries) != 0 {
		t.Fatalf("expected batch failure to leave entries untouched, got %d", len(savedCollection.Entries))
	}
}
