package sites

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
)

type Handler struct {
	reader     Reader
	authorizer Authorizer
}

type Authorizer interface {
	RequireWorkspaceMember(ctx context.Context, workspaceID string, allowedRoles ...string) (authorization.Scope, error)
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

func NewHandler(db DB) *Handler {
	return &Handler{
		reader:     NewPostgresReader(db),
		authorizer: authorization.New(db),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("GET /api/sites", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/sites/{siteId}", requireUser(http.HandlerFunc(h.get)))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}
	workspaceID := user.WorkspaceID
	if workspaceID == "" {
		writeError(w, http.StatusForbidden, "forbidden", "workspace access is required")
		return
	}
	if _, err := h.authorizer.RequireWorkspaceMember(r.Context(), workspaceID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	sites, err := h.reader.ListSites(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_sites_failed", "could not list sites")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sites": sites})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	draft, err := h.reader.LoadDraft(r.Context(), siteID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "site_not_found", "site was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load_site_failed", "could not load site draft")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
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
