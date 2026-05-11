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

func TestCreatePageAppendsPageWithUniqueSlug(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	updated, err := mutator.CreatePage(context.Background(), "workspace-1", "site-1", CreatePageInput{
		Title: "Home",
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if len(updated.Pages) != 2 {
		t.Fatalf("expected second page, got %#v", updated.Pages)
	}
	if updated.Pages[1].Slug != "/home" {
		t.Fatalf("expected unique generated page slug, got %q", updated.Pages[1].Slug)
	}
}

func TestUpdatePageCanHidePageFromNavigation(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	draft.Pages = append(draft.Pages, siteconfig.PageDraft{
		ID:    "page_contact",
		Title: "Contact",
		Slug:  "/contact",
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      "block_contact",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "Contact",
					"body":    "Get in touch.",
				},
			},
		},
	})
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	includeInNavigation := false
	updated, err := mutator.UpdatePage(context.Background(), "workspace-1", "site-1", "page_contact", UpdatePageInput{
		IncludeInNavigation: &includeInNavigation,
	})
	if err != nil {
		t.Fatalf("update page: %v", err)
	}
	if len(updated.Navigation.Primary) != 1 || updated.Navigation.Primary[0].PageID != "page_home" {
		t.Fatalf("expected hidden page to be removed from navigation, got %#v", updated.Navigation.Primary)
	}
}

func TestUpdatePageSyncsExplicitNavigationLabelAndPreservesExternalLinks(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	draft.Pages = append(draft.Pages, siteconfig.PageDraft{
		ID:    "page_contact",
		Title: "Contact",
		Slug:  "/contact",
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      "block_contact",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "Contact",
					"body":    "Get in touch.",
				},
			},
		},
	})
	draft.Navigation.Primary = []siteconfig.NavigationItem{
		{Label: "Home", PageID: "page_home"},
		{Label: "Contact", PageID: "page_contact"},
		{Label: "Instagram", Href: "https://example.com/instagram"},
	}
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	title := "Reach us"
	updated, err := mutator.UpdatePage(context.Background(), "workspace-1", "site-1", "page_contact", UpdatePageInput{
		Title: &title,
	})
	if err != nil {
		t.Fatalf("update page: %v", err)
	}
	if got := updated.Navigation.Primary[1].Label; got != "Reach us" {
		t.Fatalf("expected page-backed navigation label to sync, got %#v", updated.Navigation.Primary)
	}
	if got := updated.Navigation.Primary[2].Href; got != "https://example.com/instagram" {
		t.Fatalf("expected external navigation link to survive, got %#v", updated.Navigation.Primary)
	}
}

func TestDeletePageRejectsDeletingHomepage(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	draft.Pages = append(draft.Pages, siteconfig.PageDraft{
		ID:    "page_about",
		Title: "About",
		Slug:  "/about",
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      "block_about",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "About",
					"body":    "About page copy.",
				},
			},
		},
	})
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	_, err := mutator.DeletePage(context.Background(), "workspace-1", "site-1", "page_home")
	if !errors.Is(err, ErrHomepageDeleteForbidden) {
		t.Fatalf("expected homepage delete error, got %v", err)
	}
}

func TestReorderPagesPersistsRequestedOrder(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	draft.Pages = append(draft.Pages, siteconfig.PageDraft{
		ID:    "page_contact",
		Title: "Contact",
		Slug:  "/contact",
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      "block_contact",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "Contact",
					"body":    "Contact copy.",
				},
			},
		},
	})
	draft.Navigation.Primary = append(draft.Navigation.Primary, siteconfig.NavigationItem{
		Label: "Instagram",
		Href:  "https://example.com/instagram",
	})
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	updated, err := mutator.ReorderPages(context.Background(), "workspace-1", "site-1", []string{"page_contact", "page_home"})
	if err != nil {
		t.Fatalf("reorder pages: %v", err)
	}
	if updated.Pages[0].ID != "page_contact" || updated.Navigation.Primary[0].PageID != "page_contact" {
		t.Fatalf("expected reordered pages and navigation, got %#v %#v", updated.Pages, updated.Navigation.Primary)
	}
	if got := updated.Navigation.Primary[2].Href; got != "https://example.com/instagram" {
		t.Fatalf("expected external navigation item to be preserved, got %#v", updated.Navigation.Primary)
	}
}

