package respin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
)

// memBudgetStore is an in-memory BudgetStore keyed by UTC calendar date.
type memBudgetStore struct {
	days map[string]int64
}

func newMemBudgetStore() *memBudgetStore { return &memBudgetStore{days: map[string]int64{}} }

func dayKey(day time.Time) string { return day.UTC().Format("2006-01-02") }

func (m *memBudgetStore) AddDailyTokens(_ context.Context, day time.Time, tokens int64) (int64, error) {
	m.days[dayKey(day)] += tokens
	return m.days[dayKey(day)], nil
}

func (m *memBudgetStore) DailyTokens(_ context.Context, day time.Time) (int64, error) {
	return m.days[dayKey(day)], nil
}

func fixedClock(t time.Time) timestamps.Clock {
	return timestamps.ClockFunc(func() time.Time { return t })
}

func TestBudgetAllowsUntilLimitReached(t *testing.T) {
	store := newMemBudgetStore()
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	budget := NewBudget(store, 1000).WithClock(fixedClock(now))

	if err := budget.Check(context.Background()); err != nil {
		t.Fatalf("empty day should pass: %v", err)
	}
	if err := budget.Record(context.Background(), 600); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := budget.Check(context.Background()); err != nil {
		t.Fatalf("under limit should pass: %v", err)
	}
	if err := budget.Record(context.Background(), 600); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := budget.Check(context.Background()); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("over limit should be exhausted, got %v", err)
	}
}

func TestBudgetResetsNextDay(t *testing.T) {
	store := newMemBudgetStore()
	day1 := time.Date(2026, 7, 11, 23, 0, 0, 0, time.UTC)
	b1 := NewBudget(store, 500).WithClock(fixedClock(day1))
	if err := b1.Record(context.Background(), 500); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := b1.Check(context.Background()); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("day1 should be exhausted, got %v", err)
	}

	day2 := time.Date(2026, 7, 12, 1, 0, 0, 0, time.UTC)
	b2 := NewBudget(store, 500).WithClock(fixedClock(day2))
	if err := b2.Check(context.Background()); err != nil {
		t.Fatalf("day2 should reset and pass: %v", err)
	}
}

func TestBudgetNilAlwaysAllowed(t *testing.T) {
	var budget *Budget // session-bound path passes nil
	if err := budget.Check(context.Background()); err != nil {
		t.Fatalf("nil budget should pass: %v", err)
	}
	if err := budget.Record(context.Background(), 999); err != nil {
		t.Fatalf("nil budget record should no-op: %v", err)
	}
}

func TestNewBudgetNilStoreIsNil(t *testing.T) {
	if NewBudget(nil, 1000) != nil {
		t.Fatal("nil store should yield a nil (always-allow) budget")
	}
}

func TestBudgetZeroLimitDisablesEnforcement(t *testing.T) {
	store := newMemBudgetStore()
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	budget := NewBudget(store, 0).WithClock(fixedClock(now))

	if err := budget.Record(context.Background(), 10_000); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := budget.Check(context.Background()); err != nil {
		t.Fatalf("zero limit should never exhaust: %v", err)
	}
	// Spend is still observable in the ledger even with enforcement off.
	if got, _ := store.DailyTokens(context.Background(), now); got != 10_000 {
		t.Fatalf("expected recorded spend 10000, got %d", got)
	}
}
