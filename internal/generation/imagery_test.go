package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/MattiSig/snaelda/internal/assets"
	"github.com/MattiSig/snaelda/internal/imagery"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type fakeImageryProvider struct {
	photos    []imagery.Photo
	pickError error
	bodies    map[string][]byte
	searched  []string
}

func (f *fakeImageryProvider) Name() string { return "fake" }

func (f *fakeImageryProvider) Search(_ context.Context, input imagery.SearchInput) ([]imagery.Photo, error) {
	f.searched = append(f.searched, input.Query)
	if f.pickError != nil {
		return nil, f.pickError
	}
	if len(f.photos) == 0 {
		return nil, imagery.ErrNoResults
	}
	return f.photos, nil
}

func (f *fakeImageryProvider) Download(_ context.Context, photo imagery.Photo) (imagery.PhotoData, error) {
	if body, ok := f.bodies[photo.ProviderID]; ok {
		return imagery.PhotoData{Photo: photo, Body: body}, nil
	}
	return imagery.PhotoData{Photo: photo, Body: []byte("fake-jpeg")}, nil
}

type fakeAssetImporter struct {
	imports []assets.ImportExternalInput
	err     error
}

func (f *fakeAssetImporter) ImportExternal(_ context.Context, input assets.ImportExternalInput) (assets.Asset, error) {
	if f.err != nil {
		return assets.Asset{}, f.err
	}
	f.imports = append(f.imports, input)
	return assets.Asset{
		ID:          "asset-" + input.Provenance.ProviderID,
		WorkspaceID: input.WorkspaceID,
		SiteID:      input.SiteID,
		Kind:        "image",
		Metadata: assets.AssetMetadata{
			FileName:    input.FileName,
			ContentType: input.ContentType,
			Provenance:  &input.Provenance,
		},
	}, nil
}

func TestEnrichDraftWithStarterImageryFillsHeroSlot(t *testing.T) {
	provider := &fakeImageryProvider{photos: []imagery.Photo{{
		Provider:    imagery.ProviderPexels,
		ProviderID:  "42",
		DownloadURL: "https://example.test/photo-42.jpg",
		ContentType: "image/jpeg",
		Author:      "Test Photographer",
		AuthorURL:   "https://www.pexels.com/@test",
		License:     imagery.LicensePexels,
		SourceURL:   "https://www.pexels.com/photo/42",
	}}}
	importer := &fakeAssetImporter{}

	service := &Service{
		writer:        &savedDraftWriter{},
		imagery:       NewStarterImagery(provider),
		assetImporter: importer,
	}

	draft := siteconfig.SiteDraft{
		Site:  siteconfig.DraftSite{ID: "site-1", Name: "Studio", Slug: "studio", Status: "draft"},
		Theme: siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-1"}},
		},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-1",
			Title: "Home",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{{
				ID:      "block-hero",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"headline": "A confident new studio",
					"layout":   "centered",
				},
			}},
		}},
	}

	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "A premium photography studio in Stockholm", nil)
	if !ok {
		t.Fatal("expected enrichment to succeed")
	}
	hero := enriched.Pages[0].Blocks[0]
	image, ok := hero.Props["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected hero image map, got %#v", hero.Props["image"])
	}
	if assetID, _ := image["assetId"].(string); assetID != "asset-42" {
		t.Fatalf("expected asset id to be set, got %q", assetID)
	}
	if len(importer.imports) != 1 {
		t.Fatalf("expected one import, got %d", len(importer.imports))
	}
	imported := importer.imports[0]
	if imported.Provenance.Provider != imagery.ProviderPexels {
		t.Fatalf("expected provenance provider %q, got %q", imagery.ProviderPexels, imported.Provenance.Provider)
	}
	if imported.Provenance.Author != "Test Photographer" {
		t.Fatalf("expected provenance author, got %q", imported.Provenance.Author)
	}
	if imported.SiteID != draft.Site.ID {
		t.Fatalf("expected site id propagated, got %q", imported.SiteID)
	}
}

