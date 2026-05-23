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
}

func (f *fakeImageryProvider) Name() string { return "fake" }

func (f *fakeImageryProvider) Search(_ context.Context, _ imagery.SearchInput) ([]imagery.Photo, error) {
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

	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "A premium photography studio in Stockholm")
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

	enriched, _ := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "premium photography")
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

	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "photography prompt")
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
