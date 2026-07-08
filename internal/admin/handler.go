package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/MattiSig/snaelda/internal/auth"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// DB is the database surface the admin handler needs.
type DB interface {
	QueryStore
}

type overviewReader interface {
	LoadOverview(ctx context.Context) (Overview, error)
	ListRecentGenerationJobs(ctx context.Context, limit int) ([]GenerationJobSummary, error)
	ListRecentSites(ctx context.Context, limit int) ([]SiteSummary, error)
}

// Handler exposes the operator-only control-room API.
type Handler struct {
	reader overviewReader
}

// NewHandler wires the admin reader.
func NewHandler(store DB) *Handler {
	return &Handler{reader: NewReader(store)}
}

// Mount registers the admin routes on the supplied mux. All routes require an
// authenticated operator session.
func (h *Handler) Mount(mux *http.ServeMux, requireSession func(http.Handler) http.Handler) {
	mux.Handle("GET /api/admin/overview", requireSession(http.HandlerFunc(h.overview)))
	mux.Handle("GET /api/admin/generation-jobs", requireSession(http.HandlerFunc(h.recentGenerationJobs)))
	mux.Handle("GET /api/admin/sites", requireSession(http.HandlerFunc(h.recentSites)))
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	overview, err := h.reader.LoadOverview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_overview_failed", "could not load the platform overview")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"overview": overview})
}

func (h *Handler) recentGenerationJobs(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	jobs, err := h.reader.ListRecentGenerationJobs(r.Context(), listLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_generation_jobs_failed", "could not load recent generation jobs")
		return
	}
	if jobs == nil {
		jobs = []GenerationJobSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (h *Handler) recentSites(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	sites, err := h.reader.ListRecentSites(r.Context(), listLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_sites_failed", "could not load recent sites")
		return
	}
	if sites == nil {
		sites = []SiteSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sites": sites})
}

func (h *Handler) requireOperator(w http.ResponseWriter, r *http.Request) bool {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok || !session.IsOperator {
		writeError(w, http.StatusForbidden, "forbidden", "operator access is required")
		return false
	}
	return true
}

func listLimit(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultListLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}
