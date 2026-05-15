package analytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryStore is the minimal database surface for analytics reads.
type QueryStore interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Window names the supported analytics windows for the MVP surface.
type Window string

const (
	WindowLast7Days  Window = "7d"
	WindowLast30Days Window = "30d"
	WindowAllTime    Window = "all"
)

// SiteAnalytics is the analytics payload returned by the API.
type SiteAnalytics struct {
	SiteID     string       `json:"siteId"`
	Window     Window       `json:"window"`
	RangeStart *time.Time   `json:"rangeStart,omitempty"`
	RangeEnd   *time.Time   `json:"rangeEnd,omitempty"`
	TotalViews int64        `json:"totalViews"`
	Pages      []PageViews  `json:"pages"`
	DailyViews []DailyViews `json:"dailyViews"`
}

// PageViews aggregates lifetime-window views for a single page.
type PageViews struct {
	PageID    string `json:"pageId"`
	Title     string `json:"title,omitempty"`
	Slug      string `json:"slug,omitempty"`
	ViewCount int64  `json:"viewCount"`
}

// DailyViews is a single day's bucket within the window.
type DailyViews struct {
	Date      string `json:"date"`
	ViewCount int64  `json:"viewCount"`
}

// PageMeta carries page label data joined from the pages table.
type PageMeta struct {
	ID    string
	Title string
	Slug  string
}

// Reader queries the page_view_daily aggregates with page metadata.
type Reader struct {
	db    QueryStore
	clock clockFunc
}

type clockFunc func() time.Time

// NewReader creates a Reader with a system clock.
func NewReader(db QueryStore) *Reader {
	return &Reader{db: db, clock: func() time.Time { return time.Now().UTC() }}
}

// WithClock overrides the clock used for window calculations.
func (r *Reader) WithClock(clock func() time.Time) *Reader {
	if clock == nil {
		return r
	}
	return &Reader{db: r.db, clock: clock}
}

// NormalizeWindow validates and returns a supported window value.
func NormalizeWindow(raw string) (Window, error) {
	switch Window(raw) {
	case "", WindowLast7Days:
		return WindowLast7Days, nil
	case WindowLast30Days:
		return WindowLast30Days, nil
	case WindowAllTime:
		return WindowAllTime, nil
	default:
		return "", fmt.Errorf("unsupported analytics window: %q", raw)
	}
}

// LoadSiteAnalytics returns aggregate counts for the site within the window.
func (r *Reader) LoadSiteAnalytics(ctx context.Context, siteID string, window Window) (SiteAnalytics, error) {
	if r == nil || r.db == nil {
		return SiteAnalytics{}, ErrUnavailable
	}

	now := r.clock().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	result := SiteAnalytics{
		SiteID: siteID,
		Window: window,
	}

	var (
		rangeStart *time.Time
		rangeEnd   *time.Time
		startDate  time.Time
		endDate    = today
	)
	switch window {
	case WindowLast7Days:
		startDate = today.AddDate(0, 0, -6)
		rs, re := startDate, endDate
		rangeStart, rangeEnd = &rs, &re
	case WindowLast30Days:
		startDate = today.AddDate(0, 0, -29)
		rs, re := startDate, endDate
		rangeStart, rangeEnd = &rs, &re
	case WindowAllTime:
	default:
		return SiteAnalytics{}, fmt.Errorf("unsupported analytics window: %q", window)
	}
	result.RangeStart = rangeStart
	result.RangeEnd = rangeEnd

	pageRows, err := r.queryPageViews(ctx, siteID, rangeStart, rangeEnd)
	if err != nil {
		return SiteAnalytics{}, err
	}
	pageMeta, err := r.loadPageMeta(ctx, siteID, pageIDs(pageRows))
	if err != nil {
		return SiteAnalytics{}, err
	}
	mergedPages := mergePageMeta(pageRows, pageMeta)
	sort.SliceStable(mergedPages, func(i, j int) bool {
		if mergedPages[i].ViewCount != mergedPages[j].ViewCount {
			return mergedPages[i].ViewCount > mergedPages[j].ViewCount
		}
		return strings.ToLower(mergedPages[i].Title) < strings.ToLower(mergedPages[j].Title)
	})
	result.Pages = mergedPages
	for _, page := range mergedPages {
		result.TotalViews += page.ViewCount
	}

	daily, err := r.queryDailyViews(ctx, siteID, rangeStart, rangeEnd)
	if err != nil {
		return SiteAnalytics{}, err
	}
	if rangeStart != nil && rangeEnd != nil {
		daily = fillDailyGaps(daily, *rangeStart, *rangeEnd)
	}
	result.DailyViews = daily

	return result, nil
}

