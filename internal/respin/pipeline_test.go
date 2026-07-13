package respin

import (
	"context"
	"encoding/json"
	"image/color"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/MattiSig/snaelda/internal/generation"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// fakePipelineStore records the state transitions the pipeline drives so tests
// can assert the import lifecycle without a database.
type fakePipelineStore struct {
	mu          sync.Mutex
	statuses    []string
	modes       []string
	degradedFor string
	failed      bool
	linkedJob   string
	shareSlug   string
	extractions int
}

func (f *fakePipelineStore) UpdateStatus(_ context.Context, _, status, mode string) (Import, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	if mode != "" {
		f.modes = append(f.modes, mode)
	}
	return Import{FetchStatus: status}, nil
}

func (f *fakePipelineStore) SaveExtraction(_ context.Context, _ string, _, _ json.RawMessage) (Import, error) {
	f.mu.Lock()
	f.extractions++
	f.mu.Unlock()
	return Import{}, nil
}

func (f *fakePipelineStore) SavePulledAssets(_ context.Context, _ string, _ []string) (Import, error) {
	return Import{}, nil
}

func (f *fakePipelineStore) MarkDegraded(_ context.Context, _, reason string) (Import, error) {
	f.mu.Lock()
	f.degradedFor = reason
	f.mu.Unlock()
	return Import{FetchStatus: StatusDegraded}, nil
}

func (f *fakePipelineStore) Fail(_ context.Context, _ string, _ json.RawMessage) (Import, error) {
	f.mu.Lock()
	f.failed = true
	f.mu.Unlock()
	return Import{FetchStatus: StatusFailed}, nil
}

func (f *fakePipelineStore) AssignShareSlug(_ context.Context, _, slug string) (Import, error) {
	f.mu.Lock()
	f.shareSlug = slug
	f.mu.Unlock()
	return Import{ShareSlug: slug}, nil
}

func (f *fakePipelineStore) LinkGenerationJob(_ context.Context, jobID, _ string) error {
	f.mu.Lock()
	f.linkedJob = jobID
	f.mu.Unlock()
	return nil
}

// fakeGenerator records whether generation ran and returns a canned draft.
type fakeGenerator struct {
	called    bool
	err       error
	lastInput generation.GenerateInput
}

func (g *fakeGenerator) GenerateWithProgress(_ context.Context, _, _ string, input generation.GenerateInput, sink generation.ProgressSink) (generation.GenerateResult, error) {
	g.called = true
	g.lastInput = input
	if g.err != nil {
		return generation.GenerateResult{}, g.err
	}
	if sink != nil {
		sink.OnJobCreated("job-1")
		sink.OnProgress(generation.ProgressStep{Name: "prompt.normalize", Index: 0, Total: 1})
	}
	return generation.GenerateResult{
		JobID: "job-1",
		Draft: siteconfig.SiteDraft{Site: siteconfig.DraftSite{ID: "site-1"}},
	}, g.err
}

// fakeReserver records the pre-allocated-site lifecycle so tests can assert the
// re-spin path reserves a site before the brand pull and cleans it up on
// failure.
type fakeReserver struct {
	reservedID    string
	reserveName   string
	reserveLocale string
	reserveErr    error
	deletedID     string
}

func (r *fakeReserver) ReserveSite(_ context.Context, _ string, nameHint string, locale string) (string, error) {
	r.reserveName = nameHint
	r.reserveLocale = locale
	if r.reserveErr != nil {
		return "", r.reserveErr
	}
	if r.reservedID == "" {
		r.reservedID = "reserved-site"
	}
	return r.reservedID, nil
}

func (r *fakeReserver) DeleteReservedSite(_ context.Context, _ string, siteID string) error {
	r.deletedID = siteID
	return nil
}

func servePage(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const richPage = `<!doctype html><html lang="is"><head><title>Klippt</title></head><body><main>
<h1>Hárgreiðslustofan Klippt</h1>
<p>Við klippum og litum hár í hjarta Reykjavíkur og bjóðum upp á faglega þjónustu fyrir alla fjölskylduna alla virka daga vikunnar.</p>
<p>Hafðu samband við okkur í síma 555-1234 eða komdu við á stofunni okkar í miðbænum.</p>
</main></body></html>`

func newTestPipeline(store pipelineStore, analyzer *Analyzer, gen Generator) *Pipeline {
	return NewPipeline(PipelineConfig{
		Store:     store,
		Fetcher:   testFetcher(),
		Analyzer:  analyzer,
		Generator: gen,
	})
}

func TestPipelineGeneratesOnSufficientContent(t *testing.T) {
	srv := servePage(t, richPage)
	store := &fakePipelineStore{}
	gen := &fakeGenerator{}
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"salon","services":["klipping"],"locale":"is","tone":"warm","confidence":0.9}`,
		"respin_extraction":     `{"businessName":"Klippt","contact":{"phone":"555-1234"}}`,
	}}
	analyzer := NewAnalyzer(completer)

	result, err := newTestPipeline(store, analyzer, gen).Run(context.Background(), RunParams{
		ImportID: "imp-1", WorkspaceID: "ws-1", SourceURL: srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !gen.called {
		t.Fatal("generator should run on sufficient content")
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("expected succeeded, got %q", result.Status)
	}
	if result.SiteID != "site-1" {
		t.Fatalf("expected site-1, got %q", result.SiteID)
	}
	if store.linkedJob != "job-1" {
		t.Fatalf("generation job not linked, got %q", store.linkedJob)
	}
	if store.shareSlug == "" {
		t.Fatal("share slug should be assigned on success")
	}
	for _, want := range []string{StatusFetching, StatusExtracting, StatusComposing, StatusSucceeded} {
		assertContains(t, store.statuses, want)
	}
}

// brandPage references a header logo, a content hero photo, and an og:image, all
// served by the same test server so the brand puller can ingest them.
const brandPage = `<!doctype html><html lang="is"><head>
<meta property="og:image" content="/og.png">
<title>Klippt</title></head><body>
<header><img src="/logo.png" class="logo" alt="Klippt"></header>
<main>
<h1>Hárgreiðslustofan Klippt</h1>
<p>Við klippum og litum hár í hjarta Reykjavíkur og bjóðum upp á faglega þjónustu fyrir alla fjölskylduna alla virka daga vikunnar.</p>
<p>Hafðu samband við okkur í síma 555-1234 eða komdu við á stofunni okkar í miðbænum þar sem reynt starfsfólk tekur vel á móti þér.</p>
<img src="/hero.png" alt="Vinnan okkar">
</main></body></html>`

func serveBrandSite(t *testing.T) *httptest.Server {
	t.Helper()
	logo := solidPNG(t, 240, 80, color.RGBA{R: 10, G: 90, B: 200, A: 255})
	hero := solidPNG(t, 800, 600, color.RGBA{R: 200, G: 120, B: 40, A: 255})
	og := solidPNG(t, 1200, 630, color.RGBA{R: 60, G: 160, B: 90, A: 255})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(brandPage))
	})
	serveImg := func(body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(body)
		}
	}
	mux.HandleFunc("/logo.png", serveImg(logo))
	mux.HandleFunc("/hero.png", serveImg(hero))
	mux.HandleFunc("/og.png", serveImg(og))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestPipelineReservesSiteAndThreadsBrandIntoGeneration is the core brand-pull
// wiring regression: a site is reserved before the brand pull, the source's
// assets ingest against that site, and the reserved id plus the pulled hero
// photos reach the generation input.
func TestPipelineReservesSiteAndThreadsBrandIntoGeneration(t *testing.T) {
	srv := serveBrandSite(t)
	store := &fakePipelineStore{}
	gen := &fakeGenerator{}
	reserver := &fakeReserver{reservedID: "reserved-site-1"}
	ingestor := &fakeIngestor{}
	brand := NewBrandPuller(testFetcher(), ingestor)
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"salon","services":["klipping"],"locale":"is","tone":"warm","confidence":0.9}`,
		"respin_extraction":     `{"businessName":"Klippt","contact":{"phone":"555-1234"}}`,
	}}

	pipeline := NewPipeline(PipelineConfig{
		Store:     store,
		Fetcher:   testFetcher(),
		Analyzer:  NewAnalyzer(completer),
		Brand:     brand,
		Reserver:  reserver,
		Generator: gen,
	})

	result, err := pipeline.Run(context.Background(), RunParams{
		ImportID: "imp-brand", WorkspaceID: "ws-1", SourceURL: srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("expected succeeded, got %q", result.Status)
	}

	// A site was reserved with the extracted business name before the brand pull.
	if reserver.reserveName != "Klippt" {
		t.Fatalf("expected reserve with business name, got %q", reserver.reserveName)
	}
	if reserver.deletedID != "" {
		t.Fatalf("reserved site must not be deleted on success, deleted %q", reserver.deletedID)
	}

	// Every ingested asset is scoped to the reserved site (so it validates
	// against the final draft, which reuses the same id).
	if len(ingestor.calls) == 0 {
		t.Fatal("expected brand assets to be ingested")
	}
	for _, c := range ingestor.calls {
		if c.SiteID != "reserved-site-1" {
			t.Fatalf("expected ingest scoped to reserved site, got %q", c.SiteID)
		}
	}

	// The generation input reuses the reserved id and carries the pulled hero
	// photos as seeds.
	if gen.lastInput.SiteID != "reserved-site-1" {
		t.Fatalf("expected generation to reuse reserved site id, got %q", gen.lastInput.SiteID)
	}
	if len(gen.lastInput.SeedAssetIDs) == 0 {
		t.Fatal("expected pulled hero photos to seed the generation input")
	}
	if gen.lastInput.Brand.Logo == nil || gen.lastInput.Brand.Logo.AssetID == "" {
		t.Fatalf("expected a pulled logo in the brand config, got %#v", gen.lastInput.Brand)
	}
}

