package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv                string
	HTTPAddr              string
	AppBaseURL            string
	DatabaseURL           string
	AuthJWTSecret         string
	AuthIssuer            string
	AuthAudience          string
	AuthAccessTokenTTL    time.Duration
	AuthRefreshTokenTTL   time.Duration
	AuthCookieSecure      bool
	PublishedArtifactsDir string
	S3Endpoint            string
	S3Bucket              string
	S3Region              string
	S3AccessKeyID         string
	S3SecretAccessKey     string
	S3ForcePathStyle      bool
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
	cookieSecure, err := getEnvBool("AUTH_COOKIE_SECURE", appEnv != "development" && appEnv != "test")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:                appEnv,
		HTTPAddr:              getEnv("HTTP_ADDR", ":8080"),
		AppBaseURL:            getEnv("APP_BASE_URL", "http://localhost:3000"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AuthJWTSecret:         getEnv("AUTH_JWT_SECRET", "development-auth-secret-change-me"),
		AuthIssuer:            getEnv("AUTH_ISSUER", "snaelda-api"),
		AuthAudience:          getEnv("AUTH_AUDIENCE", "snaelda-web"),
		AuthAccessTokenTTL:    accessTokenTTL,
		AuthRefreshTokenTTL:   refreshTokenTTL,
		AuthCookieSecure:      cookieSecure,
		PublishedArtifactsDir: getEnv("PUBLISHED_ARTIFACTS_DIR", "var/published-artifacts"),
		S3Endpoint:            getEnv("S3_ENDPOINT", "http://localhost:8333"),
		S3Bucket:              getEnv("S3_BUCKET", "snaelda-local"),
		S3Region:              getEnv("S3_REGION", "us-east-1"),
		S3AccessKeyID:         getEnv("S3_ACCESS_KEY_ID", "snaelda"),
		S3SecretAccessKey:     getEnv("S3_SECRET_ACCESS_KEY", "snaelda-secret"),
		S3ForcePathStyle:      forcePathStyle,
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

	return cfg, nil
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
