package sites

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type fakeReader struct {
	summaries []Summary
	draft     siteconfig.SiteDraft
}

func (r fakeReader) ListSites(context.Context, string) ([]Summary, error) {
	return r.summaries, nil
}

func (r fakeReader) LoadDraft(context.Context, string) (siteconfig.SiteDraft, error) {
	return r.draft, nil
}

type fakeAuthorizer struct{}

func (fakeAuthorizer) RequireWorkspaceMember(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", Role: authorization.RoleOwner}, nil
}

func (fakeAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site_demo", Role: authorization.RoleOwner}, nil
}

func TestListSitesReturnsAuthenticatedWorkspaceSites(t *testing.T) {
	handler := Handler{
		reader: fakeReader{
			summaries: []Summary{
				{
					ID:            "site_demo",
					WorkspaceID:   "workspace-1",
					Name:          "Nordic Studio",
					Slug:          "nordic-studio",
					Status:        "draft",
					DefaultLocale: "en",
					PageCount:     1,
				},
			},
		},
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sites", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.list(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	var payload struct {
		Sites []Summary `json:"sites"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Sites) != 1 || payload.Sites[0].ID != "site_demo" {
		t.Fatalf("expected site summary, got %#v", payload.Sites)
	}
}

func TestGetSiteReturnsCanonicalDraft(t *testing.T) {
	handler := Handler{
		reader: fakeReader{
			draft: validHandlerDraft(),
		},
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sites/site_demo", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.get(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	var payload struct {
		Draft siteconfig.SiteDraft `json:"draft"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Draft.Site.ID != "site_demo" {
		t.Fatalf("expected site draft, got %#v", payload.Draft.Site)
	}
}

func validHandlerDraft() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#f8f7f4",
					"foreground": "#1d2520",
					"primary":    "#315c4f",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block_hero",
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"headline": "Clear websites for focused teams",
							"layout":   "centered",
						},
					},
				},
			},
		},
	}
}
