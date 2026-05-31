package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/analytics"
	"github.com/MattiSig/snaelda/internal/assets"
	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/MattiSig/snaelda/internal/blocks"
	"github.com/MattiSig/snaelda/internal/collections"
	"github.com/MattiSig/snaelda/internal/domains"
	"github.com/MattiSig/snaelda/internal/email"
	"github.com/MattiSig/snaelda/internal/forms"
	"github.com/MattiSig/snaelda/internal/generation"
	"github.com/MattiSig/snaelda/internal/imagery"
	"github.com/MattiSig/snaelda/internal/pages"
	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/MattiSig/snaelda/internal/platform/config"
	"github.com/MattiSig/snaelda/internal/publishing"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/MattiSig/snaelda/internal/themes"
	"github.com/MattiSig/snaelda/internal/workspaces"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	mailer   email.Mailer
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

	var mailer email.Mailer
	if authStore != nil {
		replyTo := strings.TrimSpace(cfg.Config.EmailReplyTo)
		var replyToAddress *email.Address
		if replyTo != "" {
			replyToAddress = &email.Address{Email: replyTo}
		}
		mailer, err = email.NewMailer(email.Config{
			Transport: cfg.Config.EmailTransport,
			DefaultFrom: email.Address{
				Email: cfg.Config.EmailFromAddress,
				Name:  cfg.Config.EmailFromName,
			},
			ReplyTo:         replyToAddress,
			ResendAPIKey:    cfg.Config.ResendAPIKey,
			MailpitSMTPAddr: cfg.Config.MailpitSMTPAddr,
			Logger:          logger,
		})
		if err != nil {
			logger.Error("configure email transport", "error", err)
		}
	}

	return &Server{
		config:   cfg.Config,
		logger:   logger,
		database: cfg.Database,
		mailer:   mailer,
		auth: auth.NewHandler(auth.HandlerConfig{
			Store:           authStore,
			Tokens:          tokenManager,
			RefreshTokenTTL: firstPositiveDuration(cfg.Config.AuthRefreshTokenTTL, 30*24*time.Hour),
			CookieSecure:    cfg.Config.AuthCookieSecure,
			AppBaseURL:      cfg.Config.AppBaseURL,
			APIBaseURL:      cfg.Config.APIBaseURL,
			EmailSender: email.Sender{
				Mailer: mailer,
				DefaultFrom: email.Address{
					Email: cfg.Config.EmailFromAddress,
					Name:  cfg.Config.EmailFromName,
				},
			},
			EmailRateLimiter: email.NewRateLimiter(authStore),
			IPRateLimiter:    auth.NewIPRateLimiter(authStore, logger),
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
	var generationHandlerConfig generation.HandlerConfig
	var themeHandlerConfig themes.HandlerConfig
	if generationPlanner != nil {
		generationHandlerConfig.Planner = generationPlanner.BuildPlan
		generationHandlerConfig.BlockSuggester = generationPlanner
		generationHandlerConfig.ImageQueryRewriter = generationPlanner
		generationHandlerConfig.PageChangeSetPlanner = generationPlanner
		generationHandlerConfig.ClarifyingPlanner = generationPlanner
		generationHandlerConfig.DecomposedPlanner = generationPlanner
		themeHandlerConfig.Regenerator = generationPlanner
	}

	var auditRecorder *audit.Recorder
	if auditStore, ok := s.database.(audit.Store); ok {
		auditRecorder = audit.NewRecorder(auditStore)
	}

	var assetService *assets.Service
	if assetStore, ok := s.database.(assets.DB); ok {
		assetStorage, err := assets.NewS3Storage(assets.StorageConfig{
			Endpoint:        s.config.S3Endpoint,
			Bucket:          s.config.S3Bucket,
			Region:          s.config.S3Region,
			AccessKeyID:     s.config.S3AccessKeyID,
			SecretAccessKey: s.config.S3SecretAccessKey,
			ForcePathStyle:  s.config.S3ForcePathStyle,
		})
		if err != nil {
			s.logger.Error("configure asset storage", "error", err)
		} else {
			assetOptions := []assets.Option{assets.WithLogger(s.logger)}
			if auditRecorder != nil {
				assetOptions = append(assetOptions, assets.WithAuditRecorder(auditRecorder))
			}
			assetService = assets.NewService(assetStore, assetStorage, assetOptions...)
		}
	}

	if pexelsClient := imagery.NewPexelsClient(imagery.PexelsConfig{
		APIKey: s.config.PexelsAPIKey,
	}); pexelsClient != nil {
		generationHandlerConfig.StarterImagery = generation.NewStarterImagery(pexelsClient)
		generationHandlerConfig.Logger = s.logger
		if assetService != nil {
			generationHandlerConfig.AssetImporter = assetService
		} else {
			s.logger.Warn("starter imagery configured without asset service; disabling starter imagery")
			generationHandlerConfig.StarterImagery = nil
		}
	}
	if generationHandlerConfig.Logger == nil {
		generationHandlerConfig.Logger = s.logger
	}
	if auditRecorder != nil {
		generationHandlerConfig.AuditRecorder = auditRecorder
	}

	mountAuthenticatedPlaceholderModule(mux, s.auth, workspaces.Module{})
	if store, ok := s.database.(sites.DB); ok {
		sites.NewHandlerWithConfig(store, sites.HandlerConfig{
			PreviewTokenTTL: s.config.PreviewTokenTTL,
			AuditRecorder:   auditRecorder,
			Logger:          s.logger,
		}).Mount(mux, s.auth.RequireSession)
		collectionsConfig := collections.HandlerConfig{}
		if generationPlanner != nil {
			collectionsConfig.Drafter = collectionDrafterAdapter{planner: generationPlanner}
		}
		collections.NewHandlerWithConfig(store, collectionsConfig).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, sites.Module{})
		mountAuthenticatedPlaceholderModule(mux, s.auth, collections.Module{})
	}
	mountAuthenticatedPlaceholderModule(mux, s.auth, pages.Module{})
	mountAuthenticatedPlaceholderModule(mux, s.auth, blocks.Module{})
	if store, ok := s.database.(themes.DB); ok {
		themes.NewHandler(store, themeHandlerConfig).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, themes.Module{})
	}
	if store, ok := s.database.(generation.DB); ok {
		generation.NewHandler(store, generationHandlerConfig).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, generation.Module{})
	}
	publishedSiteCache := publishing.NewPublishedSiteCache()
	if store, ok := s.database.(publishing.DB); ok {
		publishConfig := publishing.ServiceConfig{
			AppBaseURL:       s.config.AppBaseURL,
			PublicBaseURL:    s.config.PublicBaseURL,
			PublicBaseDomain: s.config.PublicBaseDomain,
			ArtifactsDir:     s.config.PublishedArtifactsDir,
			Cache:            publishedSiteCache,
			Logger:           s.logger,
		}
		if strings.EqualFold(s.config.PublishedArtifactsBackend, "s3") {
			artifactStore, err := s.newPublishedArtifactsS3Store()
			if err != nil {
				s.logger.Error("configure published artifacts S3 store", "error", err)
			} else {
				publishConfig.Store = artifactStore
			}
		}
		if assetService != nil {
			publishConfig.AssetProvenance = assetService
		}
		publishHandler := publishing.NewHandlerWithConfig(store, publishConfig, s.config.AppBaseURL, s.config.PublicBaseURL).WithLogger(s.logger)
		if analyticsStore, ok := s.database.(analytics.Store); ok {
			publishHandler = publishHandler.WithViewRecorder(analytics.NewRecorder(analyticsStore, s.logger))
		}
		publishHandler.Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, publishing.Module{})
	}
	if store, ok := s.database.(analytics.DB); ok {
		analytics.NewHandler(store).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, analytics.Module{})
	}
	if store, ok := s.database.(domains.DB); ok {
		domains.NewHandler(store, domains.HandlerConfig{
			AppBaseURL:       s.config.AppBaseURL,
			PublicBaseURL:    s.config.PublicBaseURL,
			PublicBaseDomain: s.config.PublicBaseDomain,
			Cache:            publishedSiteCache,
		}).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, domains.Module{})
	}
	if store, ok := s.database.(assets.DB); ok && assetService != nil {
		assets.NewHandlerWithService(store, assetService).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, assets.Module{})
	}
	if store, ok := s.database.(forms.DB); ok {
		forms.NewHandlerWithConfig(store, forms.HandlerConfig{
			EmailSender: email.Sender{
				Mailer: s.mailer,
				DefaultFrom: email.Address{
					Email: s.config.EmailFromAddress,
					Name:  s.config.EmailFromName,
				},
			},
			EmailRateLimiter: email.NewRateLimiter(store),
			Logger:           s.logger,
			ProductName:      "Snaelda",
		}).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, forms.Module{})
	}
	if store, ok := s.database.(billing.DB); ok {
		billing.NewHandler(store, billing.HandlerConfig{
			AppBaseURL:             s.config.AppBaseURL,
			StripeSecretKey:        s.config.StripeSecretKey,
			StripeWebhookSecret:    s.config.StripeWebhookSecret,
			BasicPriceID:           s.config.StripePriceBasic,
			ProPriceID:             s.config.StripePricePro,
			OnceOverPriceID:        s.config.StripePriceOnceOver,
			BillingSuccessURL:      s.config.BillingSuccessURL,
			BillingCancelURL:       s.config.BillingCancelURL,
			BillingPortalReturnURL: s.config.BillingPortalReturnURL,
			ProductName:            "Snaelda",
			EmailSender: email.Sender{
				Mailer: s.mailer,
				DefaultFrom: email.Address{
					Email: s.config.EmailFromAddress,
					Name:  s.config.EmailFromName,
				},
			},
		}).Mount(mux, s.auth.RequireSession)
	} else {
		mountAuthenticatedPlaceholderModule(mux, s.auth, billing.Module{})
	}

	return s.cors(s.recover(s.logRequests(s.securityHeaders(s.noCache(s.csrf(mux))))))
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
		case origin != "" && s.privateOriginAllowed(origin):
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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

		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !s.privateOriginAllowed(origin) {
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
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy(r))

		next.ServeHTTP(w, r)
	})
}

