package generation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type fakeSeedCollectionPlanner struct {
	suggestions    []SeedCollectionSuggestion
	suggestErr     error
	drafts         map[string]SeedCollectionDraftResponse
	draftErr       error
	draftRequests  []SeedCollectionDraftRequest
	suggestRequest SeedCollectionSuggestRequest
}

func (f *fakeSeedCollectionPlanner) SuggestSeedCollections(_ context.Context, request SeedCollectionSuggestRequest) ([]SeedCollectionSuggestion, error) {
	f.suggestRequest = request
	return f.suggestions, f.suggestErr
}

func (f *fakeSeedCollectionPlanner) DraftSeedCollection(_ context.Context, request SeedCollectionDraftRequest) (SeedCollectionDraftResponse, error) {
	f.draftRequests = append(f.draftRequests, request)
	if f.draftErr != nil {
		return SeedCollectionDraftResponse{}, f.draftErr
	}
	return f.drafts[request.PluralLabel], nil
}

func servicesDraftResponse() SeedCollectionDraftResponse {
	return SeedCollectionDraftResponse{
		Schema: []siteconfig.FieldDefinition{
			{Key: "title", Label: "Titill", Type: siteconfig.FieldTypeText, Required: true},
			{Key: "description", Label: "Lýsing", Type: siteconfig.FieldTypeLongText},
			{Key: "price", Label: "Verð", Type: siteconfig.FieldTypeText},
			{Key: "image", Label: "Mynd", Type: siteconfig.FieldTypeAsset},
		},
		Entries: []SeedEntryDraft{
			{Title: "Klipping", Fields: map[string]any{"title": "Klipping", "description": "Klassísk klipping og þvottur.", "price": "6.900 kr."}},
			{Title: "Litun", Fields: map[string]any{"title": "Litun", "price": "12.900 kr.", "image": "not-an-asset", "bogus": "dropped", "count": 3}},
			{Title: "Litun", Fields: map[string]any{"price": "14.900 kr."}},
		},
	}
}

func TestFinishSeedCollectionBuildsPublishableCollection(t *testing.T) {
	request := SeedCollectionInput{SingularLabel: "Þjónusta", PluralLabel: "Þjónusta", ItemsText: "Klipping..."}
	collection, ok := finishSeedCollection(request, servicesDraftResponse(), 0, map[string]bool{})
	if !ok {
		t.Fatalf("expected collection to finish")
	}
	if collection.Slug != "thjonusta" {
		t.Fatalf("slug = %q, want thjonusta", collection.Slug)
	}
	if !collection.Settings.ExposeDetailURLs {
		t.Fatalf("seed collections must expose detail URLs")
	}
	if len(collection.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(collection.Entries))
	}
	slugsSeen := map[string]bool{}
	for _, entry := range collection.Entries {
		if entry.Status != siteconfig.EntryStatusPublished {
			t.Fatalf("entry %q not seeded published", entry.Slug)
		}
		if slugsSeen[entry.Slug] {
			t.Fatalf("duplicate entry slug %q", entry.Slug)
		}
		slugsSeen[entry.Slug] = true
		title, _ := entry.Fields["title"].(string)
		if strings.TrimSpace(title) == "" {
			t.Fatalf("entry %q missing title field", entry.Slug)
		}
	}
	second := collection.Entries[1]
	if _, ok := second.Fields["bogus"]; ok {
		t.Fatalf("non-schema field survived sanitize: %+v", second.Fields)
	}
	if _, ok := second.Fields["count"]; ok {
		t.Fatalf("non-string field survived sanitize: %+v", second.Fields)
	}
	if _, ok := second.Fields["image"]; ok {
		t.Fatalf("asset field survived sanitize: %+v", second.Fields)
	}

	// The finished collection must ride the existing seed pipeline: attached to
	// the draft, index + detail pages appended, and publishable.
	draft, err := buildDraftFromPlan(planWithHome(), "klippt", "is", siteconfig.BrandConfig{}, "", []siteconfig.Collection{collection})
	if err != nil {
		t.Fatalf("buildDraftFromPlan: %v", err)
	}
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
		t.Fatalf("intake collection does not pass the publish gate: %v", err)
	}
}

