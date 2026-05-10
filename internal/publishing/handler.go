package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
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
	Rollback(ctx context.Context, siteID string, versionID string, userID string) (RollbackResult, error)
	ListVersions(ctx context.Context, siteID string) ([]VersionSummary, error)
	LoadPublishedSiteBySlug(ctx context.Context, siteSlug string, pagePath string) (PublishedSiteResult, error)
	LoadPublishedSiteByHostname(ctx context.Context, hostname string, pagePath string) (PublishedSiteResult, error)
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
	mux.Handle("POST /api/sites/{siteId}/rollback/{versionId}", requireUser(http.HandlerFunc(h.rollback)))
	mux.HandleFunc("GET /api/public/sites/{siteSlug}", h.getPublishedSite)
	mux.HandleFunc("GET /api/public/render", h.getPublishedSiteByHostname)
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

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	versionID := strings.TrimSpace(r.PathValue("versionId"))
	if versionID == "" {
		writeError(w, http.StatusBadRequest, "invalid_version_id", "version id is required")
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

	result, err := h.service.Rollback(r.Context(), siteID, versionID, user.ID)
	if err != nil {
		writePublishError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":   result.Version,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURLFromSlug(result.SiteSlug),
	})
}

func (h *Handler) getPublishedSite(w http.ResponseWriter, r *http.Request) {
	siteSlug := strings.TrimSpace(r.PathValue("siteSlug"))
	if siteSlug == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_slug", "site slug is required")
		return
	}

	pagePath := r.URL.Query().Get("path")
	result, err := h.service.LoadPublishedSiteBySlug(r.Context(), siteSlug, pagePath)
	if err != nil {
		writePublishError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"siteSlug":  result.SiteSlug,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURLFromPath(result.SiteSlug, result.PagePath),
		"version":   result.Version,
		"pagePath":  result.PagePath,
		"page":      result.Page,
		"snapshot":  result.Snapshot,
	})
}

func (h *Handler) getPublishedSiteByHostname(w http.ResponseWriter, r *http.Request) {
	hostname := publicHostnameFromRequest(r)
	if hostname == "" {
		writeError(w, http.StatusBadRequest, "invalid_hostname", "hostname is required")
		return
	}

	pagePath := r.URL.Query().Get("path")
	result, err := h.service.LoadPublishedSiteByHostname(r.Context(), hostname, pagePath)
	if err != nil {
		writePublishError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"siteSlug":  result.SiteSlug,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURLFromHostname(result.Hostname, result.PagePath),
		"version":   result.Version,
		"pagePath":  result.PagePath,
		"page":      result.Page,
		"snapshot":  result.Snapshot,
	})
}

func (h *Handler) publicURLFromSlug(siteSlug string) string {
	return h.publicURLFromPath(siteSlug, "/")
}

func (h *Handler) publicURLFromPath(siteSlug string, pagePath string) string {
	path := "/public/" + siteSlug
	if pagePath != "" && pagePath != "/" {
		path += pagePath
	}
	if h.appBaseURL == "" {
		return path
	}
	return h.appBaseURL + path
}

func (h *Handler) publicURLFromHostname(hostname string, pagePath string) string {
	normalizedHostname := normalizeHostname(hostname)
	if normalizedHostname == "" {
		return normalizePublishedPagePath(pagePath)
	}

	scheme := "http"
	port := ""
	if h.appBaseURL != "" {
		if baseURL, err := url.Parse(h.appBaseURL); err == nil {
			if baseURL.Scheme != "" {
				scheme = baseURL.Scheme
			}
			port = baseURL.Port()
		}
	}

	host := normalizedHostname
	if port != "" && !strings.Contains(normalizedHostname, ":") {
		host = net.JoinHostPort(normalizedHostname, port)
	}

	return (&url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   normalizePublishedPagePath(pagePath),
	}).String()
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

func writePublishError(w http.ResponseWriter, err error) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "published_site_not_found", "published site was not found")
	case errors.Is(err, ErrPageNotFound):
		writeError(w, http.StatusNotFound, "published_page_not_found", "published page was not found")
	case errors.Is(err, ErrVersionNotFound):
		writeError(w, http.StatusNotFound, "published_version_not_found", "published version was not found")
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
