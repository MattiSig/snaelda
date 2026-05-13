package generation

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

type fakeGenerator struct {
	input         GenerateInput
	siteReprompt  RepromptInput
	pageReprompt  RepromptInput
	siteID        string
	pageID        string
	result        GenerateResult
	undoResult    siteconfig.SiteDraft
	err           error
}

func (g *fakeGenerator) Generate(_ context.Context, _ string, _ string, input GenerateInput) (GenerateResult, error) {
	g.input = input
	return g.result, g.err
}

func (g *fakeGenerator) RepromptSite(_ context.Context, _ string, _ string, siteID string, input RepromptInput) (GenerateResult, error) {
	g.siteID = siteID
	g.siteReprompt = input
	return g.result, g.err
}

func (g *fakeGenerator) RepromptPage(_ context.Context, _ string, _ string, siteID string, pageID string, input RepromptInput) (GenerateResult, error) {
	g.siteID = siteID
	g.pageID = pageID
	g.pageReprompt = input
	return g.result, g.err
}

func (g *fakeGenerator) UndoLastDraftRevision(_ context.Context, _ string, _ string) (siteconfig.SiteDraft, error) {
	return g.undoResult, g.err
}

type fakeWorkspaceAuthorizer struct{}

func (fakeWorkspaceAuthorizer) RequireWorkspaceMember(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", Role: authorization.RoleOwner}, nil
}

func (fakeWorkspaceAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site-1", Role: authorization.RoleOwner}, nil
}

func TestGenerateReturnsCreatedDraft(t *testing.T) {
	service := &fakeGenerator{
		result: GenerateResult{
			JobID: "job-1",
			Draft: siteconfig.SiteDraft{
				Site: siteconfig.DraftSite{
					ID:            "site-1",
					Name:          "North Light Studio",
					Slug:          "north-light-studio",
					Status:        "draft",
					DefaultLocale: "en",
				},
				Theme: siteconfig.ThemeConfig{
					Version: siteconfig.ThemeVersionV1,
					Tokens: siteconfig.ThemeTokens{
						Colors: map[string]string{
							"background": "#f6f2ec",
							"foreground": "#2b2324",
							"primary":    "#356fbd",
						},
					},
				},
				Navigation: siteconfig.NavigationConfig{
					Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-home"}},
				},
				Pages: []siteconfig.PageDraft{
					{
						ID:    "page-home",
						Title: "Home",
						Slug:  "/",
						Blocks: []siteconfig.BlockInstance{
							{
								ID:      "block-hero",
								Type:    "hero",
								Version: siteconfig.BlockVersionV1,
								Props: map[string]any{
									"headline": "Natural photography for real people, places, and moments",
									"layout":   "split-left",
								},
							},
						},
					},
				},
			},
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/generate", strings.NewReader(`{"name":"North Light Studio","prompt":"A calm portfolio site for a photography studio."}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.generate(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if service.input.Prompt != "A calm portfolio site for a photography studio." {
		t.Fatalf("expected prompt to reach service, got %#v", service.input)
	}
	var payload GenerateResult
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.JobID != "job-1" {
		t.Fatalf("expected job id in payload, got %#v", payload)
	}
}

func TestGenerateReturnsValidationProblem(t *testing.T) {
	service := &fakeGenerator{
		err: siteconfig.ValidationError{
			Issues: []siteconfig.Issue{
				{Path: "pages[0].blocks[0].props.headline", Code: "required", Message: "value is required"},
			},
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/generate", strings.NewReader(`{"prompt":"test"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.generate(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestRepromptSiteReturnsCreatedDraft(t *testing.T) {
	service := &fakeGenerator{
		result: GenerateResult{
			JobID: "job-2",
			Draft: validGenerationDraft(),
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site-1/reprompt", strings.NewReader(`{"prompt":"Make it warmer and add pricing."}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	res := httptest.NewRecorder()

	handler.repromptSite(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if service.siteID != "site-1" || service.siteReprompt.Prompt != "Make it warmer and add pricing." {
		t.Fatalf("expected site reprompt input to reach service, got %#v", service)
	}
}

func TestRepromptPageReturnsCreatedDraft(t *testing.T) {
	service := &fakeGenerator{
		result: GenerateResult{
			JobID: "job-3",
			Draft: validGenerationDraft(),
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site-1/pages/page-1/reprompt", strings.NewReader(`{"prompt":"Turn this into a pricing overview."}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("pageId", "page-1")
	res := httptest.NewRecorder()

	handler.repromptPage(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
	if service.pageID != "page-1" || service.pageReprompt.Prompt != "Turn this into a pricing overview." {
		t.Fatalf("expected page reprompt input to reach service, got %#v", service)
	}
}

func TestUndoSiteReturnsRestoredDraft(t *testing.T) {
	service := &fakeGenerator{
		undoResult: validGenerationDraft(),
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site-1/undo", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	res := httptest.NewRecorder()

	handler.undoSite(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

func validGenerationDraft() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            "site-1",
			Name:          "North Light Studio",
			Slug:          "north-light-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#f6f2ec",
					"foreground": "#2b2324",
					"primary":    "#356fbd",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-home"}},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page-home",
				Title: "Home",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block-hero",
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"headline": "Natural photography for real people, places, and moments",
							"layout":   "split-left",
						},
					},
				},
			},
		},
	}
}
