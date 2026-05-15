package sites

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type Handler struct {
	reader     Reader
	mutator    Mutator
	authorizer Authorizer
	previews   PreviewTokenService
}

type Authorizer interface {
	RequireWorkspaceMember(ctx context.Context, workspaceID string, allowedRoles ...string) (authorization.Scope, error)
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

func NewHandler(db DB, previewTokenTTL time.Duration) *Handler {
	return &Handler{
		reader:     NewPostgresReader(db),
		mutator:    NewPostgresMutator(db),
		authorizer: authorization.New(db),
		previews:   NewPostgresPreviewTokenService(db, previewTokenTTL),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sites", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/sites", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/sites/{siteId}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /api/sites/{siteId}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("POST /api/sites/{siteId}/pages", requireUser(http.HandlerFunc(h.createPage)))
	mux.Handle("PATCH /api/sites/{siteId}/pages/{pageId}", requireUser(http.HandlerFunc(h.updatePage)))
	mux.Handle("DELETE /api/sites/{siteId}/pages/{pageId}", requireUser(http.HandlerFunc(h.deletePage)))
	mux.Handle("POST /api/sites/{siteId}/pages/reorder", requireUser(http.HandlerFunc(h.reorderPages)))
	mux.Handle("POST /api/sites/{siteId}/navigation/reorder", requireUser(http.HandlerFunc(h.reorderNavigation)))
	mux.Handle("PUT /api/sites/{siteId}/navigation", requireUser(http.HandlerFunc(h.updateNavigation)))
	mux.Handle("POST /api/sites/{siteId}/pages/{pageId}/blocks", requireUser(http.HandlerFunc(h.createBlock)))
	mux.Handle("PATCH /api/sites/{siteId}/pages/{pageId}/blocks/{blockId}", requireUser(http.HandlerFunc(h.updateBlock)))
	mux.Handle("DELETE /api/sites/{siteId}/pages/{pageId}/blocks/{blockId}", requireUser(http.HandlerFunc(h.deleteBlock)))
	mux.Handle("POST /api/sites/{siteId}/pages/{pageId}/blocks/{blockId}/duplicate", requireUser(http.HandlerFunc(h.duplicateBlock)))
	mux.Handle("POST /api/sites/{siteId}/pages/{pageId}/blocks/reorder", requireUser(http.HandlerFunc(h.reorderBlocks)))
	mux.Handle("POST /api/sites/{siteId}/preview-token", requireUser(http.HandlerFunc(h.issuePreviewToken)))
	mux.Handle("DELETE /api/sites/{siteId}/preview-token", requireUser(http.HandlerFunc(h.revokePreviewToken)))
	mux.Handle("DELETE /api/sites/{siteId}", requireUser(http.HandlerFunc(h.delete)))
	mux.HandleFunc("GET /api/public/preview/{token}", h.getPreviewByToken)
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

type createPageRequest struct {
	Title               string `json:"title"`
	Slug                string `json:"slug,omitempty"`
	IncludeInNavigation *bool  `json:"includeInNavigation,omitempty"`
}

type updatePageRequest struct {
	Title               *string               `json:"title,omitempty"`
	Slug                *string               `json:"slug,omitempty"`
	SEO                 *siteconfig.SEOConfig `json:"seo,omitempty"`
	IncludeInNavigation *bool                 `json:"includeInNavigation,omitempty"`
}

type reorderPagesRequest struct {
	PageIDs []string `json:"pageIds"`
}

type reorderNavigationRequest struct {
	PageIDs []string `json:"pageIds"`
}

type navigationItemRequest struct {
	Label  string `json:"label"`
	PageID string `json:"pageId,omitempty"`
	Href   string `json:"href,omitempty"`
}

type updateNavigationRequest struct {
	Primary []navigationItemRequest `json:"primary"`
}

type createBlockRequest struct {
	Type    string `json:"type"`
	Version string `json:"version,omitempty"`
}

type updateBlockRequest struct {
	Props  map[string]any `json:"props,omitempty"`
	Hidden *bool          `json:"hidden,omitempty"`
}

type reorderBlocksRequest struct {
	BlockIDs []string `json:"blockIds"`
}

type previewTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
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
	generation, err := h.reader.LoadGenerationMetadata(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load_site_failed", "could not load site draft")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"draft":         draft,
		"generation":    generation,
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

func (h *Handler) createPage(w http.ResponseWriter, r *http.Request) {
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

	var payload createPageRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.CreatePage(r.Context(), scope.WorkspaceID, siteID, CreatePageInput{
		Title:               strings.TrimSpace(payload.Title),
		Slug:                strings.TrimSpace(payload.Slug),
		IncludeInNavigation: payload.IncludeInNavigation,
	})
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"draft": draft})
}

func (h *Handler) updatePage(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	pageID := r.PathValue("pageId")
	if siteID == "" || pageID == "" {
		writeError(w, http.StatusBadRequest, "invalid_page_resource", "site and page ids are required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload updatePageRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.UpdatePage(r.Context(), scope.WorkspaceID, siteID, pageID, UpdatePageInput{
		Title:               payload.Title,
		Slug:                payload.Slug,
		SEO:                 payload.SEO,
		IncludeInNavigation: payload.IncludeInNavigation,
	})
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) deletePage(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	pageID := r.PathValue("pageId")
	if siteID == "" || pageID == "" {
		writeError(w, http.StatusBadRequest, "invalid_page_resource", "site and page ids are required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	draft, err := h.mutator.DeletePage(r.Context(), scope.WorkspaceID, siteID, pageID)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) reorderPages(w http.ResponseWriter, r *http.Request) {
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

	var payload reorderPagesRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.ReorderPages(r.Context(), scope.WorkspaceID, siteID, payload.PageIDs)
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

func (h *Handler) reorderNavigation(w http.ResponseWriter, r *http.Request) {
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

	var payload reorderNavigationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.ReorderNavigation(r.Context(), scope.WorkspaceID, siteID, payload.PageIDs)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) updateNavigation(w http.ResponseWriter, r *http.Request) {
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

	var payload updateNavigationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	items := make([]siteconfig.NavigationItem, 0, len(payload.Primary))
	for _, raw := range payload.Primary {
		items = append(items, siteconfig.NavigationItem{
			Label:  strings.TrimSpace(raw.Label),
			PageID: strings.TrimSpace(raw.PageID),
			Href:   strings.TrimSpace(raw.Href),
		})
	}

	draft, err := h.mutator.UpdateNavigation(r.Context(), scope.WorkspaceID, siteID, items)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) createBlock(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	pageID := r.PathValue("pageId")
	if siteID == "" || pageID == "" {
		writeError(w, http.StatusBadRequest, "invalid_block_resource", "site and page ids are required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload createBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.CreateBlock(r.Context(), scope.WorkspaceID, siteID, pageID, CreateBlockInput{
		Type:    strings.TrimSpace(payload.Type),
		Version: strings.TrimSpace(payload.Version),
	})
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"draft": draft})
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

func (h *Handler) deleteBlock(w http.ResponseWriter, r *http.Request) {
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

	draft, err := h.mutator.DeleteBlock(r.Context(), scope.WorkspaceID, siteID, pageID, blockID)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) duplicateBlock(w http.ResponseWriter, r *http.Request) {
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

	draft, err := h.mutator.DuplicateBlock(r.Context(), scope.WorkspaceID, siteID, pageID, blockID)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"draft": draft})
}

func (h *Handler) reorderBlocks(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	pageID := r.PathValue("pageId")
	if siteID == "" || pageID == "" {
		writeError(w, http.StatusBadRequest, "invalid_block_resource", "site and page ids are required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload reorderBlocksRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	draft, err := h.mutator.ReorderBlocks(r.Context(), scope.WorkspaceID, siteID, pageID, payload.BlockIDs)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) issuePreviewToken(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	if _, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}

	previewToken, err := h.previews.Issue(r.Context(), siteID, user.ID)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, previewTokenResponse{
		Token:     previewToken.Token,
		ExpiresAt: previewToken.ExpiresAt,
	})
}

func (h *Handler) revokePreviewToken(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	if _, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	if err := h.previews.Revoke(r.Context(), siteID); err != nil {
		writeSiteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getPreviewByToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "invalid_preview_token", "preview token is required")
		return
	}

	draft, err := h.previews.LoadDraft(r.Context(), token)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"draft": draft,
	})
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
	case errors.Is(err, ErrPageTitleRequired):
		writeError(w, http.StatusBadRequest, "invalid_page_title", "page title is required")
	case errors.Is(err, ErrPageSlugInvalid):
		writeError(w, http.StatusBadRequest, "invalid_page_slug", "page slug must be / or a slash-prefixed slug")
	case errors.Is(err, ErrPageSlugConflict):
		writeError(w, http.StatusConflict, "page_slug_conflict", "page slug is already in use")
	case errors.Is(err, ErrNoPageChanges):
		writeError(w, http.StatusBadRequest, "no_page_changes", "at least one page field must change")
	case errors.Is(err, ErrPageLimitReached):
		writeError(w, http.StatusBadRequest, "page_limit_reached", "site already has the maximum number of pages")
	case errors.Is(err, ErrPageOrderInvalid):
		writeError(w, http.StatusBadRequest, "invalid_page_order", "page reorder must include every page exactly once")
	case errors.Is(err, ErrHomepageSlugLocked):
		writeError(w, http.StatusBadRequest, "homepage_slug_locked", "homepage slug cannot be changed")
	case errors.Is(err, ErrHomepageDeleteForbidden):
		writeError(w, http.StatusBadRequest, "homepage_delete_forbidden", "homepage cannot be deleted")
	case errors.Is(err, ErrMinimumPagesRequired):
		writeError(w, http.StatusBadRequest, "minimum_pages_required", "site must keep at least one page")
	case errors.Is(err, ErrBlockTypeRequired):
		writeError(w, http.StatusBadRequest, "invalid_block_type", "block type is required")
	case errors.Is(err, ErrNoBlockChanges):
		writeError(w, http.StatusBadRequest, "no_block_changes", "at least one block field must change")
	case errors.Is(err, ErrBlockOrderInvalid):
		writeError(w, http.StatusBadRequest, "invalid_block_order", "block reorder must include every block exactly once")
	case errors.Is(err, ErrNavigationOrderInvalid):
		writeError(w, http.StatusBadRequest, "invalid_navigation_order", "navigation reorder must include every visible navigation page exactly once")
	case errors.Is(err, ErrNavigationLabelRequired):
		writeError(w, http.StatusBadRequest, "invalid_navigation_label", "navigation item label is required")
	case errors.Is(err, ErrNavigationLabelTooLong):
		writeError(w, http.StatusBadRequest, "invalid_navigation_label", "navigation item label is too long")
	case errors.Is(err, ErrNavigationItemInvalid):
		writeError(w, http.StatusBadRequest, "invalid_navigation_item", "navigation item must reference a page or include an href, not both")
	case errors.Is(err, ErrNavigationPageUnknown):
		writeError(w, http.StatusBadRequest, "invalid_navigation_item", "navigation item references a page that does not exist")
	case errors.Is(err, ErrNavigationHrefInvalid):
		writeError(w, http.StatusBadRequest, "invalid_navigation_href", "navigation item href is invalid")
	case errors.Is(err, ErrPreviewTokenNotFound):
		writeError(w, http.StatusNotFound, "preview_token_not_found", "preview link is invalid or expired")
	case errors.Is(err, ErrPreviewTokenInvalid):
		writeError(w, http.StatusBadRequest, "invalid_preview_token", "preview token is invalid")
	case errors.Is(err, siteconfig.ErrBlockTypeUnknown):
		writeError(w, http.StatusBadRequest, "unknown_block_type", "block type is not registered")
	case errors.Is(err, siteconfig.ErrBlockVersionUnknown):
		writeError(w, http.StatusBadRequest, "unknown_block_version", "block version is not registered")
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
