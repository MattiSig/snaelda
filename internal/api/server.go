package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MattiSig/snaelda/internal/assets"
	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/blocks"
	"github.com/MattiSig/snaelda/internal/domains"
	"github.com/MattiSig/snaelda/internal/forms"
	"github.com/MattiSig/snaelda/internal/generation"
	"github.com/MattiSig/snaelda/internal/pages"
	"github.com/MattiSig/snaelda/internal/platform/config"
	"github.com/MattiSig/snaelda/internal/publishing"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/MattiSig/snaelda/internal/themes"
	"github.com/MattiSig/snaelda/internal/workspaces"
)

type ServerConfig struct {
	Config   config.Config
	Logger   *slog.Logger
	Database Pinger
}

type Server struct {
	config   Config
	logger   *slog.Logger
	database Pinger
}

type Config = config.Config

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewServer(cfg ServerConfig) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	return &Server{
		config:   cfg.Config,
		logger:   logger,
		database: cfg.Database,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/readyz", s.ready)

	mountPlaceholderModule(mux, auth.Module{})
	mountPlaceholderModule(mux, workspaces.Module{})
	mountPlaceholderModule(mux, sites.Module{})
	mountPlaceholderModule(mux, pages.Module{})
	mountPlaceholderModule(mux, blocks.Module{})
	mountPlaceholderModule(mux, themes.Module{})
	mountPlaceholderModule(mux, generation.Module{})
	mountPlaceholderModule(mux, publishing.Module{})
	mountPlaceholderModule(mux, domains.Module{})
	mountPlaceholderModule(mux, assets.Module{})
	mountPlaceholderModule(mux, forms.Module{})

	return s.recover(s.logRequests(mux))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"env":    s.config.AppEnv,
	})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "database is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.database.Ping(ctx); err != nil {
		s.logger.Error("database readiness check failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "database is unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ready",
		"database": "ok",
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic recovered", "error", recovered)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
