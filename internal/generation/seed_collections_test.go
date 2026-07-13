package generation

import (
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func seedServicesCollection() siteconfig.Collection {
	return siteconfig.Collection{
		ID:            "col_services_seed",
		Slug:          "thjonusta",
		SingularLabel: "Þjónusta",
		PluralLabel:   "Þjónusta",
		SchemaVersion: 1,
		Settings:      siteconfig.CollectionSettings{DefaultSort: siteconfig.CollectionSortManual, ExposeDetailURLs: true},
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Titill", Type: siteconfig.FieldTypeText, Required: true},
			{Key: "price", Label: "Verð", Type: siteconfig.FieldTypeText},
		},
		Entries: []siteconfig.CollectionEntry{
			{ID: "entry_a", Slug: "klipping", Status: siteconfig.EntryStatusPublished, Fields: map[string]any{"title": "Klipping", "price": "6.900 kr."}},
			{ID: "entry_b", Slug: "litun", Status: siteconfig.EntryStatusPublished, Fields: map[string]any{"title": "Litun", "price": "12.900 kr."}},
		},
	}
}

func planWithHome() generationPlan {
	return generationPlan{
		SiteName: "Klippt",
		SiteGoal: "Hárgreiðslustofa í Reykjavík.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Pages: []generationPagePlan{{
			Title:  "Heim",
			Slug:   "/",
			SEO:    siteconfig.SEOConfig{Title: "Heim", Description: "Velkomin"},
			Blocks: []generationBlockPlan{{Type: "hero", Props: map[string]any{"headline": "Fallegt hár"}}},
		}},
	}
}

func TestBuildDraftFromPlanAttachesSeedCollections(t *testing.T) {
	col := seedServicesCollection()
	draft, err := buildDraftFromPlan(planWithHome(), "klippt", "is", siteconfig.BrandConfig{}, "", []siteconfig.Collection{col})
	if err != nil {
		t.Fatalf("buildDraftFromPlan: %v", err)
	}

	if len(draft.Collections) != 1 || draft.Collections[0].ID != col.ID {
		t.Fatalf("collection not attached: %+v", draft.Collections)
	}

	// Home + index + detail.
	if len(draft.Pages) != 3 {
		t.Fatalf("expected 3 pages (home, index, detail), got %d", len(draft.Pages))
	}
	var index, detail *siteconfig.PageDraft
	for i := range draft.Pages {
		switch draft.Pages[i].Type {
		case siteconfig.PageTypeCollectionIndex:
			index = &draft.Pages[i]
		case siteconfig.PageTypeCollectionDetail:
			detail = &draft.Pages[i]
		}
	}
	if index == nil || detail == nil {
		t.Fatalf("missing index or detail page: %+v", draft.Pages)
	}
	if index.Slug != "/thjonusta" {
		t.Fatalf("index slug = %q, want /thjonusta", index.Slug)
	}
	if index.CollectionID != col.ID || detail.CollectionID != col.ID {
		t.Fatalf("collection pages not bound to collection")
	}
	if len(index.Blocks) != 1 || index.Blocks[0].Type != "collection_index" {
		t.Fatalf("index page missing collection_index block: %+v", index.Blocks)
	}
	if len(detail.Blocks) != 1 || detail.Blocks[0].Type != "collection_detail" {
		t.Fatalf("detail page missing collection_detail block: %+v", detail.Blocks)
	}

	// The index page is in nav; the detail template is not.
	inNav := map[string]bool{}
	for _, item := range draft.Navigation.Primary {
		inNav[item.PageID] = true
	}
	if !inNav[index.ID] {
		t.Fatalf("index page not added to navigation")
	}
	if inNav[detail.ID] {
		t.Fatalf("detail template should not be in navigation")
	}

	// The draft must be valid and publishable (entries are seeded published):
	// map it into a published snapshot and validate through the publish gate,
	// which enforces the no_published_entries rule on collection_detail templates
	// and entry-URL collisions that the draft-time check skips.
	if err := siteconfig.ValidateDraft(draft); err != nil {
		t.Fatalf("draft invalid: %v", err)
	}
	snapshot := siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site: siteconfig.PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO:           draft.Site.SEO,
		},
		Brand:       draft.Brand,
		Theme:       draft.Theme,
		Navigation:  draft.Navigation,
		Pages:       draft.Pages,
		Collections: draft.Collections,
	}
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		t.Fatalf("seeded collection draft does not pass the publish gate: %v", err)
	}
}

func TestBuildDraftFromPlanCollectionSlugDoesNotCollide(t *testing.T) {
	// A planner page already occupies the detail template address /thjonusta-entry;
	// the seeded detail page must be suffixed rather than duplicating the slug, and
	// the draft must stay valid.
	plan := planWithHome()
	plan.Pages = append(plan.Pages, generationPagePlan{
		Title:  "Ítarlegt",
		Slug:   "/thjonusta-entry",
		SEO:    siteconfig.SEOConfig{Title: "Ítarlegt", Description: "x"},
		Blocks: []generationBlockPlan{{Type: "hero", Props: map[string]any{"headline": "Ítarlegt"}}},
	})

	draft, err := buildDraftFromPlan(plan, "klippt", "is", siteconfig.BrandConfig{}, "", []siteconfig.Collection{seedServicesCollection()})
	if err != nil {
		t.Fatalf("buildDraftFromPlan: %v", err)
	}
	if err := siteconfig.ValidateDraft(draft); err != nil {
		t.Fatalf("draft invalid after slug collision: %v", err)
	}
	slugs := map[string]int{}
	for _, p := range draft.Pages {
		slugs[p.Slug]++
	}
	for slug, count := range slugs {
		if count > 1 {
			t.Fatalf("duplicate page slug %q", slug)
		}
	}
}
