package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

	generationPlanner, err := generation.NewOpenAIPlanner(generation.OpenAIPlannerConfig{
		APIKey: s.config.OpenAIAPIKey,
		Model:  s.config.OpenAIModel,
	})
	if err != nil {
		s.logger.Error("configure generation planner", "error", err)
	}
	var generationPlanBuilder generation.HandlerConfig
	var themeHandlerConfig themes.HandlerConfig
	if generationPlanner != nil {
		generationPlanBuilder.Planner = generationPlanner.BuildPlan
		themeHandlerConfig.Regenerator = generationPlanner
	}

	mountAuthenticatedPlaceholderModule(mux, s.auth, workspaces.Module{})
	if store, ok := s.database.(sites.DB); ok {
		sites.NewHandler(store, s.config.PreviewTokenTTL).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, sites.Module{})
	}
	mountAuthenticatedPlaceholderModule(mux, s.auth, pages.Module{})
	mountAuthenticatedPlaceholderModule(mux, s.auth, blocks.Module{})
	if store, ok := s.database.(themes.DB); ok {
		themes.NewHandler(store, themeHandlerConfig).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, themes.Module{})
	}
	if store, ok := s.database.(generation.DB); ok {
		generation.NewHandler(store, generationPlanBuilder).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, generation.Module{})
	}
	if store, ok := s.database.(publishing.DB); ok {
		publishing.NewHandler(
			store,
			s.config.AppBaseURL,
			s.config.PublicBaseURL,
			s.config.PublicBaseDomain,
			s.config.PublishedArtifactsDir,
		).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, publishing.Module{})
	}
	if store, ok := s.database.(domains.DB); ok {
		domains.NewHandler(store, domains.HandlerConfig{
			AppBaseURL:       s.config.AppBaseURL,
			PublicBaseURL:    s.config.PublicBaseURL,
			PublicBaseDomain: s.config.PublicBaseDomain,
		}).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, domains.Module{})
	}
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
	if store, ok := s.database.(forms.DB); ok {
		forms.NewHandler(store).Mount(mux, s.auth.RequireUser)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, forms.Module{})
	}
	mountAuthenticatedPlaceholderModule(mux, s.auth, billing.Module{})

	return s.recover(s.logRequests(s.noCache(s.csrf(s.cors(mux)))))
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
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		durationMs := time.Since(start).Milliseconds()
		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", durationMs,
		}

		failureCategory := requestFailureCategory(r)
		if failureCategory != "" && recorder.status >= http.StatusBadRequest {
			attrs = append(attrs, "category", failureCategory)
			if recorder.status >= http.StatusInternalServerError {
				s.logger.Error("request failed", attrs...)
				return
			}
			s.logger.Warn("request failed", attrs...)
			return
		}

		s.logger.Info("http request", attrs...)
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
		case isPublicRoute(r):
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		case origin != "" && origin == s.config.AppBaseURL:
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-CSRF-Token")
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

func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requiresCSRFMitigation(r) {
			next.ServeHTTP(w, r)
			return
		}

		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && origin != s.config.AppBaseURL {
			writeError(w, http.StatusForbidden, "invalid_origin", "request origin is not allowed")
			return
		}

		cookieToken, err := auth.CSRFCookieFromRequest(r)
		if err != nil || strings.TrimSpace(cookieToken) == "" {
			writeError(w, http.StatusForbidden, "csrf_invalid", "csrf token is required")
			return
		}

		headerToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
		if headerToken == "" || headerToken != cookieToken {
			writeError(w, http.StatusForbidden, "csrf_invalid", "csrf token is invalid")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPrivateAPIRoute(r) {
			w.Header().Set("Cache-Control", "no-store, private")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		next.ServeHTTP(w, r)
	})
}

func isPublicRoute(r *http.Request) bool {
	if len(r.URL.Path) < len("/api/public/") || r.URL.Path[:len("/api/public/")] != "/api/public/" {
		return false
	}
	if r.Method == http.MethodGet {
		return true
	}
	if len(r.URL.Path) >= len("/api/public/forms/") &&
		r.URL.Path[:len("/api/public/forms/")] == "/api/public/forms/" {
		return r.Method == http.MethodPost || r.Method == http.MethodOptions
	}
	return false
}

func isPrivateAPIRoute(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/") && !isPublicRoute(r)
}

func requiresCSRFMitigation(r *http.Request) bool {
	if !isPrivateAPIRoute(r) {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return r.URL.Path != "/api/auth/login"
}

func requestFailureCategory(r *http.Request) string {
	switch {
	case r.URL.Path == "/api/sites/generate" || strings.Contains(r.URL.Path, "/reprompt") || strings.Contains(r.URL.Path, "/undo"):
		return "generation"
	case strings.Contains(r.URL.Path, "/publish") || strings.Contains(r.URL.Path, "/rollback"):
		return "publishing"
	case strings.HasPrefix(r.URL.Path, "/api/public/forms/"):
		return "form_submission"
	case r.URL.Path == "/api/public/render" || strings.HasPrefix(r.URL.Path, "/api/public/sites/"):
		return "public_render"
	default:
		return ""
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
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
