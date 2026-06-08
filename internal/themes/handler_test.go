package themes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/generation"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type stubThemeService struct {
	state     ThemeState
	updateIn  UpdateInput
	workspace string
	siteID    string
	err       error
	regenCall bool
}

func (s *stubThemeService) Load(context.Context, string) (ThemeState, error) {
	return s.state, s.err
}

func (s *stubThemeService) Update(_ context.Context, workspaceID string, siteID string, input UpdateInput) (ThemeState, error) {
	s.workspace = workspaceID
	s.siteID = siteID
	s.updateIn = input
	return s.state, s.err
}

func (s *stubThemeService) Regenerate(_ context.Context, workspaceID string, siteID string) (ThemeState, error) {
	s.workspace = workspaceID
	s.siteID = siteID
	s.regenCall = true
	return s.state, s.err
}

func (s *stubThemeService) RegenerateWithProgress(_ context.Context, workspaceID string, siteID string, sink generation.ProgressSink) (ThemeState, error) {
	s.workspace = workspaceID
	s.siteID = siteID
	s.regenCall = true
	if sink != nil {
		sink.OnJobCreated("job-1")
		sink.OnProgress(generation.ProgressStep{
			Name:  "plan.theme",
			Label: "Picking colors and typography",
			Index: 2,
			Total: 4,
		})
	}
	return s.state, s.err
}

type stubThemeAuthorizer struct{}

func (stubThemeAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site_demo", Role: authorization.RoleOwner}, nil
}

func TestGetThemeReturnsThemeState(t *testing.T) {
	handler := Handler{
		service: &stubThemeService{
			state: ThemeState{
				Theme:     siteconfig.ThemePreset(siteconfig.ThemePaletteAfterHours),
				Selection: siteconfig.DefaultThemeSelection(),
				Options:   siteconfig.DefaultThemeEditorCatalog(),
			},
		},
		authorizer: stubThemeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sites/site_demo/theme", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: authorization.RoleOwner,
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.get(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	var payload ThemeState
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Selection.Palette != siteconfig.ThemePaletteCleanLocal {
		t.Fatalf("expected theme selection in response, got %#v", payload.Selection)
	}
}

func TestUpdateThemePassesTrimmedSelection(t *testing.T) {
	service := &stubThemeService{
		state: ThemeState{
			Theme:     siteconfig.ThemePreset(siteconfig.ThemePaletteBrightShopfront),
			Selection: siteconfig.DetectThemeSelection(siteconfig.ThemePreset(siteconfig.ThemePaletteBrightShopfront)),
			Options:   siteconfig.DefaultThemeEditorCatalog(),
		},
	}
	handler := Handler{
		service:    service,
		authorizer: stubThemeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/sites/site_demo/theme", strings.NewReader(`{"palette":" bright-shopfront ","fontPreset":" studio-sans ","typeScale":" expressive ","sectionSpacing":" snug ","contentWidth":" wide ","radius":" pillowy ","buttonStyle":" ink-solid ","imageStyle":" paper-cut "}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: authorization.RoleOwner,
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.update(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if service.workspace != "workspace-1" || service.siteID != "site_demo" {
		t.Fatalf("expected scoped update call, got workspace=%q site=%q", service.workspace, service.siteID)
	}
	if service.updateIn.Palette == nil || *service.updateIn.Palette != siteconfig.ThemePaletteBrightShopfront {
		t.Fatalf("expected trimmed palette, got %#v", service.updateIn.Palette)
	}
	if service.updateIn.ButtonStyle == nil || *service.updateIn.ButtonStyle != siteconfig.ThemeButtonInkSolid {
		t.Fatalf("expected trimmed button style, got %#v", service.updateIn.ButtonStyle)
	}
	if service.updateIn.TypeScale == nil || *service.updateIn.TypeScale != siteconfig.ThemeTypeScaleExpressive {
		t.Fatalf("expected trimmed type scale, got %#v", service.updateIn.TypeScale)
	}
	if service.updateIn.ContentWidth == nil || *service.updateIn.ContentWidth != siteconfig.ThemeContentWidthWide {
		t.Fatalf("expected trimmed content width, got %#v", service.updateIn.ContentWidth)
	}
	if service.updateIn.ImageStyle == nil || *service.updateIn.ImageStyle != siteconfig.ThemeImagePaperCut {
		t.Fatalf("expected trimmed image style, got %#v", service.updateIn.ImageStyle)
	}
}

func TestRegenerateThemeUsesScopedWorkspace(t *testing.T) {
	service := &stubThemeService{
		state: ThemeState{
			Theme:     siteconfig.ThemePreset(siteconfig.ThemePaletteBrightShopfront),
			Selection: siteconfig.DetectThemeSelection(siteconfig.ThemePreset(siteconfig.ThemePaletteBrightShopfront)),
			Options:   siteconfig.DefaultThemeEditorCatalog(),
		},
	}
	handler := Handler{
		service:    service,
		authorizer: stubThemeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/theme/regenerate", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: authorization.RoleOwner,
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.regenerate(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if !service.regenCall || service.workspace != "workspace-1" || service.siteID != "site_demo" {
		t.Fatalf("expected scoped regenerate call, got regen=%t workspace=%q site=%q", service.regenCall, service.workspace, service.siteID)
	}
}

func TestRegenerateThemeStreamsProgressEvents(t *testing.T) {
	service := &stubThemeService{
		state: ThemeState{
			Theme:     siteconfig.ThemePreset(siteconfig.ThemePaletteBrightShopfront),
			Selection: siteconfig.DetectThemeSelection(siteconfig.ThemePreset(siteconfig.ThemePaletteBrightShopfront)),
			Options:   siteconfig.DefaultThemeEditorCatalog(),
		},
	}
	handler := Handler{
		service:    service,
		authorizer: stubThemeAuthorizer{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/theme/regenerate", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: authorization.RoleOwner,
	}))
	req.Header.Set("Accept", "text/event-stream")
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.regenerate(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "event: job") {
		t.Fatalf("expected job event in stream, got %q", body)
	}
	if !strings.Contains(body, "event: progress") {
		t.Fatalf("expected progress event in stream, got %q", body)
	}
	if !strings.Contains(body, "event: complete") {
		t.Fatalf("expected complete event in stream, got %q", body)
	}
}
