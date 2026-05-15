package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	testSiteID = "00000000-0000-4000-8000-000000000201"
	testPageID = "00000000-0000-4000-8000-000000000301"
)

type fakeRecorderStore struct {
	sql  string
	args []any
	err  error
}

func (s *fakeRecorderStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.sql = sql
	s.args = args
	return pgconn.NewCommandTag("INSERT 0 1"), s.err
}

func TestRecordUpsertsPageViewDaily(t *testing.T) {
	store := &fakeRecorderStore{}
	when := time.Date(2026, 5, 6, 15, 30, 0, 0, time.UTC)
	recorder := NewRecorder(store, nil).WithClock(timestamps.ClockFunc(func() time.Time { return when }))

	err := recorder.Record(context.Background(), PageView{
		SiteID: testSiteID,
		PageID: testPageID,
	})
	if err != nil {
		t.Fatalf("record page view: %v", err)
	}

	if !strings.Contains(store.sql, "insert into page_view_daily") {
		t.Fatalf("expected page view insert, got %q", store.sql)
	}
	if !strings.Contains(store.sql, "on conflict") {
		t.Fatalf("expected upsert via on conflict, got %q", store.sql)
	}
	if store.args[0] != testSiteID {
		t.Fatalf("unexpected site arg %v", store.args[0])
	}
	if store.args[1] != testPageID {
		t.Fatalf("unexpected page arg %v", store.args[1])
	}
	if store.args[2] != "2026-05-06" {
		t.Fatalf("unexpected date arg %v", store.args[2])
	}
}

func TestRecordValidatesIDs(t *testing.T) {
	store := &fakeRecorderStore{}
	recorder := NewRecorder(store, nil)

	tests := []PageView{
		{},
		{SiteID: testSiteID},
		{PageID: testPageID},
		{SiteID: "not-a-uuid", PageID: testPageID},
	}

	for _, view := range tests {
		if err := recorder.Record(context.Background(), view); !errors.Is(err, ErrInvalidView) {
			t.Fatalf("expected invalid view error for %#v, got %v", view, err)
		}
	}
}

func TestRecordRequiresStore(t *testing.T) {
	if err := NewRecorder(nil, nil).Record(context.Background(), PageView{SiteID: testSiteID, PageID: testPageID}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestCountableRequestFiltersBotsAndHealthChecks(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		userAgent string
		want      bool
	}{
		{"normal browser", "/", "Mozilla/5.0 (X11; Linux) Firefox/126", true},
		{"empty UA", "/", "", false},
		{"google bot", "/", "Mozilla/5.0 (compatible; Googlebot/2.1)", false},
		{"curl", "/", "curl/8.4.0", false},
		{"health path", "/healthz", "Mozilla/5.0", false},
		{"readiness path", "/readyz", "Mozilla/5.0", false},
		{"axios", "/about", "axios/1.6.0", false},
		{"safari", "/about", "Mozilla/5.0 (Macintosh; Intel Mac OS X) Safari/605", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com"+tc.path, nil)
			req.Header.Set("User-Agent", tc.userAgent)
			if got := CountableRequest(req); got != tc.want {
				t.Fatalf("CountableRequest(%q, %q) = %v, want %v", tc.path, tc.userAgent, got, tc.want)
			}
		})
	}
}
