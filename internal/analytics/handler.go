package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/authorization"
)

// DB is the database surface the handler needs to back its reader and authorizer.
type DB interface {
	QueryStore
	authorization.Store
}

// Authorizer captures the subset of the authorization API used here.
type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

// Handler exposes the analytics HTTP surface.
type Handler struct {
	reader     *Reader
	authorizer Authorizer
}

// NewHandler wires the analytics reader and authorizer.
func NewHandler(db DB) *Handler {
	return &Handler{
		reader:     NewReader(db),
		authorizer: authorization.New(db),
	}
}

// Mount registers the analytics routes on the supplied mux.
func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("GET /api/sites/{siteId}/analytics", requireUser(http.HandlerFunc(h.getSiteAnalytics)))
}

func (h *Handler) getSiteAnalytics(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	window, err := NormalizeWindow(strings.TrimSpace(r.URL.Query().Get("window")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_window", err.Error())
		return
	}

	analytics, err := h.reader.LoadSiteAnalytics(r.Context(), siteID, window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load_analytics_failed", "could not load site analytics")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"analytics": analytics,
	})
}

func writeAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authorization.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
	case errors.Is(err, authorization.ErrInvalidResourceID):
		writeError(w, http.StatusBadRequest, "invalid_resource", "resource id is required")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access is not allowed")
	case errors.Is(err, authorization.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "authorization_unavailable", "authorization is not configured")
	default:
		writeError(w, http.StatusInternalServerError, "authorization_failed", "authorization failed")
	}
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
