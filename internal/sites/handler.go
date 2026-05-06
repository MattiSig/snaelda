package sites

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type Handler struct {
	reader     Reader
	mutator    Mutator
	authorizer Authorizer
}

type Authorizer interface {
	RequireWorkspaceMember(ctx context.Context, workspaceID string, allowedRoles ...string) (authorization.Scope, error)
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

func NewHandler(db DB) *Handler {
	return &Handler{
		reader:     NewPostgresReader(db),
		mutator:    NewPostgresMutator(db),
		authorizer: authorization.New(db),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sites", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/sites", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/sites/{siteId}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /api/sites/{siteId}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("PATCH /api/sites/{siteId}/pages/{pageId}/blocks/{blockId}", requireUser(http.HandlerFunc(h.updateBlock)))
	mux.Handle("DELETE /api/sites/{siteId}", requireUser(http.HandlerFunc(h.delete)))
}

type createSiteRequest struct {
	Name   string `json:"name"`
	Slug   string `json:"slug,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type updateSiteRequest struct {
	Name *string `json:"name,omitempty"`
	Slug *string `json:"slug,omitempty"`
}

type updateBlockRequest struct {
	Props  map[string]any `json:"props,omitempty"`
	Hidden *bool          `json:"hidden,omitempty"`
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
	writeJSON(w, http.StatusOK, map[string]any{
		"draft":         draft,
		"blockRegistry": siteconfig.DefaultBlockRegistry().Definitions(),
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.authorizer.RequireWorkspaceMember(r.Context(), workspaceID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload createSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.CreateSite(r.Context(), workspaceID, CreateSiteInput{
		Name:   strings.TrimSpace(payload.Name),
		Slug:   strings.TrimSpace(payload.Slug),
		Prompt: strings.TrimSpace(payload.Prompt),
	})
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"draft": draft})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload updateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.UpdateSite(r.Context(), scope.WorkspaceID, siteID, UpdateSiteInput{
		Name: payload.Name,
		Slug: payload.Slug,
	})
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	if err := h.mutator.DeleteSite(r.Context(), scope.WorkspaceID, siteID); err != nil {
		writeSiteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateBlock(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	pageID := r.PathValue("pageId")
	blockID := r.PathValue("blockId")
	if siteID == "" || pageID == "" || blockID == "" {
		writeError(w, http.StatusBadRequest, "invalid_block_resource", "site, page, and block ids are required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload updateBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.UpdateBlock(r.Context(), scope.WorkspaceID, siteID, pageID, blockID, UpdateBlockInput{
		Props:  payload.Props,
		Hidden: payload.Hidden,
	})
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func writeSiteError(w http.ResponseWriter, err error) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "site_not_found", "site was not found")
	case errors.Is(err, ErrPageNotFound):
		writeError(w, http.StatusNotFound, "page_not_found", "page was not found")
	case errors.Is(err, ErrBlockNotFound):
		writeError(w, http.StatusNotFound, "block_not_found", "block was not found")
	case errors.Is(err, ErrSiteNameRequired):
		writeError(w, http.StatusBadRequest, "invalid_site_name", "site name is required")
	case errors.Is(err, ErrSiteSlugInvalid):
		writeError(w, http.StatusBadRequest, "invalid_site_slug", "site slug must use lowercase words separated by hyphens")
	case errors.Is(err, ErrSiteSlugConflict):
		writeError(w, http.StatusConflict, "site_slug_conflict", "site slug is already in use")
	case errors.Is(err, ErrNoSiteChanges):
		writeError(w, http.StatusBadRequest, "no_site_changes", "at least one site field must change")
	case errors.Is(err, ErrNoBlockChanges):
		writeError(w, http.StatusBadRequest, "no_block_changes", "at least one block field must change")
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    "invalid_site_draft",
				"message": "draft changes failed validation",
			},
			"issues": validationErr.Issues,
		})
	default:
		writeError(w, http.StatusInternalServerError, "site_write_failed", "could not save site")
	}
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
