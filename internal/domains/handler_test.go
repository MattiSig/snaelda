package domains

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
)

type fakeDomainService struct {
	siteID string
	result SiteDomainsResult
	err    error
}

func (f *fakeDomainService) List(_ context.Context, siteID string) (SiteDomainsResult, error) {
	f.siteID = siteID
	return f.result, f.err
}

type fakeDomainAuthorizer struct{}

func (fakeDomainAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site_demo", Role: authorization.RoleOwner}, nil
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

	req := httptest.NewRequest(http.MethodGet, "/api/sites/site_demo/domains", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
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
