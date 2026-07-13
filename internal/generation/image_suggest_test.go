package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/MattiSig/snaelda/internal/imagery"
)

type fakeImageRewriter struct {
	query   string
	err     error
	request ImageQueryRequest
}

func (f *fakeImageRewriter) RewriteImageQuery(_ context.Context, request ImageQueryRequest) (string, error) {
	f.request = request
	return f.query, f.err
}

func TestSuggestImageReturnsModelQueryAndCandidates(t *testing.T) {
	store := newFakeGenerationStore()
	provider := &fakeImageryProvider{photos: []imagery.Photo{{
		Provider:    imagery.ProviderPexels,
		ProviderID:  "42",
		DownloadURL: "https://example.test/42.jpg",
		ContentType: "image/jpeg",
		Author:      "Test",
		AuthorURL:   "https://www.pexels.com/@test",
		License:     imagery.LicensePexels,
		SourceURL:   "https://www.pexels.com/photo/42",
		Description: "A florist arranging flowers",
	}}}
	rewriter := &fakeImageRewriter{query: "florist arranging dahlias"}
	service := Service{
		db:            store,
		reader:        store,
		writer:        store,
		imagery:       NewStarterImagery(provider),
		assetImporter: &fakeAssetImporter{},
		imageRewriter: rewriter,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "Bloom Workshop",
		Prompt: "A local florist studio with a workshop schedule and gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	heroBlock := initial.Draft.Pages[0].Blocks[0]
	if heroBlock.Type != "hero" {
		t.Fatalf("expected first block to be hero, got %s", heroBlock.Type)
	}

	result, err := service.SuggestImage(
		context.Background(),
		"workspace-1",
		initial.Draft.Site.ID,
		heroBlock.ID,
		ImageSuggestInput{Path: []string{"image"}, Instruction: "Make it warmer"},
	)
	if err != nil {
		t.Fatalf("suggest image: %v", err)
	}
	if result.Query != "florist arranging dahlias" {
		t.Fatalf("expected model query to bubble up, got %q", result.Query)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected one candidate, got %#v", result.Candidates)
	}
	if result.Candidates[0].Provider != imagery.ProviderPexels {
		t.Fatalf("expected pexels provider, got %#v", result.Candidates[0])
	}
	if rewriter.request.UserInstruction != "Make it warmer" {
		t.Fatalf("expected user instruction to reach rewriter, got %#v", rewriter.request)
	}
	if rewriter.request.BlockType != "hero" {
		t.Fatalf("expected block type on rewriter request, got %#v", rewriter.request)
	}
}

func TestSuggestImageFallsBackToHeuristicQueryOnRewriterFailure(t *testing.T) {
	store := newFakeGenerationStore()
	provider := &fakeImageryProvider{photos: []imagery.Photo{{
		Provider:    imagery.ProviderPexels,
		ProviderID:  "7",
		DownloadURL: "https://example.test/7.jpg",
		ContentType: "image/jpeg",
	}}}
	rewriter := &fakeImageRewriter{err: errors.New("model offline")}
	service := Service{
		db:            store,
		reader:        store,
		writer:        store,
		imagery:       NewStarterImagery(provider),
		assetImporter: &fakeAssetImporter{},
		imageRewriter: rewriter,
	}
	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	heroBlock := initial.Draft.Pages[0].Blocks[0]

	result, err := service.SuggestImage(
		context.Background(),
		"workspace-1",
		initial.Draft.Site.ID,
		heroBlock.ID,
		ImageSuggestInput{Path: []string{"image"}},
	)
	if err != nil {
		t.Fatalf("suggest image: %v", err)
	}
	if result.Query == "" {
		t.Fatalf("expected fallback query, got empty")
	}
}

func TestSuggestImageRejectsInvalidPath(t *testing.T) {
	store := newFakeGenerationStore()
	provider := &fakeImageryProvider{photos: []imagery.Photo{{Provider: imagery.ProviderPexels, ProviderID: "1"}}}
	service := Service{
		db:            store,
		reader:        store,
		writer:        store,
		imagery:       NewStarterImagery(provider),
		assetImporter: &fakeAssetImporter{},
	}
	initial, _ := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name: "Studio", Prompt: "A calm portfolio site.",
	})
	heroBlock := initial.Draft.Pages[0].Blocks[0]

	_, err := service.SuggestImage(
		context.Background(),
		"workspace-1",
		initial.Draft.Site.ID,
		heroBlock.ID,
		ImageSuggestInput{Path: []string{}, Instruction: "test"},
	)
	if !errors.Is(err, ErrImageSuggestInvalidPath) {
		t.Fatalf("expected ErrImageSuggestInvalidPath, got %v", err)
	}
}

func TestSuggestImageRequiresConfiguredImagery(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{db: store, reader: store, writer: store}
	initial, _ := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name: "Studio", Prompt: "Site.",
	})
	heroBlock := initial.Draft.Pages[0].Blocks[0]

	_, err := service.SuggestImage(
		context.Background(),
		"workspace-1",
		initial.Draft.Site.ID,
		heroBlock.ID,
		ImageSuggestInput{Path: []string{"image"}},
	)
	if !errors.Is(err, ErrImageSuggestUnavailable) {
		t.Fatalf("expected ErrImageSuggestUnavailable, got %v", err)
	}
}

