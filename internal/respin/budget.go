package respin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
)

// ErrBudgetExhausted signals that the unauthenticated re-spin demo tier has
// spent its daily LLM budget. Public endpoints map it to a friendly
// "busy — try again shortly" response; session-bound flows never see it because
// they run without a budget (Spec 21: the claim-side and in-builder flows are
// quota-accounted and continue).
var ErrBudgetExhausted = errors.New("respin: daily llm budget is exhausted")

// BudgetStore is the durable backing for the daily spend ledger. A day is a UTC
// calendar date; AddDailyTokens increments atomically and returns the new
// running total for that day.
type BudgetStore interface {
	AddDailyTokens(ctx context.Context, day time.Time, tokens int64) (int64, error)
	DailyTokens(ctx context.Context, day time.Time) (int64, error)
}

// Budget enforces a durable per-UTC-day token ceiling on the unauthenticated
// demo tier. A nil Budget (the session-bound path) is always allowed and never
// records — every method is nil-safe so callers can share stage code across
// both tiers by passing a budget only for the public path.
type Budget struct {
	store      BudgetStore
	dailyLimit int64
	clock      timestamps.Clock
}

// NewBudget builds a durable daily budget. A non-positive dailyLimit disables
// enforcement (the ledger still records, so spend stays observable). A nil store
// yields a nil Budget — an unconfigured budget is an always-allow budget.
func NewBudget(store BudgetStore, dailyLimit int64) *Budget {
	if store == nil {
		return nil
	}
	return &Budget{
		store:      store,
		dailyLimit: dailyLimit,
		clock:      timestamps.SystemClock{},
	}
}

// WithClock returns a copy of the budget using the given clock (for tests).
func (b *Budget) WithClock(clock timestamps.Clock) *Budget {
	if b == nil || clock == nil {
		return b
	}
	cloned := *b
	cloned.clock = clock
	return &cloned
}

// Check reports whether the current day's spend leaves room for more work. It is
// called before each billable stage; a nil budget or a non-positive limit always
// passes. When today's recorded spend has reached the ceiling it returns
// ErrBudgetExhausted.
func (b *Budget) Check(ctx context.Context) error {
	if b == nil || b.dailyLimit <= 0 {
		return nil
	}
	spent, err := b.store.DailyTokens(ctx, b.clock.Now().UTC())
	if err != nil {
		return fmt.Errorf("read respin llm budget: %w", err)
	}
	if spent >= b.dailyLimit {
		return ErrBudgetExhausted
	}
	return nil
}

// Record adds the tokens a completed stage spent to today's ledger. A nil budget
// is a no-op; non-positive token counts are ignored. Recording is best-effort
// bookkeeping and never blocks the pipeline: a store error is returned for the
// caller to log, but the tokens were already spent regardless.
func (b *Budget) Record(ctx context.Context, tokens int) error {
	if b == nil || tokens <= 0 {
		return nil
	}
	if _, err := b.store.AddDailyTokens(ctx, b.clock.Now().UTC(), int64(tokens)); err != nil {
		return fmt.Errorf("record respin llm spend: %w", err)
	}
	return nil
}

// PostgresBudgetStore is the pgx-backed BudgetStore over the respin_llm_usage
// table. It reuses the same narrow DB seam as the respin import store.
type PostgresBudgetStore struct {
	db DB
}

// NewPostgresBudgetStore constructs the durable budget store.
func NewPostgresBudgetStore(db DB) *PostgresBudgetStore {
	return &PostgresBudgetStore{db: db}
}

// AddDailyTokens atomically increments the day's total and returns the new value.
func (s *PostgresBudgetStore) AddDailyTokens(ctx context.Context, day time.Time, tokens int64) (int64, error) {
	var total int64
	err := s.db.QueryRow(ctx, `
		insert into respin_llm_usage (usage_date, total_tokens, updated_at)
		values ($1::date, $2, now())
		on conflict (usage_date) do update
			set total_tokens = respin_llm_usage.total_tokens + excluded.total_tokens,
			    updated_at = now()
		returning total_tokens
	`, day.UTC(), tokens).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("add respin llm usage: %w", err)
	}
	return total, nil
}

// DailyTokens returns the recorded spend for the given day, 0 when unrecorded.
func (s *PostgresBudgetStore) DailyTokens(ctx context.Context, day time.Time) (int64, error) {
	var total int64
	err := s.db.QueryRow(ctx, `
		select coalesce((select total_tokens from respin_llm_usage where usage_date = $1::date), 0)
	`, day.UTC()).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("read respin llm usage: %w", err)
	}
	return total, nil
}
