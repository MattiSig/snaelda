package config

import (
	"os"
	"path/filepath"
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
	if cfg.AuthRefreshTokenTTL != 30*24*time.Hour {
		t.Fatalf("expected default auth refresh token ttl, got %s", cfg.AuthRefreshTokenTTL)
	}
	if cfg.PreviewTokenTTL != 7*24*time.Hour {
		t.Fatalf("expected default preview token ttl, got %s", cfg.PreviewTokenTTL)
	}
	if cfg.AuthCookieSecure {
		t.Fatal("expected insecure auth cookie in test")
	}
	if cfg.PublishedArtifactsDir != "var/published-artifacts" {
		t.Fatalf("expected default published artifacts dir, got %q", cfg.PublishedArtifactsDir)
	}
	if cfg.PublicBaseURL != "http://localhost:3000" {
		t.Fatalf("expected default public base url, got %q", cfg.PublicBaseURL)
	}
	if cfg.PublicBaseDomain != "localhost" {
		t.Fatalf("expected derived localhost public base domain, got %q", cfg.PublicBaseDomain)
	}
	if cfg.BillingSuccessURL != "http://localhost:3000/app/billing/success" {
		t.Fatalf("expected default billing success url, got %q", cfg.BillingSuccessURL)
	}
	if cfg.BillingCancelURL != "http://localhost:3000/app/billing/cancel" {
		t.Fatalf("expected default billing cancel url, got %q", cfg.BillingCancelURL)
	}
	if cfg.BillingPortalReturnURL != "http://localhost:3000/app/billing" {
		t.Fatalf("expected default billing portal return url, got %q", cfg.BillingPortalReturnURL)
	}
	if cfg.EmailTransport != "stdout" {
		t.Fatalf("expected default email transport, got %q", cfg.EmailTransport)
	}
	if cfg.EmailFromAddress != "hi@snaelda.app" {
		t.Fatalf("expected default email from address, got %q", cfg.EmailFromAddress)
	}
	if cfg.MailpitSMTPAddr != "localhost:1025" {
		t.Fatalf("expected default mailpit smtp addr, got %q", cfg.MailpitSMTPAddr)
	}
	if cfg.OpenAIModel != "gpt-5-mini" {
		t.Fatalf("expected default OpenAI model, got %q", cfg.OpenAIModel)
	}
	if len(cfg.OperatorEmails) != 0 {
		t.Fatalf("expected no operator emails by default, got %#v", cfg.OperatorEmails)
	}
}

func TestLoadUsesRailwayPortWhenHTTPAddrMissing(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "4242")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddr != ":4242" {
		t.Fatalf("expected PORT-derived HTTP addr, got %q", cfg.HTTPAddr)
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
	t.Setenv("PUBLISHED_ARTIFACTS_DIR", "/tmp/snaelda-artifacts")
	t.Setenv("PREVIEW_TOKEN_TTL", "96h")
	t.Setenv("PUBLIC_BASE_URL", "https://sites.snaelda.test")
	t.Setenv("PUBLIC_BASE_DOMAIN", "sites.snaelda.test")
	t.Setenv("BILLING_SUCCESS_URL", "https://app.snaelda.test/billing/success")
	t.Setenv("BILLING_CANCEL_URL", "https://app.snaelda.test/billing/cancel")
	t.Setenv("BILLING_PORTAL_RETURN_URL", "https://app.snaelda.test/billing")

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
	if cfg.PublishedArtifactsDir != "/tmp/snaelda-artifacts" {
		t.Fatalf("expected overridden published artifacts dir, got %q", cfg.PublishedArtifactsDir)
	}
	if cfg.PreviewTokenTTL != 96*time.Hour {
		t.Fatalf("expected overridden preview token ttl, got %s", cfg.PreviewTokenTTL)
	}
	if cfg.PublicBaseURL != "https://sites.snaelda.test" {
		t.Fatalf("expected overridden public base url, got %q", cfg.PublicBaseURL)
	}
	if cfg.PublicBaseDomain != "sites.snaelda.test" {
		t.Fatalf("expected overridden public base domain, got %q", cfg.PublicBaseDomain)
	}
	if cfg.BillingSuccessURL != "https://app.snaelda.test/billing/success" {
		t.Fatalf("expected overridden billing success url, got %q", cfg.BillingSuccessURL)
	}
	if cfg.BillingCancelURL != "https://app.snaelda.test/billing/cancel" {
		t.Fatalf("expected overridden billing cancel url, got %q", cfg.BillingCancelURL)
	}
	if cfg.BillingPortalReturnURL != "https://app.snaelda.test/billing" {
		t.Fatalf("expected overridden billing portal return url, got %q", cfg.BillingPortalReturnURL)
	}
}

func TestLoadParsesOperatorEmails(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OPERATOR_EMAILS", " maker@snaelda.app, support@snaelda.app ,,MAKER+alt@snaelda.app ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got, want := len(cfg.OperatorEmails), 3; got != want {
		t.Fatalf("expected %d operator emails, got %#v", want, cfg.OperatorEmails)
	}
	if cfg.OperatorEmails[0] != "maker@snaelda.app" || cfg.OperatorEmails[1] != "support@snaelda.app" || cfg.OperatorEmails[2] != "maker+alt@snaelda.app" {
		t.Fatalf("unexpected operator emails %#v", cfg.OperatorEmails)
	}
}