// TestPipelineDeletesReservedSiteOnGenerationFailure verifies the bare reserved
// site is cleaned up when generation fails, so a failed demo leaves no orphan.
func TestPipelineDeletesReservedSiteOnGenerationFailure(t *testing.T) {
	srv := serveBrandSite(t)
	store := &fakePipelineStore{}
	gen := &fakeGenerator{err: context.DeadlineExceeded}
	reserver := &fakeReserver{reservedID: "reserved-site-2"}
	brand := NewBrandPuller(testFetcher(), &fakeIngestor{})
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"salon","services":[],"locale":"is","tone":"warm","confidence":0.9}`,
		"respin_extraction":     `{"businessName":"Klippt"}`,
	}}

	pipeline := NewPipeline(PipelineConfig{
		Store:     store,
		Fetcher:   testFetcher(),
		Analyzer:  NewAnalyzer(completer),
		Brand:     brand,
		Reserver:  reserver,
		Generator: gen,
	})

	if _, err := pipeline.Run(context.Background(), RunParams{
		ImportID: "imp-fail", WorkspaceID: "ws-1", SourceURL: srv.URL,
	}, nil); err == nil {
		t.Fatal("expected generation error to propagate")
	}
	if reserver.deletedID != "reserved-site-2" {
		t.Fatalf("expected reserved site cleanup on failure, deleted %q", reserver.deletedID)
	}
	if !store.failed {
		t.Fatal("import should be marked failed")
	}
}

func TestPipelineDegradesOnThinContent(t *testing.T) {
	srv := servePage(t, `<!doctype html><html><body><main><p>Hi.</p></main></body></html>`)
	store := &fakePipelineStore{}
	gen := &fakeGenerator{}
	// A nil completer would also degrade, but thin content degrades before the
	// analyzer even runs.
	analyzer := NewAnalyzer(&fakeCompleter{})

	result, err := newTestPipeline(store, analyzer, gen).Run(context.Background(), RunParams{
		ImportID: "imp-2", WorkspaceID: "ws-1", SourceURL: srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if gen.called {
		t.Fatal("generator must not run on thin content (prompt-flow handoff)")
	}
	if !result.Degraded || result.Status != StatusDegraded {
		t.Fatalf("expected degraded, got %+v", result)
	}
	if store.degradedFor != "thin_content" {
		t.Fatalf("expected thin_content reason, got %q", store.degradedFor)
	}
}

func TestPipelineDegradesWhenAnalyzerUnavailable(t *testing.T) {
	srv := servePage(t, richPage)
	store := &fakePipelineStore{}
	gen := &fakeGenerator{}
	analyzer := NewAnalyzer(nil) // no completer configured

	if _, err := newTestPipeline(store, analyzer, gen).Run(context.Background(), RunParams{
		ImportID: "imp-3", WorkspaceID: "ws-1", SourceURL: srv.URL,
	}, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if gen.called {
		t.Fatal("generator must not run when analysis is unavailable")
	}
	if store.degradedFor != "analysis_unavailable" {
		t.Fatalf("expected analysis_unavailable, got %q", store.degradedFor)
	}
}

func TestPipelineFailsOnGenerationError(t *testing.T) {
	srv := servePage(t, richPage)
	store := &fakePipelineStore{}
	gen := &fakeGenerator{err: context.DeadlineExceeded}
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"salon","services":[],"locale":"is","tone":"warm","confidence":0.9}`,
		"respin_extraction":     `{"businessName":"Klippt"}`,
	}}

	_, err := newTestPipeline(store, NewAnalyzer(completer), gen).Run(context.Background(), RunParams{
		ImportID: "imp-4", WorkspaceID: "ws-1", SourceURL: srv.URL,
	}, nil)
	if err == nil {
		t.Fatal("expected generation error to propagate")
	}
	if !store.failed {
		t.Fatal("import should be marked failed on generation error")
	}
}
