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
	rollbackResult  RollbackResult
	publishedResult PublishedSiteResult
	versions        []VersionSummary
	publishInput    PublishInput
	publishSiteID   string
	publishUserID   string
	rollbackSiteID  string
	rollbackUserID  string
	rollbackVersion string
	publishedSlug   string
	publishedHost   string
	publishedPath   string
	artifactPath    string
	versionSiteID   string
	err             error
}

func (f *fakePublisher) Publish(_ context.Context, siteID string, userID string, input PublishInput) (PublishResult, error) {
	f.publishSiteID = siteID
	f.publishUserID = userID
	f.publishInput = input
	return f.publishResult, f.err
}

func (f *fakePublisher) Rollback(_ context.Context, siteID string, versionID string, userID string) (RollbackResult, error) {
	f.rollbackSiteID = siteID
	f.rollbackVersion = versionID
	f.rollbackUserID = userID
	return f.rollbackResult, f.err
}

func (f *fakePublisher) ListVersions(_ context.Context, siteID string) ([]VersionSummary, error) {
	f.versionSiteID = siteID
	return f.versions, f.err
}

func (f *fakePublisher) LoadPublishedSiteBySlug(_ context.Context, siteSlug string, pagePath string) (PublishedSiteResult, error) {
	f.publishedSlug = siteSlug
	f.publishedPath = pagePath
	return f.publishedResult, f.err
}

func (f *fakePublisher) LoadPublishedSiteByHostname(_ context.Context, hostname string, pagePath string) (PublishedSiteResult, error) {
	f.publishedHost = hostname
	f.publishedPath = pagePath
	return f.publishedResult, f.err
}

func (f *fakePublisher) LoadPublishedArtifactBySlug(_ context.Context, siteSlug string, artifactPath string) (PublishedArtifactResult, error) {
	f.publishedSlug = siteSlug
	f.artifactPath = artifactPath
	return PublishedArtifactResult{
		SiteSlug: siteSlug,
		File: ArtifactFile{
			Path:        artifactPath,
			ContentType: "text/plain; charset=utf-8",
			Body:        "artifact body",
		},
	}, f.err
}

