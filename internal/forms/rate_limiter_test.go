package forms

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeAttemptStore struct {
	attempts []attemptRow
	now      time.Time
	failNext bool
}

type attemptRow struct {
	siteID      string
	blockID     string
	ipHash      string
	attemptedAt time.Time
}

func (s *fakeAttemptStore) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeAttemptStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if !strings.Contains(sql, "form_submission_attempts") {
		return fakeFormRow{err: errors.New("unexpected query")}
	}
	siteID := args[0].(string)
	blockID := args[1].(string)
	ipHash := args[2].(string)
	cutoff := args[3].(time.Time)
	count := 0
	for _, attempt := range s.attempts {
		if attempt.siteID == siteID && attempt.blockID == blockID && attempt.ipHash == ipHash && attempt.attemptedAt.After(cutoff) {
			count++
		}
	}
	return fakeFormRow{values: []any{count}}
}

func (s *fakeAttemptStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if s.failNext {
		s.failNext = false
		return pgconn.CommandTag{}, errors.New("transient db failure")
	}
	switch {
	case strings.Contains(sql, "insert into form_submission_attempts"):
		s.attempts = append(s.attempts, attemptRow{
			siteID:      args[0].(string),
			blockID:     args[1].(string),
			ipHash:      args[2].(string),
			attemptedAt: args[3].(time.Time),
		})
	case strings.Contains(sql, "delete from form_submission_attempts"):
		siteID := args[0].(string)
		blockID := args[1].(string)
		ipHash := args[2].(string)
		cutoff := args[3].(time.Time)
		kept := s.attempts[:0]
		for _, attempt := range s.attempts {
			if attempt.siteID == siteID && attempt.blockID == blockID && attempt.ipHash == ipHash && !attempt.attemptedAt.After(cutoff) {
				continue
			}
			kept = append(kept, attempt)
		}
		s.attempts = kept
	}
	return pgconn.CommandTag{}, nil
}

func (s *fakeAttemptStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}

func TestDurableSubmissionRateLimiterAllowsUpToLimit(t *testing.T) {
	store := &fakeAttemptStore{}
	base := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	limiter := NewDurableSubmissionRateLimiter(store, 3, time.Minute, nil).
		WithClock(timestamps.ClockFunc(func() time.Time { return base }))

	ctx := context.Background()
	for attempt := 0; attempt < 3; attempt++ {
		if !limiter.Allow(ctx, "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", "ip-hash") {
			t.Fatalf("attempt %d should be allowed", attempt+1)
		}
	}
	if limiter.Allow(ctx, "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", "ip-hash") {
		t.Fatal("expected fourth attempt to be denied")
	}
}

func TestDurableSubmissionRateLimiterResetsAfterWindow(t *testing.T) {
	store := &fakeAttemptStore{}
	base := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	clock := base
	limiter := NewDurableSubmissionRateLimiter(store, 2, time.Minute, nil).
		WithClock(timestamps.ClockFunc(func() time.Time { return clock }))

	ctx := context.Background()
	if !limiter.Allow(ctx, "site", "block", "ip") || !limiter.Allow(ctx, "site", "block", "ip") {
		t.Fatal("first two attempts should pass")
	}
	if limiter.Allow(ctx, "site", "block", "ip") {
		t.Fatal("third attempt should be denied within the window")
	}

	clock = base.Add(2 * time.Minute)
	if !limiter.Allow(ctx, "site", "block", "ip") {
		t.Fatal("expected limiter to reset after the window elapsed")
	}
}

func TestDurableSubmissionRateLimiterSegregatesKeys(t *testing.T) {
	store := &fakeAttemptStore{}
	base := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	limiter := NewDurableSubmissionRateLimiter(store, 1, time.Minute, nil).
		WithClock(timestamps.ClockFunc(func() time.Time { return base }))

	ctx := context.Background()
	if !limiter.Allow(ctx, "site-1", "block", "ip-A") {
		t.Fatal("first attempt for ip-A should pass")
	}
	if limiter.Allow(ctx, "site-1", "block", "ip-A") {
		t.Fatal("second attempt for ip-A within window should be denied")
	}
	if !limiter.Allow(ctx, "site-1", "block", "ip-B") {
		t.Fatal("first attempt for ip-B should pass even when ip-A is limited")
	}
	if !limiter.Allow(ctx, "site-2", "block", "ip-A") {
		t.Fatal("different site should not share the limit")
	}
}

func TestHashClientIPIsStableAndAvoidsEmpty(t *testing.T) {
	if HashClientIP("") != "unknown" {
		t.Fatal("expected empty ip to fall back to unknown")
	}
	a := HashClientIP("192.0.2.1")
	b := HashClientIP("192.0.2.1")
	if a != b {
		t.Fatal("expected stable hash for the same input")
	}
	if a == HashClientIP("192.0.2.2") {
		t.Fatal("expected distinct hash for different inputs")
	}
	if a == "192.0.2.1" {
		t.Fatal("expected hash to obscure the underlying address")
	}
}