func TestFinishSeedCollectionDropsUnusableShapes(t *testing.T) {
	request := SeedCollectionInput{SingularLabel: "Service", PluralLabel: "Services", ItemsText: "x"}

	noTitleField := servicesDraftResponse()
	noTitleField.Schema[0].Type = siteconfig.FieldTypeLongText
	if _, ok := finishSeedCollection(request, noTitleField, 0, map[string]bool{}); ok {
		t.Fatalf("schema without a leading text field must be dropped")
	}

	noEntries := servicesDraftResponse()
	noEntries.Entries = []SeedEntryDraft{{Title: "   "}}
	if _, ok := finishSeedCollection(request, noEntries, 0, map[string]bool{}); ok {
		t.Fatalf("collection without usable entries must be dropped")
	}
}

func TestFinishSeedCollectionKeepsCollectionSlugsUnique(t *testing.T) {
	used := map[string]bool{}
	first, ok := finishSeedCollection(SeedCollectionInput{SingularLabel: "Service", PluralLabel: "Services", ItemsText: "x"}, servicesDraftResponse(), 0, used)
	if !ok {
		t.Fatalf("first collection should finish")
	}
	second, ok := finishSeedCollection(SeedCollectionInput{SingularLabel: "Service", PluralLabel: "Services", ItemsText: "x"}, servicesDraftResponse(), 1, used)
	if !ok {
		t.Fatalf("second collection should finish")
	}
	if first.Slug == second.Slug {
		t.Fatalf("collection slugs collide: %q", first.Slug)
	}
}

func TestDraftSeedCollectionsDropsFailedDrafts(t *testing.T) {
	planner := &fakeSeedCollectionPlanner{
		drafts: map[string]SeedCollectionDraftResponse{
			"Þjónusta": servicesDraftResponse(),
		},
	}
	service := Service{seedCollectionPlanner: planner}
	input := GenerateInput{
		Prompt: "Hárgreiðslustofa í Reykjavík.",
		SeedCollectionInputs: []SeedCollectionInput{
			{SingularLabel: "Þjónusta", PluralLabel: "Þjónusta", ItemsText: "Klipping - 6.900 kr."},
			{SingularLabel: "Vara", PluralLabel: "Vörur", ItemsText: "Sjampó"}, // no draft → empty response → dropped
		},
	}
	collections := service.draftSeedCollections(context.Background(), input)
	if len(collections) != 1 {
		t.Fatalf("expected 1 collection to survive, got %d", len(collections))
	}
	if collections[0].PluralLabel != "Þjónusta" {
		t.Fatalf("wrong collection survived: %+v", collections[0])
	}
	if len(planner.draftRequests) != 2 {
		t.Fatalf("expected a draft call per input, got %d", len(planner.draftRequests))
	}

	planner.draftErr = errors.New("model unavailable")
	if collections := service.draftSeedCollections(context.Background(), input); len(collections) != 0 {
		t.Fatalf("draft errors must drop collections, got %d", len(collections))
	}

	bare := Service{}
	if collections := bare.draftSeedCollections(context.Background(), input); collections != nil {
		t.Fatalf("expected nil without a planner, got %#v", collections)
	}
}

