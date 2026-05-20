package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeIPRateStore struct {
	attempts map[string][]time.Time
}

func newFakeIPRateStore() *fakeIPRateStore {
	return &fakeIPRateStore{attempts: map[string][]time.Time{}}
}

func (s *fakeIPRateStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if !strings.Contains(sql, "count(*)") {
		return fakeRow{err: pgx.ErrNoRows}
	}
	purpose := args[0].(string)
	keyHash := args[1].(string)
	cutoff := args[2].(time.Time)
	bucket := purpose + "|" + keyHash
	count := 0
	for _, attempt := range s.attempts[bucket] {
		if attempt.After(cutoff) {
			count++
		}
	}
	return fakeRow{values: []any{count}}
}

func (s *fakeIPRateStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "insert into auth_rate_limit_attempts") {
		purpose := args[0].(string)
		keyHash := args[1].(string)
		bucket := purpose + "|" + keyHash
		s.attempts[bucket] = append(s.attempts[bucket], args[2].(time.Time))
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}
	if strings.Contains(sql, "delete from auth_rate_limit_attempts") {
		purpose := args[0].(string)
		keyHash := args[1].(string)
		cutoff := args[2].(time.Time)
		bucket := purpose + "|" + keyHash
		kept := s.attempts[bucket][:0]
		for _, attempt := range s.attempts[bucket] {
			if attempt.After(cutoff) {
				kept = append(kept, attempt)
			}
		}
		s.attempts[bucket] = kept
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	return pgconn.NewCommandTag("OK"), nil
}

func TestIPRateLimiterEnforcesPerIPLimitsAcrossDifferentEmails(t *testing.T) {
	store := newFakeIPRateStore()
	limiter := NewIPRateLimiter(store, nil)
	rule := IPRateLimitRule{Limit: 3, Window: time.Hour}

	ip := "203.0.113.10"
	for i := 0; i < 3; i++ {
		if !limiter.Allow(context.Background(), RateLimitPurposeMagicLinkVerify, ip, rule) {
			t.Fatalf("attempt %d should be allowed within the limit", i+1)
		}
	}
	if limiter.Allow(context.Background(), RateLimitPurposeMagicLinkVerify, ip, rule) {
		t.Fatal("fourth attempt from the same IP should be rate-limited")
	}

	// A different IP should be unaffected — the limit is per-IP per-purpose.
	if !limiter.Allow(context.Background(), RateLimitPurposeMagicLinkVerify, "203.0.113.11", rule) {
		t.Fatal("different IP should not be rate-limited")
	}
}

func TestIPRateLimiterAllowsAgainAfterWindow(t *testing.T) {
	store := newFakeIPRateStore()
	current := time.Unix(1700000000, 0).UTC()
	limiter := NewIPRateLimiter(store, nil).WithClock(func() time.Time { return current })
	rule := IPRateLimitRule{Limit: 1, Window: time.Minute}

	if !limiter.Allow(context.Background(), "magic_link_verify", "203.0.113.20", rule) {
		t.Fatal("first attempt should succeed")
	}
	if limiter.Allow(context.Background(), "magic_link_verify", "203.0.113.20", rule) {
		t.Fatal("second attempt within window should be blocked")
	}
	current = current.Add(2 * time.Minute)
	if !limiter.Allow(context.Background(), "magic_link_verify", "203.0.113.20", rule) {
		t.Fatal("attempt after window should be allowed")
	}
}

func TestClientIPFromRequestPrefersForwardedHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	req.RemoteAddr = "10.0.0.1:54321"

	if got := ClientIPFromRequest(req); got != "203.0.113.5" {
		t.Fatalf("expected leftmost X-Forwarded-For entry, got %q", got)
	}
}

func TestClientIPFromRequestFallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", nil)
	req.RemoteAddr = "198.51.100.7:443"

	if got := ClientIPFromRequest(req); got != "198.51.100.7" {
		t.Fatalf("expected RemoteAddr without port, got %q", got)
	}
}

func TestRequestMagicLinkReturnsRateLimitedAfterBurst(t *testing.T) {
	rateStore := newFakeIPRateStore()
	limiter := NewIPRateLimiter(rateStore, nil)
	handler := NewHandler(HandlerConfig{
		Tokens:        newHandlerTestTokenManager(t),
		IPRateLimiter: limiter,
	})

	// Exhaust the configured per-15-minute rule (10) — keep configuration
	// in sync with DefaultMagicLinkRequestRules.
	for i := 0; i < DefaultMagicLinkRequestRules[0].Limit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", strings.NewReader(`{"email":"demo@snaelda.local"}`))
		req.RemoteAddr = "198.51.100.42:443"
		res := httptest.NewRecorder()
		handler.requestMagicLink(res, req)
		// The handler reports success even when the user lookup fails so we
		// only need to ensure no 429 is returned within the limit.
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d should not be rate-limited yet", i+1)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", strings.NewReader(`{"email":"demo@snaelda.local"}`))
	req.RemoteAddr = "198.51.100.42:443"
	res := httptest.NewRecorder()
	handler.requestMagicLink(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d once IP burst is exhausted, got %d", http.StatusTooManyRequests, res.Code)
	}
}
