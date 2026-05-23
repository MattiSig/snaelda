package generation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestGenerationRateLimiterEnforcesUserAndWorkspaceLimits(t *testing.T) {
	store := newFakeGenerationAttemptStore()
	now := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	limiter := NewGenerationRateLimiter(store, nil).WithClock(timestamps.ClockFunc(func() time.Time { return now }))

	for attempt := 0; attempt < 6; attempt++ {
		if !limiter.Allow(context.Background(), "workspace-1", "user-1", "site") {
			t.Fatal("expected first six attempts to pass")
		}
	}
	if limiter.Allow(context.Background(), "workspace-1", "user-1", "site") {
		t.Fatal("expected user-scoped burst limit to block the seventh attempt")
	}

	for index := 0; index < 6; index++ {
		userID := "user-" + string(rune('2'+index))
		if !limiter.Allow(context.Background(), "workspace-1", userID, "site") {
			t.Fatalf("expected workspace attempt %d to pass", index)
		}
	}
	if limiter.Allow(context.Background(), "workspace-1", "user-9", "site") {
		t.Fatal("expected workspace-scoped burst limit to block the thirteenth attempt")
	}
}

func TestGenerationRateLimiterAllowsAgainAfterWindow(t *testing.T) {
	store := newFakeGenerationAttemptStore()
	now := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	clock := timestamps.ClockFunc(func() time.Time { return now })
	limiter := NewGenerationRateLimiter(store, nil).WithClock(clock)

	for attempt := 0; attempt < 6; attempt++ {
		if !limiter.Allow(context.Background(), "workspace-1", "user-1", "site_reprompt") {
			t.Fatal("expected initial attempts to pass")
		}
	}
	clock = timestamps.ClockFunc(func() time.Time { return now.Add(11 * time.Minute) })
	limiter = limiter.WithClock(clock)
	if !limiter.Allow(context.Background(), "workspace-1", "user-1", "site_reprompt") {
		t.Fatal("expected limiter to reset after the burst window")
	}
}

type fakeGenerationAttemptStore struct {
	attempts []generationAttempt
}

type generationAttempt struct {
	workspaceID string
	userID      string
	scope       string
	attemptedAt time.Time
}

func newFakeGenerationAttemptStore() *fakeGenerationAttemptStore {
	return &fakeGenerationAttemptStore{}
}

func (s *fakeGenerationAttemptStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	var count int
	switch {
	case strings.Contains(sql, "and user_id = $2::uuid"):
		workspaceID := args[0].(string)
		userID := args[1].(string)
		scope := args[2].(string)
		cutoff := args[3].(time.Time)
		for _, attempt := range s.attempts {
			if attempt.workspaceID == workspaceID && attempt.userID == userID && attempt.scope == scope && attempt.attemptedAt.After(cutoff) {
				count++
			}
		}
	default:
		workspaceID := args[0].(string)
		scope := args[1].(string)
		cutoff := args[2].(time.Time)
		for _, attempt := range s.attempts {
			if attempt.workspaceID == workspaceID && attempt.scope == scope && attempt.attemptedAt.After(cutoff) {
				count++
			}
		}
	}
	return generationRateLimitRow{count: count}
}

func (s *fakeGenerationAttemptStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "insert into generation_attempts"):
		s.attempts = append(s.attempts, generationAttempt{
			workspaceID: args[0].(string),
			userID:      args[1].(string),
			scope:       args[2].(string),
			attemptedAt: args[3].(time.Time),
		})
	case strings.Contains(sql, "delete from generation_attempts"):
		workspaceID := args[0].(string)
		scope := args[1].(string)
		cutoff := args[2].(time.Time)
		filtered := s.attempts[:0]
		for _, attempt := range s.attempts {
			if attempt.workspaceID == workspaceID && attempt.scope == scope && !attempt.attemptedAt.After(cutoff) {
				continue
			}
			filtered = append(filtered, attempt)
		}
		s.attempts = filtered
	}
	return pgconn.CommandTag{}, nil
}

type generationRateLimitRow struct {
	count int
}

func (r generationRateLimitRow) Scan(dest ...any) error {
	*dest[0].(*int) = r.count
	return nil
}
