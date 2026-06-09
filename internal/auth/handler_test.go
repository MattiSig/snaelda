package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestCSRFCookieUsesAppBaseDomainForProductionSubdomains(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		AppBaseURL:   "https://www.snaelda.io",
		CookieSecure: true,
	})

	cookie := handler.csrfCookie("csrf-token", 3600)

	if cookie.Domain != "snaelda.io" {
		t.Fatalf("expected csrf cookie domain snaelda.io, got %q", cookie.Domain)
	}
	if cookie.HttpOnly {
		t.Fatal("expected csrf cookie to be readable by the frontend")
	}
}

func TestCSRFCookieKeepsHostOnlyScopeForLocalhost(t *testing.T) {
	handler := NewHandler(HandlerConfig{
		AppBaseURL: "http://localhost:3000",
	})

	cookie := handler.csrfCookie("csrf-token", 3600)

	if cookie.Domain != "" {
		t.Fatalf("expected host-only csrf cookie for localhost, got %q", cookie.Domain)
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

func TestTrialGenerationBlocked(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		session Session
		blocked bool
	}{
		{
			name: "active trial",
			session: Session{
				Kind:           SessionKindTrial,
				TrialStartedAt: &now,
				PromptLimit:    trialPromptLimit,
			},
		},
		{
			name: "expired trial",
			session: Session{
				Kind:           SessionKindTrial,
				TrialStartedAt: &now,
				TrialExpired:   true,
				PromptLimit:    trialPromptLimit,
			},
			blocked: true,
		},
		{
			name: "exhausted trial",
			session: Session{
				Kind:           SessionKindTrial,
				TrialStartedAt: &now,
				PromptsUsed:    trialPromptLimit,
				PromptLimit:    trialPromptLimit,
			},
			blocked: true,
		},
		{
			name: "subscribed trial workspace",
			session: Session{
				Kind:             SessionKindTrial,
				TrialStartedAt:   &now,
				TrialExpired:     true,
				PromptsUsed:      trialPromptLimit,
				PromptLimit:      trialPromptLimit,
				SubscriptionLive: true,
			},
		},
		{
			name: "authenticated session with expired trial metadata",
			session: Session{
				Kind:           SessionKindAuthenticated,
				User:           &User{ID: "user-1"},
				TrialStartedAt: &now,
				TrialExpired:   true,
				PromptsUsed:    trialPromptLimit,
				PromptLimit:    trialPromptLimit,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := trialGenerationBlocked(test.session); got != test.blocked {
				t.Fatalf("expected blocked=%v, got %v", test.blocked, got)
			}
		})
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

func TestConsumeMagicLinkTokenCreatesSessionAndConsumesTokenAtomically(t *testing.T) {
	store := newFakeMagicLinkStore("magic-token", magicLinkLoginPurpose, time.Now().UTC().Add(time.Minute))
	handler := NewHandler(HandlerConfig{Store: store})

	user, sessionID, err := handler.consumeMagicLinkToken(context.Background(), "magic-token", "refresh-token", time.Hour, "test-agent")
	if err != nil {
		t.Fatalf("consume magic link token: %v", err)
	}
	if user.ID != "user-1" {
		t.Fatalf("expected user-1, got %#v", user)
	}
	if sessionID == "" {
		t.Fatal("expected auth session id")
	}
	if !store.state.consumed {
		t.Fatal("expected magic link to be consumed on commit")
	}
	if store.state.sessionsCreated != 1 {
		t.Fatalf("expected one committed session, got %d", store.state.sessionsCreated)
	}
}

func TestConsumeMagicLinkTokenRollsBackWhenSessionCreationFails(t *testing.T) {
	store := newFakeMagicLinkStore("magic-token", magicLinkLoginPurpose, time.Now().UTC().Add(time.Minute))
	store.state.sessionInsertErr = context.DeadlineExceeded
	handler := NewHandler(HandlerConfig{Store: store})

	_, _, err := handler.consumeMagicLinkToken(context.Background(), "magic-token", "refresh-token", time.Hour, "test-agent")
	if err == nil {
		t.Fatal("expected session creation failure")
	}
	if store.state.consumed {
		t.Fatal("expected consumed_at rollback on failure")
	}
	if store.state.sessionsCreated != 0 {
		t.Fatalf("expected no committed session, got %d", store.state.sessionsCreated)
	}

	store.state.sessionInsertErr = nil
	if _, _, err := handler.consumeMagicLinkToken(context.Background(), "magic-token", "refresh-token-2", time.Hour, "test-agent"); err != nil {
		t.Fatalf("expected retry after rollback to succeed, got %v", err)
	}
}

func TestConsumeMagicLinkTokenRejectsExpiredReplayAndWrongPurpose(t *testing.T) {
	tests := []struct {
		name      string
		purpose   string
		expiresAt time.Time
	}{
		{
			name:      "expired",
			purpose:   magicLinkLoginPurpose,
			expiresAt: time.Now().UTC().Add(-time.Minute),
		},
		{
			name:      "wrong purpose",
			purpose:   "password_reset",
			expiresAt: time.Now().UTC().Add(time.Minute),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeMagicLinkStore("magic-token", test.purpose, test.expiresAt)
			handler := NewHandler(HandlerConfig{Store: store})

			_, _, err := handler.consumeMagicLinkToken(context.Background(), "magic-token", "refresh-token", time.Hour, "test-agent")
			if err != pgx.ErrNoRows {
				t.Fatalf("expected pgx.ErrNoRows, got %v", err)
			}
			if store.state.consumed {
				t.Fatal("expected token to remain unconsumed")
			}
		})
	}

	store := newFakeMagicLinkStore("magic-token", magicLinkLoginPurpose, time.Now().UTC().Add(time.Minute))
	handler := NewHandler(HandlerConfig{Store: store})
	if _, _, err := handler.consumeMagicLinkToken(context.Background(), "magic-token", "refresh-token", time.Hour, "test-agent"); err != nil {
		t.Fatalf("initial consume should succeed: %v", err)
	}
	if _, _, err := handler.consumeMagicLinkToken(context.Background(), "magic-token", "refresh-token-2", time.Hour, "test-agent"); err != pgx.ErrNoRows {
		t.Fatalf("expected replay to fail with pgx.ErrNoRows, got %v", err)
	}
}

func TestConsumeMagicLinkTokenAllowsOnlyOneConcurrentRedemption(t *testing.T) {
	store := newFakeMagicLinkStore("magic-token", magicLinkLoginPurpose, time.Now().UTC().Add(time.Minute))
	handler := NewHandler(HandlerConfig{Store: store})

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, _, err := handler.consumeMagicLinkToken(
				context.Background(),
				"magic-token",
				"refresh-token-"+string(rune('a'+index)),
				time.Hour,
				"test-agent",
			)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if err == pgx.ErrNoRows {
			failures++
			continue
		}
		t.Fatalf("unexpected concurrent redemption error: %v", err)
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("expected one success and one replay failure, got successes=%d failures=%d", successes, failures)
	}
}

type fakeMagicLinkStore struct {
	state *fakeMagicLinkState
}

type fakeMagicLinkState struct {
	mu               sync.Mutex
	cond             *sync.Cond
	locked           bool
	tokenHashValue   string
	purpose          string
	expiresAt        time.Time
	consumed         bool
	sessionInsertErr error
	sessionsCreated  int
}

func newFakeMagicLinkStore(token string, purpose string, expiresAt time.Time) *fakeMagicLinkStore {
	state := &fakeMagicLinkState{
		tokenHashValue: tokenHash(token),
		purpose:        purpose,
		expiresAt:      expiresAt,
	}
	state.cond = sync.NewCond(&state.mu)
	return &fakeMagicLinkStore{state: state}
}

func (s *fakeMagicLinkStore) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (s *fakeMagicLinkStore) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, pgx.ErrNoRows
}

