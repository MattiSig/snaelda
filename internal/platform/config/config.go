package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                     string
	HTTPAddr                   string
	AppBaseURL                 string
	APIBaseURL                 string
	PublicBaseURL              string
	PublicBaseDomain           string
	StripeSecretKey            string
	StripeWebhookSecret        string
	StripePriceBasic           string
	StripePricePro             string
	StripePriceOnceOver        string
	BillingSuccessURL          string
	BillingCancelURL           string
	BillingPortalReturnURL     string
	EmailTransport             string
	EmailFromAddress           string
	EmailFromName              string
	EmailReplyTo               string
	ResendAPIKey               string
	MailpitSMTPAddr            string
	OpenAIAPIKey               string
	OpenAIModel                string
	PexelsAPIKey               string
	DatabaseURL                string
	AuthJWTSecret              string
	AuthIssuer                 string
	AuthAudience               string
	AuthAccessTokenTTL         time.Duration
	AuthRefreshTokenTTL        time.Duration
	AuthCookieSecure           bool
	PreviewTokenTTL            time.Duration
	PublishedArtifactsDir      string
	PublishedArtifactsBackend  string
	PublishedArtifactsS3Bucket string
	PublishedArtifactsS3Prefix string
	S3Endpoint                 string
	S3Bucket                   string
	S3Region                   string
	S3AccessKeyID              string
	S3SecretAccessKey          string
	S3ForcePathStyle           bool
}