func TestReorderNavigationPersistsRequestedOrderWithoutMovingExternalLinks(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	draft.Pages = append(draft.Pages, siteconfig.PageDraft{
		ID:    "page_contact",
		Title: "Contact",
		Slug:  "/contact",
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      "block_contact",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "Contact",
					"body":    "Contact copy.",
				},
			},
		},
		Settings: map[string]any{
			"includeInNavigation": true,
		},
	})
	draft.Navigation.Primary = []siteconfig.NavigationItem{
		{Label: "Home", PageID: "page_home"},
		{Label: "Contact", PageID: "page_contact"},
		{Label: "Instagram", Href: "https://example.com/instagram"},
	}
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	updated, err := mutator.ReorderNavigation(context.Background(), "workspace-1", "site-1", []string{"page_contact", "page_home"})
	if err != nil {
		t.Fatalf("reorder navigation: %v", err)
	}
	if updated.Navigation.Primary[0].PageID != "page_contact" || updated.Navigation.Primary[1].PageID != "page_home" {
		t.Fatalf("expected navigation order to change, got %#v", updated.Navigation.Primary)
	}
	if got := updated.Navigation.Primary[2].Href; got != "https://example.com/instagram" {
		t.Fatalf("expected external navigation item to stay appended, got %#v", updated.Navigation.Primary)
	}
}

func TestReorderNavigationRejectsMissingIncludedPage(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	draft.Pages = append(draft.Pages, siteconfig.PageDraft{
		ID:    "page_contact",
		Title: "Contact",
		Slug:  "/contact",
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      "block_contact",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "Contact",
					"body":    "Contact copy.",
				},
			},
		},
		Settings: map[string]any{
			"includeInNavigation": true,
		},
	})
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	_, err := mutator.ReorderNavigation(context.Background(), "workspace-1", "site-1", []string{"page_home"})
	if !errors.Is(err, ErrNavigationOrderInvalid) {
		t.Fatalf("expected navigation order error, got %v", err)
	}
}

func TestCreateBlockAppendsRegistryDefaultProps(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	updated, err := mutator.CreateBlock(context.Background(), "workspace-1", "site-1", "page_home", CreateBlockInput{
		Type: "cta_band",
	})
	if err != nil {
		t.Fatalf("create block: %v", err)
	}
	block := updated.Pages[0].Blocks[len(updated.Pages[0].Blocks)-1]
	if block.Type != "cta_band" || block.Props["heading"] != "Ready to begin?" {
		t.Fatalf("expected appended block with defaults, got %#v", block)
	}
}

func TestDuplicateBlockCreatesCopyAfterOriginal(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	updated, err := mutator.DuplicateBlock(context.Background(), "workspace-1", "site-1", "page_home", "block_hero")
	if err != nil {
		t.Fatalf("duplicate block: %v", err)
	}
	if len(updated.Pages[0].Blocks) != 2 {
		t.Fatalf("expected duplicated block, got %#v", updated.Pages[0].Blocks)
	}
	if updated.Pages[0].Blocks[0].ID == updated.Pages[0].Blocks[1].ID {
		t.Fatalf("expected duplicated block id to change, got %#v", updated.Pages[0].Blocks)
	}
	if updated.Pages[0].Blocks[1].Props["headline"] != updated.Pages[0].Blocks[0].Props["headline"] {
		t.Fatalf("expected duplicated props to match, got %#v", updated.Pages[0].Blocks)
	}
}

func TestReorderBlocksPersistsRequestedOrder(t *testing.T) {
	store := newFakeMutationStore()
	draft := validHandlerDraft()
	draft.Site.ID = "site-1"
	draft.Pages[0].Blocks = append(draft.Pages[0].Blocks, siteconfig.BlockInstance{
		ID:      "block_cta",
		Type:    "cta_band",
		Version: siteconfig.BlockVersionV1,
		Props: map[string]any{
			"heading": "Ready to begin?",
			"body":    "Open the next step.",
			"variant": "primary",
		},
	})
	store.drafts[draft.Site.ID] = draft

	mutator := &PostgresMutator{
		db:     store,
		reader: store,
		writer: store,
	}

	updated, err := mutator.ReorderBlocks(context.Background(), "workspace-1", "site-1", "page_home", []string{"block_cta", "block_hero"})
	if err != nil {
		t.Fatalf("reorder blocks: %v", err)
	}
	if updated.Pages[0].Blocks[0].ID != "block_cta" {
		t.Fatalf("expected block order to change, got %#v", updated.Pages[0].Blocks)
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
