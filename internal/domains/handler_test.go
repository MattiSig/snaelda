package domains

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/billing"
)

type fakeDomainService struct {
	siteID   string
	hostname string
	domainID string
	result   SiteDomainsResult
	err      error
}

func (f *fakeDomainService) List(_ context.Context, siteID string) (SiteDomainsResult, error) {
	f.siteID = siteID
	return f.result, f.err
}

func (f *fakeDomainService) Create(_ context.Context, siteID string, hostname string) error {
	f.siteID = siteID
	f.hostname = hostname
	return f.err
}

func (f *fakeDomainService) Verify(_ context.Context, siteID string, domainID string) error {
	f.siteID = siteID
	f.domainID = domainID
	return f.err
}

func (f *fakeDomainService) Delete(_ context.Context, siteID string, domainID string) error {
	f.siteID = siteID
	f.domainID = domainID
	return f.err
}

type fakeDomainAuthorizer struct{}

func (fakeDomainAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site_demo", Role: authorization.RoleOwner}, nil
}

func authenticatedRequest(method string, target string, body string) *http.Request {
	return httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
}

func TestListReturnsSiteDomains(t *testing.T) {
	service := &fakeDomainService{
		result: SiteDomainsResult{
			SiteID:         "site_demo",
			SiteSlug:       "nordic-studio",
			Published:      true,
			HostedHostname: "nordic-studio.snaelda.test",
			PublicURL:      "https://nordic-studio.snaelda.test/",
			Domains: []DomainEntry{{
				ID:        "domain-1",
				Hostname:  "nordic-studio.snaelda.test",
				Type:      "subdomain",
				Status:    "active",
				PublicURL: "https://nordic-studio.snaelda.test/",
			}},
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeDomainAuthorizer{},
	}

	req := authenticatedRequest(http.MethodGet, "/api/sites/site_demo/domains", "")
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.list(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if service.siteID != "site_demo" {
		t.Fatalf("expected site id to reach service, got %q", service.siteID)
	}

	var payload SiteDomainsResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.PublicURL != "https://nordic-studio.snaelda.test/" {
		t.Fatalf("expected public url in response, got %q", payload.PublicURL)
	}
	if len(payload.Domains) != 1 || payload.Domains[0].Hostname != "nordic-studio.snaelda.test" {
		t.Fatalf("expected domains in response, got %#v", payload.Domains)
	}
}

func TestCreatePassesHostnameAndReturnsCurrentState(t *testing.T) {
	service := &fakeDomainService{
		result: SiteDomainsResult{
			SiteID:               "site_demo",
			SiteSlug:             "nordic-studio",
			Published:            true,
			HostedHostname:       "nordic-studio.snaelda.test",
			CustomDomainsEnabled: true,
			Domains: []DomainEntry{{
				ID:                   "domain-2",
				Hostname:             "example.com",
				Type:                 "custom",
				Status:               "pending",
				VerificationHostname: "_snaelda-verify.example.com",
				VerificationValue:    "snaelda-site-verification=token",
			}},
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeDomainAuthorizer{},
	}

	req := authenticatedRequest(http.MethodPost, "/api/sites/site_demo/domains", `{"hostname":"example.com"}`)
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.create(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if service.hostname != "example.com" {
		t.Fatalf("expected hostname to reach service, got %q", service.hostname)
	}

	var payload SiteDomainsResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Domains) != 1 || payload.Domains[0].VerificationHostname == "" {
		t.Fatalf("expected verification details in response, got %#v", payload.Domains)
	}
}

func TestUpdateVerifiesDomainByID(t *testing.T) {
	service := &fakeDomainService{
		result: SiteDomainsResult{
			SiteID:               "site_demo",
			SiteSlug:             "nordic-studio",
			Published:            true,
			HostedHostname:       "nordic-studio.snaelda.test",
			CustomDomainsEnabled: true,
			PublicURL:            "https://example.com/",
			Domains: []DomainEntry{{
				ID:        "domain-2",
				Hostname:  "example.com",
				Type:      "custom",
				Status:    "active",
				PublicURL: "https://example.com/",
			}},
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeDomainAuthorizer{},
	}

	req := authenticatedRequest(http.MethodPatch, "/api/sites/site_demo/domains/domain-2", `{"action":"verify"}`)
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("domainId", "domain-2")
	res := httptest.NewRecorder()

	handler.update(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if service.domainID != "domain-2" {
		t.Fatalf("expected domain id to reach service, got %q", service.domainID)
	}
}

func TestDeleteRemovesDomainByID(t *testing.T) {
	service := &fakeDomainService{
		result: SiteDomainsResult{
			SiteID:               "site_demo",
			SiteSlug:             "nordic-studio",
			Published:            true,
			HostedHostname:       "nordic-studio.snaelda.test",
			CustomDomainsEnabled: true,
			Domains:              []DomainEntry{},
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeDomainAuthorizer{},
	}

	req := authenticatedRequest(http.MethodDelete, "/api/sites/site_demo/domains/domain-2", "")
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("domainId", "domain-2")
	res := httptest.NewRecorder()

	handler.delete(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if service.domainID != "domain-2" {
		t.Fatalf("expected domain id to reach service, got %q", service.domainID)
	}
}

func TestWriteDomainErrorMapsSubscriptionRequired(t *testing.T) {
	res := httptest.NewRecorder()

	writeDomainError(res, billing.ErrSubscriptionRequired)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
}
