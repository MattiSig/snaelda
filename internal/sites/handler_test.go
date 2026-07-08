package sites

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
)

type fakeReader struct {
	summaries []Summary
	draft     siteconfig.SiteDraft
	metadata  GenerationMetadata
}

func (r fakeReader) ListSites(context.Context, string) ([]Summary, error) {
	return r.summaries, nil
}

func (r fakeReader) LoadDraft(context.Context, string) (siteconfig.SiteDraft, error) {
	return r.draft, nil
}

func (r fakeReader) LoadGenerationMetadata(context.Context, string) (GenerationMetadata, error) {
	return r.metadata, nil
}

type fakeMutator struct {
	created                  siteconfig.SiteDraft
	updated                  siteconfig.SiteDraft
	pageCreated              siteconfig.SiteDraft
	pageUpdated              siteconfig.SiteDraft
	pageDeleted              siteconfig.SiteDraft
	pagesReordered           siteconfig.SiteDraft
	navigationReordered      siteconfig.SiteDraft
	navigationUpdated        siteconfig.SiteDraft
	navigationUpdatedItems   siteconfig.NavigationConfig
	blockCreated             siteconfig.SiteDraft
	blockUpdated             siteconfig.SiteDraft
	blockDeleted             siteconfig.SiteDraft
	blockDuplicated          siteconfig.SiteDraft
	blocksReordered          siteconfig.SiteDraft
	createInput              CreateSiteInput
	updateInput              UpdateSiteInput
	createPageInput          CreatePageInput
	updatePageInput          UpdatePageInput
	reorderPageIDs           []string
	reorderNavigationPageIDs []string
	createBlockInput         CreateBlockInput
	updateBlockInput         UpdateBlockInput
	reorderBlockIDs          []string
	deleteSiteID             string
	deleteWorkspaceID        string
	pageSiteID               string
	updatePageID             string
	updateBlockID            string
	err                      error
}

type fakePreviewTokens struct {
	issuedSiteID  string
	issuedUserID  string
	issued        PreviewToken
	revokedSiteID string
	draft         siteconfig.SiteDraft
	err           error
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

func (m *fakeMutator) CreatePage(_ context.Context, workspaceID string, siteID string, input CreatePageInput) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.createPageInput = input
	return m.pageCreated, m.err
}

func (m *fakeMutator) UpdatePage(_ context.Context, workspaceID string, siteID string, pageID string, input UpdatePageInput) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.updatePageID = pageID
	m.updatePageInput = input
	return m.pageUpdated, m.err
}

func (m *fakeMutator) DeletePage(_ context.Context, workspaceID string, siteID string, pageID string) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.updatePageID = pageID
	return m.pageDeleted, m.err
}

func (m *fakeMutator) ReorderPages(_ context.Context, workspaceID string, siteID string, pageIDs []string) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.reorderPageIDs = pageIDs
	return m.pagesReordered, m.err
}

func (m *fakeMutator) ReorderNavigation(_ context.Context, workspaceID string, siteID string, pageIDs []string) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.reorderNavigationPageIDs = pageIDs
	return m.navigationReordered, m.err
}

func (m *fakeMutator) UpdateNavigation(_ context.Context, workspaceID string, siteID string, navigation siteconfig.NavigationConfig) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.navigationUpdatedItems = navigation
	return m.navigationUpdated, m.err
}

func (m *fakeMutator) CreateBlock(_ context.Context, workspaceID string, siteID string, pageID string, input CreateBlockInput) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.updatePageID = pageID
	m.createBlockInput = input
	return m.blockCreated, m.err
}

func (m *fakeMutator) UpdateBlock(_ context.Context, workspaceID string, siteID string, pageID string, blockID string, input UpdateBlockInput) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.updatePageID = pageID
	m.updateBlockID = blockID
	m.updateBlockInput = input
	return m.blockUpdated, m.err
}

func (m *fakeMutator) DeleteBlock(_ context.Context, workspaceID string, siteID string, pageID string, blockID string) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.updatePageID = pageID
	m.updateBlockID = blockID
	return m.blockDeleted, m.err
}

func (m *fakeMutator) DuplicateBlock(_ context.Context, workspaceID string, siteID string, pageID string, blockID string) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.updatePageID = pageID
	m.updateBlockID = blockID
	return m.blockDuplicated, m.err
}