func Load() (Config, error) {
	env, err := loadEnvironment()
	if err != nil {
		return Config{}, err
	}

	appEnv := env.get("APP_ENV", "development")
	forcePathStyle, err := env.getBool("S3_FORCE_PATH_STYLE", true)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := env.getDuration("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTokenTTL, err := env.getDuration("AUTH_REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	previewTokenTTL, err := env.getDuration("PREVIEW_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cookieSecure, err := env.getBool("AUTH_COOKIE_SECURE", appEnv != "development" && appEnv != "test")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:                     appEnv,
		HTTPAddr:                   resolveHTTPAddr(env),
		AppBaseURL:                 env.get("APP_BASE_URL", "http://localhost:3000"),
		APIBaseURL:                 env.get("API_BASE_URL", "http://localhost:8080"),
		PublicBaseURL:              env.get("PUBLIC_BASE_URL", "http://localhost:3000"),
		StripeSecretKey:            strings.TrimSpace(env.lookup("STRIPE_SECRET_KEY")),
		StripeWebhookSecret:        strings.TrimSpace(env.lookup("STRIPE_WEBHOOK_SECRET")),
		StripePriceBasic:           strings.TrimSpace(env.lookup("STRIPE_PRICE_BASIC")),
		StripePricePro:             strings.TrimSpace(env.lookup("STRIPE_PRICE_PRO")),
		StripePriceOnceOver:        strings.TrimSpace(env.lookup("STRIPE_PRICE_ONCE_OVER")),
		BillingSuccessURL:          strings.TrimSpace(env.get("BILLING_SUCCESS_URL", "http://localhost:3000/app/billing/success")),
		BillingCancelURL:           strings.TrimSpace(env.get("BILLING_CANCEL_URL", "http://localhost:3000/app/billing/cancel")),
		BillingPortalReturnURL:     strings.TrimSpace(env.get("BILLING_PORTAL_RETURN_URL", "http://localhost:3000/app/billing")),
		EmailTransport:             strings.ToLower(strings.TrimSpace(env.get("EMAIL_TRANSPORT", "stdout"))),
		EmailFromAddress:           strings.TrimSpace(env.get("EMAIL_FROM_ADDRESS", "hi@snaelda.app")),
		EmailFromName:              strings.TrimSpace(env.get("EMAIL_FROM_NAME", "Snaelda")),
		EmailReplyTo:               strings.TrimSpace(env.lookup("EMAIL_REPLY_TO")),
		ResendAPIKey:               strings.TrimSpace(env.lookup("RESEND_API_KEY")),
		MailpitSMTPAddr:            strings.TrimSpace(env.get("MAILPIT_SMTP_ADDR", "localhost:1025")),
		OpenAIAPIKey:               strings.TrimSpace(env.lookup("OPENAI_API_KEY")),
		OpenAIModel:                env.get("OPENAI_MODEL", "gpt-5-mini"),
		PexelsAPIKey:               strings.TrimSpace(env.lookup("PEXELS_API_KEY")),
		DatabaseURL:                env.lookup("DATABASE_URL"),
		AuthJWTSecret:              env.get("AUTH_JWT_SECRET", "development-auth-secret-change-me"),
		AuthIssuer:                 env.get("AUTH_ISSUER", "snaelda-api"),
		AuthAudience:               env.get("AUTH_AUDIENCE", "snaelda-web"),
		AuthAccessTokenTTL:         accessTokenTTL,
		AuthRefreshTokenTTL:        refreshTokenTTL,
		AuthCookieSecure:           cookieSecure,
		PreviewTokenTTL:            previewTokenTTL,
		PublishedArtifactsDir:      env.get("PUBLISHED_ARTIFACTS_DIR", "var/published-artifacts"),
		PublishedArtifactsBackend:  strings.ToLower(strings.TrimSpace(env.lookup("PUBLISHED_ARTIFACTS_BACKEND"))),
		PublishedArtifactsS3Bucket: strings.TrimSpace(env.lookup("PUBLISHED_ARTIFACTS_S3_BUCKET")),
		PublishedArtifactsS3Prefix: strings.TrimSpace(env.get("PUBLISHED_ARTIFACTS_S3_PREFIX", "published-artifacts")),
		S3Endpoint:                 env.get("S3_ENDPOINT", "http://localhost:8333"),
		S3Bucket:                   env.get("S3_BUCKET", "snaelda-local"),
		S3Region:                   env.get("S3_REGION", "us-east-1"),
		S3AccessKeyID:              env.get("S3_ACCESS_KEY_ID", "snaelda"),
		S3SecretAccessKey:          env.get("S3_SECRET_ACCESS_KEY", "snaelda-secret"),
		S3ForcePathStyle:           forcePathStyle,
	}

	if cfg.AppEnv == "" {
		return Config{}, fmt.Errorf("APP_ENV is required")
	}
	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR is required")
	}
	if cfg.AppEnv != "test" && cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	publicBaseDomain, err := resolvePublicBaseDomain(cfg.PublicBaseURL, env.lookup("PUBLIC_BASE_DOMAIN"))
	if err != nil {
		return Config{}, err
	}
	cfg.PublicBaseDomain = publicBaseDomain
	if cfg.AuthJWTSecret == "" {
		return Config{}, fmt.Errorf("AUTH_JWT_SECRET is required")
	}
	if cfg.AppEnv == "production" && cfg.AuthJWTSecret == "development-auth-secret-change-me" {
		return Config{}, fmt.Errorf("AUTH_JWT_SECRET must be set in production")
	}
	if cfg.AuthIssuer == "" {
		return Config{}, fmt.Errorf("AUTH_ISSUER is required")
	}
	if cfg.AuthAudience == "" {
		return Config{}, fmt.Errorf("AUTH_AUDIENCE is required")
	}
	if cfg.AuthAccessTokenTTL <= 0 {
		return Config{}, fmt.Errorf("AUTH_ACCESS_TOKEN_TTL must be positive")
	}
	if cfg.AuthRefreshTokenTTL <= 0 {
		return Config{}, fmt.Errorf("AUTH_REFRESH_TOKEN_TTL must be positive")
	}
	if cfg.PreviewTokenTTL <= 0 {
		return Config{}, fmt.Errorf("PREVIEW_TOKEN_TTL must be positive")
	}
	if cfg.PublicBaseURL == "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL is required")
	}
	if cfg.PublicBaseDomain == "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_DOMAIN is required")
	}
	if cfg.PublishedArtifactsS3Bucket == "" {
		cfg.PublishedArtifactsS3Bucket = cfg.S3Bucket
	}
	if cfg.PublishedArtifactsBackend == "" {
		if cfg.PublishedArtifactsS3Bucket != "" {
			cfg.PublishedArtifactsBackend = "s3"
		} else {
			cfg.PublishedArtifactsBackend = "local"
		}
	}
	switch cfg.PublishedArtifactsBackend {
	case "s3", "local":
	default:
		return Config{}, fmt.Errorf("PUBLISHED_ARTIFACTS_BACKEND must be one of s3, local")
	}
	if cfg.PublishedArtifactsBackend == "s3" && cfg.PublishedArtifactsS3Bucket == "" {
		return Config{}, fmt.Errorf("PUBLISHED_ARTIFACTS_S3_BUCKET (or S3_BUCKET) is required when PUBLISHED_ARTIFACTS_BACKEND=s3")
	}
	if (cfg.StripeSecretKey != "" || cfg.StripeWebhookSecret != "" || cfg.StripePriceBasic != "" || cfg.StripePricePro != "" || cfg.StripePriceOnceOver != "") &&
		(cfg.BillingSuccessURL == "" || cfg.BillingCancelURL == "" || cfg.BillingPortalReturnURL == "") {
		return Config{}, fmt.Errorf("billing urls are required when Stripe billing is configured")
	}
	if cfg.StripeWebhookSecret != "" && cfg.StripeSecretKey == "" {
		return Config{}, fmt.Errorf("STRIPE_SECRET_KEY is required when STRIPE_WEBHOOK_SECRET is set")
	}
	if cfg.StripeSecretKey != "" && cfg.StripePriceBasic == "" && cfg.StripePricePro == "" && cfg.StripePriceOnceOver == "" {
		return Config{}, fmt.Errorf("at least one Stripe price id is required when STRIPE_SECRET_KEY is set")
	}
	switch cfg.EmailTransport {
	case "stdout", "mailpit", "resend":
	default:
		return Config{}, fmt.Errorf("EMAIL_TRANSPORT must be one of stdout, mailpit, resend")
	}
	if cfg.EmailFromAddress == "" {
		return Config{}, fmt.Errorf("EMAIL_FROM_ADDRESS is required")
	}
	if cfg.EmailTransport == "resend" && cfg.ResendAPIKey == "" {
		return Config{}, fmt.Errorf("RESEND_API_KEY is required when EMAIL_TRANSPORT=resend")
	}
	if cfg.OpenAIAPIKey == "" {
		cfg.OpenAIModel = strings.TrimSpace(cfg.OpenAIModel)
	} else if strings.TrimSpace(cfg.OpenAIModel) == "" {
		return Config{}, fmt.Errorf("OPENAI_MODEL is required when OPENAI_API_KEY is set")
	}

	return cfg, nil
}

