package sites

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCreateSiteGeneratesUniqueSlugAndStoresPrompt(t *testing.T) {
	store := newFakeMutationStore()
	existing := validHandlerDraft()
	existing.Site.ID = "existing-site"
	existing.Site.Slug = "nordic-studio"
	store.drafts[existing.Site.ID] = existing

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	draft, err := mutator.CreateSite(context.Background(), "workspace-1", CreateSiteInput{
		Name:   "Nordic Studio",
		Prompt: "A compact site for a local design studio.",
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if draft.Site.Slug != "nordic-studio-2" {
		t.Fatalf("expected unique slug, got %q", draft.Site.Slug)
	}
	if store.prompts[draft.Site.ID] != "A compact site for a local design studio." {
		t.Fatalf("expected prompt to be stored, got %#v", store.prompts)
	}
	if len(draft.Pages) != 1 || len(draft.Pages[0].Blocks) != 4 {
		t.Fatalf("expected starter draft blocks, got %#v", draft.Pages)
	}
}

func TestUpdateSiteRejectsSlugConflict(t *testing.T) {
	store := newFakeMutationStore()
	first := validHandlerDraft()
	first.Site.ID = "site-1"
	first.Site.Slug = "nordic-studio"
	second := validHandlerDraft()
	second.Site.ID = "site-2"
	second.Site.Slug = "quiet-florist"
	store.drafts[first.Site.ID] = first
	store.drafts[second.Site.ID] = second

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	slugValue := "nordic-studio"
	_, err := mutator.UpdateSite(context.Background(), "workspace-1", "site-2", UpdateSiteInput{
		Slug: &slugValue,
	})
	if !errors.Is(err, ErrSiteSlugConflict) {
		t.Fatalf("expected slug conflict, got %v", err)
	}
}

func TestDeleteSiteRemovesDraft(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	if err := mutator.DeleteSite(context.Background(), "workspace-1", "site-1"); err != nil {
		t.Fatalf("delete site: %v", err)
	}
	if _, ok := store.drafts["site-1"]; ok {
		t.Fatal("expected draft to be deleted")
	}
}

func TestUpdateBlockPersistsValidatedProps(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	updated, err := mutator.UpdateBlock(context.Background(), "workspace-1", "site-1", "page_home", "block_hero", UpdateBlockInput{
		Props: map[string]any{
			"eyebrow":     "Nordic Studio",
			"headline":    "A more focused homepage",
			"subheadline": "Shorter, clearer, and ready to preview.",
			"layout":      "split-left",
		},
	})
	if err != nil {
		t.Fatalf("update block: %v", err)
	}
	if got := updated.Pages[0].Blocks[0].Props["headline"]; got != "A more focused homepage" {
		t.Fatalf("expected saved block props, got %#v", updated.Pages[0].Blocks[0].Props)
	}
}

func TestUpdateBlockRejectsInvalidProps(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	_, err := mutator.UpdateBlock(context.Background(), "workspace-1", "site-1", "page_home", "block_hero", UpdateBlockInput{
		Props: map[string]any{
			"headline": "",
			"layout":   "centered",
		},
	})
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

type fakeMutationStore struct {
	drafts  map[string]siteconfig.SiteDraft
	prompts map[string]string
}

func newFakeMutationStore() *fakeMutationStore {
	return &fakeMutationStore{
		drafts:  map[string]siteconfig.SiteDraft{},
		prompts: map[string]string{},
	}
}

func (s *fakeMutationStore) ListSites(context.Context, string) ([]Summary, error) {
	return nil, nil
}

func (s *fakeMutationStore) LoadDraft(_ context.Context, siteID string) (siteconfig.SiteDraft, error) {
	draft, ok := s.drafts[siteID]
	if !ok {
		return siteconfig.SiteDraft{}, ErrNotFound
	}
	return draft, nil
}

func (s *fakeMutationStore) SaveDraft(_ context.Context, _ string, draft siteconfig.SiteDraft) error {
	if err := siteconfig.ValidateDraft(draft); err != nil {
		return err
	}
	s.drafts[draft.Site.ID] = draft
	return nil
}

func (s *fakeMutationStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "select exists("):
		workspaceID := args[0].(string)
		slugValue := args[1].(string)
		excludeSiteID := args[2].(string)
		exists := false
		for siteID, draft := range s.drafts {
			if siteID == excludeSiteID {
				continue
			}
			if workspaceID == "workspace-1" && draft.Site.Slug == slugValue {
				exists = true
				break
			}
		}
		return fakeMutationRow{values: []any{exists}}
	default:
		return fakeMutationRow{err: pgx.ErrNoRows}
	}
}

func (s *fakeMutationStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "update sites"):
		siteID := args[1].(string)
		if _, ok := s.drafts[siteID]; !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		s.prompts[siteID] = args[0].(string)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "delete from sites"):
		siteID := args[0].(string)
		if _, ok := s.drafts[siteID]; !ok {
			return pgconn.NewCommandTag("DELETE 0"), nil
		}
		delete(s.drafts, siteID)
		delete(s.prompts, siteID)
		return pgconn.NewCommandTag("DELETE 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
}

type fakeMutationRow struct {
	values []any
	err    error
}

func (r fakeMutationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *bool:
			*target = value.(bool)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}
