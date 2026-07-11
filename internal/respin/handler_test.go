package respin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/sites"
)

// fakeHandlerStore is a minimal handlerStore for guard-path tests.
type fakeHandlerStore struct {
	cached    *Import
	created   Import
	createErr error
	getByID   map[string]Import
}

func (f *fakeHandlerStore) Create(_ context.Context, input CreateInput) (Import, error) {
	if f.createErr != nil {
		return Import{}, f.createErr
	}
	imp := f.created
	imp.SourceURL = input.SourceURL
	imp.NormalizedURL = input.NormalizedURL
	imp.GuestSessionID = input.GuestSessionID
	imp.WorkspaceID = input.WorkspaceID
	return imp, nil
}

func (f *fakeHandlerStore) Get(_ context.Context, id string) (Import, error) {
	if imp, ok := f.getByID[id]; ok {
		return imp, nil
	}
	return Import{}, ErrNotFound
}

func (f *fakeHandlerStore) GetByShareSlug(context.Context, string) (Import, error) {
	return Import{}, ErrNotFound
}

func (f *fakeHandlerStore) FindCached(context.Context, string, time.Time) (Import, error) {
	if f.cached != nil {
		return *f.cached, nil
	}
	return Import{}, ErrNotFound
}

func (f *fakeHandlerStore) Claim(context.Context, string, string, string) (Import, error) {
	return Import{}, ErrNotFound
}

func (f *fakeHandlerStore) LinkedGeneration(context.Context, string) (GenerationLink, error) {
	return GenerationLink{}, ErrNotFound
}

type fakeLimiter struct{ allow bool }

func (f fakeLimiter) Allow(context.Context, string, string, ...auth.IPRateLimitRule) bool {
	return f.allow
}

type fakeSessions struct {
	demo auth.RespinDemoSession
}

func (f fakeSessions) StartRespinDemoSession(context.Context, string) (auth.RespinDemoSession, error) {
	return f.demo, nil
}

func (f fakeSessions) AdoptRespinDemoSession(http.ResponseWriter, *http.Request, string) (auth.Session, error) {
	return auth.Session{}, nil
}

type fakePreviews struct{}

func (fakePreviews) Issue(context.Context, string, string) (sites.PreviewToken, error) {
	return sites.PreviewToken{Token: "tok", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func newTestHandler(store handlerStore, limiter IPLimiter) *Handler {
	return NewHandler(HandlerConfig{
		Store:     store,
		Runner:    NewRunner(2, nil),
		Fetcher:   testFetcher(), // allowPrivate: ValidatePublicURL skips DNS checks
		Previews:  fakePreviews{},
		Sessions:  fakeSessions{demo: auth.RespinDemoSession{WorkspaceID: "ws-1", GuestSessionID: "gs-1"}},
		IPLimiter: limiter,
	})
}

func postJSON(t *testing.T, h http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestStartDemoRejectsInvalidJSON(t *testing.T) {
	h := newTestHandler(&fakeHandlerStore{}, fakeLimiter{allow: true})
	rec := postJSON(t, h.startDemo, "/api/respin", "{not json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "invalid_request")
}

func TestStartDemoRejectsInvalidURL(t *testing.T) {
	h := newTestHandler(&fakeHandlerStore{}, fakeLimiter{allow: true})
	rec := postJSON(t, h.startDemo, "/api/respin", `{"url":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "invalid_url")
}

func TestStartDemoRateLimited(t *testing.T) {
	h := newTestHandler(&fakeHandlerStore{}, fakeLimiter{allow: false})
	rec := postJSON(t, h.startDemo, "/api/respin", `{"url":"https://example.com"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "rate_limited")
}

func TestStartDemoServesCachedResult(t *testing.T) {
	store := &fakeHandlerStore{cached: &Import{ID: "imp-cached", FetchStatus: StatusSucceeded, ShareSlug: "abc123"}}
	h := newTestHandler(store, fakeLimiter{allow: true})
	rec := postJSON(t, h.startDemo, "/api/respin", `{"url":"https://example.com"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cache hit, got %d", rec.Code)
	}
	var resp startResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ImportID != "imp-cached" || !resp.Cached || resp.ShareSlug != "abc123" {
		t.Fatalf("unexpected cached response: %+v", resp)
	}
}

func TestStatusNotFound(t *testing.T) {
	h := newTestHandler(&fakeHandlerStore{}, fakeLimiter{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/api/respin/missing", nil)
	req.SetPathValue("importId", "missing")
	rec := httptest.NewRecorder()
	h.status(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestClaimRejectsAlreadyClaimed(t *testing.T) {
	store := &fakeHandlerStore{getByID: map[string]Import{
		"imp-1": {ID: "imp-1", WorkspaceID: "ws-existing", GuestSessionID: "gs-1"},
	}}
	h := newTestHandler(store, fakeLimiter{allow: true})
	req := httptest.NewRequest(http.MethodPost, "/api/respin/imp-1/claim", nil)
	req.SetPathValue("importId", "imp-1")
	rec := httptest.NewRecorder()
	h.claim(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for claimed import, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "already_claimed")
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if payload.Error.Code != want {
		t.Fatalf("expected error code %q, got %q (%s)", want, payload.Error.Code, body)
	}
}
