package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MattiSig/snaelda/internal/auth"
)

type stubReader struct {
	overview     Overview
	jobs         []GenerationJobSummary
	sites        []SiteSummary
	lastJobLimit int
}

func (s *stubReader) LoadOverview(ctx context.Context) (Overview, error) {
	return s.overview, nil
}

func (s *stubReader) ListRecentGenerationJobs(ctx context.Context, limit int) ([]GenerationJobSummary, error) {
	s.lastJobLimit = limit
	return s.jobs, nil
}

func (s *stubReader) ListRecentSites(ctx context.Context, limit int) ([]SiteSummary, error) {
	return s.sites, nil
}

func newTestMux(reader overviewReader, session auth.Session) *http.ServeMux {
	handler := &Handler{reader: reader}
	mux := http.NewServeMux()
	handler.Mount(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithSession(r.Context(), session)))
		})
	})
	return mux
}

func TestOverviewRequiresOperator(t *testing.T) {
	mux := newTestMux(&stubReader{}, auth.Session{Kind: auth.SessionKindAuthenticated})

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestOverviewReturnsStatsForOperator(t *testing.T) {
	reader := &stubReader{
		overview: Overview{
			GenerationJobs: GenerationJobStats{Last24Hours: 3, Total: 42},
		},
	}
	mux := newTestMux(reader, auth.Session{Kind: auth.SessionKindAuthenticated, IsOperator: true})

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Overview Overview `json:"overview"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Overview.GenerationJobs.Last24Hours != 3 {
		t.Fatalf("expected 3 jobs in last 24h, got %d", payload.Overview.GenerationJobs.Last24Hours)
	}
	if payload.Overview.GenerationJobs.Total != 42 {
		t.Fatalf("expected 42 total jobs, got %d", payload.Overview.GenerationJobs.Total)
	}
}

func TestRecentGenerationJobsClampsLimit(t *testing.T) {
	reader := &stubReader{}
	mux := newTestMux(reader, auth.Session{Kind: auth.SessionKindAuthenticated, IsOperator: true})

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/generation-jobs?limit=5000", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if reader.lastJobLimit != maxListLimit {
		t.Fatalf("expected limit clamped to %d, got %d", maxListLimit, reader.lastJobLimit)
	}
}

func TestRecentSitesReturnsEmptyListNotNull(t *testing.T) {
	mux := newTestMux(&stubReader{}, auth.Session{Kind: auth.SessionKindAuthenticated, IsOperator: true})

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/sites", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if string(payload["sites"]) != "[]" {
		t.Fatalf("expected empty array, got %s", payload["sites"])
	}
}
