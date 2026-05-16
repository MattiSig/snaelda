package assets

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
)

type AssetService interface {
	CreateUpload(ctx context.Context, input CreateUploadInput) (CreateUploadResult, error)
	CompleteUpload(ctx context.Context, assetID string, input CompleteUploadInput) (Asset, error)
	DownloadURL(ctx context.Context, assetID string) (string, error)
	PublicDownloadURLBySiteSlug(ctx context.Context, siteSlug string, assetID string) (string, error)
	PublicDownloadURLByHostname(ctx context.Context, hostname string, assetID string) (string, error)
	ListBySite(ctx context.Context, siteID string) ([]Asset, error)
	Update(ctx context.Context, assetID string, input UpdateAssetInput) (Asset, error)
	Delete(ctx context.Context, assetID string) error
}

type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
	RequireAsset(ctx context.Context, assetID string, allowedRoles ...string) (authorization.Scope, error)
}

type Handler struct {
	service    AssetService
	authorizer Authorizer
}

type createUploadRequest struct {
	SiteID      string `json:"siteId"`
	Kind        string `json:"kind,omitempty"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	AltText     string `json:"altText,omitempty"`
}

type completeUploadRequest struct {
	AssetID string  `json:"assetId"`
	AltText *string `json:"altText,omitempty"`
	Width   *int    `json:"width,omitempty"`
	Height  *int    `json:"height,omitempty"`
}

type updateAssetRequest struct {
	AltText *string `json:"altText,omitempty"`
}

func NewHandler(db DB, storage Storage) *Handler {
	return &Handler{
		service:    NewService(db, storage),
		authorizer: authorization.New(db),
	}
}

// NewHandlerWithService reuses an existing Service so the same instance can
// back both the public handler and the generation flow's asset import path.
func NewHandlerWithService(db DB, service AssetService) *Handler {
	return &Handler{
		service:    service,
		authorizer: authorization.New(db),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/assets/upload-url", requireUser(http.HandlerFunc(h.createUploadURL)))
	mux.Handle("POST /api/assets/complete", requireUser(http.HandlerFunc(h.completeUpload)))
	mux.Handle("GET /api/assets/{assetId}/content", requireUser(http.HandlerFunc(h.redirectAssetContent)))
	mux.Handle("GET /api/sites/{siteId}/assets", requireUser(http.HandlerFunc(h.listSiteAssets)))
	mux.Handle("PATCH /api/assets/{assetId}", requireUser(http.HandlerFunc(h.updateAsset)))
	mux.Handle("DELETE /api/assets/{assetId}", requireUser(http.HandlerFunc(h.deleteAsset)))
	mux.HandleFunc("GET /api/public/sites/{siteSlug}/assets/{assetId}", h.redirectPublicAssetContent)
	mux.HandleFunc("GET /api/public/assets/{assetId}", h.redirectPublicAssetContentByHostname)
}

func (h *Handler) createUploadURL(w http.ResponseWriter, r *http.Request) {
	var payload createUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	scope, err := h.authorizer.RequireSite(r.Context(), strings.TrimSpace(payload.SiteID), authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	session, _ := auth.SessionFromContext(r.Context())
	if session.User == nil {
		if user, ok := auth.UserFromContext(r.Context()); ok {
			session.User = &user
		}
	}
	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}

	result, err := h.service.CreateUpload(r.Context(), CreateUploadInput{
		WorkspaceID: scope.WorkspaceID,
		SiteID:      scope.SiteID,
		UserID:      userID,
		Kind:        strings.TrimSpace(payload.Kind),
		FileName:    strings.TrimSpace(payload.FileName),
		ContentType: strings.TrimSpace(payload.ContentType),
		SizeBytes:   payload.SizeBytes,
		AltText:     strings.TrimSpace(payload.AltText),
	})
	if err != nil {
		writeAssetError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) completeUpload(w http.ResponseWriter, r *http.Request) {
	var payload completeUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	assetID := strings.TrimSpace(payload.AssetID)
	if _, err := h.authorizer.RequireAsset(r.Context(), assetID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	asset, err := h.service.CompleteUpload(r.Context(), assetID, CompleteUploadInput{
		AltText: payload.AltText,
		Width:   payload.Width,
		Height:  payload.Height,
	})
	if err != nil {
		writeAssetError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"asset": asset})
}

func (h *Handler) redirectAssetContent(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimSpace(r.PathValue("assetId"))
	if _, err := h.authorizer.RequireAsset(r.Context(), assetID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	downloadURL, err := h.service.DownloadURL(r.Context(), assetID)
	if err != nil {
		writeAssetError(w, err)
		return
	}

	http.Redirect(w, r, downloadURL, http.StatusTemporaryRedirect)
}

func (h *Handler) redirectPublicAssetContent(w http.ResponseWriter, r *http.Request) {
	siteSlug := strings.TrimSpace(r.PathValue("siteSlug"))
	assetID := strings.TrimSpace(r.PathValue("assetId"))
	if siteSlug == "" || assetID == "" {
		writeError(w, http.StatusBadRequest, "invalid_public_asset", "site slug and asset id are required")
		return
	}

	downloadURL, err := h.service.PublicDownloadURLBySiteSlug(r.Context(), siteSlug, assetID)
	if err != nil {
		writeAssetError(w, err)
		return
	}

	http.Redirect(w, r, downloadURL, http.StatusTemporaryRedirect)
}

func (h *Handler) redirectPublicAssetContentByHostname(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimSpace(r.PathValue("assetId"))
	hostname := publicHostnameFromRequest(r)
	if assetID == "" || hostname == "" {
		writeError(w, http.StatusBadRequest, "invalid_public_asset", "hostname and asset id are required")
		return
	}

	downloadURL, err := h.service.PublicDownloadURLByHostname(r.Context(), hostname, assetID)
	if err != nil {
		writeAssetError(w, err)
		return
	}

	http.Redirect(w, r, downloadURL, http.StatusTemporaryRedirect)
}

func publicHostnameFromRequest(r *http.Request) string {
	for _, value := range []string{
		r.URL.Query().Get("hostname"),
		r.Header.Get("X-Forwarded-Host"),
		r.Host,
	} {
		if normalized := normalizeHostname(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizeHostname(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	} else if strings.Count(value, ":") == 1 {
		host, port, found := strings.Cut(value, ":")
		if found && host != "" && port != "" {
			value = host
		}
	}
	return strings.TrimSuffix(value, ".")
}

func (h *Handler) listSiteAssets(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	assets, err := h.service.ListBySite(r.Context(), siteID)
	if err != nil {
		writeAssetError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
}

func (h *Handler) updateAsset(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimSpace(r.PathValue("assetId"))
	if _, err := h.authorizer.RequireAsset(r.Context(), assetID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload updateAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	asset, err := h.service.Update(r.Context(), assetID, UpdateAssetInput{
		AltText: payload.AltText,
	})
	if err != nil {
		writeAssetError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"asset": asset})
}

func (h *Handler) deleteAsset(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimSpace(r.PathValue("assetId"))
	if _, err := h.authorizer.RequireAsset(r.Context(), assetID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	if err := h.service.Delete(r.Context(), assetID); err != nil {
		writeAssetError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeAssetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSiteRequired):
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
	case errors.Is(err, ErrAssetKindInvalid):
		writeError(w, http.StatusBadRequest, "invalid_asset_kind", "asset kind is not supported")
	case errors.Is(err, ErrAssetNameRequired):
		writeError(w, http.StatusBadRequest, "invalid_asset_name", "asset file name is required")
	case errors.Is(err, ErrAssetContentTypeInvalid):
		writeError(w, http.StatusBadRequest, "invalid_asset_content_type", "asset content type is not supported")
	case errors.Is(err, ErrAssetSizeInvalid):
		writeError(w, http.StatusBadRequest, "invalid_asset_size", "asset size must be between 1 byte and 20 MB")
	case errors.Is(err, ErrAssetNotFound):
		writeError(w, http.StatusNotFound, "asset_not_found", "asset was not found")
	case errors.Is(err, ErrNoAssetChanges):
		writeError(w, http.StatusBadRequest, "no_asset_changes", "at least one asset field must change")
	case errors.Is(err, ErrAssetUploadIncomplete):
		writeError(w, http.StatusConflict, "asset_upload_incomplete", "uploaded file is not ready yet")
	case errors.Is(err, ErrAssetUploadMismatch):
		writeError(w, http.StatusBadRequest, "asset_upload_mismatch", "uploaded file did not match the requested file")
	default:
		writeError(w, http.StatusInternalServerError, "asset_write_failed", "could not process asset")
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
