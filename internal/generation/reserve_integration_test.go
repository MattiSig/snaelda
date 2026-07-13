//go:build integration

package generation

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/database"
	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
)

// TestReserveSiteThenGenerationUpsertsAndValidatesBrandAsset exercises the
// re-spin brand-pull DB interplay end to end against Postgres: a site is
// reserved (draft_revision 0, UUID placeholder slug), a brand-logo asset is
// ingested scoped to it, and the generation SaveDraft upsert then fills the
// reserved row (revision -> 1, real slug) with the logo reference intact —
// proving validateAssetReferences accepts the site-scoped asset.
func TestReserveSiteThenGenerationUpsertsAndValidatesBrandAsset(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for the integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	wsID, err := ids.New()
	if err != nil {
		t.Fatalf("workspace id: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into workspaces (id, name) values ($1::uuid, 'Re-spin brand test')`, wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		_, _ = pool.Exec(cctx, `delete from workspaces where id = $1::uuid`, wsID)
	})

	svc := &Service{db: pool, writer: sites.NewPostgresWriter(pool)}

	// Reserve the site up front, as the pipeline does before the brand pull.
	siteID, err := svc.ReserveSite(ctx, wsID, "My Sewer Guys", "is")
	if err != nil {
		t.Fatalf("reserve site: %v", err)
	}

	var (
		reservedSlug   string
		reservedRev    int64
		reservedLocale string
	)
	if err := pool.QueryRow(ctx, `select slug, draft_revision, default_locale from sites where id = $1::uuid`, siteID).
		Scan(&reservedSlug, &reservedRev, &reservedLocale); err != nil {
		t.Fatalf("load reserved site: %v", err)
	}
	if reservedSlug != siteID {
		t.Fatalf("expected UUID placeholder slug, got %q", reservedSlug)
	}
	if reservedRev != 0 {
		t.Fatalf("expected reserved draft_revision 0, got %d", reservedRev)
	}
	if reservedLocale != "is" {
		t.Fatalf("expected default_locale is, got %q", reservedLocale)
	}

	// Simulate the brand pull ingesting a logo asset scoped to the reserved site.
	assetID, err := ids.New()
	if err != nil {
		t.Fatalf("asset id: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into assets (id, workspace_id, site_id, kind, storage_key)
		values ($1::uuid, $2::uuid, $3::uuid, 'image', $4)
	`, assetID, wsID, siteID, "workspaces/"+wsID+"/sites/"+siteID+"/assets/logo.png"); err != nil {
		t.Fatalf("insert brand asset: %v", err)
	}

	// Generation computes the real slug and builds the draft under the reserved id.
	slug, err := svc.createSlug(ctx, wsID, "", "My Sewer Guys")
	if err != nil {
		t.Fatalf("create slug: %v", err)
	}
	plan := generationPlan{
		SiteName: "My Sewer Guys",
		SiteGoal: "Fast 24/7 drain service.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCalmNordic),
		Pages: []generationPagePlan{{
			Title: "Home",
			Slug:  "/",
			SEO:   siteconfig.SEOConfig{Title: "Home", Description: "Fast drain service"},
			Blocks: []generationBlockPlan{{
				Type:  "hero",
				Props: map[string]any{"headline": "Clogged drain? We fix them 24/7"},
			}},
		}},
	}
	draft, err := buildDraftFromPlan(plan, slug, "is", siteconfig.BrandConfig{
		BusinessName: "My Sewer Guys",
		Logo:         &siteconfig.BrandLogo{AssetID: assetID, Alt: "My Sewer Guys"},
	}, siteID)
	if err != nil {
		t.Fatalf("build draft: %v", err)
	}
	if draft.Site.ID != siteID {
		t.Fatalf("draft did not reuse reserved id: %q", draft.Site.ID)
	}

	// The upsert must fill the reserved row (not conflict) and accept the
	// site-scoped brand asset reference.
	if err := svc.writer.SaveDraft(ctx, wsID, draft); err != nil {
		t.Fatalf("save draft over reserved site: %v", err)
	}

	var (
		finalSlug string
		finalRev  int64
		brandJSON string
	)
	if err := pool.QueryRow(ctx, `select slug, draft_revision, brand::text from sites where id = $1::uuid`, siteID).
		Scan(&finalSlug, &finalRev, &brandJSON); err != nil {
		t.Fatalf("load saved site: %v", err)
	}
	if finalSlug != slug {
		t.Fatalf("expected the real slug %q to overwrite the placeholder, got %q", slug, finalSlug)
	}
	if finalRev != 1 {
		t.Fatalf("expected draft_revision 1 after fill, got %d", finalRev)
	}
	if !strings.Contains(brandJSON, assetID) {
		t.Fatalf("expected brand json to carry the logo asset id %q, got %s", assetID, brandJSON)
	}

	// DeleteReservedSite is a no-op once the draft has been populated.
	if err := svc.DeleteReservedSite(ctx, wsID, siteID); err != nil {
		t.Fatalf("delete populated site (should no-op): %v", err)
	}
	var stillThere bool
	if err := pool.QueryRow(ctx, `select exists(select 1 from sites where id = $1::uuid)`, siteID).Scan(&stillThere); err != nil {
		t.Fatalf("existence check: %v", err)
	}
	if !stillThere {
		t.Fatal("populated site must survive DeleteReservedSite (revision guard)")
	}

	// A bare reservation that never generated is cleaned up.
	orphanID, err := svc.ReserveSite(ctx, wsID, "Abandoned", "en")
	if err != nil {
		t.Fatalf("reserve orphan: %v", err)
	}
	if err := svc.DeleteReservedSite(ctx, wsID, orphanID); err != nil {
		t.Fatalf("delete orphan reservation: %v", err)
	}
	var orphanGone bool
	if err := pool.QueryRow(ctx, `select not exists(select 1 from sites where id = $1::uuid)`, orphanID).Scan(&orphanGone); err != nil {
		t.Fatalf("orphan existence check: %v", err)
	}
	if !orphanGone {
		t.Fatal("bare reserved site should be deleted")
	}
}
