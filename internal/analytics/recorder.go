package analytics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/timestamps"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidView = errors.New("page view is invalid")
	ErrUnavailable = errors.New("analytics store is not configured")
)

// Store is the minimal database surface needed by the recorder.
type Store interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PageView captures a single counted visit.
type PageView struct {
	SiteID string
	PageID string
	Date   time.Time
}

// Recorder upserts daily page-view aggregates into page_view_daily.
type Recorder struct {
	store  Store
	clock  timestamps.Clock
	logger *slog.Logger
}

// NewRecorder returns a Recorder backed by the given store.
func NewRecorder(store Store, logger *slog.Logger) *Recorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &Recorder{
		store:  store,
		clock:  timestamps.SystemClock{},
		logger: logger,
	}
}

// WithClock returns a Recorder using the supplied clock for date stamping.
func (r *Recorder) WithClock(clock timestamps.Clock) *Recorder {
	if clock == nil {
		return r
	}
	return &Recorder{
		store:  r.store,
		clock:  clock,
		logger: r.logger,
	}
}

// Record increments today's view counter for the given site/page.
// Date defaults to the recorder clock when zero.
func (r *Recorder) Record(ctx context.Context, view PageView) error {
	if r == nil || r.store == nil {
		return ErrUnavailable
	}

	if !ids.IsValid(view.SiteID) {
		return fmt.Errorf("%w: site id must be a valid UUID", ErrInvalidView)
	}
	if !ids.IsValid(view.PageID) {
		return fmt.Errorf("%w: page id must be a valid UUID", ErrInvalidView)
	}

	when := view.Date
	if when.IsZero() {
		when = r.clock.Now()
	}
	viewDate := when.UTC().Format("2006-01-02")

	_, err := r.store.Exec(ctx, `
		insert into page_view_daily (site_id, page_id, view_date, view_count)
		values ($1::uuid, $2::uuid, $3::date, 1)
		on conflict (site_id, page_id, view_date)
		do update set view_count = page_view_daily.view_count + 1
	`, view.SiteID, view.PageID, viewDate)
	if err != nil {
		return fmt.Errorf("record page view: %w", err)
	}
	return nil
}

// RecordAsync starts a detached goroutine that records the view, using a
// fresh timeout context so the public response is never blocked.
func (r *Recorder) RecordAsync(view PageView) {
	if r == nil || r.store == nil {
		return
	}
	go func(v PageView) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.Record(ctx, v); err != nil {
			r.logger.Warn("record page view failed", "error", err, "siteId", v.SiteID, "pageId", v.PageID)
		}
	}(view)
}

// CountableRequest reports whether a public request should be counted.
// MVP heuristics from spec 16: ignore obvious health checks and bots.
func CountableRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if isHealthCheckPath(r.URL.Path) {
		return false
	}
	if isBotUserAgent(r.UserAgent()) {
		return false
	}
	return true
}

func isHealthCheckPath(path string) bool {
	switch strings.ToLower(strings.TrimSpace(path)) {
	case "/healthz", "/health", "/api/healthz", "/readyz", "/api/readyz", "/ping", "/status":
		return true
	}
	return false
}

func isBotUserAgent(ua string) bool {
	value := strings.ToLower(strings.TrimSpace(ua))
	if value == "" {
		return true
	}
	for _, token := range botUserAgentTokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

var botUserAgentTokens = []string{
	"bot",
	"crawler",
	"spider",
	"scrape",
	"slurp",
	"facebookexternalhit",
	"facebookcatalog",
	"embedly",
	"pingdom",
	"uptimerobot",
	"newrelicpinger",
	"datadog",
	"statuscake",
	"applebot",
	"baiduspider",
	"bingpreview",
	"chrome-lighthouse",
	"headlesschrome",
	"http_request",
	"curl/",
	"wget/",
	"python-requests",
	"go-http-client",
	"node-fetch",
	"axios/",
	"okhttp/",
	"httpie/",
	"libwww-perl",
	"java/",
}
