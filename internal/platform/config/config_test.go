package config

import (
	"testing"
	"time"
)

func TestLoadUsesLocalStorageDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.S3Endpoint != "http://localhost:8333" {
		t.Fatalf("expected local S3 endpoint, got %q", cfg.S3Endpoint)
	}
	if cfg.S3Bucket != "snaelda-local" {
		t.Fatalf("expected local S3 bucket, got %q", cfg.S3Bucket)
	}
	if cfg.S3Region != "us-east-1" {
		t.Fatalf("expected local S3 region, got %q", cfg.S3Region)
	}
	if cfg.S3AccessKeyID != "snaelda" {
		t.Fatalf("expected local S3 access key, got %q", cfg.S3AccessKeyID)
	}
	if cfg.S3SecretAccessKey != "snaelda-secret" {
		t.Fatalf("expected local S3 secret key, got %q", cfg.S3SecretAccessKey)
	}
	if !cfg.S3ForcePathStyle {
		t.Fatal("expected path-style S3 addressing for local SeaweedFS")
	}
	if cfg.AuthIssuer != "snaelda-api" {
		t.Fatalf("expected default auth issuer, got %q", cfg.AuthIssuer)
	}
	if cfg.AuthAudience != "snaelda-web" {
		t.Fatalf("expected default auth audience, got %q", cfg.AuthAudience)
	}
	if cfg.AuthAccessTokenTTL != 15*time.Minute {
		t.Fatalf("expected default auth access token ttl, got %s", cfg.AuthAccessTokenTTL)
	}
	if cfg.AuthCookieSecure {
		t.Fatal("expected insecure auth cookie in test")
	}
}

func TestLoadAllowsStorageOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("S3_ENDPOINT", "http://storage.example.test")
	t.Setenv("S3_BUCKET", "custom-bucket")
	t.Setenv("S3_REGION", "eu-north-1")
	t.Setenv("S3_ACCESS_KEY_ID", "custom-key")
	t.Setenv("S3_SECRET_ACCESS_KEY", "custom-secret")
	t.Setenv("S3_FORCE_PATH_STYLE", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.S3Endpoint != "http://storage.example.test" {
		t.Fatalf("expected overridden S3 endpoint, got %q", cfg.S3Endpoint)
	}
	if cfg.S3Bucket != "custom-bucket" {
		t.Fatalf("expected overridden S3 bucket, got %q", cfg.S3Bucket)
	}
	if cfg.S3Region != "eu-north-1" {
		t.Fatalf("expected overridden S3 region, got %q", cfg.S3Region)
	}
	if cfg.S3AccessKeyID != "custom-key" {
		t.Fatalf("expected overridden S3 access key, got %q", cfg.S3AccessKeyID)
	}
	if cfg.S3SecretAccessKey != "custom-secret" {
		t.Fatalf("expected overridden S3 secret key, got %q", cfg.S3SecretAccessKey)
	}
	if cfg.S3ForcePathStyle {
		t.Fatal("expected path-style override to be false")
	}
}

func TestLoadRejectsInvalidStorageBool(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("S3_FORCE_PATH_STYLE", "sometimes")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid bool error")
	}
}

func TestLoadRejectsInvalidAuthDuration(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "soon")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid auth duration error")
	}
}

func TestLoadRequiresProductionAuthSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	unsetStorageEnv(t)

	if _, err := Load(); err == nil {
		t.Fatal("expected production auth secret error")
	}
}

func unsetStorageEnv(t *testing.T) {
	t.Helper()

	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_REGION", "")
	t.Setenv("S3_ACCESS_KEY_ID", "")
	t.Setenv("S3_SECRET_ACCESS_KEY", "")
	t.Setenv("S3_FORCE_PATH_STYLE", "")
}
