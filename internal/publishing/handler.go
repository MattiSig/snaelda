package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type Handler struct {
	service    Publisher
	authorizer Authorizer
	appBaseURL string
}

type Publisher interface {
	Publish(ctx context.Context, siteID string, userID string, input PublishInput) (PublishResult, error)
	ListVersions(ctx context.Context, siteID string) ([]VersionSummary, error)
	LoadPublishedSiteBySlug(ctx context.Context, siteSlug string) (PublishedSiteResult, error)
}

type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

type publishRequest struct {
	PublishNote string `json:"publishNote,omitempty"`
}

func NewHandler(db DB, appBaseURL string) *Handler {
	return &Handler{
		service:    NewService(db),
		authorizer: authorization.New(db),
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sites/{siteId}/publish", requireUser(http.HandlerFunc(h.publish)))
	mux.Handle("GET /api/sites/{siteId}/versions", requireUser(http.HandlerFunc(h.listVersions)))
	mux.HandleFunc("GET /api/public/sites/{siteSlug}", h.getPublishedSite)
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
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

	var payload publishRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	result, err := h.service.Publish(r.Context(), siteID, user.ID, PublishInput{
		PublishNote: strings.TrimSpace(payload.PublishNote),
	})
	if err != nil {
		writePublishError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":   result.Version,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURLFromSlug(result.SiteSlug),
		"snapshot":  result.Snapshot,
	})
}

func (h *Handler) listVersions(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	versions, err := h.service.ListVersions(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_versions_failed", "could not load publish history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"versions": versions,
	})
}

func (h *Handler) getPublishedSite(w http.ResponseWriter, r *http.Request) {
	siteSlug := strings.TrimSpace(r.PathValue("siteSlug"))
	if siteSlug == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_slug", "site slug is required")
		return
	}

	result, err := h.service.LoadPublishedSiteBySlug(r.Context(), siteSlug)
	if err != nil {
		writePublishError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"siteSlug":  result.SiteSlug,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURLFromSlug(result.SiteSlug),
		"version":   result.Version,
		"snapshot":  result.Snapshot,
	})
}

func (h *Handler) publicURLFromSlug(siteSlug string) string {
	if h.appBaseURL == "" {
		return "/public/" + siteSlug
	}
	return h.appBaseURL + "/public/" + siteSlug
}

func writePublishError(w http.ResponseWriter, err error) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "published_site_not_found", "published site was not found")
	case errors.Is(err, ErrHostnameConflict):
		writeError(w, http.StatusConflict, "published_hostname_conflict", "published hostname is already in use")
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    "invalid_publish_snapshot",
				"message": "site cannot be published until validation passes",
			},
			"issues": validationErr.Issues,
		})
	default:
		writeError(w, http.StatusInternalServerError, "publish_failed", "could not publish site")
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
