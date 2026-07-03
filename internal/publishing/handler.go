package publishing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/MattiSig/snaelda/internal/analytics"
	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type Handler struct {
	service       Publisher
	authorizer    Authorizer
	appBaseURL    string
	publicBaseURL string
	viewRecorder  PageViewRecorder
	billingDB     billing.AccessStore
	logger        *slog.Logger
}

// PageViewRecorder is implemented by analytics.Recorder. The interface lets the
// handler stay decoupled from the concrete analytics package in tests.
type PageViewRecorder interface {
	RecordAsync(view analytics.PageView)
}

type Publisher interface {
	Publish(ctx context.Context, siteID string, userID string, input PublishInput) (PublishResult, error)
	Rollback(ctx context.Context, siteID string, versionID string, userID string) (RollbackResult, error)
	ListVersions(ctx context.Context, siteID string) ([]VersionSummary, error)
	LoadPublishedSiteBySlug(ctx context.Context, siteSlug string, pagePath string) (PublishedSiteResult, error)
	LoadPublishedSiteByHostname(ctx context.Context, hostname string, pagePath string) (PublishedSiteResult, error)
	LoadPublishedArtifactBySlug(ctx context.Context, siteSlug string, artifactPath string) (PublishedArtifactResult, error)
	LoadPublishedArtifactByHostname(ctx context.Context, hostname string, artifactPath string) (PublishedArtifactResult, error)
}

type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

type publishRequest struct {
	PublishNote string `json:"publishNote,omitempty"`
}

func NewHandler(db DB, appBaseURL string, publicBaseURL string, publicBaseDomain string, artifactsDir string) *Handler {
	return NewHandlerWithConfig(db, ServiceConfig{
		AppBaseURL:       appBaseURL,
		PublicBaseURL:    publicBaseURL,
		PublicBaseDomain: publicBaseDomain,
		ArtifactsDir:     artifactsDir,
	}, appBaseURL, publicBaseURL)
}

// NewHandlerWithConfig accepts a full ServiceConfig so callers can inject
// extras such as the asset provenance lookup used for publish-time
// attribution credits.
func NewHandlerWithConfig(db DB, cfg ServiceConfig, appBaseURL string, publicBaseURL string) *Handler {
	return &Handler{
		service:       NewService(db, cfg),
		authorizer:    authorization.New(db),
		appBaseURL:    strings.TrimRight(appBaseURL, "/"),
		publicBaseURL: strings.TrimSpace(publicBaseURL),
		billingDB:     db,
		logger:        slog.Default(),
	}
}

// WithLogger sets the structured logger used for unmapped publish errors.
func (h *Handler) WithLogger(logger *slog.Logger) *Handler {
	if h == nil || logger == nil {
		return h
	}
	h.logger = logger
	return h
}

