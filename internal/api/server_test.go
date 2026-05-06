package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/platform/config"
)

type fakePinger struct {
	err error
}

func (p fakePinger) Ping(context.Context) error {
	return p.err
}

func TestHealth(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var payload map[string]string
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected ok status, got %q", payload["status"])
	}
	if payload["env"] != "test" {
		t.Fatalf("expected test env, got %q", payload["env"])
	}
}

func TestModulePlaceholderRequiresAuth(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sites", nil)
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func TestModulePlaceholderWithAuth(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sites", nil)
	req.AddCookie(validAuthCookie(t))
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, res.Code)
	}
}

func TestAuthMe(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(validAuthCookie(t))
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var payload struct {
		User struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			WorkspaceID string `json:"workspaceId"`
		} `json:"user"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.User.ID != "user-1" {
		t.Fatalf("expected user-1, got %q", payload.User.ID)
	}
	if payload.User.Email != "demo@snaelda.local" {
		t.Fatalf("expected demo email, got %q", payload.User.Email)
	}
	if payload.User.WorkspaceID != "workspace-1" {
		t.Fatalf("expected workspace-1, got %q", payload.User.WorkspaceID)
	}
}

func TestReady(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger:   slog.New(slog.DiscardHandler),
		Database: fakePinger{},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

func TestReadyWithoutDatabase(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, res.Code)
	}
}

func TestReadyWithDatabaseError(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger:   slog.New(slog.DiscardHandler),
		Database: fakePinger{err: errors.New("down")},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, res.Code)
	}
}

func validAuthCookie(t *testing.T) *http.Cookie {
	t.Helper()

	manager, err := auth.NewTokenManager(auth.TokenConfig{
		Secret:   "test-auth-secret",
		Issuer:   "snaelda-api",
		Audience: "snaelda-web",
		TTL:      15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	token, _, err := manager.Issue(auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		Name:          "Demo User",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: "owner",
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	return &http.Cookie{
		Name:  auth.AccessTokenCookieName,
		Value: token,
	}
}
