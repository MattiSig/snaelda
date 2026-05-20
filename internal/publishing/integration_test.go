//go:build integration

package publishing

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// TestWorkerRendererAndS3StoreRoundTrip exercises the long-lived Node render
// worker and the S3 artifact store end-to-end against the local SeaweedFS in
// compose.yaml. Run with: go test -tags=integration ./internal/publishing/
func TestWorkerRendererAndS3StoreRoundTrip(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if os.Getenv("INTEGRATION_VERBOSE") != "" {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	endpoint := envOr("S3_ENDPOINT", "http://localhost:8333")
	bucket := envOr("S3_BUCKET", "snaelda-local")

	awsConfig, err := awscfg.LoadDefaultConfig(
		context.Background(),
		awscfg.WithRegion(envOr("S3_REGION", "us-east-1")),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			envOr("S3_ACCESS_KEY_ID", "snaelda"),
			envOr("S3_SECRET_ACCESS_KEY", "snaelda-secret"),
			"",
		)),
	)
	if err != nil {
		t.Fatalf("aws config: %v", err)
	}
	client := s3.NewFromConfig(awsConfig, func(opt *s3.Options) {
		opt.UsePathStyle = true
		opt.BaseEndpoint = aws.String(endpoint)
	})
	store, err := NewS3ArtifactStore(S3ArtifactStoreConfig{
		Client: client,
		Bucket: bucket,
		Prefix: "integration-tests",
	})
	if err != nil {
		t.Fatalf("create s3 artifact store: %v", err)
	}

	renderer := NewWorkerArtifactRenderer(WorkerRendererConfig{
		PublicBaseURL: "http://localhost:3000",
		Logger:        logger,
	})
	t.Cleanup(func() {
		if closer, ok := renderer.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	})

	siteID := "00000000-0000-4000-8000-0000000000ff"
	versionID := "00000000-0000-4000-8000-0000000a0001"
	t.Cleanup(func() {
		if deleter, ok := store.(interface {
			Delete(ctx context.Context, siteID string, versionID string) error
		}); ok {
			_ = deleter.Delete(context.Background(), siteID, versionID)
		}
	})

	input := ArtifactRenderInput{
		PublicBaseURL: "http://localhost:3000",
		SiteSlug:      "integration-test",
		Hostname:      "integration-test.localhost",
		Version: VersionSummary{
			ID:            versionID,
			SiteID:        siteID,
			VersionNumber: 1,
			CreatedAt:     time.Now().UTC(),
		},
		Snapshot: siteconfig.PublishedSnapshot{
			SchemaVersion: siteconfig.SiteConfigVersionV1,
			Site: siteconfig.PublishedSite{
				ID:            "site_integration",
				Name:          "Integration Test",
				DefaultLocale: "en",
				SEO: siteconfig.SEOConfig{
					Title:       "Integration Test",
					Description: "End-to-end publish pipeline smoke test.",
				},
			},
			Theme: siteconfig.ThemeConfig{
				Version: siteconfig.ThemeVersionV1,
				Tokens: siteconfig.ThemeTokens{
					Colors: map[string]string{
						"background": "#151215",
						"foreground": "#f6f2ec",
						"primary":    "#8fc6ff",
						"border":     "#5a3e57",
					},
				},
			},
			Navigation: siteconfig.NavigationConfig{
				Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
			},
			Pages: []siteconfig.PageDraft{{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				SEO: siteconfig.SEOConfig{
					Title:       "Integration Test",
					Description: "End-to-end publish pipeline smoke test.",
				},
				Blocks: []siteconfig.BlockInstance{{
					ID:      "block_hero",
					Type:    "hero",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{
						"headline": "Hello from the integration test",
						"layout":   "centered",
					},
				}},
			}},
		},
	}

	bundle, err := renderer.Render(context.Background(), input)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if bundle.SchemaVersion != "published_artifacts.v1" {
		t.Fatalf("unexpected schema version: %q", bundle.SchemaVersion)
	}

	// Re-render to prove the worker stays alive between requests.
	if _, err := renderer.Render(context.Background(), input); err != nil {
		t.Fatalf("second render: %v", err)
	}

	if err := store.Save(context.Background(), siteID, versionID, bundle); err != nil {
		t.Fatalf("save: %v", err)
	}

	manifestFile, err := store.Load(context.Background(), siteID, versionID, "manifest.json")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal([]byte(manifestFile.Body), &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.SiteSlug != "integration-test" {
		t.Fatalf("manifest site slug mismatch: %q", manifest.SiteSlug)
	}
	if len(manifest.Pages) != 1 {
		t.Fatalf("manifest pages count: %d", len(manifest.Pages))
	}

	page, err := store.Load(context.Background(), siteID, versionID, manifest.Pages[0].FilePath)
	if err != nil {
		t.Fatalf("load page artifact: %v", err)
	}
	if !strings.Contains(page.Body, "Hello from the integration test") {
		t.Fatalf("page body missing headline: %q", page.Body[:min(200, len(page.Body))])
	}
}

func envOr(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
