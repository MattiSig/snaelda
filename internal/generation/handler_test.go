package generation

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

type fakeGenerator struct {
	input            GenerateInput
	siteReprompt     RepromptInput
	pageReprompt     RepromptInput
	blockSuggest     BlockSuggestInput
	imageSuggest     ImageSuggestInput
	imageApply       ImageApplyInput
	imageResult      ImageSuggestResult
	imageApplyResult ImageApplyResult
	siteID           string
	pageID           string
	blockID          string
	revisionID       string
	repromptID       string
	result           GenerateResult
	undoResult       siteconfig.SiteDraft
	repromptHistory  []RepromptHistoryEntry
	revision         DraftRevision
	err              error
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

func (g *fakeGenerator) SuggestBlock(_ context.Context, _ string, _ string, siteID string, blockID string, input BlockSuggestInput) (GenerateResult, error) {
	g.siteID = siteID
	g.blockID = blockID
	g.blockSuggest = input
	return g.result, g.err
}

func (g *fakeGenerator) SuggestImage(_ context.Context, _ string, siteID string, blockID string, input ImageSuggestInput) (ImageSuggestResult, error) {
	g.siteID = siteID
	g.blockID = blockID
	g.imageSuggest = input
	return g.imageResult, g.err
}

func (g *fakeGenerator) ApplyImageSuggestion(_ context.Context, _ string, _ string, siteID string, blockID string, input ImageApplyInput) (ImageApplyResult, error) {
	g.siteID = siteID
	g.blockID = blockID
	g.imageApply = input
	return g.imageApplyResult, g.err
}

func (g *fakeGenerator) ListRepromptHistory(_ context.Context, _ string, _ string) ([]RepromptHistoryEntry, error) {
	return g.repromptHistory, g.err
}

func (g *fakeGenerator) LoadDraftRevision(_ context.Context, _ string, _ string, revisionID string) (DraftRevision, error) {
	g.revisionID = revisionID
	return g.revision, g.err
}

func (g *fakeGenerator) RevertReprompt(_ context.Context, _ string, _ string, repromptID string) (siteconfig.SiteDraft, error) {
	g.repromptID = repromptID
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

func TestGenerateReturnsPlanLimitExceeded(t *testing.T) {
	service := &fakeGenerator{}
	handler := Handler{
		billingDB: billingAccessStoreStub{
			entitlement: billing.Entitlement{
				WorkspaceID:        "workspace-1",
				Plan:               "basic",
				Status:             "active",
				SubscriptionLive:   true,
				MonthlyPromptLimit: intPtr(1),
			},
			periodStart: timePtr(time.Now().UTC().Add(-time.Hour)),
			periodEnd:   timePtr(time.Now().UTC().Add(time.Hour)),
			promptCount: 1,
		},
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

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestGenerateRejectsOverlongPrompt(t *testing.T) {
	service := &fakeGenerator{}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/generate", strings.NewReader(`{"prompt":"`+strings.Repeat("a", maxGenerationPromptCharacters+1)+`"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
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

func TestGenerateReturnsRateLimitedWhenBurstIsExhausted(t *testing.T) {
	service := &fakeGenerator{}
	store := newFakeGenerationAttemptStore()
	now := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	for attempt := 0; attempt < 6; attempt++ {
		store.attempts = append(store.attempts, generationAttempt{
			workspaceID: "workspace-1",
			userID:      "user-1",
			scope:       "site",
			attemptedAt: now,
		})
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
		limiter:    NewGenerationRateLimiter(store, nil).WithClock(fakeClock{now: now}),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/generate", strings.NewReader(`{"prompt":"test"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	res := httptest.NewRecorder()

	handler.generate(res, req)

	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, res.Code)
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

func TestListRepromptsReturnsHistory(t *testing.T) {
	now := time.Date(2026, 5, 23, 10, 30, 0, 0, time.UTC)
	service := &fakeGenerator{
		repromptHistory: []RepromptHistoryEntry{{
			ID:               "reprompt-1",
			Scope:            "page",
			TargetID:         "page-1",
			Prompt:           "Tighten the pricing page.",
			PreviousRevision: "revision-1",
			ResultRevision:   "revision-2",
			CreatedAt:        now,
		}},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sites/site-1/reprompts", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	res := httptest.NewRecorder()

	handler.listReprompts(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	var payload struct {
		Reprompts []RepromptHistoryEntry `json:"reprompts"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Reprompts) != 1 || payload.Reprompts[0].ID != "reprompt-1" {
		t.Fatalf("expected reprompt history in payload, got %#v", payload)
	}
}

func TestGetDraftRevisionReturnsSnapshot(t *testing.T) {
	service := &fakeGenerator{
		revision: DraftRevision{
			ID:       "revision-2",
			Scope:    "page",
			TargetID: "page-1",
			Prompt:   "Tighten the pricing page.",
			Draft:    validGenerationDraft(),
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sites/site-1/revisions/revision-2", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("revisionId", "revision-2")
	res := httptest.NewRecorder()

	handler.getDraftRevision(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if service.revisionID != "revision-2" {
		t.Fatalf("expected revision id to reach service, got %#v", service)
	}
}

func TestRevertRepromptReturnsRestoredDraft(t *testing.T) {
	service := &fakeGenerator{
		undoResult: validGenerationDraft(),
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site-1/reprompts/reprompt-1/revert", nil).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("repromptId", "reprompt-1")
	res := httptest.NewRecorder()

	handler.revertReprompt(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if service.repromptID != "reprompt-1" {
		t.Fatalf("expected reprompt id to reach service, got %#v", service)
	}
}

func TestSuggestBlockReturnsUpdatedDraft(t *testing.T) {
	service := &fakeGenerator{
		result: GenerateResult{
			JobID: "job-suggest",
			Draft: validGenerationDraft(),
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site-1/blocks/block-hero/suggest", strings.NewReader(`{"action":"tighten"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("blockId", "block-hero")
	res := httptest.NewRecorder()

	handler.suggestBlock(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, res.Code, res.Body.String())
	}
	if service.blockID != "block-hero" {
		t.Fatalf("expected block id to reach service, got %#v", service.blockID)
	}
	if service.blockSuggest.Action != "tighten" {
		t.Fatalf("expected action to reach service, got %#v", service.blockSuggest)
	}
}

func TestSuggestBlockRequiresBothIDs(t *testing.T) {
	handler := Handler{
		service:    &fakeGenerator{},
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites//blocks//suggest", strings.NewReader(`{"action":"tighten"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "")
	req.SetPathValue("blockId", "")
	res := httptest.NewRecorder()

	handler.suggestBlock(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestSuggestBlockSurfacesActionError(t *testing.T) {
	service := &fakeGenerator{err: ErrBlockSuggestActionUnknown}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site-1/blocks/block-hero/suggest", strings.NewReader(`{"action":"explode"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("blockId", "block-hero")
	res := httptest.NewRecorder()

	handler.suggestBlock(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestSuggestBlockSurfacesUnavailable(t *testing.T) {
	service := &fakeGenerator{err: ErrBlockSuggestUnavailable}
	handler := Handler{
		service:    service,
		authorizer: fakeWorkspaceAuthorizer{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sites/site-1/blocks/block-hero/suggest", strings.NewReader(`{"action":"tighten"}`)).WithContext(auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("blockId", "block-hero")
	res := httptest.NewRecorder()

	handler.suggestBlock(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusServiceUnavailable, res.Code, res.Body.String())
	}
}

func TestStreamGenerateDetachesRunContextFromClientConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodPost, "/api/sites/generate", nil).WithContext(ctx)
	res := httptest.NewRecorder()
	handler := Handler{}

	runCtxErr := make(chan error, 1)
	handler.streamGenerate(res, req, func(runCtx context.Context, sink ProgressSink) (GenerateResult, error) {
		runCtxErr <- runCtx.Err()
		return GenerateResult{
			JobID: "job-1",
			Draft: validGenerationDraft(),
		}, nil
	})

	select {
	case err := <-runCtxErr:
		if err != nil {
			t.Fatalf("expected detached context to stay active, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stream run context")
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
		return generationRowStub{values: []any{
			s.entitlement.WorkspaceID,
			s.entitlement.Plan,
			s.entitlement.Status,
			s.entitlement.SubscriptionLive,
			s.entitlement.CustomDomainsEnabled,
			s.entitlement.ActiveSiteLimit,
			s.entitlement.MonthlyPromptLimit,
			s.entitlement.AssetStorageLimitBytes,
			time.Now().UTC(),
		}}
	case strings.Contains(sql, "from sites"):
		return generationRowStub{values: []any{s.activeSiteCount}}
	case strings.Contains(sql, "from assets"):
		return generationRowStub{values: []any{s.assetBytes}}
	case strings.Contains(sql, "from billing_subscriptions"):
		if s.periodStart == nil || s.periodEnd == nil {
			return generationRowStub{err: pgx.ErrNoRows}
		}
		return generationRowStub{values: []any{s.periodStart, s.periodEnd}}
	case strings.Contains(sql, "from generation_jobs"):
		return generationRowStub{values: []any{s.promptCount}}
	default:
		return generationRowStub{err: pgx.ErrNoRows}
	}
}

type generationRowStub struct {
	values []any
	err    error
}

func (r generationRowStub) Scan(dest ...any) error {
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

func timePtr(value time.Time) *time.Time {
	return &value
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}
