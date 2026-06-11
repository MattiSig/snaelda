//go:build integration

package sites

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/analytics"
	"github.com/MattiSig/snaelda/internal/forms"
	"github.com/MattiSig/snaelda/internal/platform/database"
	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestArchivedPublishedPagesPreserveFormsAndAnalytics(t *testing.T) {
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

	workspaceID := mustIntegrationID(t)
	siteID := mustIntegrationID(t)
	pageV1ID := mustIntegrationID(t)
	pageV2ID := mustIntegrationID(t)
	blockV1ID := mustIntegrationID(t)
	blockV2ID := mustIntegrationID(t)
	versionV1ID := mustIntegrationID(t)
	versionV2ID := mustIntegrationID(t)

	if _, err := pool.Exec(ctx, `
		insert into workspaces (id, name)
		values ($1::uuid, 'Published page identity integration')
	`, workspaceID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from workspaces where id = $1::uuid`, workspaceID)
	})

	writer := NewPostgresWriter(pool)
	reader := NewPostgresReader(pool)
	formService := forms.NewService(pool)
	analyticsRecorder := analytics.NewRecorder(pool, nil)
	analyticsReader := analytics.NewReader(pool)

	draftV1 := integrationDraft(siteID, pageV1ID, blockV1ID, "Home V1", "Warm sites for serious small businesses.")
	if err := writer.SaveDraft(ctx, workspaceID, draftV1); err != nil {
		t.Fatalf("save v1 draft: %v", err)
	}

	liveDraftV1, err := reader.LoadDraft(ctx, siteID)
	if err != nil {
		t.Fatalf("load v1 draft: %v", err)
	}
	if err := setPublishedVersion(ctx, pool, versionV1ID, siteID, 1, integrationSnapshot(liveDraftV1)); err != nil {
		t.Fatalf("publish v1 snapshot: %v", err)
	}

	submitResult, err := formService.Submit(ctx, forms.SubmitInput{
		SiteID:  siteID,
		BlockID: blockV1ID,
		Payload: map[string]any{
			"name":    "Ada Lovelace",
			"email":   "ada@example.com",
			"message": "Need a calmer site.",
		},
	})
	if err != nil {
		t.Fatalf("submit form against v1: %v", err)
	}
	if submitResult.Submission.PageID != pageV1ID {
		t.Fatalf("expected first submission page %q, got %q", pageV1ID, submitResult.Submission.PageID)
	}

	if err := analyticsRecorder.Record(ctx, analytics.PageView{SiteID: siteID, PageID: pageV1ID}); err != nil {
		t.Fatalf("record analytics against v1: %v", err)
	}

	draftV2 := liveDraftV1
	draftV2.Pages = []siteconfig.PageDraft{integrationHomepage(pageV2ID, blockV2ID, "Home V2", "Fresh draft copy after republish.")}
	draftV2.Navigation = siteconfig.NavigationConfig{
		Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: pageV2ID}},
	}
	if err := writer.SaveDraft(ctx, workspaceID, draftV2); err != nil {
		t.Fatalf("save v2 draft: %v", err)
	}

	liveDraftV2, err := reader.LoadDraft(ctx, siteID)
	if err != nil {
		t.Fatalf("load v2 draft: %v", err)
	}
	if err := setPublishedVersion(ctx, pool, versionV2ID, siteID, 2, integrationSnapshot(liveDraftV2)); err != nil {
		t.Fatalf("publish v2 snapshot: %v", err)
	}

	rows, err := pool.Query(ctx, `
		select id::text, title, slug, in_draft
		from pages
		where site_id = $1::uuid
		order by created_at asc, id asc
	`, siteID)
	if err != nil {
		t.Fatalf("query persisted pages: %v", err)
	}
	defer rows.Close()

	type persistedPage struct {
		ID      string
		Title   string
		Slug    string
		InDraft bool
	}
	persisted := []persistedPage{}
	for rows.Next() {
		var page persistedPage
		if err := rows.Scan(&page.ID, &page.Title, &page.Slug, &page.InDraft); err != nil {
			t.Fatalf("scan persisted page: %v", err)
		}
		persisted = append(persisted, page)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate persisted pages: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("expected archived + active page rows, got %#v", persisted)
	}
	if persisted[0].ID != pageV1ID || persisted[0].Slug != "/" || persisted[0].InDraft {
		t.Fatalf("expected archived published page row to survive, got %#v", persisted[0])
	}
	if persisted[1].ID != pageV2ID || persisted[1].Slug != "/" || !persisted[1].InDraft {
		t.Fatalf("expected replacement draft page row to stay active, got %#v", persisted[1])
	}

	submissions, err := formService.ListBySite(ctx, siteID)
	if err != nil {
		t.Fatalf("list submissions after republish: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("expected one submission, got %#v", submissions)
	}
	if submissions[0].PageID != pageV1ID || submissions[0].PageTitle != "Home V1" {
		t.Fatalf("expected old submission to keep page identity, got %#v", submissions[0])
	}

	report, err := analyticsReader.LoadSiteAnalytics(ctx, siteID, analytics.WindowLast7Days)
	if err != nil {
		t.Fatalf("load analytics after republish: %v", err)
	}
	foundArchivedPage := false
	for _, page := range report.Pages {
		if page.PageID == pageV1ID {
			foundArchivedPage = true
			if page.Title != "Home V1" || page.Slug != "/" || page.ViewCount < 1 {
				t.Fatalf("expected analytics to keep archived page labels, got %#v", page)
			}
		}
	}
	if !foundArchivedPage {
		t.Fatalf("expected archived published page to remain in analytics report, got %#v", report.Pages)
	}

	_, err = formService.Submit(ctx, forms.SubmitInput{
		SiteID:  siteID,
		BlockID: blockV1ID,
		Payload: map[string]any{
			"name":    "Grace Hopper",
			"email":   "grace@example.com",
			"message": "Still trying the removed form.",
		},
	})
	if !errors.Is(err, forms.ErrFormBlockNotFound) {
		t.Fatalf("expected v2 publish to hide removed form block, got %v", err)
	}

	if _, err := pool.Exec(ctx, `
		update sites
		set published_version_id = $2::uuid
		where id = $1::uuid
	`, siteID, versionV1ID); err != nil {
		t.Fatalf("simulate rollback to v1: %v", err)
	}

	rollbackSubmission, err := formService.Submit(ctx, forms.SubmitInput{
		SiteID:  siteID,
		BlockID: blockV1ID,
		Payload: map[string]any{
			"name":    "Grace Hopper",
			"email":   "grace@example.com",
			"message": "The rollback put the form back.",
		},
	})
	if err != nil {
		t.Fatalf("submit form after rollback: %v", err)
	}
	if rollbackSubmission.Submission.PageID != pageV1ID {
		t.Fatalf("expected rollback submission to target original page, got %#v", rollbackSubmission.Submission)
	}
}

func integrationDraft(siteID string, pageID string, blockID string, title string, description string) siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            siteID,
			Name:          "Published page identity integration",
			Slug:          "published-page-identity-integration",
			Status:        "draft",
			DefaultLocale: "en",
			SEO: siteconfig.SEOConfig{
				Title:       "Published page identity integration",
				Description: description,
			},
		},
		Brand: siteconfig.BrandConfig{
			BusinessName: "Published page identity integration",
			PrimaryColor: "#86d8cf",
		},
		Theme: siteconfig.ThemePreset(siteconfig.ThemePaletteAfterHours),
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: pageID}},
		},
		Pages: []siteconfig.PageDraft{integrationHomepage(pageID, blockID, title, description)},
	}
}

func integrationHomepage(pageID string, blockID string, title string, description string) siteconfig.PageDraft {
	return siteconfig.PageDraft{
		ID:     pageID,
		Title:  title,
		Slug:   "/",
		Status: siteconfig.PageStatusPublished,
		SEO: siteconfig.SEOConfig{
			Title:       title + " | Published page identity integration",
			Description: description,
		},
		Blocks: []siteconfig.BlockInstance{
			{
				ID:      blockID,
				Type:    "contact_form",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading":     "Reach out",
					"submitLabel": "Send",
					"fields": []any{
						map[string]any{"name": "name", "label": "Name", "type": "name", "required": true},
						map[string]any{"name": "email", "label": "Email", "type": "email", "required": true},
						map[string]any{"name": "message", "label": "Message", "type": "message", "required": true},
					},
				},
			},
		},
	}
}

func integrationSnapshot(draft siteconfig.SiteDraft) siteconfig.PublishedSnapshot {
	snapshot := siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site: siteconfig.PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO: siteconfig.SEOConfig{
				Title:       draft.Site.SEO.Title,
				Description: draft.Site.SEO.Description,
			},
		},
		Brand:      draft.Brand,
		Theme:      draft.Theme,
		Navigation: draft.Navigation,
		Pages:      draft.Pages,
	}
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		panic(err)
	}
	return snapshot
}

func setPublishedVersion(ctx context.Context, pool *pgxpool.Pool, versionID string, siteID string, versionNumber int, snapshot siteconfig.PublishedSnapshot) error {
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `
		insert into site_versions (id, site_id, version_number, snapshot)
		values ($1::uuid, $2::uuid, $3, $4::jsonb)
	`, versionID, siteID, versionNumber, snapshotJSON); err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		update sites
		set published_version_id = $2::uuid
		where id = $1::uuid
	`, siteID, versionID)
	return err
}

func mustIntegrationID(t *testing.T) string {
	t.Helper()
	value, err := ids.New()
	if err != nil {
		t.Fatalf("generate id: %v", err)
	}
	return value
}
