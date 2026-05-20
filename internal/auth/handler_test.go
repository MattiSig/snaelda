package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeAuthStore struct {
	createdWorkspaceMember bool
	sessionID              string
	refreshHash            string
	revoked                bool
	guestSessionID         string
	guestWorkspaceID       string
	guestCookieHash        string
	guestRecoveryHash      string
	guestPromptsUsed       int
	guestTrialStartedAt    time.Time
	guestTrialExpiresAt    time.Time
	guestHasRecoveryKey    bool
}

func (s *fakeAuthStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "insert into users"):
		return fakeRow{values: []any{"user-1", args[0].(string), args[1].(string)}}
	case strings.Contains(sql, "insert into auth_sessions"):
		s.sessionID = "session-1"
		s.refreshHash = args[1].(string)
		return fakeRow{values: []any{s.sessionID}}
	case strings.Contains(sql, "from auth_sessions s"):
		if !s.revoked && args[0].(string) == s.refreshHash {
			return fakeRow{values: []any{s.sessionID, "user-1", "demo@snaelda.local", "Demo User", "workspace-1", "owner"}}
		}
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from auth_sessions"):
		if !s.revoked && args[0].(string) == s.sessionID && args[1].(string) == "user-1" {
			return fakeRow{values: []any{s.sessionID}}
		}
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from workspaces"):
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "insert into workspaces"):
		return fakeRow{values: []any{"workspace-1"}}
	case strings.Contains(sql, "from guest_sessions gs"):
		hash := args[0].(string)
		if hash != s.guestCookieHash && hash != s.guestRecoveryHash {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []any{
			s.guestSessionID,
			s.guestWorkspaceID,
			s.guestPromptsUsed,
			s.guestTrialStartedAt,
			s.guestTrialExpiresAt,
			nil,
			"",
			s.guestHasRecoveryKey,
			"",
			"",
			"",
		}}
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