type environment struct {
	values map[string]string
}

func loadEnvironment() (environment, error) {
	values := map[string]string{}

	if err := mergeDotEnvFile(values, ".env.local"); err != nil {
		return environment{}, err
	}
	if err := mergeDotEnvFile(values, ".env"); err != nil {
		return environment{}, err
	}

	for _, pair := range os.Environ() {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		values[key] = value
	}

	return environment{values: values}, nil
}

func mergeDotEnvFile(values map[string]string, path string) error {
	fileValues, err := godotenv.Read(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	for key, value := range fileValues {
		values[key] = value
	}

	return nil
}

func resolvePublicBaseDomain(publicBaseURL string, override string) (string, error) {
	if trimmed := normalizeHostname(override); trimmed != "" {
		return trimmed, nil
	}

	parsed, err := url.Parse(strings.TrimSpace(publicBaseURL))
	if err != nil {
		return "", fmt.Errorf("PUBLIC_BASE_URL must be a valid URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("PUBLIC_BASE_URL must include scheme and host")
	}

	return normalizeHostname(parsed.Hostname()), nil
}

func normalizeHostname(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}

func resolveHTTPAddr(env environment) string {
	if addr := strings.TrimSpace(env.lookup("HTTP_ADDR")); addr != "" {
		return addr
	}
	if port := strings.TrimSpace(env.lookup("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}
	return ":8080"
}

func (e environment) lookup(key string) string {
	return e.values[key]
}

func (e environment) get(key string, fallback string) string {
	value := e.lookup(key)
	if value == "" {
		return fallback
	}
	return value
}

func (e environment) getBool(key string, fallback bool) (bool, error) {
	value := e.lookup(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}

	return parsed, nil
}

func (e environment) getDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := e.lookup(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}

	return parsed, nil
}