// WithViewRecorder attaches a non-blocking page view recorder used after each
// public page resolution. Returns the handler for chaining.
func (h *Handler) WithViewRecorder(recorder PageViewRecorder) *Handler {
	if h == nil {
		return nil
	}
	h.viewRecorder = recorder
	return h
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sites/{siteId}/publish", requireUser(http.HandlerFunc(h.publish)))
	mux.Handle("GET /api/sites/{siteId}/versions", requireUser(http.HandlerFunc(h.listVersions)))
	mux.Handle("POST /api/sites/{siteId}/rollback/{versionId}", requireUser(http.HandlerFunc(h.rollback)))
	mux.HandleFunc("GET /api/public/sites/{siteSlug}", h.getPublishedSite)
	mux.HandleFunc("GET /api/public/render", h.getPublishedSiteByHostname)
	mux.HandleFunc("GET /api/public/artifact", h.getPublishedArtifact)
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
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
	if h.billingDB != nil && scope.WorkspaceID != "" {
		if err := billing.EnforceSiteLimit(r.Context(), h.billingDB, scope.WorkspaceID); err != nil {
			h.writePublishError(w, r, err)
			return
		}
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

	var payload publishRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	result, err := h.service.Publish(r.Context(), siteID, userID, PublishInput{
		PublishNote: strings.TrimSpace(payload.PublishNote),
	})
	if err != nil {
		h.writePublishError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":   result.Version,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURL(result.SiteSlug, result.Hostname, "/"),
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
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if h.billingDB != nil && scope.WorkspaceID != "" {
		if err := billing.EnforceSiteLimit(r.Context(), h.billingDB, scope.WorkspaceID); err != nil {
			h.writePublishError(w, r, err)
			return
		}
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

	result, err := h.service.Rollback(r.Context(), siteID, versionID, userID)
	if err != nil {
		h.writePublishError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":   result.Version,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURL(result.SiteSlug, result.Hostname, "/"),
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
		h.writePublishError(w, r, err)
		return
	}

	h.recordPageView(r, result)

	writeJSON(w, http.StatusOK, map[string]any{
		"siteSlug":  result.SiteSlug,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURL(result.SiteSlug, result.Hostname, result.PagePath),
		"version":   result.Version,
		"pagePath":  result.PagePath,
		"page":      result.Page,
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
		h.writePublishError(w, r, err)
		return
	}

	h.recordPageView(r, result)

	writeJSON(w, http.StatusOK, map[string]any{
		"siteSlug":  result.SiteSlug,
		"hostname":  result.Hostname,
		"publicUrl": h.publicURL(result.SiteSlug, result.Hostname, result.PagePath),
		"version":   result.Version,
		"pagePath":  result.PagePath,
		"page":      result.Page,
	})
}

func (h *Handler) recordPageView(r *http.Request, result PublishedSiteResult) {
	if h.viewRecorder == nil {
		return
	}
	if !analytics.CountableRequest(r) {
		return
	}
	if result.Version.SiteID == "" || result.Page.PageID == "" {
		return
	}
	h.viewRecorder.RecordAsync(analytics.PageView{
		SiteID: result.Version.SiteID,
		PageID: result.Page.PageID,
	})
}

func (h *Handler) getPublishedArtifact(w http.ResponseWriter, r *http.Request) {
	artifactPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if artifactPath == "" {
		writeError(w, http.StatusBadRequest, "invalid_artifact_path", "artifact path is required")
		return
	}

	siteSlug := strings.TrimSpace(r.URL.Query().Get("siteSlug"))
	hostname := publicHostnameFromRequest(r)

	var (
		result PublishedArtifactResult
		err    error
	)

	switch {
	case hostname != "":
		result, err = h.service.LoadPublishedArtifactByHostname(r.Context(), hostname, artifactPath)
	case siteSlug != "":
		result, err = h.service.LoadPublishedArtifactBySlug(r.Context(), siteSlug, artifactPath)
	default:
		writeError(w, http.StatusBadRequest, "invalid_artifact_lookup", "site slug or hostname is required")
		return
	}
	if err != nil {
		h.writePublishError(w, r, err)
		return
	}

	body := []byte(result.File.Body)
	etag := buildArtifactETag(result.Version.ID, body)
	w.Header().Set("Content-Type", result.File.ContentType)
	// The URL path (e.g. /api/public/artifact?path=robots.txt) is not versioned,
	// but the response body is keyed to the active published version. Use ETag
	// validation so caches revalidate quickly and serve 304s when unchanged.
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", cacheControlForArtifact(artifactPath))
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Add("Vary", "Host")
	if ifNoneMatchMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func buildArtifactETag(versionID string, body []byte) string {
	hash := sha256.Sum256(body)
	return `"` + strings.TrimSpace(versionID) + ":" + hex.EncodeToString(hash[:8]) + `"`
}

func cacheControlForArtifact(artifactPath string) string {
	switch {
	case strings.HasSuffix(strings.ToLower(artifactPath), ".html"),
		strings.HasSuffix(strings.ToLower(artifactPath), "/index.html"),
		artifactPath == "manifest.json":
		return "public, max-age=60, must-revalidate, stale-while-revalidate=600"
	case strings.HasSuffix(strings.ToLower(artifactPath), ".xml"),
		strings.HasSuffix(strings.ToLower(artifactPath), ".txt"):
		return "public, max-age=300, stale-while-revalidate=3600"
	default:
		return "public, max-age=3600, stale-while-revalidate=86400"
	}
}

func ifNoneMatchMatches(header string, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" || etag == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		// Strip weak prefix.
		if strings.HasPrefix(candidate, "W/") {
			candidate = strings.TrimPrefix(candidate, "W/")
		}
		if candidate == etag {
			return true
		}
	}
	return false
}

func (h *Handler) publicURL(siteSlug string, hostname string, pagePath string) string {
	if normalizedHostname := normalizeHostname(hostname); normalizedHostname != "" {
		return h.publicURLFromHostname(normalizedHostname, pagePath)
	}

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
	if h.publicBaseURL != "" {
		if baseURL, err := url.Parse(h.publicBaseURL); err == nil {
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

func (h *Handler) writePublishError(w http.ResponseWriter, r *http.Request, err error) {
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
		logger := h.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("publish blocked by validation",
			"method", r.Method,
			"path", r.URL.Path,
			"issues", validationErr.Error(),
		)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    "invalid_publish_snapshot",
				"message": "site cannot be published until validation passes",
			},
			"issues": validationErr.Issues,
		})
	case errors.Is(err, billing.ErrPlanLimitExceeded):
		writeError(w, http.StatusForbidden, "plan_limit_exceeded", err.Error())
	default:
		logger := h.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Error("publish site", "method", r.Method, "path", r.URL.Path, "error", err.Error())
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
