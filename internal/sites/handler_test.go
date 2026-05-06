package sites

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

type fakeMutator struct {
	created           siteconfig.SiteDraft
	updated           siteconfig.SiteDraft
	createInput       CreateSiteInput
	updateInput       UpdateSiteInput
	deleteSiteID      string
	deleteWorkspaceID string
	err               error
}

func (m *fakeMutator) CreateSite(_ context.Context, _ string, input CreateSiteInput) (siteconfig.SiteDraft, error) {
	m.createInput = input
	return m.created, m.err
}

func (m *fakeMutator) UpdateSite(_ context.Context, workspaceID string, siteID string, input UpdateSiteInput) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.deleteSiteID = siteID
	m.updateInput = input
	return m.updated, m.err
}

func (m *fakeMutator) DeleteSite(_ context.Context, workspaceID string, siteID string) error {
	m.deleteWorkspaceID = workspaceID
	m.deleteSiteID = siteID
	return m.err
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
		mutator:    &fakeMutator{},
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

func TestCreateSiteReturnsCreatedDraft(t *testing.T) {
	mutator := &fakeMutator{created: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites", strings.NewReader(`{"name":"Nordic Studio","prompt":"A compact site for a local studio."}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.create(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if mutator.createInput.Name != "Nordic Studio" {
		t.Fatalf("expected create name to reach mutator, got %#v", mutator.createInput)
	}
}

func TestUpdateSiteReturnsUpdatedDraft(t *testing.T) {
	mutator := &fakeMutator{updated: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/sites/site_demo", strings.NewReader(`{"name":"Renamed Studio","slug":"renamed-studio"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.update(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if mutator.deleteSiteID != "site_demo" {
		t.Fatalf("expected site id to reach mutator, got %q", mutator.deleteSiteID)
	}
	if mutator.updateInput.Name == nil || *mutator.updateInput.Name != "Renamed Studio" {
		t.Fatalf("expected updated name to reach mutator, got %#v", mutator.updateInput.Name)
	}
}

func TestDeleteSiteReturnsNoContent(t *testing.T) {
	mutator := &fakeMutator{}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/sites/site_demo", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.delete(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
	if mutator.deleteSiteID != "site_demo" {
		t.Fatalf("expected delete site id to reach mutator, got %q", mutator.deleteSiteID)
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