func (s *fakeMagicLinkStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return &fakeMagicLinkTx{state: s.state}, nil
}

type fakeMagicLinkTx struct {
	state          *fakeMagicLinkState
	hasLock        bool
	consumePending bool
	sessionPending bool
}

func (tx *fakeMagicLinkTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, pgx.ErrTxClosed
}

func (tx *fakeMagicLinkTx) Commit(context.Context) error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	if tx.consumePending {
		tx.state.consumed = true
	}
	if tx.sessionPending {
		tx.state.sessionsCreated++
	}
	if tx.hasLock {
		tx.state.locked = false
		tx.hasLock = false
		tx.state.cond.Broadcast()
	}
	return nil
}

func (tx *fakeMagicLinkTx) Rollback(context.Context) error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.consumePending = false
	tx.sessionPending = false
	if tx.hasLock {
		tx.state.locked = false
		tx.hasLock = false
		tx.state.cond.Broadcast()
	}
	return nil
}

func (tx *fakeMagicLinkTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, pgx.ErrTxClosed
}

func (tx *fakeMagicLinkTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeMagicLinkTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeMagicLinkTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, pgx.ErrTxClosed
}

func (tx *fakeMagicLinkTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if !strings.Contains(sql, "update magic_links") {
		return pgconn.CommandTag{}, pgx.ErrNoRows
	}

	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	if !tx.hasLock || tx.state.consumed || tx.consumePending {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	tx.consumePending = true
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (tx *fakeMagicLinkTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, pgx.ErrTxClosed
}

func (tx *fakeMagicLinkTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "from magic_links ml"):
		tx.state.mu.Lock()
		for tx.state.locked {
			tx.state.cond.Wait()
		}
		tx.state.locked = true
		tx.hasLock = true
		if args[0].(string) != tx.state.tokenHashValue ||
			tx.state.consumed ||
			!tx.state.expiresAt.After(time.Now().UTC()) ||
			(tx.state.purpose != args[1].(string) && tx.state.purpose != args[2].(string)) {
			tx.state.locked = false
			tx.hasLock = false
			tx.state.cond.Broadcast()
			tx.state.mu.Unlock()
			return fakeRow{err: pgx.ErrNoRows}
		}
		tx.state.mu.Unlock()
		return fakeRow{values: []any{
			"magic-link-1",
			"user-1",
			"demo@snaelda.local",
			"Demo User",
			"workspace-1",
			"owner",
		}}
	case strings.Contains(sql, "insert into auth_sessions"):
		tx.state.mu.Lock()
		defer tx.state.mu.Unlock()
		if tx.state.sessionInsertErr != nil {
			return fakeRow{err: tx.state.sessionInsertErr}
		}
		tx.sessionPending = true
		return fakeRow{values: []any{"session-1"}}
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

func (tx *fakeMagicLinkTx) Conn() *pgx.Conn {
	return nil
}
