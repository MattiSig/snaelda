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
}

func (s *fakeAuthStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "insert into users"):
		return fakeRow{values: []string{"user-1", args[0].(string), args[1].(string)}}
	case strings.Contains(sql, "from workspaces"):
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "insert into workspaces"):
		return fakeRow{values: []string{"workspace-1"}}
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

func (s *fakeAuthStore) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	s.createdWorkspaceMember = true
	return pgconn.CommandTag{}, nil
}

type fakeRow struct {
	values []string
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		target := dest[index].(*string)
		*target = value
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
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != AccessTokenCookieName {
		t.Fatalf("expected access token cookie, got %q", cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected auth cookie to be HTTP-only")
	}
	if !cookie.Secure {
		t.Fatal("expected secure auth cookie")
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