func TestEnrichDraftWithStarterImageryDeduplicatesPhotos(t *testing.T) {
	provider := &fakeImageryProvider{photos: []imagery.Photo{
		{Provider: imagery.ProviderPexels, ProviderID: "10", DownloadURL: "https://e/10.jpg", ContentType: "image/jpeg"},
		{Provider: imagery.ProviderPexels, ProviderID: "11", DownloadURL: "https://e/11.jpg", ContentType: "image/jpeg"},
	}}
	importer := &fakeAssetImporter{}

	service := &Service{
		writer:        &savedDraftWriter{},
		imagery:       NewStarterImagery(provider),
		assetImporter: importer,
	}

	draft := siteconfig.SiteDraft{
		Site:  siteconfig.DraftSite{ID: "site-1", Name: "Studio", Slug: "studio", Status: "draft"},
		Theme: siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-1"}},
		},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-1",
			Title: "Home",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{
				{
					ID:      "block-hero",
					Type:    "hero",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{
						"headline": "Welcome",
						"layout":   "centered",
					},
				},
				{
					ID:      "block-image-text",
					Type:    "image_text",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{
						"heading":       "How we work",
						"body":          "A short paragraph explaining the workflow",
						"imagePosition": "right",
					},
				},
			},
		}},
	}

	enriched, _ := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "premium photography", nil)
	hero := enriched.Pages[0].Blocks[0].Props["image"].(map[string]any)
	support := enriched.Pages[0].Blocks[1].Props["image"].(map[string]any)
	if hero["assetId"] == support["assetId"] {
		t.Fatalf("expected dedup to produce distinct assets, got %q == %q", hero["assetId"], support["assetId"])
	}
	if len(importer.imports) != 2 {
		t.Fatalf("expected two imports, got %d", len(importer.imports))
	}
}

func TestEnrichDraftWithStarterImageryFallsBackOnPickFailure(t *testing.T) {
	provider := &fakeImageryProvider{pickError: errors.New("rate limited")}
	importer := &fakeAssetImporter{}

	service := &Service{
		writer:        &savedDraftWriter{},
		imagery:       NewStarterImagery(provider),
		assetImporter: importer,
	}

	draft := siteconfig.SiteDraft{
		Site:  siteconfig.DraftSite{ID: "site-1", Name: "Studio", Slug: "studio", Status: "draft"},
		Theme: siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-1"}},
		},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-1",
			Title: "Home",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{{
				ID:      "block-hero",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"headline": "Welcome",
					"layout":   "centered",
				},
			}},
		}},
	}

	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "photography prompt", nil)
	if ok {
		t.Fatal("expected enrichment to report no changes on fallback")
	}
	if _, hasImage := enriched.Pages[0].Blocks[0].Props["image"]; hasImage {
		t.Fatalf("expected no image attached on fallback, got %#v", enriched.Pages[0].Blocks[0].Props["image"])
	}
	if len(importer.imports) != 0 {
		t.Fatalf("expected no imports on failure, got %d", len(importer.imports))
	}
}

type savedDraftWriter struct {
	saved []siteconfig.SiteDraft
}

func (w *savedDraftWriter) SaveDraft(_ context.Context, _ string, draft siteconfig.SiteDraft) error {
	w.saved = append(w.saved, draft)
	return nil
}

// seedDraft builds a two-slot draft (a hero and a gallery with two items) for
// exercising the seed-vs-stock imagery preference.
func seedDraft() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site:  siteconfig.DraftSite{ID: "site-seed", Name: "Sewer Guys", Slug: "sewer-guys", Status: "draft"},
		Theme: siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-1"}},
		},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-1",
			Title: "Home",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{
				{
					ID:      "block-hero",
					Type:    "hero",
					Version: siteconfig.BlockVersionV1,
					Props:   map[string]any{"headline": "Clogged drain? We fix them 24/7", "layout": "centered"},
				},
				{
					ID:      "block-gallery",
					Type:    "gallery",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{"images": []any{
						map[string]any{"caption": "Our work"},
						map[string]any{"caption": "On the job"},
					}},
				},
			},
		}},
	}
}