func (f *fakePublisher) LoadPublishedArtifactByHostname(_ context.Context, hostname string, artifactPath string) (PublishedArtifactResult, error) {
	f.publishedHost = hostname
	f.artifactPath = artifactPath
	return PublishedArtifactResult{
		Hostname: hostname,
		File: ArtifactFile{
			Path:        artifactPath,
			ContentType: "text/plain; charset=utf-8",
			Body:        "artifact body",
		},
	}, f.err
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
		service:       publisher,
		authorizer:    fakePublishAuthorizer{},
		appBaseURL:    "http://localhost:3000",
		publicBaseURL: "http://localhost:3000",
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
	if payload.PublicURL != "http://nordic-studio.localhost:3000/" {
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

func TestRollbackReturnsVersionAndPublicURL(t *testing.T) {
	publisher := &fakePublisher{
		rollbackResult: RollbackResult{
			Version: VersionSummary{
				ID:            "version-1",
				SiteID:        "site_demo",
				VersionNumber: 1,
				CreatedAt:     time.Date(2026, 5, 7, 9, 30, 0, 0, time.UTC),
				IsCurrent:     true,
			},
			SiteSlug: "nordic-studio",
			Hostname: "nordic-studio.localhost",
		},
	}
	handler := Handler{
		service:       publisher,
		authorizer:    fakePublishAuthorizer{},
		appBaseURL:    "http://localhost:3000",
		publicBaseURL: "http://localhost:3000",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/rollback/version-1", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("versionId", "version-1")
	res := httptest.NewRecorder()

	handler.rollback(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.rollbackSiteID != "site_demo" || publisher.rollbackVersion != "version-1" || publisher.rollbackUserID != "user-1" {
		t.Fatalf("expected rollback inputs to reach service, got site=%q version=%q user=%q", publisher.rollbackSiteID, publisher.rollbackVersion, publisher.rollbackUserID)
	}

	var payload struct {
		Version   VersionSummary `json:"version"`
		PublicURL string         `json:"publicUrl"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Version.ID != "version-1" {
		t.Fatalf("expected rolled back version in response, got %#v", payload.Version)
	}
	if payload.PublicURL != "http://nordic-studio.localhost:3000/" {
		t.Fatalf("expected public url, got %q", payload.PublicURL)
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
			PagePath: "/contact",
			Page: PublishedPageArtifact{
				PagePath:     "/contact",
				Title:        "Contact | Nordic Studio",
				Description:  "Send a note to plan your next launch.",
				CanonicalURL: "http://nordic-studio.localhost:3000/contact",
				HTML:         "<div>Contact</div>",
			},
		},
	}
	handler := Handler{
		service:       publisher,
		authorizer:    fakePublishAuthorizer{},
		appBaseURL:    "http://localhost:3000",
		publicBaseURL: "http://localhost:3000",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/public/sites/nordic-studio?path=/contact", nil)
	req.SetPathValue("siteSlug", "nordic-studio")
	res := httptest.NewRecorder()

	handler.getPublishedSite(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.publishedSlug != "nordic-studio" {
		t.Fatalf("expected slug to reach loader, got %q", publisher.publishedSlug)
	}
	if publisher.publishedPath != "/contact" {
		t.Fatalf("expected page path to reach loader, got %q", publisher.publishedPath)
	}

	var payload struct {
		PublicURL string `json:"publicUrl"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.PublicURL != "http://nordic-studio.localhost:3000/contact" {
		t.Fatalf("expected hosted public url, got %q", payload.PublicURL)
	}
}

func TestGetPublishedSiteByHostnameReturnsSnapshotWithoutAuth(t *testing.T) {
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
			PagePath: "/contact",
			Page: PublishedPageArtifact{
				PagePath:     "/contact",
				Title:        "Contact | Nordic Studio",
				Description:  "Send a note to plan your next launch.",
				CanonicalURL: "http://nordic-studio.localhost:3000/contact",
				HTML:         "<div>Contact</div>",
			},
		},
	}
	handler := Handler{
		service:       publisher,
		authorizer:    fakePublishAuthorizer{},
		appBaseURL:    "http://localhost:3000",
		publicBaseURL: "http://localhost:3000",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/public/render?hostname=nordic-studio.localhost:3000&path=/contact", nil)
	res := httptest.NewRecorder()

	handler.getPublishedSiteByHostname(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.publishedHost != "nordic-studio.localhost" {
		t.Fatalf("expected normalized hostname to reach loader, got %q", publisher.publishedHost)
	}
	if publisher.publishedPath != "/contact" {
		t.Fatalf("expected page path to reach loader, got %q", publisher.publishedPath)
	}

	var payload struct {
		PublicURL string `json:"publicUrl"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.PublicURL != "http://nordic-studio.localhost:3000/contact" {
		t.Fatalf("expected hosted public url, got %q", payload.PublicURL)
	}
}

func TestGetPublishedArtifactByHostnameReturnsStoredArtifact(t *testing.T) {
	publisher := &fakePublisher{}
	handler := Handler{service: publisher}

	req := httptest.NewRequest(http.MethodGet, "/api/public/artifact?hostname=nordic-studio.localhost&path=robots.txt", nil)
	res := httptest.NewRecorder()

	handler.getPublishedArtifact(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.publishedHost != "nordic-studio.localhost" {
		t.Fatalf("expected hostname to reach loader, got %q", publisher.publishedHost)
	}
	if publisher.artifactPath != "robots.txt" {
		t.Fatalf("expected artifact path to reach loader, got %q", publisher.artifactPath)
	}
	if body := strings.TrimSpace(res.Body.String()); body != "artifact body" {
		t.Fatalf("expected artifact body, got %q", body)
	}
}

func TestGetPublishedArtifactSetsETagAndCacheControl(t *testing.T) {
	publisher := &fakePublisher{}
	handler := Handler{service: publisher}

	req := httptest.NewRequest(http.MethodGet, "/api/public/artifact?siteSlug=nordic-studio&path=robots.txt", nil)
	res := httptest.NewRecorder()

	handler.getPublishedArtifact(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	etag := res.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on artifact response")
	}
	cacheControl := res.Header().Get("Cache-Control")
	if cacheControl == "" || strings.Contains(cacheControl, "no-store") {
		t.Fatalf("expected cacheable Cache-Control, got %q", cacheControl)
	}

	// If-None-Match should produce a 304 response with the same ETag.
	req304 := httptest.NewRequest(http.MethodGet, "/api/public/artifact?siteSlug=nordic-studio&path=robots.txt", nil)
	req304.Header.Set("If-None-Match", etag)
	res304 := httptest.NewRecorder()
	handler.getPublishedArtifact(res304, req304)
	if res304.Code != http.StatusNotModified {
		t.Fatalf("expected 304 on If-None-Match, got %d", res304.Code)
	}
	if res304.Body.Len() > 0 {
		t.Fatalf("expected empty body on 304, got %d bytes", res304.Body.Len())
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

func TestRollbackVersionNotFoundReturnsNotFound(t *testing.T) {
	publisher := &fakePublisher{err: ErrVersionNotFound}
	handler := Handler{
		service:    publisher,
		authorizer: fakePublishAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/rollback/version-missing", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site_demo")
	req.SetPathValue("versionId", "version-missing")
	res := httptest.NewRecorder()

	handler.rollback(res, req)

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

func validSnapshotWithContact() siteconfig.PublishedSnapshot {
	snapshot := validSnapshot()
	snapshot.Navigation.Primary = append(snapshot.Navigation.Primary, siteconfig.NavigationItem{
		Label:  "Contact",
		PageID: "page_contact",
	})
	snapshot.Pages = append(snapshot.Pages, siteconfig.PageDraft{
		ID:    "page_contact",
		Title: "Contact",
		Slug:  "/contact",
		SEO: siteconfig.SEOConfig{
			Title:       "Contact | Nordic Studio",
			Description: "Start a new project conversation.",
		},
		Blocks: []siteconfig.BlockInstance{{
			ID:      "block_text_contact",
			Type:    "text_section",
			Version: siteconfig.BlockVersionV1,
			Props: map[string]any{
				"heading": "Say hello",
				"body":    "Send a note with your launch timeline.",
			},
		}},
	})
	return snapshot
}

type fakeRespinGate struct {
	originated bool
	err        error
	gotSiteID  string
}

func (f *fakeRespinGate) IsRespinOriginatedSite(_ context.Context, siteID string) (bool, error) {
	f.gotSiteID = siteID
	return f.originated, f.err
}

func respinGatePublisher() *fakePublisher {
	return &fakePublisher{
		publishResult: PublishResult{
			Version:  VersionSummary{ID: "version-1", SiteID: "site_demo", VersionNumber: 1, IsCurrent: true},
			SiteSlug: "nordic-studio",
			Hostname: "nordic-studio.localhost",
			Snapshot: validSnapshot(),
		},
	}
}

func respinPublishRequest(ctx context.Context) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPost, "/api/sites/site_demo/publish", strings.NewReader(`{}`)).WithContext(ctx)
	req.SetPathValue("siteId", "site_demo")
	return req, httptest.NewRecorder()
}

func TestPublishRespinDraftWithoutClaimedIdentityIsGated(t *testing.T) {
	gate := &fakeRespinGate{originated: true}
	publisher := respinGatePublisher()
	handler := Handler{service: publisher, authorizer: fakePublishAuthorizer{}, respinGate: gate}

	// An unclaimed trial session (L0/L1): no user, no claim.
	ctx := auth.WithSession(context.Background(), auth.Session{
		Kind:        auth.SessionKindTrial,
		WorkspaceID: "workspace-1",
	})
	req, res := respinPublishRequest(ctx)

	handler.publish(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
	if gate.gotSiteID != "site_demo" {
		t.Fatalf("expected gate to receive site id, got %q", gate.gotSiteID)
	}
	if publisher.publishSiteID != "" {
		t.Fatalf("expected publish to be blocked before reaching the service")
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "identity_required" {
		t.Fatalf("expected identity_required error, got %q", payload.Error.Code)
	}
}

func TestPublishRespinDraftWithAuthenticatedIdentityProceeds(t *testing.T) {
	gate := &fakeRespinGate{originated: true}
	publisher := respinGatePublisher()
	handler := Handler{service: publisher, authorizer: fakePublishAuthorizer{}, appBaseURL: "http://localhost:3000", publicBaseURL: "http://localhost:3000", respinGate: gate}

	ctx := auth.WithSession(context.Background(), auth.Session{
		Kind:        auth.SessionKindAuthenticated,
		WorkspaceID: "workspace-1",
		User:        &auth.User{ID: "user-1", Email: "owner@snaelda.local", WorkspaceID: "workspace-1", WorkspaceRole: "owner"},
	})
	req, res := respinPublishRequest(ctx)

	handler.publish(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if publisher.publishSiteID != "site_demo" {
		t.Fatalf("expected publish to reach the service, got %q", publisher.publishSiteID)
	}
}

func TestPublishRespinDraftWithClaimedTrialProceeds(t *testing.T) {
	gate := &fakeRespinGate{originated: true}
	publisher := respinGatePublisher()
	handler := Handler{service: publisher, authorizer: fakePublishAuthorizer{}, appBaseURL: "http://localhost:3000", publicBaseURL: "http://localhost:3000", respinGate: gate}

	// A claimed trial session: an email was added (L2), so ClaimedByUserID is set.
	ctx := auth.WithSession(context.Background(), auth.Session{
		Kind:            auth.SessionKindTrial,
		WorkspaceID:     "workspace-1",
		ClaimedByUserID: "user-1",
		User:            &auth.User{ID: "user-1", Email: "owner@snaelda.local", WorkspaceID: "workspace-1", WorkspaceRole: "owner"},
	})
	req, res := respinPublishRequest(ctx)

	handler.publish(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

func TestPublishNonRespinDraftIsNotGated(t *testing.T) {
	gate := &fakeRespinGate{originated: false}
	publisher := respinGatePublisher()
	handler := Handler{service: publisher, authorizer: fakePublishAuthorizer{}, appBaseURL: "http://localhost:3000", publicBaseURL: "http://localhost:3000", respinGate: gate}

	// Unclaimed trial, but a prompt-originated draft: the gate must not block it.
	ctx := auth.WithSession(context.Background(), auth.Session{
		Kind:        auth.SessionKindTrial,
		WorkspaceID: "workspace-1",
	})
	req, res := respinPublishRequest(ctx)

	handler.publish(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

func TestPublishRespinGateErrorReturnsInternalServerError(t *testing.T) {
	gate := &fakeRespinGate{err: errors.New("db down")}
	publisher := respinGatePublisher()
	handler := Handler{service: publisher, authorizer: fakePublishAuthorizer{}, respinGate: gate}

	ctx := auth.WithSession(context.Background(), auth.Session{
		Kind:        auth.SessionKindAuthenticated,
		WorkspaceID: "workspace-1",
		User:        &auth.User{ID: "user-1", Email: "owner@snaelda.local", WorkspaceID: "workspace-1", WorkspaceRole: "owner"},
	})
	req, res := respinPublishRequest(ctx)

	handler.publish(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if publisher.publishSiteID != "" {
		t.Fatalf("expected publish to be blocked when the gate errors")
	}
}

func TestWritePublishErrorFallsBackToInternalServerError(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/test/publish", nil)
	handler.writePublishError(recorder, req, errors.New("boom"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}
