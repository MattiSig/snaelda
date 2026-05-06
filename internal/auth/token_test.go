package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTokenManagerValidatesIssuedToken(t *testing.T) {
	manager := newTestTokenManager(t)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	token, _, err := manager.IssueForSession(testUser(), "session-1")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("expected subject user-1, got %q", claims.Subject)
	}
	if claims.Issuer != "issuer" {
		t.Fatalf("expected issuer, got %q", claims.Issuer)
	}
	if claims.Audience != "audience" {
		t.Fatalf("expected audience, got %q", claims.Audience)
	}
	if claims.WorkspaceID != "workspace-1" {
		t.Fatalf("expected workspace claim, got %q", claims.WorkspaceID)
	}
	if claims.SessionID != "session-1" {
		t.Fatalf("expected session claim, got %q", claims.SessionID)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := newTestTokenManager(t)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	token, _, err := manager.Issue(testUser())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	manager.now = func() time.Time { return now.Add(16 * time.Minute) }
	if _, err := manager.Validate(token); err != ErrTokenExpired {
		t.Fatalf("expected expired token error, got %v", err)
	}
}

func TestTokenManagerRejectsTamperedToken(t *testing.T) {
	manager := newTestTokenManager(t)
	token, _, err := manager.Issue(testUser())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	tampered := strings.TrimSuffix(token, token[len(token)-1:]) + "x"
	if _, err := manager.Validate(tampered); err != ErrTokenInvalid {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}

func newTestTokenManager(t *testing.T) *TokenManager {
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

func testUser() User {
	return User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		Name:          "Demo User",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	}
}
