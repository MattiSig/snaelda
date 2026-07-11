package respin

import (
	"context"
	"encoding/json"
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
	called bool
	err    error
}

func (g *fakeGenerator) GenerateWithProgress(_ context.Context, _, _ string, _ generation.GenerateInput, sink generation.ProgressSink) (generation.GenerateResult, error) {
	g.called = true
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
