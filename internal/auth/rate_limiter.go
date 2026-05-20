package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// RateLimitStore is the minimal database surface the IP-based rate limiter
// needs. Production wiring uses the same Postgres pool that backs the rest
// of the auth handler; tests can provide a lightweight fake.
type RateLimitStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// IPRateLimitRule defines a windowed quota: at most Limit attempts inside
// Window for a given (purpose, IP) pair.
type IPRateLimitRule struct {
	Limit  int
	Window time.Duration
}

// IPRateLimiter is a Postgres-backed per-IP rate limiter for auth endpoints.
// Attempts are persisted to auth_rate_limit_attempts so quotas survive
// process restarts and are enforced across API replicas. Failures fall open
// and are logged: transient database errors must not silently lock users
// out of magic-link or recovery flows.
type IPRateLimiter struct {
	store  RateLimitStore
	now    func() time.Time
	logger *slog.Logger
}

// NewIPRateLimiter returns an IP rate limiter backed by the supplied store.
// A nil store disables the limiter; Allow becomes a no-op so tests and
// local fixtures can run without the migration applied.
func NewIPRateLimiter(store RateLimitStore, logger *slog.Logger) *IPRateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &IPRateLimiter{
		store:  store,
		now:    time.Now,
		logger: logger,
	}
}

// WithClock returns a limiter using the supplied clock for window math.
// Used by tests to advance time deterministically.
func (l *IPRateLimiter) WithClock(now func() time.Time) *IPRateLimiter {
	if l == nil || now == nil {
		return l
	}
	cloned := *l
	cloned.now = now
	return &cloned
}

// Allow reports whether an attempt for the given purpose from the given
// IP should be accepted. Allowed attempts are recorded so subsequent calls
// inside any provided window count against the limit.
func (l *IPRateLimiter) Allow(ctx context.Context, purpose string, ip string, rules ...IPRateLimitRule) bool {
	if l == nil || l.store == nil {
		return true
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return true
	}
	if len(rules) == 0 {
		return true
	}

	hash := HashIP(ip)
	now := l.now().UTC()
	for _, rule := range rules {
		if rule.Limit <= 0 || rule.Window <= 0 {
			continue
		}
		cutoff := now.Add(-rule.Window)
		var attempts int
		if err := l.store.QueryRow(ctx, `
			select count(*)
			from auth_rate_limit_attempts
			where purpose = $1
			  and key_hash = $2
			  and attempted_at > $3
		`, purpose, hash, cutoff).Scan(&attempts); err != nil {
			l.logger.Warn("count auth rate-limit attempts failed", "purpose", purpose, "error", err)
			return true
		}
		if attempts >= rule.Limit {
			return false
		}
	}

	if _, err := l.store.Exec(ctx, `
		insert into auth_rate_limit_attempts (purpose, key_hash, attempted_at)
		values ($1, $2, $3)
	`, purpose, hash, now); err != nil {
		l.logger.Warn("record auth rate-limit attempt failed", "purpose", purpose, "error", err)
		return true
	}

	// Best-effort pruning of old attempts under the longest rule window.
	longest := time.Duration(0)
	for _, rule := range rules {
		if rule.Window > longest {
			longest = rule.Window
		}
	}
	if longest > 0 {
		if _, err := l.store.Exec(ctx, `
			delete from auth_rate_limit_attempts
			where purpose = $1
			  and key_hash = $2
			  and attempted_at <= $3
		`, purpose, hash, now.Add(-longest)); err != nil {
			l.logger.Debug("prune auth rate-limit attempts failed", "purpose", purpose, "error", err)
		}
	}
	return true
}

// HashIP produces a stable, prefix-truncated hash of the supplied IP. The
// fixed-width output keeps the persisted rate-limit row narrow while
// remaining collision-resistant for rate limiting buckets.
func HashIP(ip string) string {
	normalized := strings.TrimSpace(ip)
	if normalized == "" {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:16])
}

// ClientIPFromRequest returns a best-effort client IP for rate-limiting
// buckets. It prefers the leftmost entry of X-Forwarded-For (the
// originating client per RFC 7239) when present, then falls back to
// X-Real-IP, and finally to the TCP peer address. The returned value is
// the bare IP, with any port component stripped.
func ClientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return stripPort(first)
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		return stripPort(real)
	}
	return stripPort(r.RemoteAddr)
}

func stripPort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return value
}

// Common purposes; defined as constants so callers and tests reference the
// same identifiers and accidental typos do not silently create new buckets.
const (
	RateLimitPurposeMagicLinkRequest = "magic_link_request"
	RateLimitPurposeMagicLinkVerify  = "magic_link_verify"
	RateLimitPurposeRecoveryRestore  = "recovery_restore"
	RateLimitPurposeRecoveryIssue    = "recovery_issue"
)

// Default rate-limit rules. Magic-link verify is intentionally tight per
// spec 18 (3 attempts/hour); the other endpoints are scoped to prevent
// trivial brute-force while leaving room for legitimate retries.
var (
	DefaultMagicLinkRequestRules = []IPRateLimitRule{
		{Limit: 10, Window: 15 * time.Minute},
		{Limit: 40, Window: 24 * time.Hour},
	}
	DefaultMagicLinkVerifyRules = []IPRateLimitRule{
		{Limit: 3, Window: time.Hour},
	}
	DefaultRecoveryRestoreRules = []IPRateLimitRule{
		{Limit: 5, Window: 15 * time.Minute},
		{Limit: 20, Window: 24 * time.Hour},
	}
	DefaultRecoveryIssueRules = []IPRateLimitRule{
		{Limit: 5, Window: 15 * time.Minute},
		{Limit: 20, Window: 24 * time.Hour},
	}
)

