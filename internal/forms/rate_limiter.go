package forms

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
)

// DurableSubmissionRateLimiter persists submission attempts to Postgres so
// public form rate limits survive process restarts and run across multiple
// API replicas.
type DurableSubmissionRateLimiter struct {
	db     DB
	limit  int
	window time.Duration
	clock  timestamps.Clock
	logger *slog.Logger
}

// NewDurableSubmissionRateLimiter returns a Postgres-backed limiter that
// allows up to limit submissions per key within window.
func NewDurableSubmissionRateLimiter(db DB, limit int, window time.Duration, logger *slog.Logger) *DurableSubmissionRateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	if limit <= 0 {
		limit = 5
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &DurableSubmissionRateLimiter{
		db:     db,
		limit:  limit,
		window: window,
		clock:  timestamps.SystemClock{},
		logger: logger,
	}
}

// WithClock returns a limiter using the supplied clock for window math.
func (l *DurableSubmissionRateLimiter) WithClock(clock timestamps.Clock) *DurableSubmissionRateLimiter {
	if clock == nil {
		return l
	}
	cloned := *l
	cloned.clock = clock
	return &cloned
}

// Allow reports whether a submission for the given site/block from the given
// IP hash should be accepted. Allowed attempts are recorded so subsequent
// calls in the same window count against the limit. Failures fall open and
// are logged so transient database errors do not silently block legitimate
// users.
func (l *DurableSubmissionRateLimiter) Allow(ctx context.Context, siteID string, blockID string, clientIPHash string) bool {
	if l == nil || l.db == nil {
		return true
	}

	now := l.clock.Now().UTC()
	cutoff := now.Add(-l.window)

	var attempts int
	err := l.db.QueryRow(ctx, `
		select count(*)
		from form_submission_attempts
		where site_id = $1::uuid
		  and block_id = $2::uuid
		  and client_ip_hash = $3
		  and attempted_at > $4
	`, siteID, blockID, clientIPHash, cutoff).Scan(&attempts)
	if err != nil {
		l.logger.Warn("count form submission attempts failed", "error", err, "siteId", siteID, "blockId", blockID)
		return true
	}
	if attempts >= l.limit {
		return false
	}

	if _, err := l.db.Exec(ctx, `
		insert into form_submission_attempts (site_id, block_id, client_ip_hash, attempted_at)
		values ($1::uuid, $2::uuid, $3, $4)
	`, siteID, blockID, clientIPHash, now); err != nil {
		l.logger.Warn("record form submission attempt failed", "error", err, "siteId", siteID, "blockId", blockID)
		return true
	}

	if _, err := l.db.Exec(ctx, `
		delete from form_submission_attempts
		where site_id = $1::uuid
		  and block_id = $2::uuid
		  and client_ip_hash = $3
		  and attempted_at <= $4
	`, siteID, blockID, clientIPHash, cutoff); err != nil {
		l.logger.Debug("prune form submission attempts failed", "error", err)
	}
	return true
}

// HashClientIP produces a stable, prefix-truncated hash suitable for storing
// in form_submission_attempts.client_ip_hash. The truncation keeps the
// per-attempt row narrow while remaining collision-resistant for rate
// limiting.
func HashClientIP(ip string) string {
	if ip == "" {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:16])
}

// inMemorySubmissionRateLimiter is a single-process fallback used when no
// database-backed limiter is wired in (tests, or future in-process modes).
type inMemorySubmissionRateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   int
	window  time.Duration
	entries map[string][]time.Time
}

func newInMemorySubmissionRateLimiter(limit int, window time.Duration) *inMemorySubmissionRateLimiter {
	return &inMemorySubmissionRateLimiter{
		now:     time.Now,
		limit:   limit,
		window:  window,
		entries: map[string][]time.Time{},
	}
}

func (l *inMemorySubmissionRateLimiter) Allow(_ context.Context, siteID string, blockID string, clientIPHash string) bool {
	if l == nil {
		return true
	}

	key := siteID + "|" + blockID + "|" + clientIPHash

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	cutoff := now.Add(-l.window)
	candidates := l.entries[key][:0]
	for _, attemptedAt := range l.entries[key] {
		if attemptedAt.After(cutoff) {
			candidates = append(candidates, attemptedAt)
		}
	}
	if len(candidates) >= l.limit {
		l.entries[key] = candidates
		return false
	}

	l.entries[key] = append(candidates, now)
	return true
}
