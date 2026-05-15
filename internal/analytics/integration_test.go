//go:build integration

package analytics_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/analytics"
	"github.com/MattiSig/snaelda/internal/platform/database"
)

func TestRecorderAndReaderRoundTrip(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for the analytics integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	row := pool.QueryRow(ctx, `
		select s.id::text, p.id::text
		from sites s
		join pages p on p.site_id = s.id
		where s.published_version_id is not null
		order by s.created_at desc, p.sort_order asc
		limit 1
	`)
	var siteID, pageID string
	if err := row.Scan(&siteID, &pageID); err != nil {
		t.Skipf("no published site/page available for integration test: %v", err)
	}
	t.Logf("integration test using site %s page %s", siteID, pageID)

	if _, err := pool.Exec(ctx, `delete from page_view_daily where site_id = $1::uuid and page_id = $2::uuid and view_date = current_date`, siteID, pageID); err != nil {
		t.Fatalf("clear previous test row: %v", err)
	}

	recorder := analytics.NewRecorder(pool, nil)

	for i := 0; i < 3; i++ {
		if err := recorder.Record(ctx, analytics.PageView{SiteID: siteID, PageID: pageID}); err != nil {
			t.Fatalf("record view %d: %v", i, err)
		}
	}

	var stored int64
	if err := pool.QueryRow(ctx, `select view_count from page_view_daily where site_id = $1::uuid and page_id = $2::uuid and view_date = current_date`, siteID, pageID).Scan(&stored); err != nil {
		t.Fatalf("load stored view count: %v", err)
	}
	if stored != 3 {
		t.Fatalf("expected 3 stored views, got %d", stored)
	}

	reader := analytics.NewReader(pool)
	report, err := reader.LoadSiteAnalytics(ctx, siteID, analytics.WindowLast7Days)
	if err != nil {
		t.Fatalf("load site analytics: %v", err)
	}
	if report.TotalViews < 3 {
		t.Fatalf("expected total views >= 3, got %d", report.TotalViews)
	}

	found := false
	for _, page := range report.Pages {
		if page.PageID == pageID {
			if page.ViewCount < 3 {
				t.Fatalf("expected at least 3 views for page %s, got %d", pageID, page.ViewCount)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected page %s in analytics result", pageID)
	}

	if len(report.DailyViews) != 7 {
		t.Fatalf("expected 7 daily buckets for 7d window, got %d", len(report.DailyViews))
	}
}
