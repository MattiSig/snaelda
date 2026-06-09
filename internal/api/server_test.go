package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/platform/config"
	"github.com/MattiSig/snaelda/internal/publishing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestBillingModulePlaceholderWithAuth(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:   "test",
			HTTPAddr: "127.0.0.1:0",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/billing", nil)
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

func TestPublicRoutesAllowCrossOriginGET(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public/render?hostname=demo.localhost", nil)
	req.Header.Set("Origin", "http://demo.localhost:3000")
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin for public route, got %q", got)
	}
}

func TestPublicFormRoutesAllowCrossOriginPOST(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/public/forms/site-1/block-1/submit", nil)
	req.Header.Set("Origin", "http://demo.localhost:3000")
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin for public form route, got %q", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("expected public form route to allow POST, got %q", got)
	}
}

func TestPrivateRoutesKeepBuilderOriginPolicy(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sites", nil)
	req.Header.Set("Origin", "http://demo.localhost:3000")
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header for non-builder private origin, got %q", got)
	}
}

func TestPrivateRoutesAllowCSRFCorsHeaderForBuilderOrigin(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/auth/logout", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-CSRF-Token") {
		t.Fatalf("expected csrf header in private CORS allowlist, got %q", got)
	}
}

func TestPrivateRoutesAllowCSRFCorsHeaderForWWWBuilderOrigin(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "https://snaelda.io",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/auth/logout", nil)
	req.Header.Set("Origin", "https://www.snaelda.io")
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://www.snaelda.io" {
		t.Fatalf("expected www builder origin in CORS allowlist, got %q", got)
	}
}

func TestPrivateWriteRoutesAllowCSRFForWWWBuilderOrigin(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "https://snaelda.io",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Origin", "https://www.snaelda.io")
	req.Header.Set("X-CSRF-Token", "csrf-token")
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf-token"})
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code == http.StatusForbidden {
		t.Fatalf("expected csrf origin check to allow www builder origin, got status %d", res.Code)
	}
}

func TestPrivateReadRoutesDisableCaching(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(validAuthCookie(t))
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected no-store cache policy, got %q", got)
	}
}

func TestPrivateRoutesApplySecurityHeaders(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(validAuthCookie(t))
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected x-frame-options deny, got %q", got)
	}
	if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff, got %q", got)
	}
	if got := res.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("expected hsts header, got %q", got)
	}
	if got := res.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") {
		t.Fatalf("expected private api csp, got %q", got)
	}
}

func TestPublicRoutesApplyRenderableSecurityHeaders(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public/render?hostname=missing.localhost", nil)
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if got := res.Header().Get("Content-Security-Policy"); !strings.Contains(got, "style-src 'self' 'unsafe-inline'") {
		t.Fatalf("expected public csp to allow inline styles, got %q", got)
	}
	if got := res.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("expected public csp to block scripts, got %q", got)
	}
}

func TestPrivateWriteRoutesRequireCSRFCookieAndHeader(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(validAuthCookie(t))
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestPrivateWriteRoutesAcceptMatchingCSRFCookieAndHeader(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(validAuthCookie(t))
	req.AddCookie(validCSRFCookie())
	req.Header.Set("X-CSRF-Token", "csrf-token")
	req.Header.Set("Origin", "http://localhost:3000")
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

func TestPublicFormWriteRoutesDoNotRequireCSRF(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/public/forms/site-1/block-1/submit", strings.NewReader(`{}`))
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code == http.StatusForbidden {
		t.Fatalf("expected public form route to bypass csrf, got %d", res.Code)
	}
}

func TestBillingWebhookBypassesCSRF(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.DiscardHandler),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/billing/webhook", strings.NewReader(`{}`))
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if res.Code == http.StatusForbidden {
		t.Fatalf("expected billing webhook to bypass csrf, got %d", res.Code)
	}
}

func TestFailureLoggingIncludesRequestCategory(t *testing.T) {
	var logOutput bytes.Buffer
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:     "test",
			HTTPAddr:   "127.0.0.1:0",
			AppBaseURL: "http://localhost:3000",
		},
		Logger: slog.New(slog.NewTextHandler(&logOutput, nil)),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public/render?hostname=missing.localhost", nil)
	res := httptest.NewRecorder()

	server.Handler().ServeHTTP(res, req)

	if !strings.Contains(logOutput.String(), "category=public_render") {
		t.Fatalf("expected failure log category, got %q", logOutput.String())
	}
}

func TestBuildHandlerFailsForInvalidPublishedArtifactsS3Config(t *testing.T) {
	server := NewServer(ServerConfig{
		Config: config.Config{
			AppEnv:                     "test",
			HTTPAddr:                   "127.0.0.1:0",
			AppBaseURL:                 "http://localhost:3000",
			PublicBaseURL:              "http://localhost:3000",
			PublicBaseDomain:           "localhost",
			PublishedArtifactsBackend:  "s3",
			PublishedArtifactsS3Bucket: "published",
			S3Endpoint:                 "://bad-endpoint",
		},
		Logger:   slog.New(slog.DiscardHandler),
		Database: fakePublishingDB{},
	})

	if _, err := server.BuildHandler(); err == nil {
		t.Fatal("expected build handler to fail for invalid published artifacts s3 config")
	}
}

type fakePublishingDB struct{}

func (fakePublishingDB) Ping(context.Context) error { return nil }
func (fakePublishingDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}
func (fakePublishingDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}
func (fakePublishingDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (fakePublishingDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, nil
}

var _ publishing.DB = fakePublishingDB{}

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

func validCSRFCookie() *http.Cookie {
	return &http.Cookie{
		Name:  auth.CSRFCookieName,
		Value: "csrf-token",
	}
}
