//go:build integration

package respin

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/database"
	"github.com/MattiSig/snaelda/internal/platform/timestamps"
)

func TestPostgresBudgetStoreLifecycle(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for the integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	store := NewPostgresBudgetStore(pool)
	// A distant, fixed day keeps the test isolated from real spend rows.
	day := time.Date(2000, 1, 2, 3, 0, 0, 0, time.UTC)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from respin_llm_usage where usage_date = $1::date`, day)
	})

	if got, err := store.DailyTokens(ctx, day); err != nil || got != 0 {
		t.Fatalf("unrecorded day should be 0: got %d err %v", got, err)
	}

	total, err := store.AddDailyTokens(ctx, day, 400)
	if err != nil || total != 400 {
		t.Fatalf("first add: got %d err %v", total, err)
	}
	// Upsert accumulates on conflict rather than overwriting.
	total, err = store.AddDailyTokens(ctx, day, 350)
	if err != nil || total != 750 {
		t.Fatalf("second add should accumulate: got %d err %v", total, err)
	}
	if got, _ := store.DailyTokens(ctx, day); got != 750 {
		t.Fatalf("expected 750 recorded, got %d", got)
	}

	// A durable Budget over the real store enforces the ceiling.
	budget := NewBudget(store, 700).WithClock(timestamps.ClockFunc(func() time.Time { return day }))
	if err := budget.Check(ctx); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("750 spent over a 700 limit should be exhausted, got %v", err)
	}
}
