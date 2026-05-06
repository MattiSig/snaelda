package publishing

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
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type fakePublisher struct {
	publishResult   PublishResult
	publishedResult PublishedSiteResult
	versions        []VersionSummary
	publishInput    PublishInput
	publishSiteID   string
	publishUserID   string
	publishedSlug   string
	versionSiteID   string
	err             error
}

func (f *fakePublisher) Publish(_ context.Context, siteID string, userID string, input PublishInput) (PublishResult, error) {
	f.publishSiteID = siteID
	f.publishUserID = userID
	f.publishInput = input
	return f.publishResult, f.err
}

func (f *fakePublisher) ListVersions(_ context.Context, siteID string) ([]VersionSummary, error) {
	f.versionSiteID = siteID
	return f.versions, f.err
}

func (f *fakePublisher) LoadPublishedSiteBySlug(_ context.Context, siteSlug string) (PublishedSiteResult, error) {
	f.publishedSlug = siteSlug
	return f.publishedResult, f.err
}

type fakePublishAuthorizer struct{}

func (fakePublishAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site_demo", Role: authorization.RoleOwner}, nil
}

func TestPublishReturnsVersionAndPublicURL(t *testing.T) {
	publisher := &fakePublisher{
		publishResult: PublishResult{
			Version: VersionSummary{
				ID:            "version-1",
				SiteID:        "site_demo",
				VersionNumber: 1,
				CreatedAt:     time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
				IsCurrent:     true,
			},
			SiteSlug: "nordic-studio",
			Hostname: "nordic-studio.localhost",
			Snapshot: validSnapshot(),
		},
	}
	handler := Handler{
		service:    publisher,
		authorizer: fakePublishAuthorizer{},
		appBaseURL: "http://localhost:3000",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/publish", strings.NewReader(`{"publishNote":"First public draft"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.publish(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.publishSiteID != "site_demo" || publisher.publishUserID != "user-1" {
		t.Fatalf("expected publish input to reach service, got %q and %q", publisher.publishSiteID, publisher.publishUserID)
	}
	if publisher.publishInput.PublishNote != "First public draft" {
		t.Fatalf("expected publish note to reach service, got %q", publisher.publishInput.PublishNote)
	}

	var payload struct {
		Version   VersionSummary               `json:"version"`
		PublicURL string                       `json:"publicUrl"`
		Snapshot  siteconfig.PublishedSnapshot `json:"snapshot"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Version.ID != "version-1" {
		t.Fatalf("expected version in response, got %#v", payload.Version)
	}
	if payload.PublicURL != "http://localhost:3000/public/nordic-studio" {
		t.Fatalf("expected public url, got %q", payload.PublicURL)
	}
}

func TestListVersionsReturnsPublishHistory(t *testing.T) {
	publisher := &fakePublisher{
		versions: []VersionSummary{{ID: "version-2", SiteID: "site_demo", VersionNumber: 2, IsCurrent: true}},
	}
	handler := Handler{
		service:    publisher,
		authorizer: fakePublishAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sites/site_demo/versions", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.listVersions(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.versionSiteID != "site_demo" {
		t.Fatalf("expected site id to reach version service, got %q", publisher.versionSiteID)
	}
}

func TestGetPublishedSiteReturnsSnapshotWithoutAuth(t *testing.T) {
	publisher := &fakePublisher{
		publishedResult: PublishedSiteResult{
			SiteSlug: "nordic-studio",
			Hostname: "nordic-studio.localhost",
			Version: VersionSummary{
				ID:            "version-2",
				SiteID:        "site_demo",
				VersionNumber: 2,
				IsCurrent:     true,
			},
			Snapshot: validSnapshot(),
		},
	}
	handler := Handler{
		service:    publisher,
		authorizer: fakePublishAuthorizer{},
		appBaseURL: "http://localhost:3000",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/public/sites/nordic-studio", nil)
	req.SetPathValue("siteSlug", "nordic-studio")
	res := httptest.NewRecorder()

	handler.getPublishedSite(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.publishedSlug != "nordic-studio" {
		t.Fatalf("expected slug to reach loader, got %q", publisher.publishedSlug)
	}
}

func TestPublishValidationErrorsReturnBadRequest(t *testing.T) {
	publisher := &fakePublisher{
		err: siteconfig.ValidationError{Issues: []siteconfig.Issue{{Code: "required"}}},
	}
	handler := Handler{
		service:    publisher,
		authorizer: fakePublishAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/publish", strings.NewReader(`{}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	res := httptest.NewRecorder()

	handler.publish(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestPublishedSiteNotFoundReturnsNotFound(t *testing.T) {
	publisher := &fakePublisher{err: ErrNotFound}
	handler := Handler{service: publisher}

	req := httptest.NewRequest(http.MethodGet, "/api/public/sites/missing", nil)
	req.SetPathValue("siteSlug", "missing")
	res := httptest.NewRecorder()

	handler.getPublishedSite(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func validSnapshot() siteconfig.PublishedSnapshot {
	return siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site: siteconfig.PublishedSite{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			DefaultLocale: "en",
			SEO: siteconfig.SEOConfig{
				Title:       "Nordic Studio",
				Description: "Calm design systems for focused teams.",
			},
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#151215",
					"foreground": "#f6f2ec",
					"primary":    "#8fc6ff",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
		},
		Pages: []siteconfig.PageDraft{{
			ID:    "page_home",
			Title: "Home",
			Slug:  "/",
			SEO: siteconfig.SEOConfig{
				Title:       "Nordic Studio",
				Description: "Calm design systems for focused teams.",
			},
			Blocks: []siteconfig.BlockInstance{{
				ID:      "block_hero",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"headline": "Clear websites for focused teams",
					"layout":   "centered",
				},
			}},
		}},
	}
}

func TestWritePublishErrorFallsBackToInternalServerError(t *testing.T) {
	recorder := httptest.NewRecorder()
	writePublishError(recorder, errors.New("boom"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}
