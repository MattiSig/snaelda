package email

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRateLimitStore struct {
	attempts []rateLimitAttempt
}

type rateLimitAttempt struct {
	hash       string
	purpose    string
	occurredAt time.Time
}

func (s *fakeRateLimitStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "from email_send_attempts") {
		hash := args[0].(string)
		purpose := args[1].(string)
		cutoff := args[2].(time.Time)
		count := 0
		for _, attempt := range s.attempts {
			if attempt.hash == hash && attempt.purpose == purpose && attempt.occurredAt.After(cutoff) {
				count++
			}
		}
		return fakeCountRow{count: count}
	}
	return fakeCountRow{err: pgx.ErrNoRows}
}

func (s *fakeRateLimitStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "insert into email_send_attempts") {
		s.attempts = append(s.attempts, rateLimitAttempt{
			hash:       args[0].(string),
			purpose:    args[1].(string),
			occurredAt: args[2].(time.Time),
		})
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type fakeCountRow struct {
	count int
	err   error
}

func (r fakeCountRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*int)) = r.count
	return nil
}

func TestRateLimiterAllowsWithinWindow(t *testing.T) {
	store := &fakeRateLimitStore{}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(store).WithClock(func() time.Time { return now })

	allowed, err := limiter.Allow(context.Background(), "demo@example.com", "magic_link_login", RateLimitRule{
		Limit:  2,
		Window: 15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if !allowed {
		t.Fatal("expected first send to be allowed")
	}
}

func TestRateLimiterBlocksWhenLimitReached(t *testing.T) {
	store := &fakeRateLimitStore{}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	hash := HashAddress("demo@example.com")
	store.attempts = []rateLimitAttempt{
		{hash: hash, purpose: "magic_link_login", occurredAt: now.Add(-5 * time.Minute)},
		{hash: hash, purpose: "magic_link_login", occurredAt: now.Add(-2 * time.Minute)},
	}
	limiter := NewRateLimiter(store).WithClock(func() time.Time { return now })

	allowed, err := limiter.Allow(context.Background(), "demo@example.com", "magic_link_login", RateLimitRule{
		Limit:  2,
		Window: 15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if allowed {
		t.Fatal("expected send to be blocked")
	}
}
