package generation

import (
	"context"
	"fmt"
	"log/slog"
)

// FailInterruptedJobs marks every generation job still pending or running as
// failed. Generation work runs as an in-process goroutine tied to the request
// (see the SSE handler), so any job in a live state at boot belonged to a
// previous process and can never finish — without this sweep it would hang
// until the client's poll timeout and keep holding a prompt reservation.
// Call once at API startup before serving traffic.
func FailInterruptedJobs(ctx context.Context, db DB, logger *slog.Logger) error {
	tag, err := db.Exec(ctx, `
		update generation_jobs
		set state = 'failed',
		    status = 'failed',
		    error = '{"message":"generation was interrupted by a server restart; please retry"}'::jsonb,
		    error_reason = 'interrupted_restart',
		    completed_at = now(),
		    updated_at = now()
		where state in ('pending', 'running')
	`)
	if err != nil {
		return fmt.Errorf("fail interrupted generation jobs: %w", err)
	}
	if count := tag.RowsAffected(); count > 0 && logger != nil {
		logger.Info("failed interrupted generation jobs from previous process", "count", count)
	}
	return nil
}