// TestEnrichDraftPrefersSeedAssetsOverStock verifies that pulled source photos
// (seed asset ids) fill image slots ahead of any stock imagery, in order, and
// stock only fills what the seeds could not.
func TestEnrichDraftPrefersSeedAssetsOverStock(t *testing.T) {
	provider := &fakeImageryProvider{photos: []imagery.Photo{{
		Provider: imagery.ProviderPexels, ProviderID: "99", DownloadURL: "https://e/99.jpg", ContentType: "image/jpeg",
	}}}
	importer := &fakeAssetImporter{}
	service := &Service{writer: &savedDraftWriter{}, imagery: NewStarterImagery(provider), assetImporter: importer}

	// Two seeds for three slots (hero + two gallery items): the third falls back
	// to stock.
	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", seedDraft(), "24/7 plumber", []string{"seed-a", "seed-b"})
	if !ok {
		t.Fatal("expected enrichment to succeed")
	}

	hero := enriched.Pages[0].Blocks[0].Props["image"].(map[string]any)
	if got := hero["assetId"]; got != "seed-a" {
		t.Fatalf("expected hero to use first seed, got %v", got)
	}
	gallery := enriched.Pages[0].Blocks[1].Props["images"].([]any)
	if got := gallery[0].(map[string]any)["image"].(map[string]any)["assetId"]; got != "seed-b" {
		t.Fatalf("expected first gallery item to use second seed, got %v", got)
	}
	// The third slot has no seed left and must fall back to a stock import.
	third := gallery[1].(map[string]any)["image"].(map[string]any)
	if got := third["assetId"]; got != "asset-99" {
		t.Fatalf("expected third slot to fall back to stock, got %v", got)
	}
	if len(importer.imports) != 1 {
		t.Fatalf("expected exactly one stock import (only the unseeded slot), got %d", len(importer.imports))
	}
}

// TestEnrichDraftSeedsWithoutStockProvider verifies that seeds fill slots even
// when no stock imagery provider is configured — the source's own photos should
// still land in the draft.
func TestEnrichDraftSeedsWithoutStockProvider(t *testing.T) {
	service := &Service{writer: &savedDraftWriter{}} // no imagery, no importer

	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", seedDraft(), "24/7 plumber", []string{"seed-a"})
	if !ok {
		t.Fatal("expected seed enrichment to succeed without a stock provider")
	}
	hero := enriched.Pages[0].Blocks[0].Props["image"].(map[string]any)
	if got := hero["assetId"]; got != "seed-a" {
		t.Fatalf("expected hero to use the seed, got %v", got)
	}
	// The remaining slots have no seed and no stock provider, so they stay empty.
	gallery := enriched.Pages[0].Blocks[1].Props["images"].([]any)
	if _, filled := gallery[0].(map[string]any)["image"]; filled {
		t.Fatal("expected gallery slot to stay empty with no seed and no stock provider")
	}
}

// TestBuildDraftFromPlanUsesPreallocatedSiteID verifies the re-spin path pins the
// draft's site id so pre-ingested brand/hero assets validate against it.
func TestBuildDraftFromPlanUsesPreallocatedSiteID(t *testing.T) {
	plan := generationPlan{
		SiteName: "Sewer Guys",
		SiteGoal: "Fast 24/7 drain service.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Pages: []generationPagePlan{{
			Title: "Home",
			Slug:  "/",
			SEO:   siteconfig.SEOConfig{Title: "Home", Description: "Welcome"},
			Blocks: []generationBlockPlan{{
				Type:  "hero",
				Props: map[string]any{"headline": "Clogged drain?"},
			}},
		}},
	}

	draft, err := buildDraftFromPlan(plan, "sewer-guys", "en", siteconfig.BrandConfig{}, "pre-allocated-site-id", nil)
	if err != nil {
		t.Fatalf("buildDraftFromPlan: %v", err)
	}
	if draft.Site.ID != "pre-allocated-site-id" {
		t.Fatalf("expected pre-allocated site id, got %q", draft.Site.ID)
	}

	// Without a pre-allocated id, a fresh one is minted (and differs).
	minted, err := buildDraftFromPlan(plan, "sewer-guys", "en", siteconfig.BrandConfig{}, "", nil)
	if err != nil {
		t.Fatalf("buildDraftFromPlan mint: %v", err)
	}
	if minted.Site.ID == "" || minted.Site.ID == "pre-allocated-site-id" {
		t.Fatalf("expected a freshly minted id, got %q", minted.Site.ID)
	}
}