func TestBuildInterviewMergesQuestionsAndSuggestions(t *testing.T) {
	clarifying := &fakeClarifyingPlanner{
		questions: []ClarifyingQuestion{{ID: "q1", Prompt: "?", Kind: ClarifyingQuestionKindSingle}},
	}
	seeds := &fakeSeedCollectionPlanner{
		suggestions: []SeedCollectionSuggestion{
			{ID: "services", SingularLabel: "Service", PluralLabel: "Services"},
			{ID: "team", SingularLabel: "Team member", PluralLabel: "Team"},
			{ID: "extra", SingularLabel: "Extra", PluralLabel: "Extras"},
		},
	}
	service := Service{clarifyingPlanner: clarifying, seedCollectionPlanner: seeds}
	result, err := service.BuildInterview(context.Background(), GenerateInput{Prompt: "A hair salon site."})
	if err != nil {
		t.Fatalf("interview: %v", err)
	}
	if len(result.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(result.Questions))
	}
	if len(result.Collections) != MaxSeedCollectionSuggestions {
		t.Fatalf("expected suggestions capped at %d, got %d", MaxSeedCollectionSuggestions, len(result.Collections))
	}
	if seeds.suggestRequest.Prompt != "A hair salon site." {
		t.Fatalf("suggest request missing prompt: %+v", seeds.suggestRequest)
	}
}

func TestBuildInterviewToleratesSuggestionFailure(t *testing.T) {
	clarifying := &fakeClarifyingPlanner{
		questions: []ClarifyingQuestion{{ID: "q1", Prompt: "?", Kind: ClarifyingQuestionKindSingle}},
	}
	seeds := &fakeSeedCollectionPlanner{suggestErr: errors.New("model unavailable")}
	service := Service{clarifyingPlanner: clarifying, seedCollectionPlanner: seeds}
	result, err := service.BuildInterview(context.Background(), GenerateInput{Prompt: "A hair salon site."})
	if err != nil {
		t.Fatalf("suggestion failure must not fail the interview: %v", err)
	}
	if len(result.Questions) != 1 || len(result.Collections) != 0 {
		t.Fatalf("unexpected interview result: %+v", result)
	}
}

func TestSeedCollectionsPromptDirective(t *testing.T) {
	if directive := seedCollectionsPromptDirective(nil); directive != "" {
		t.Fatalf("expected empty directive without collections, got %q", directive)
	}
	collection, ok := finishSeedCollection(
		SeedCollectionInput{SingularLabel: "Þjónusta", PluralLabel: "Þjónusta", ItemsText: "x"},
		servicesDraftResponse(), 0, map[string]bool{},
	)
	if !ok {
		t.Fatalf("collection should finish")
	}
	directive := seedCollectionsPromptDirective([]siteconfig.Collection{collection})
	for _, want := range []string{"Þjónusta", "3 entries", "Klipping", "placeholder"} {
		if !strings.Contains(directive, want) {
			t.Fatalf("directive missing %q: %s", want, directive)
		}
	}
}

func TestTrimSeedCollectionInputs(t *testing.T) {
	long := strings.Repeat("æ", maxSeedCollectionItemsCharacters+50)
	input := []SeedCollectionInput{
		{SingularLabel: " Service ", PluralLabel: " Services ", ItemsText: " " + long + " "},
		{SingularLabel: "Empty", PluralLabel: "Empties", ItemsText: "   "},
		{SingularLabel: "Team member", PluralLabel: "Team", ItemsText: "Anna — CEO"},
		{SingularLabel: "Extra", PluralLabel: "Extras", ItemsText: "over the cap"},
	}
	trimmed := trimSeedCollectionInputs(input)
	if len(trimmed) != MaxSeedCollectionSuggestions {
		t.Fatalf("expected %d inputs, got %d", MaxSeedCollectionSuggestions, len(trimmed))
	}
	if trimmed[0].SingularLabel != "Service" || trimmed[0].PluralLabel != "Services" {
		t.Fatalf("labels not trimmed: %+v", trimmed[0])
	}
	if got := len([]rune(trimmed[0].ItemsText)); got != maxSeedCollectionItemsCharacters {
		t.Fatalf("items text not capped at %d runes, got %d", maxSeedCollectionItemsCharacters, got)
	}
	if trimmed[1].PluralLabel != "Team" {
		t.Fatalf("blank-items collection not dropped: %+v", trimmed[1])
	}
	if trimSeedCollectionInputs(nil) != nil {
		t.Fatalf("nil input should stay nil")
	}
}