func TestApplyImageSuggestionImportsAssetAndUpdatesBlock(t *testing.T) {
	store := newFakeGenerationStore()
	provider := &fakeImageryProvider{
		photos: []imagery.Photo{{
			Provider:    imagery.ProviderPexels,
			ProviderID:  "42",
			DownloadURL: "https://example.test/42.jpg",
			ContentType: "image/jpeg",
		}},
		bodies: map[string][]byte{"42": []byte("fresh-image-bytes")},
	}
	importer := &fakeAssetImporter{}
	service := Service{
		db:            store,
		reader:        store,
		writer:        store,
		imagery:       NewStarterImagery(provider),
		assetImporter: importer,
	}
	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "Bloom Workshop",
		Prompt: "A local florist with a portfolio and a workshop schedule.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	heroBlock := initial.Draft.Pages[0].Blocks[0]

	candidate := ImageSuggestCandidate{
		Provider:    imagery.ProviderPexels,
		ProviderID:  "42",
		DownloadURL: "https://example.test/42.jpg",
		ContentType: "image/jpeg",
		Author:      "Test Photographer",
		AuthorURL:   "https://www.pexels.com/@test",
		License:     imagery.LicensePexels,
		SourceURL:   "https://www.pexels.com/photo/42",
		Description: "A florist arranging flowers",
	}
	startRevisions := len(store.revisions)
	startHistory := len(store.repromptHistory)

	result, err := service.ApplyImageSuggestion(
		context.Background(),
		"workspace-1",
		"user-1",
		initial.Draft.Site.ID,
		heroBlock.ID,
		ImageApplyInput{
			Path:        []string{"image"},
			Photo:       candidate,
			Alt:         "Florist arranging dahlias",
			Query:       "florist arranging dahlias",
			Instruction: "Pick something warmer",
		},
	)
	if err != nil {
		t.Fatalf("apply image: %v", err)
	}
	if result.JobID == "" {
		t.Fatal("expected job id")
	}
	if result.Asset == nil || result.Asset.ID == "" {
		t.Fatalf("expected new asset, got %#v", result.Asset)
	}
	updatedBlock := result.Draft.Pages[0].Blocks[0]
	image, ok := updatedBlock.Props["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected image to be set, got %#v", updatedBlock.Props["image"])
	}
	if assetID, _ := image["assetId"].(string); assetID != result.Asset.ID {
		t.Fatalf("expected assetId to match, got %#v", image)
	}
	if alt, _ := image["alt"].(string); alt != "Florist arranging dahlias" {
		t.Fatalf("expected alt text to be persisted, got %#v", image)
	}
	var imported *fakeAssetImporter
	imported = importer
	var applied bool
	for _, item := range imported.imports {
		if item.Provenance.ProviderID == "42" && item.Provenance.Author == "Test Photographer" {
			applied = true
			if string(item.Body) != "fresh-image-bytes" {
				t.Fatalf("expected downloaded bytes to flow through, got %q", string(item.Body))
			}
		}
	}
	if !applied {
		t.Fatalf("expected the applied candidate to be imported, got %#v", imported.imports)
	}
	if len(store.revisions) != startRevisions+2 {
		t.Fatalf("expected before/after revisions, got %d", len(store.revisions)-startRevisions)
	}
	if len(store.repromptHistory) != startHistory+1 {
		t.Fatalf("expected one history entry, got %d", len(store.repromptHistory)-startHistory)
	}
	entry := store.repromptHistory[0]
	if entry.Scope != "block" || entry.TargetID != heroBlock.ID {
		t.Fatalf("expected block-scoped history entry, got %#v", entry)
	}
}

func TestApplyImageSuggestionRejectsMissingPhoto(t *testing.T) {
	store := newFakeGenerationStore()
	provider := &fakeImageryProvider{}
	service := Service{
		db:            store,
		reader:        store,
		writer:        store,
		imagery:       NewStarterImagery(provider),
		assetImporter: &fakeAssetImporter{},
	}
	initial, _ := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name: "Studio", Prompt: "A studio site.",
	})
	heroBlock := initial.Draft.Pages[0].Blocks[0]
	_, err := service.ApplyImageSuggestion(
		context.Background(),
		"workspace-1",
		"user-1",
		initial.Draft.Site.ID,
		heroBlock.ID,
		ImageApplyInput{Path: []string{"image"}, Photo: ImageSuggestCandidate{}},
	)
	if !errors.Is(err, ErrImageSuggestMissingPhoto) {
		t.Fatalf("expected ErrImageSuggestMissingPhoto, got %v", err)
	}
}

func TestResolveImageSlotWalksGalleryItems(t *testing.T) {
	props := map[string]any{
		"images": []any{
			map[string]any{"image": map[string]any{"assetId": "a"}},
			map[string]any{"image": map[string]any{"assetId": "b"}},
		},
	}
	parent, leaf, err := resolveImageSlot(props, []string{"images", "1", "image"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if leaf != "image" {
		t.Fatalf("expected leaf 'image', got %q", leaf)
	}
	if got, _ := parent["image"].(map[string]any)["assetId"].(string); got != "b" {
		t.Fatalf("expected to land on second item, got %q", got)
	}
	parent["image"] = map[string]any{"assetId": "z"}
	if got, _ := props["images"].([]any)[1].(map[string]any)["image"].(map[string]any)["assetId"].(string); got != "z" {
		t.Fatalf("expected mutation to propagate, got %q", got)
	}
}

func TestResolveImageSlotRejectsBadIndex(t *testing.T) {
	props := map[string]any{"images": []any{}}
	_, _, err := resolveImageSlot(props, []string{"images", "0", "image"})
	if err == nil {
		t.Fatal("expected an error for out-of-range index")
	}
}

func TestNormalizeImagePathDropsEmpties(t *testing.T) {
	got := normalizeImagePath([]string{" image ", "", "  "})
	if len(got) != 1 || got[0] != "image" {
		t.Fatalf("expected single trimmed segment, got %#v", got)
	}
}
