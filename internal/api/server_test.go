package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestModulePlaceholder(t *testing.T) {
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

	if res.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, res.Code)
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