func (s *fakeAuthStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "set cookie_token_hash") {
		if args[1].(string) == s.guestSessionID {
			s.guestCookieHash = args[0].(string)
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	if strings.Contains(sql, "set refresh_token_hash") {
		if !s.revoked && args[1].(string) == s.sessionID && args[2].(string) == s.refreshHash {
			s.refreshHash = args[0].(string)
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	if strings.Contains(sql, "set revoked_at") {
		if strings.Contains(sql, "refresh_token_hash") && args[0].(string) == s.refreshHash {
			s.revoked = true
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		if strings.Contains(sql, "where id") && args[0].(string) == s.sessionID {
			s.revoked = true
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}

	s.createdWorkspaceMember = true
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			if value == nil {
				*target = ""
				continue
			}
			*target = value.(string)
		case *int:
			*target = value.(int)
		case *bool:
			*target = value.(bool)
		case *time.Time:
			*target = value.(time.Time)
		case **time.Time:
			if value == nil {
				*target = nil
				continue
			}
			t := value.(time.Time)
			*target = &t
		default:
			return pgx.ErrNoRows
		}
	}
	return nil
}

func TestLoginSetsHTTPOnlyCookieAndReturnsUser(t *testing.T) {
	store := &fakeAuthStore{}
	handler := NewHandler(HandlerConfig{
		Store:        store,
		Tokens:       newHandlerTestTokenManager(t),
		CookieSecure: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{
		"email": "Demo@Snaelda.Local",
		"name": "Demo User"
	}`))
	res := httptest.NewRecorder()

	handler.login(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if !store.createdWorkspaceMember {
		t.Fatal("expected default workspace membership to be created")
	}

	cookies := res.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected three cookies, got %d", len(cookies))
	}
	cookie := cookieNamed(t, cookies, AccessTokenCookieName)
	if cookie.Name != AccessTokenCookieName {
		t.Fatalf("expected access token cookie, got %q", cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected auth cookie to be HTTP-only")
	}
	if !cookie.Secure {
		t.Fatal("expected secure auth cookie")
	}
	refreshCookie := cookieNamed(t, cookies, RefreshTokenCookieName)
	if !refreshCookie.HttpOnly {
		t.Fatal("expected refresh cookie to be HTTP-only")
	}
	if !refreshCookie.Secure {
		t.Fatal("expected secure refresh cookie")
	}
	csrfCookie := cookieNamed(t, cookies, CSRFCookieName)
	if csrfCookie.HttpOnly {
		t.Fatal("expected csrf cookie to be readable by the frontend")
	}
	if !csrfCookie.Secure {
		t.Fatal("expected secure csrf cookie")
	}
	if csrfCookie.Value == "" {
		t.Fatal("expected csrf cookie value")
	}

	var payload authResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.User.Email != "demo@snaelda.local" {
		t.Fatalf("expected normalized email, got %q", payload.User.Email)
	}
	if payload.User.WorkspaceID != "workspace-1" {
		t.Fatalf("expected default workspace, got %q", payload.User.WorkspaceID)
	}
}

func TestRefreshRotatesRefreshCookieAndReturnsUser(t *testing.T) {
	store := &fakeAuthStore{}
	handler := NewHandler(HandlerConfig{
		Store:  store,
		Tokens: newHandlerTestTokenManager(t),
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{
		"email": "demo@snaelda.local",
		"name": "Demo User"
	}`))
	loginRes := httptest.NewRecorder()

	handler.login(loginRes, loginReq)

	refreshCookie := cookieNamed(t, loginRes.Result().Cookies(), RefreshTokenCookieName)
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	refreshReq.AddCookie(refreshCookie)
	refreshRes := httptest.NewRecorder()

	handler.refresh(refreshRes, refreshReq)

	if refreshRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, refreshRes.Code)
	}
	nextRefreshCookie := cookieNamed(t, refreshRes.Result().Cookies(), RefreshTokenCookieName)
	if nextRefreshCookie.Value == refreshCookie.Value {
		t.Fatal("expected refresh token rotation")
	}
	nextCSRFCookie := cookieNamed(t, refreshRes.Result().Cookies(), CSRFCookieName)
	if nextCSRFCookie.Value == "" || nextCSRFCookie.Value == cookieNamed(t, loginRes.Result().Cookies(), CSRFCookieName).Value {
		t.Fatal("expected csrf token rotation")
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	replayReq.AddCookie(refreshCookie)
	replayRes := httptest.NewRecorder()

	handler.refresh(replayRes, replayReq)

	if replayRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d for replayed refresh token, got %d", http.StatusUnauthorized, replayRes.Code)
	}
}

func TestLogoutRevokesActiveSession(t *testing.T) {
	store := &fakeAuthStore{}
	handler := NewHandler(HandlerConfig{
		Store:  store,
		Tokens: newHandlerTestTokenManager(t),
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{
		"email": "demo@snaelda.local",
		"name": "Demo User"
	}`))
	loginRes := httptest.NewRecorder()
	handler.login(loginRes, loginReq)
	accessCookie := cookieNamed(t, loginRes.Result().Cookies(), AccessTokenCookieName)
	refreshCookie := cookieNamed(t, loginRes.Result().Cookies(), RefreshTokenCookieName)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(accessCookie)
	logoutReq.AddCookie(refreshCookie)
	logoutReq.AddCookie(cookieNamed(t, loginRes.Result().Cookies(), CSRFCookieName))
	logoutRes := httptest.NewRecorder()

	handler.logout(logoutRes, logoutReq)

	if logoutRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, logoutRes.Code)
	}
	if cookieNamed(t, logoutRes.Result().Cookies(), CSRFCookieName).MaxAge != -1 {
		t.Fatal("expected logout to clear csrf cookie")
	}

	protected := handler.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	protectedReq := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	protectedReq.AddCookie(accessCookie)
	protectedRes := httptest.NewRecorder()

	protected.ServeHTTP(protectedRes, protectedReq)

	if protectedRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d after logout, got %d", http.StatusUnauthorized, protectedRes.Code)
	}
}

func TestRequireUserRejectsMissingCookie(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		Tokens: newHandlerTestTokenManager(t),
	})
	protected := handler.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	res := httptest.NewRecorder()

	protected.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func TestRestoreSessionRotatesGuestCookieAndAllowsSessionAccess(t *testing.T) {
	trialStartedAt := time.Now().UTC().Add(-time.Hour)
	store := &fakeAuthStore{
		guestSessionID:      "guest-session-1",
		guestWorkspaceID:    "workspace-guest-1",
		guestRecoveryHash:   tokenHash("restore-key"),
		guestPromptsUsed:    3,
		guestTrialStartedAt: trialStartedAt,
		guestTrialExpiresAt: trialStartedAt.Add(4 * 24 * time.Hour),
		guestHasRecoveryKey: true,
	}
	handler := NewHandler(HandlerConfig{
		Store:  store,
		Tokens: newHandlerTestTokenManager(t),
	})

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/sessions/restore", strings.NewReader(`{"key":"restore-key"}`))
	restoreRes := httptest.NewRecorder()

	handler.restoreSession(restoreRes, restoreReq)

	if restoreRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, restoreRes.Code)
	}
	guestCookie := cookieNamed(t, restoreRes.Result().Cookies(), GuestSessionCookieName)
	if guestCookie.Value == "restore-key" {
		t.Fatal("expected restore flow to issue a fresh guest cookie token")
	}
	if tokenHash(guestCookie.Value) != store.guestCookieHash {
		t.Fatal("expected restore flow to persist the rotated guest cookie token")
	}

	protected := handler.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected session in context")
		}
		if session.WorkspaceID != "workspace-guest-1" {
			t.Fatalf("expected restored workspace, got %q", session.WorkspaceID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	protectedReq := httptest.NewRequest(http.MethodGet, "/api/sessions/me", nil)
	protectedReq.AddCookie(guestCookie)
	protectedRes := httptest.NewRecorder()

	protected.ServeHTTP(protectedRes, protectedReq)

	if protectedRes.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, protectedRes.Code)
	}
}

func TestWriteSessionCookiesRefreshesTrialCookies(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		Tokens:          newHandlerTestTokenManager(t),
		RefreshTokenTTL: 30 * 24 * time.Hour,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/me", nil)
	req.AddCookie(&http.Cookie{Name: GuestSessionCookieName, Value: "guest-token-value"})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "csrf-token-value"})
	res := httptest.NewRecorder()

	trialStartedAt := time.Now().UTC().Add(-time.Hour)
	handler.writeSessionCookies(res, req, Session{
		Kind:           SessionKindTrial,
		TrialStartedAt: &trialStartedAt,
	})

	cookies := res.Result().Cookies()
	guest := cookieNamed(t, cookies, GuestSessionCookieName)
	if guest.Value != "guest-token-value" {
		t.Fatalf("expected guest cookie to preserve value, got %q", guest.Value)
	}
	if guest.MaxAge <= 0 {
		t.Fatalf("expected guest cookie MaxAge > 0, got %d", guest.MaxAge)
	}
	csrf := cookieNamed(t, cookies, CSRFCookieName)
	if csrf.Value != "csrf-token-value" {
		t.Fatalf("expected csrf cookie to preserve value, got %q", csrf.Value)
	}
	if csrf.MaxAge <= 0 {
		t.Fatalf("expected csrf cookie MaxAge > 0, got %d", csrf.MaxAge)
	}
}

func TestWriteSessionCookiesIgnoresAuthenticatedSessions(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		Tokens:          newHandlerTestTokenManager(t),
		RefreshTokenTTL: 30 * 24 * time.Hour,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/me", nil)
	req.AddCookie(&http.Cookie{Name: GuestSessionCookieName, Value: "guest-token-value"})
	res := httptest.NewRecorder()

	handler.writeSessionCookies(res, req, Session{Kind: SessionKindAuthenticated, User: &User{ID: "user-1"}})

	if len(res.Result().Cookies()) != 0 {
		t.Fatalf("expected no cookies refreshed for authenticated session, got %d", len(res.Result().Cookies()))
	}
}

func cookieNamed(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("expected cookie %q", name)
	return nil
}

func newHandlerTestTokenManager(t *testing.T) *TokenManager {
	t.Helper()

	manager, err := NewTokenManager(TokenConfig{
		Secret:   "secret",
		Issuer:   "issuer",
		Audience: "audience",
		TTL:      15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	return manager
}
