package generation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MattiSig/snaelda/internal/imagery"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type fakeStarterImageQueryPlanner struct {
	queries  []string
	err      error
	requests []StarterImageQueryRequest
}

func (f *fakeStarterImageQueryPlanner) NormalizeStarterImageQueries(_ context.Context, request StarterImageQueryRequest) ([]string, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	return f.queries, nil
}

func TestCollectStarterImageSlotsWalksFillOrder(t *testing.T) {
	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{ID: "site-1", Name: "Stofa"},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page-1",
				Title: "Heim",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:   "hero-1",
						Type: "hero",
						Props: map[string]any{
							"headline":    "Blöðrukrabbamein",
							"subheadline": "Greining og stigun",
						},
					},
					{
						ID:   "img-1",
						Type: "image_text",
						Props: map[string]any{
							"heading": "Filled already",
							"image":   map[string]any{"assetId": "asset-9"},
						},
					},
				},
			},
			{
				ID:    "page-2",
				Title: "Myndir",
				Slug:  "/myndir",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:   "gallery-1",
						Type: "gallery",
						Props: map[string]any{
							"images": []any{
								map[string]any{"title": "Fyrsta", "image": map[string]any{"assetId": "asset-1"}},
								map[string]any{"title": "Önnur", "caption": "Lýsing"},
							},
						},
					},
				},
			},
		},
	}

	keys, slots := collectStarterImageSlots(draft)
	if len(keys) != 2 || len(slots) != 2 {
		t.Fatalf("expected 2 empty slots, got keys=%v slots=%d", keys, len(slots))
	}
	if keys[0] != starterImageSlotKey(0, 0, -1) {
		t.Fatalf("expected hero slot first, got %q", keys[0])
	}
	if slots[0].BlockType != "hero" || slots[0].Heading != "Blöðrukrabbamein" || slots[0].Body != "Greining og stigun" {
		t.Fatalf("unexpected hero slot context: %+v", slots[0])
	}
	if keys[1] != starterImageSlotKey(1, 0, 1) {
		t.Fatalf("expected second gallery item slot, got %q", keys[1])
	}
	if slots[1].Heading != "Önnur" || slots[1].Body != "Lýsing" || slots[1].PageTitle != "Myndir" {
		t.Fatalf("unexpected gallery slot context: %+v", slots[1])
	}
}

func TestApplyStarterImagerySearchesNormalizedQueryFirst(t *testing.T) {
	provider := &fakeImageryProvider{photos: []imagery.Photo{{
		Provider:    imagery.ProviderPexels,
		ProviderID:  "42",
		DownloadURL: "https://example.test/photo-42.jpg",
		ContentType: "image/jpeg",
		License:     imagery.LicensePexels,
	}}}
	planner := &fakeStarterImageQueryPlanner{queries: []string{"doctor consulting patient clinic"}}

	service := &Service{
		writer:            &savedDraftWriter{},
		imagery:           NewStarterImagery(provider),
		assetImporter:     &fakeAssetImporter{},
		imageQueryPlanner: planner,
	}

	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{ID: "site-1", Name: "Stofa", Slug: "stofa", Status: "draft", DefaultLocale: "is"},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-1",
			Title: "Heim",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{{
				ID:      "hero-1",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Props:   map[string]any{"headline": "Blöðrukrabbamein", "layout": "centered"},
			}},
		}},
	}

	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "Upplýsingavefur um blöðrukrabbamein", nil)
	if !ok {
		t.Fatal("expected enrichment to succeed")
	}
	if len(planner.requests) != 1 {
		t.Fatalf("expected one batched normalization call, got %d", len(planner.requests))
	}
	request := planner.requests[0]
	if request.Locale != "is" || len(request.Slots) != 1 || request.Slots[0].Heading != "Blöðrukrabbamein" {
		t.Fatalf("unexpected normalization request: %+v", request)
	}
	if len(provider.searched) == 0 || provider.searched[0] != "doctor consulting patient clinic" {
		t.Fatalf("expected normalized query searched first, got %v", provider.searched)
	}
	image, _ := enriched.Pages[0].Blocks[0].Props["image"].(map[string]any)
	if assetID, _ := image["assetId"].(string); assetID == "" {
		t.Fatalf("expected hero image filled, got %#v", image)
	}
}

func TestApplyStarterImageryFallsBackWhenNormalizerFails(t *testing.T) {
	provider := &fakeImageryProvider{photos: []imagery.Photo{{
		Provider:    imagery.ProviderPexels,
		ProviderID:  "7",
		DownloadURL: "https://example.test/photo-7.jpg",
		ContentType: "image/jpeg",
		License:     imagery.LicensePexels,
	}}}
	planner := &fakeStarterImageQueryPlanner{err: errors.New("model unavailable")}

	service := &Service{
		writer:            &savedDraftWriter{},
		imagery:           NewStarterImagery(provider),
		assetImporter:     &fakeAssetImporter{},
		imageQueryPlanner: planner,
	}

	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{ID: "site-1", Name: "Stofa", Slug: "stofa", Status: "draft"},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-1",
			Title: "Heim",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{{
				ID:      "hero-1",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Props:   map[string]any{"headline": "Blöðrukrabbamein", "layout": "centered"},
			}},
		}},
	}

	enriched, ok := service.enrichDraftWithStarterImagery(context.Background(), "workspace-1", "user-1", draft, "Upplýsingavefur", nil)
	if !ok {
		t.Fatal("expected enrichment to succeed despite normalizer failure")
	}
	if len(provider.searched) == 0 || provider.searched[0] != "Blöðrukrabbamein" {
		t.Fatalf("expected deterministic query chain, got %v", provider.searched)
	}
	image, _ := enriched.Pages[0].Blocks[0].Props["image"].(map[string]any)
	if assetID, _ := image["assetId"].(string); assetID == "" {
		t.Fatalf("expected hero image filled via fallback, got %#v", image)
	}
}

func TestPrependQueryDedupesCaseInsensitively(t *testing.T) {
	queries := prependQuery([]string{"Cozy Cafe", "storefront"}, "cozy cafe")
	if len(queries) != 2 || queries[0] != "Cozy Cafe" {
		t.Fatalf("expected dedupe against existing chain, got %v", queries)
	}
	queries = prependQuery([]string{"storefront"}, "doctor clinic")
	if len(queries) != 2 || queries[0] != "doctor clinic" {
		t.Fatalf("expected normalized query first, got %v", queries)
	}
}

func TestAppendQueryTruncatesOnRuneBoundary(t *testing.T) {
	value := strings.Repeat("æðislegt útsýni ", 10)
	queries := appendQuery(nil, value)
	if len(queries) != 1 {
		t.Fatalf("expected one query, got %v", queries)
	}
	if len(queries[0]) > 80 {
		t.Fatalf("expected query capped at 80 bytes, got %d", len(queries[0]))
	}
	if !utf8.ValidString(queries[0]) {
		t.Fatalf("expected valid UTF-8 after truncation, got %q", queries[0])
	}
}