func (r *Reader) queryPageViews(ctx context.Context, siteID string, rangeStart, rangeEnd *time.Time) ([]PageViews, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if rangeStart != nil && rangeEnd != nil {
		rows, err = r.db.Query(ctx, `
			select page_id::text, sum(view_count)::bigint
			from page_view_daily
			where site_id = $1::uuid
			  and view_date between $2::date and $3::date
			group by page_id
		`, siteID, rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02"))
	} else {
		rows, err = r.db.Query(ctx, `
			select page_id::text, sum(view_count)::bigint
			from page_view_daily
			where site_id = $1::uuid
			group by page_id
		`, siteID)
	}
	if err != nil {
		return nil, fmt.Errorf("query page view aggregates: %w", err)
	}
	defer rows.Close()

	var result []PageViews
	for rows.Next() {
		var pv PageViews
		if err := rows.Scan(&pv.PageID, &pv.ViewCount); err != nil {
			return nil, fmt.Errorf("scan page view row: %w", err)
		}
		result = append(result, pv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate page view rows: %w", err)
	}
	return result, nil
}

func (r *Reader) queryDailyViews(ctx context.Context, siteID string, rangeStart, rangeEnd *time.Time) ([]DailyViews, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if rangeStart != nil && rangeEnd != nil {
		rows, err = r.db.Query(ctx, `
			select view_date, sum(view_count)::bigint
			from page_view_daily
			where site_id = $1::uuid
			  and view_date between $2::date and $3::date
			group by view_date
			order by view_date asc
		`, siteID, rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02"))
	} else {
		rows, err = r.db.Query(ctx, `
			select view_date, sum(view_count)::bigint
			from page_view_daily
			where site_id = $1::uuid
			group by view_date
			order by view_date asc
		`, siteID)
	}
	if err != nil {
		return nil, fmt.Errorf("query daily views: %w", err)
	}
	defer rows.Close()

	var result []DailyViews
	for rows.Next() {
		var when time.Time
		var count int64
		if err := rows.Scan(&when, &count); err != nil {
			return nil, fmt.Errorf("scan daily view row: %w", err)
		}
		result = append(result, DailyViews{
			Date:      when.UTC().Format("2006-01-02"),
			ViewCount: count,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily view rows: %w", err)
	}
	return result, nil
}

func (r *Reader) loadPageMeta(ctx context.Context, siteID string, pageIDs []string) (map[string]PageMeta, error) {
	if len(pageIDs) == 0 {
		return map[string]PageMeta{}, nil
	}
	rows, err := r.db.Query(ctx, `
		select id::text, title, slug
		from pages
		where site_id = $1::uuid
		  and id = any($2::uuid[])
	`, siteID, pageIDs)
	if err != nil {
		return nil, fmt.Errorf("query page metadata: %w", err)
	}
	defer rows.Close()

	out := make(map[string]PageMeta, len(pageIDs))
	for rows.Next() {
		var meta PageMeta
		if err := rows.Scan(&meta.ID, &meta.Title, &meta.Slug); err != nil {
			return nil, fmt.Errorf("scan page metadata row: %w", err)
		}
		out[meta.ID] = meta
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate page metadata rows: %w", err)
	}
	return out, nil
}

func pageIDs(rows []PageViews) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.PageID)
	}
	return out
}

func mergePageMeta(rows []PageViews, meta map[string]PageMeta) []PageViews {
	merged := make([]PageViews, 0, len(rows))
	for _, row := range rows {
		if info, ok := meta[row.PageID]; ok {
			row.Title = info.Title
			row.Slug = info.Slug
		}
		merged = append(merged, row)
	}
	return merged
}

func fillDailyGaps(rows []DailyViews, start, end time.Time) []DailyViews {
	if !end.After(start) && !end.Equal(start) {
		return rows
	}
	index := make(map[string]int64, len(rows))
	for _, row := range rows {
		index[row.Date] = row.ViewCount
	}

	filled := make([]DailyViews, 0)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.UTC().Format("2006-01-02")
		filled = append(filled, DailyViews{
			Date:      date,
			ViewCount: index[date],
		})
	}
	return filled
}

