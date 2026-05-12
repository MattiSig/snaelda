package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MattiSig/snaelda/internal/assets"
	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/billing"
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
	auth     *auth.Handler
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

	tokenManager, err := auth.NewTokenManager(auth.TokenConfig{
		Secret:   firstNonEmpty(cfg.Config.AuthJWTSecret, "test-auth-secret"),
		Issuer:   firstNonEmpty(cfg.Config.AuthIssuer, "snaelda-api"),
		Audience: firstNonEmpty(cfg.Config.AuthAudience, "snaelda-web"),
		TTL:      firstPositiveDuration(cfg.Config.AuthAccessTokenTTL, 15*time.Minute),
	})
	if err != nil {
		logger.Error("configure auth tokens", "error", err)
	}

	var authStore auth.UserStore
	if store, ok := cfg.Database.(auth.UserStore); ok {
		authStore = store
	}

	return &Server{
		config:   cfg.Config,
		logger:   logger,
		database: cfg.Database,
		auth: auth.NewHandler(auth.HandlerConfig{
			Store:           authStore,
			Tokens:          tokenManager,
			RefreshTokenTTL: firstPositiveDuration(cfg.Config.AuthRefreshTokenTTL, 30*24*time.Hour),
			CookieSecure:    cfg.Config.AuthCookieSecure,
		}),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/readyz", s.ready)

	s.auth.Mount(mux)

	mountAuthenticatedPlaceholderModule(mux, s.auth, workspaces.Module{})
	if store, ok := s.database.(sites.DB); ok {
		sites.NewHandler(store).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, sites.Module{})
	}
	mountAuthenticatedPlaceholderModule(mux, s.auth, pages.Module{})
	mountAuthenticatedPlaceholderModule(mux, s.auth, blocks.Module{})
	if store, ok := s.database.(themes.DB); ok {
		themes.NewHandler(store).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, themes.Module{})
	}
	if store, ok := s.database.(generation.DB); ok {
		generation.NewHandler(store).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, generation.Module{})
	}
	if store, ok := s.database.(publishing.DB); ok {
		publishing.NewHandler(store, s.config.AppBaseURL, s.config.PublishedArtifactsDir).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, publishing.Module{})
	}
	mountAuthenticatedPlaceholderModule(mux, s.auth, domains.Module{})
	if store, ok := s.database.(assets.DB); ok {
		storage, err := assets.NewS3Storage(assets.StorageConfig{
			Endpoint:        s.config.S3Endpoint,
			Bucket:          s.config.S3Bucket,
			Region:          s.config.S3Region,
			AccessKeyID:     s.config.S3AccessKeyID,
			SecretAccessKey: s.config.S3SecretAccessKey,
			ForcePathStyle:  s.config.S3ForcePathStyle,
		})
		if err != nil {
			s.logger.Error("configure asset storage", "error", err)
			mountAuthenticatedPlaceholderModule(mux, s.auth, assets.Module{})
		} else {
			assets.NewHandler(store, storage).Mount(mux, s.auth.RequireUser)
		}
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, assets.Module{})
	}
	mountAuthenticatedPlaceholderModule(mux, s.auth, forms.Module{})
	mountAuthenticatedPlaceholderModule(mux, s.auth, billing.Module{})

	return s.recover(s.logRequests(s.cors(mux)))
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

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		switch {
		case isPublicReadRoute(r):
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		case origin != "" && origin == s.config.AppBaseURL:
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isPublicReadRoute(r *http.Request) bool {
	return r.Method == http.MethodGet && len(r.URL.Path) >= len("/api/public/") && r.URL.Path[:len("/api/public/")] == "/api/public/"
}

func firstNonEmpty(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func firstPositiveDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