func (s *Server) privateOriginAllowed(origin string) bool {
	return privateOriginAllowed(origin, s.config.AppBaseURL)
}

func privateOriginAllowed(origin string, appBaseURL string) bool {
	originURL, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || originURL.Scheme == "" || originURL.Host == "" {
		return false
	}
	appURL, err := url.Parse(strings.TrimSpace(appBaseURL))
	if err != nil || appURL.Scheme == "" || appURL.Host == "" {
		return false
	}
	if originURL.Scheme != appURL.Scheme || originURL.Port() != appURL.Port() {
		return false
	}

	originHost := strings.ToLower(originURL.Hostname())
	appHost := strings.ToLower(appURL.Hostname())
	if originHost == appHost {
		return true
	}
	if appHost == "localhost" || strings.HasSuffix(appHost, ".localhost") {
		return false
	}

	baseHost := strings.TrimPrefix(appHost, "www.")
	return originHost == baseHost || originHost == "www."+baseHost
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
	switch r.URL.Path {
	case "/api/auth/login", "/api/auth/magic-link", "/api/sessions/anonymous", "/api/sessions/restore", "/api/billing/webhook":
		return false
	default:
		return true
	}
}

func contentSecurityPolicy(r *http.Request) string {
	if isPublicRoute(r) {
		return strings.Join([]string{
			"default-src 'self'",
			"base-uri 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
			"img-src 'self' data: https:",
			"style-src 'self' 'unsafe-inline'",
			"font-src 'self' data: https:",
			"connect-src 'self'",
			"object-src 'none'",
			"script-src 'none'",
		}, "; ")
	}

	return strings.Join([]string{
		"default-src 'none'",
		"base-uri 'none'",
		"frame-ancestors 'none'",
		"form-action 'none'",
		"object-src 'none'",
		"script-src 'none'",
	}, "; ")
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

// Flush forwards to the underlying ResponseWriter so SSE handlers can stream
// progress events. Without this method the http.Flusher type assertion in
// streamGenerate fails because the embedded interface hides Flush.
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *Server) newPublishedArtifactsS3Store() (publishing.ArtifactStore, error) {
	awsConfig, err := awscfg.LoadDefaultConfig(
		context.Background(),
		awscfg.WithRegion(firstNonEmpty(s.config.S3Region, "us-east-1")),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			s.config.S3AccessKeyID,
			s.config.S3SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = s.config.S3ForcePathStyle
		if endpoint := strings.TrimSpace(s.config.S3Endpoint); endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	})
	return publishing.NewS3ArtifactStore(publishing.S3ArtifactStoreConfig{
		Client: client,
		Bucket: s.config.PublishedArtifactsS3Bucket,
		Prefix: s.config.PublishedArtifactsS3Prefix,
	})
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

// collectionDrafterAdapter lets the OpenAI planner satisfy
// collections.CollectionDrafter without either package importing the other —
// each owns its own request/response types and we shuttle values across.
type collectionDrafterAdapter struct {
	planner *generation.OpenAIPlanner
}

func (a collectionDrafterAdapter) DraftCollection(ctx context.Context, request collections.CollectionDraftRequest) (collections.CollectionDraftResponse, error) {
	resp, err := a.planner.DraftCollection(ctx, generation.CollectionDraftRequest{
		Prompt:              request.Prompt,
		SiteName:            request.SiteName,
		SiteGoal:            request.SiteGoal,
		ExistingCollections: request.ExistingCollections,
	})
	if err != nil {
		return collections.CollectionDraftResponse{}, err
	}
	return collections.CollectionDraftResponse{
		Slug:          resp.Slug,
		SingularLabel: resp.SingularLabel,
		PluralLabel:   resp.PluralLabel,
		Schema:        resp.Schema,
	}, nil
}
