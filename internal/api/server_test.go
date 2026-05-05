package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MattiSig/snaelda/internal/platform/config"
)

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
