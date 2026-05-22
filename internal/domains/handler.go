package domains

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/billing"
)

type Handler struct {
	service    DomainService
	authorizer Authorizer
	billingDB  billing.AccessStore
}

type HandlerConfig struct {
	AppBaseURL       string
	PublicBaseURL    string
	PublicBaseDomain string
	Cache            CacheInvalidator
	LookupTXT        dnsLookupTXT
}

type DomainService interface {
	List(ctx context.Context, siteID string) (SiteDomainsResult, error)
	Create(ctx context.Context, siteID string, hostname string) error
	Verify(ctx context.Context, siteID string, domainID string) error
	Delete(ctx context.Context, siteID string, domainID string) error
}

type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

type createDomainRequest struct {
	Hostname string `json:"hostname"`
}

type updateDomainRequest struct {
	Action string `json:"action"`
}

func NewHandler(db DB, cfg HandlerConfig) *Handler {
	return &Handler{
		service: NewService(db, ServiceConfig{
			AppBaseURL:       cfg.AppBaseURL,
			PublicBaseURL:    cfg.PublicBaseURL,
			PublicBaseDomain: cfg.PublicBaseDomain,
			Cache:            cfg.Cache,
			LookupTXT:        cfg.LookupTXT,
		}),
		authorizer: authorization.New(db),
		billingDB:  db,
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("GET /api/sites/{siteId}/domains", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/sites/{siteId}/domains", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("PATCH /api/sites/{siteId}/domains/{domainId}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/sites/{siteId}/domains/{domainId}", requireUser(http.HandlerFunc(h.delete)))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	result, err := h.service.List(r.Context(), siteID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, ok := h.requireDomainWriteAccess(w, r, siteID)
	if !ok {
		return
	}

	var payload createDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	if err := h.service.Create(r.Context(), siteID, payload.Hostname); err != nil {
		writeDomainError(w, err)
		return
	}
	h.writeCurrentState(w, r, scope.SiteID)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, ok := h.requireDomainWriteAccess(w, r, siteID)
	if !ok {
		return
	}

	domainID := strings.TrimSpace(r.PathValue("domainId"))
	if domainID == "" {
		writeError(w, http.StatusBadRequest, "invalid_domain_id", "domain id is required")
		return
	}

	var payload updateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	action := strings.TrimSpace(payload.Action)
	if action != "" && action != "verify" {
		writeError(w, http.StatusBadRequest, "invalid_action", "only the verify domain action is supported")
		return
	}

	if err := h.service.Verify(r.Context(), siteID, domainID); err != nil {
		writeDomainError(w, err)
		return
	}
	h.writeCurrentState(w, r, scope.SiteID)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, ok := h.requireDomainWriteAccess(w, r, siteID)
	if !ok {
		return
	}

	domainID := strings.TrimSpace(r.PathValue("domainId"))
	if domainID == "" {
		writeError(w, http.StatusBadRequest, "invalid_domain_id", "domain id is required")
		return
	}

	if err := h.service.Delete(r.Context(), siteID, domainID); err != nil {
		writeDomainError(w, err)
		return
	}
	h.writeCurrentState(w, r, scope.SiteID)
}

func (h *Handler) requireDomainWriteAccess(w http.ResponseWriter, r *http.Request, siteID string) (authorization.Scope, bool) {
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return authorization.Scope{}, false
	}
	if h.billingDB != nil && scope.WorkspaceID != "" {
		if err := billing.EnforceCustomDomains(r.Context(), h.billingDB, scope.WorkspaceID); err != nil {
			writeDomainError(w, err)
			return authorization.Scope{}, false
		}
	}
	return scope, true
}

func (h *Handler) writeCurrentState(w http.ResponseWriter, r *http.Request, siteID string) {
	result, err := h.service.List(r.Context(), siteID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "site_not_found", "site was not found")
	case errors.Is(err, ErrDomainNotFound):
		writeError(w, http.StatusNotFound, "domain_not_found", "domain was not found")
	case errors.Is(err, billing.ErrSubscriptionRequired):
		writeError(w, http.StatusForbidden, "subscription_required", err.Error())
	case errors.Is(err, ErrInvalidHostname):
		writeError(w, http.StatusBadRequest, "invalid_hostname", err.Error())
	case errors.Is(err, ErrHostnameConflict):
		writeError(w, http.StatusConflict, "hostname_conflict", "that hostname is already attached to another site")
	case errors.Is(err, ErrReservedHostname):
		writeError(w, http.StatusConflict, "hostname_reserved", "that hostname is reserved for Snaelda-hosted subdomains")
	case errors.Is(err, ErrManagedDomain):
		writeError(w, http.StatusConflict, "managed_domain", "the hosted Snaelda subdomain cannot be changed here")
	case errors.Is(err, ErrVerificationNotReady):
		writeError(w, http.StatusConflict, "verification_not_ready", "the DNS TXT verification record is not visible yet")
	default:
		writeError(w, http.StatusInternalServerError, "domain_lookup_failed", "could not update site domains")
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
