package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                 string
	HTTPAddr               string
	AppBaseURL             string
	APIBaseURL             string
	PublicBaseURL          string
	PublicBaseDomain       string
	StripeSecretKey        string
	StripeWebhookSecret    string
	StripePriceBasic       string
	StripePricePro         string
	BillingSuccessURL      string
	BillingCancelURL       string
	BillingPortalReturnURL string
	EmailTransport         string
	EmailFromAddress       string
	EmailFromName          string
	EmailReplyTo           string
	ResendAPIKey           string
	MailpitSMTPAddr        string
	OpenAIAPIKey           string
	OpenAIModel            string
	PexelsAPIKey           string
	DatabaseURL            string
	AuthJWTSecret          string
	AuthIssuer             string
	AuthAudience           string
	AuthAccessTokenTTL     time.Duration
	AuthRefreshTokenTTL    time.Duration
	AuthCookieSecure       bool
	PreviewTokenTTL        time.Duration
	PublishedArtifactsDir  string
	S3Endpoint             string
	S3Bucket               string
	S3Region               string
	S3AccessKeyID          string
	S3SecretAccessKey      string
	S3ForcePathStyle       bool
}

func Load() (Config, error) {
	appEnv := getEnv("APP_ENV", "development")
	forcePathStyle, err := getEnvBool("S3_FORCE_PATH_STYLE", true)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := getEnvDuration("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTokenTTL, err := getEnvDuration("AUTH_REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	previewTokenTTL, err := getEnvDuration("PREVIEW_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cookieSecure, err := getEnvBool("AUTH_COOKIE_SECURE", appEnv != "development" && appEnv != "test")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:                 appEnv,
		HTTPAddr:               getEnv("HTTP_ADDR", ":8080"),
		AppBaseURL:             getEnv("APP_BASE_URL", "http://localhost:3000"),
		APIBaseURL:             getEnv("API_BASE_URL", "http://localhost:8080"),
		PublicBaseURL:          getEnv("PUBLIC_BASE_URL", "http://localhost:3000"),
		StripeSecretKey:        strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")),
		StripeWebhookSecret:    strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		StripePriceBasic:       strings.TrimSpace(os.Getenv("STRIPE_PRICE_BASIC")),
		StripePricePro:         strings.TrimSpace(os.Getenv("STRIPE_PRICE_PRO")),
		BillingSuccessURL:      strings.TrimSpace(getEnv("BILLING_SUCCESS_URL", "http://localhost:3000/app/billing/success")),
		BillingCancelURL:       strings.TrimSpace(getEnv("BILLING_CANCEL_URL", "http://localhost:3000/app/billing/cancel")),
		BillingPortalReturnURL: strings.TrimSpace(getEnv("BILLING_PORTAL_RETURN_URL", "http://localhost:3000/app/billing")),
		EmailTransport:         strings.ToLower(strings.TrimSpace(getEnv("EMAIL_TRANSPORT", "stdout"))),
		EmailFromAddress:       strings.TrimSpace(getEnv("EMAIL_FROM_ADDRESS", "hi@snaelda.app")),
		EmailFromName:          strings.TrimSpace(getEnv("EMAIL_FROM_NAME", "Snaelda")),
		EmailReplyTo:           strings.TrimSpace(os.Getenv("EMAIL_REPLY_TO")),
		ResendAPIKey:           strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		MailpitSMTPAddr:        strings.TrimSpace(getEnv("MAILPIT_SMTP_ADDR", "localhost:1025")),
		OpenAIAPIKey:           strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIModel:            getEnv("OPENAI_MODEL", "gpt-5-mini"),
		PexelsAPIKey:           strings.TrimSpace(os.Getenv("PEXELS_API_KEY")),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		AuthJWTSecret:          getEnv("AUTH_JWT_SECRET", "development-auth-secret-change-me"),
		AuthIssuer:             getEnv("AUTH_ISSUER", "snaelda-api"),
		AuthAudience:           getEnv("AUTH_AUDIENCE", "snaelda-web"),
		AuthAccessTokenTTL:     accessTokenTTL,
		AuthRefreshTokenTTL:    refreshTokenTTL,
		AuthCookieSecure:       cookieSecure,
		PreviewTokenTTL:        previewTokenTTL,
		PublishedArtifactsDir:  getEnv("PUBLISHED_ARTIFACTS_DIR", "var/published-artifacts"),
		S3Endpoint:             getEnv("S3_ENDPOINT", "http://localhost:8333"),
		S3Bucket:               getEnv("S3_BUCKET", "snaelda-local"),
		S3Region:               getEnv("S3_REGION", "us-east-1"),
		S3AccessKeyID:          getEnv("S3_ACCESS_KEY_ID", "snaelda"),
		S3SecretAccessKey:      getEnv("S3_SECRET_ACCESS_KEY", "snaelda-secret"),
		S3ForcePathStyle:       forcePathStyle,
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
	publicBaseDomain, err := resolvePublicBaseDomain(cfg.PublicBaseURL, os.Getenv("PUBLIC_BASE_DOMAIN"))
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
	if (cfg.StripeSecretKey != "" || cfg.StripeWebhookSecret != "" || cfg.StripePriceBasic != "" || cfg.StripePricePro != "") &&
		(cfg.BillingSuccessURL == "" || cfg.BillingCancelURL == "" || cfg.BillingPortalReturnURL == "") {
		return Config{}, fmt.Errorf("billing urls are required when Stripe billing is configured")
	}
	if cfg.StripeWebhookSecret != "" && cfg.StripeSecretKey == "" {
		return Config{}, fmt.Errorf("STRIPE_SECRET_KEY is required when STRIPE_WEBHOOK_SECRET is set")
	}
	if cfg.StripeSecretKey != "" && cfg.StripePriceBasic == "" && cfg.StripePricePro == "" {
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

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}

	return parsed, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}

	return parsed, nil
}