func (m *fakeMutator) ReorderBlocks(_ context.Context, workspaceID string, siteID string, pageID string, blockIDs []string) (siteconfig.SiteDraft, error) {
	m.deleteWorkspaceID = workspaceID
	m.pageSiteID = siteID
	m.updatePageID = pageID
	m.reorderBlockIDs = blockIDs
	return m.blocksReordered, m.err
}

func (m *fakeMutator) DeleteSite(_ context.Context, workspaceID string, siteID string) error {
	m.deleteWorkspaceID = workspaceID
	m.deleteSiteID = siteID
	return m.err
}

func (p *fakePreviewTokens) Issue(_ context.Context, siteID string, userID string) (PreviewToken, error) {
	p.issuedSiteID = siteID
	p.issuedUserID = userID
	return p.issued, p.err
}

func (p *fakePreviewTokens) Revoke(_ context.Context, siteID string) error {
	p.revokedSiteID = siteID
	return p.err
}

func (p *fakePreviewTokens) LoadDraft(_ context.Context, _ string) (siteconfig.SiteDraft, error) {
	return p.draft, p.err
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

func TestListSitesReturnsEmptyArrayForWorkspaceWithoutSites(t *testing.T) {
	handler := Handler{
		reader:     fakeReader{},
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
	if payload.Sites == nil {
		t.Fatal("expected empty sites array, got nil")
	}
	if len(payload.Sites) != 0 {
		t.Fatalf("expected no sites, got %#v", payload.Sites)
	}
}

func TestGetSiteReturnsCanonicalDraft(t *testing.T) {
	handler := Handler{
		reader: fakeReader{
			draft: validHandlerDraft(),
			metadata: GenerationMetadata{
				Prompt:      "A compact site for a local studio.",
				ThemePreset: "calm-nordic",
				AssetsNeeded: []string{
					"hero-image",
				},
			},
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
		Draft         siteconfig.SiteDraft         `json:"draft"`
		Generation    GenerationMetadata           `json:"generation"`
		BlockRegistry []siteconfig.BlockDefinition `json:"blockRegistry"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Draft.Site.ID != "site_demo" {
		t.Fatalf("expected site draft, got %#v", payload.Draft.Site)
	}
	if len(payload.BlockRegistry) == 0 {
		t.Fatal("expected block registry in payload")
	}
	if payload.Generation.Prompt == "" || payload.Generation.ThemePreset == "" {
		t.Fatalf("expected generation metadata in payload, got %#v", payload.Generation)
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

func TestCreateSiteReturnsPlanLimitExceeded(t *testing.T) {
	handler := Handler{
		reader:     fakeReader{},
		mutator:    &fakeMutator{},
		authorizer: fakeAuthorizer{},
		billingDB: billingAccessStoreStub{
			entitlement: billing.Entitlement{
				WorkspaceID:      "workspace-1",
				Plan:             "site",
				Status:           "active",
				SubscriptionLive: true,
				ActiveSiteLimit:  intPtr(1),
			},
			activeSiteCount: 1,
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites", strings.NewReader(`{"name":"Nordic Studio"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.create(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
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

func TestIssuePreviewTokenReturnsCreatedToken(t *testing.T) {
	previews := &fakePreviewTokens{
		issued: PreviewToken{
			Token:     "preview-token",
			ExpiresAt: time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC),
		},
	}
	handler := Handler{
		authorizer: fakeAuthorizer{},
		previews:   previews,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/preview-token", nil)
	req.SetPathValue("siteId", "site_demo")
	req = req.WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.issuePreviewToken(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if previews.issuedSiteID != "site_demo" || previews.issuedUserID != "user-1" {
		t.Fatalf("expected preview token issue inputs to be captured, got %q %q", previews.issuedSiteID, previews.issuedUserID)
	}
	var payload previewTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Token != "preview-token" {
		t.Fatalf("expected preview token, got %q", payload.Token)
	}
}

func TestRevokePreviewTokenReturnsNoContent(t *testing.T) {
	previews := &fakePreviewTokens{}
	handler := Handler{
		authorizer: fakeAuthorizer{},
		previews:   previews,
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/sites/site_demo/preview-token", nil)
	req.SetPathValue("siteId", "site_demo")
	req = req.WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.revokePreviewToken(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
	if previews.revokedSiteID != "site_demo" {
		t.Fatalf("expected revoked site id, got %q", previews.revokedSiteID)
	}
}

func TestGetPreviewByTokenReturnsDraft(t *testing.T) {
	handler := Handler{
		previews: &fakePreviewTokens{
			draft: validHandlerDraft(),
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/public/preview/token-demo", nil)
	req.SetPathValue("token", "token-demo")
	res := httptest.NewRecorder()

	handler.getPreviewByToken(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	var payload struct {
		Draft siteconfig.SiteDraft `json:"draft"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Draft.Site.ID != validHandlerDraft().Site.ID {
		t.Fatalf("expected returned draft, got %#v", payload.Draft.Site)
	}
}

func TestGetPreviewByTokenReturnsNotFoundForExpiredToken(t *testing.T) {
	handler := Handler{
		previews: &fakePreviewTokens{
			err: ErrPreviewTokenNotFound,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/public/preview/token-demo", nil)
	req.SetPathValue("token", "token-demo")
	res := httptest.NewRecorder()

	handler.getPreviewByToken(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestWriteSiteErrorMapsPreviewTokenInvalid(t *testing.T) {
	res := httptest.NewRecorder()

	writeSiteError(res, ErrPreviewTokenInvalid)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestWriteSiteErrorFallsBackForUnexpectedPreviewError(t *testing.T) {
	res := httptest.NewRecorder()

	writeSiteError(res, errors.New("preview backend failed"))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
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

func TestUpdateBlockReturnsUpdatedDraft(t *testing.T) {
	mutator := &fakeMutator{blockUpdated: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/sites/site_demo/pages/page_home/blocks/block_hero", strings.NewReader(`{"props":{"headline":"Refined headline","layout":"centered"},"hidden":true}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("pageId", "page_home")
	req.SetPathValue("blockId", "block_hero")
	res := httptest.NewRecorder()

	handler.updateBlock(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if mutator.updatePageID != "page_home" || mutator.updateBlockID != "block_hero" {
		t.Fatalf("expected page and block ids to reach mutator, got %q and %q", mutator.updatePageID, mutator.updateBlockID)
	}
	if mutator.updateBlockInput.Props["headline"] != "Refined headline" {
		t.Fatalf("expected updated props to reach mutator, got %#v", mutator.updateBlockInput.Props)
	}
	if mutator.updateBlockInput.Hidden == nil || !*mutator.updateBlockInput.Hidden {
		t.Fatalf("expected hidden flag to reach mutator, got %#v", mutator.updateBlockInput.Hidden)
	}
}

func TestCreatePageReturnsCreatedDraft(t *testing.T) {
	mutator := &fakeMutator{pageCreated: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/pages", strings.NewReader(`{"title":"Contact","slug":"/contact","includeInNavigation":false}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.createPage(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if mutator.createPageInput.Title != "Contact" || mutator.createPageInput.Slug != "/contact" {
		t.Fatalf("expected page create input to reach mutator, got %#v", mutator.createPageInput)
	}
	if mutator.createPageInput.IncludeInNavigation == nil || *mutator.createPageInput.IncludeInNavigation {
		t.Fatalf("expected includeInNavigation false to reach mutator, got %#v", mutator.createPageInput.IncludeInNavigation)
	}
}

func TestUpdatePageReturnsUpdatedDraft(t *testing.T) {
	mutator := &fakeMutator{pageUpdated: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/sites/site_demo/pages/page_home", strings.NewReader(`{"title":"Landing","slug":"/","seo":{"title":"Landing","description":"Primary page"}}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("pageId", "page_home")
	res := httptest.NewRecorder()

	handler.updatePage(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if mutator.updatePageID != "page_home" || mutator.pageSiteID != "site_demo" {
		t.Fatalf("expected site and page ids to reach mutator, got %q and %q", mutator.pageSiteID, mutator.updatePageID)
	}
	if mutator.updatePageInput.Title == nil || *mutator.updatePageInput.Title != "Landing" {
		t.Fatalf("expected page title to reach mutator, got %#v", mutator.updatePageInput.Title)
	}
	if mutator.updatePageInput.SEO == nil || mutator.updatePageInput.SEO.Description != "Primary page" {
		t.Fatalf("expected page seo to reach mutator, got %#v", mutator.updatePageInput.SEO)
	}
}

func TestReorderPagesReturnsUpdatedDraft(t *testing.T) {
	mutator := &fakeMutator{pagesReordered: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/pages/reorder", strings.NewReader(`{"pageIds":["page_home"]}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.reorderPages(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if len(mutator.reorderPageIDs) != 1 || mutator.reorderPageIDs[0] != "page_home" {
		t.Fatalf("expected page reorder ids to reach mutator, got %#v", mutator.reorderPageIDs)
	}
}

func TestReorderNavigationReturnsUpdatedDraft(t *testing.T) {
	mutator := &fakeMutator{navigationReordered: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/navigation/reorder", strings.NewReader(`{"pageIds":["page_home"]}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.reorderNavigation(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if len(mutator.reorderNavigationPageIDs) != 1 || mutator.reorderNavigationPageIDs[0] != "page_home" {
		t.Fatalf("expected navigation reorder ids to reach mutator, got %#v", mutator.reorderNavigationPageIDs)
	}
}

func TestCreateBlockReturnsCreatedDraft(t *testing.T) {
	mutator := &fakeMutator{blockCreated: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/pages/page_home/blocks", strings.NewReader(`{"type":"cta_band","version":"1.0.0"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("pageId", "page_home")
	res := httptest.NewRecorder()

	handler.createBlock(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if mutator.createBlockInput.Type != "cta_band" || mutator.createBlockInput.Version != "1.0.0" {
		t.Fatalf("expected block create input to reach mutator, got %#v", mutator.createBlockInput)
	}
}

func TestDuplicateBlockReturnsCreatedDraft(t *testing.T) {
	mutator := &fakeMutator{blockDuplicated: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/pages/page_home/blocks/block_hero/duplicate", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("pageId", "page_home")
	req.SetPathValue("blockId", "block_hero")
	res := httptest.NewRecorder()

	handler.duplicateBlock(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if mutator.updatePageID != "page_home" || mutator.updateBlockID != "block_hero" {
		t.Fatalf("expected page and block ids to reach mutator, got %q and %q", mutator.updatePageID, mutator.updateBlockID)
	}
}

func TestReorderBlocksReturnsUpdatedDraft(t *testing.T) {
	mutator := &fakeMutator{blocksReordered: validHandlerDraft()}
	handler := Handler{
		reader:     fakeReader{},
		mutator:    mutator,
		authorizer: fakeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/pages/page_home/blocks/reorder", strings.NewReader(`{"blockIds":["block_hero"]}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("pageId", "page_home")
	res := httptest.NewRecorder()

	handler.reorderBlocks(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if len(mutator.reorderBlockIDs) != 1 || mutator.reorderBlockIDs[0] != "block_hero" {
		t.Fatalf("expected block reorder ids to reach mutator, got %#v", mutator.reorderBlockIDs)
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

type billingAccessStoreStub struct {
	entitlement     billing.Entitlement
	activeSiteCount int
	assetBytes      int64
	promptCount     int
	periodStart     *time.Time
	periodEnd       *time.Time
}

func (s billingAccessStoreStub) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "from billing_entitlements"):
		return rowStub{values: []any{
			s.entitlement.WorkspaceID,
			s.entitlement.Plan,
			s.entitlement.Status,
			s.entitlement.SubscriptionLive,
			s.entitlement.CustomDomainsEnabled,
			s.entitlement.ActiveSiteLimit,
			s.entitlement.MonthlyPromptLimit,
			s.entitlement.AssetStorageLimitBytes,
			s.entitlement.CollectionLimit,
			s.entitlement.CollectionEntryLimit,
			time.Now().UTC(),
		}}
	case strings.Contains(sql, "select count(*)") && strings.Contains(sql, "from sites"):
		return rowStub{values: []any{s.activeSiteCount}}
	case strings.Contains(sql, "from assets"):
		return rowStub{values: []any{s.assetBytes}}
	case strings.Contains(sql, "from billing_subscriptions"):
		if s.periodStart == nil || s.periodEnd == nil {
			return rowStub{err: pgx.ErrNoRows}
		}
		return rowStub{values: []any{s.periodStart, s.periodEnd}}
	case strings.Contains(sql, "from generation_jobs"):
		return rowStub{values: []any{s.promptCount}}
	default:
		return rowStub{err: pgx.ErrNoRows}
	}
}

type rowStub struct {
	values []any
	err    error
}

func (r rowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index := range dest {
		switch target := dest[index].(type) {
		case *string:
			*target = r.values[index].(string)
		case *bool:
			*target = r.values[index].(bool)
		case *int:
			*target = r.values[index].(int)
		case *int64:
			*target = r.values[index].(int64)
		case **int:
			*target = r.values[index].(*int)
		case **int64:
			*target = r.values[index].(*int64)
		case *time.Time:
			*target = r.values[index].(time.Time)
		case **time.Time:
			*target = r.values[index].(*time.Time)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func intPtr(value int) *int {
	return &value
}