func TestLoadPrefersProcessEnvThenDotEnvThenDotEnvLocal(t *testing.T) {
	unsetEnv(t, "APP_ENV", "S3_BUCKET", "S3_REGION", "PUBLIC_BASE_URL", "PUBLIC_BASE_DOMAIN")
	t.Setenv("S3_BUCKET", "process-bucket")

	tempDir := t.TempDir()
	writeEnvFile(t, filepath.Join(tempDir, ".env.local"), "APP_ENV=test\nS3_BUCKET=local-bucket\nS3_REGION=local-region\nPUBLIC_BASE_URL=http://local.test\nPUBLIC_BASE_DOMAIN=local.test\n")
	writeEnvFile(t, filepath.Join(tempDir, ".env"), "APP_ENV=test\nS3_BUCKET=dotenv-bucket\nS3_REGION=dotenv-region\nPUBLIC_BASE_URL=http://dotenv.test\nPUBLIC_BASE_DOMAIN=dotenv.test\n")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.AppEnv != "test" {
		t.Fatalf("expected APP_ENV from dotenv files, got %q", cfg.AppEnv)
	}
	if cfg.S3Bucket != "process-bucket" {
		t.Fatalf("expected process env to win for S3 bucket, got %q", cfg.S3Bucket)
	}
	if cfg.S3Region != "dotenv-region" {
		t.Fatalf("expected .env to override .env.local for S3 region, got %q", cfg.S3Region)
	}
	if cfg.PublicBaseURL != "http://dotenv.test" {
		t.Fatalf("expected .env to override .env.local for public base url, got %q", cfg.PublicBaseURL)
	}
	if cfg.PublicBaseDomain != "dotenv.test" {
		t.Fatalf("expected .env to override .env.local for public base domain, got %q", cfg.PublicBaseDomain)
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

func TestLoadRejectsInvalidRefreshAuthDuration(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("AUTH_REFRESH_TOKEN_TTL", "later")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid auth refresh duration error")
	}
}

func TestLoadRejectsInvalidPreviewTokenDuration(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("PREVIEW_TOKEN_TTL", "briefly")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid preview token duration error")
	}
}

func TestLoadRejectsInvalidPublicBaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("PUBLIC_BASE_URL", "not-a-url")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid public base url error")
	}
}

func TestLoadRejectsInvalidEmailTransport(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("EMAIL_TRANSPORT", "carrier-pigeon")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid email transport error")
	}
}

func TestLoadRequiresResendAPIKey(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("EMAIL_TRANSPORT", "resend")
	t.Setenv("RESEND_API_KEY", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected resend api key error")
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

func TestLoadRequiresOpenAIModelWhenAPIKeyIsSet(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", " ")

	if _, err := Load(); err == nil {
		t.Fatal("expected openai model error")
	}
}

func TestLoadRequiresStripePriceWhenStripeSecretIsSet(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_PRICE_BASIC", "")
	t.Setenv("STRIPE_PRICE_PRO", "")
	t.Setenv("STRIPE_PRICE_ONCE_OVER", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected stripe price error")
	}
}

func TestLoadRequiresStripeSecretWhenWebhookSecretIsSet(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	unsetStorageEnv(t)
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")

	if _, err := Load(); err == nil {
		t.Fatal("expected stripe secret error")
	}
}

func TestLoadRequiresHTTPSURLsInProduction(t *testing.T) {
	setProductionEnv(t)
	t.Setenv("APP_BASE_URL", "http://app.snaelda.test")

	if _, err := Load(); err == nil {
		t.Fatal("expected production https url error")
	}
}

func TestLoadRequiresPublicNonLocalDomainInProduction(t *testing.T) {
	setProductionEnv(t)
	t.Setenv("PUBLIC_BASE_URL", "https://localhost:3000")
	t.Setenv("PUBLIC_BASE_DOMAIN", "localhost")

	if _, err := Load(); err == nil {
		t.Fatal("expected public non-local domain error")
	}
}

func TestLoadRequiresProductionStripeConfiguration(t *testing.T) {
	setProductionEnv(t)
	t.Setenv("STRIPE_SECRET_KEY", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected production stripe configuration error")
	}
}

func TestLoadRequiresResendInProduction(t *testing.T) {
	setProductionEnv(t)
	t.Setenv("EMAIL_TRANSPORT", "mailpit")

	if _, err := Load(); err == nil {
		t.Fatal("expected production email transport error")
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
	t.Setenv("PUBLISHED_ARTIFACTS_DIR", "")
	t.Setenv("PREVIEW_TOKEN_TTL", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	t.Setenv("PUBLIC_BASE_DOMAIN", "")
}

func setProductionEnv(t *testing.T) {
	t.Helper()

	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("AUTH_JWT_SECRET", "production-secret")
	t.Setenv("APP_BASE_URL", "https://app.snaelda.test")
	t.Setenv("PUBLIC_BASE_URL", "https://sites.snaelda.test")
	t.Setenv("PUBLIC_BASE_DOMAIN", "sites.snaelda.test")
	t.Setenv("BILLING_SUCCESS_URL", "https://app.snaelda.test/app/billing/success")
	t.Setenv("BILLING_CANCEL_URL", "https://app.snaelda.test/app/billing/cancel")
	t.Setenv("BILLING_PORTAL_RETURN_URL", "https://app.snaelda.test/app/billing")
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_PRICE_BASIC", "price_basic")
	t.Setenv("STRIPE_PRICE_PRO", "price_pro")
	t.Setenv("EMAIL_TRANSPORT", "resend")
	t.Setenv("RESEND_API_KEY", "re_123")
}

func writeEnvFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write env file %s: %v", path, err)
	}
}

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		previousValue, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
		t.Cleanup(func() {
			var err error
			if existed {
				err = os.Setenv(key, previousValue)
			} else {
				err = os.Unsetenv(key)
			}
			if err != nil {
				t.Fatalf("restore env %s: %v", key, err)
			}
		})
	}
}
