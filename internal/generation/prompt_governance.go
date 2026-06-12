package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const trialPromptLimit = 25

type PromptQuotaExceededError struct {
	Code    string
	Message string
}

func (e *PromptQuotaExceededError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type PromptActionDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type promptActionStore interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginPromptTx(ctx context.Context) (promptActionTx, error)
}

type promptActionTx interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type promptActionDBAdapter struct {
	db PromptActionDB
}

func (a promptActionDBAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.db.Query(ctx, sql, args...)
}

func (a promptActionDBAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.db.QueryRow(ctx, sql, args...)
}

func (a promptActionDBAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.db.Exec(ctx, sql, args...)
}

func (a promptActionDBAdapter) BeginPromptTx(ctx context.Context) (promptActionTx, error) {
	return a.db.BeginTx(ctx, pgx.TxOptions{})
}

type PromptActionManager struct {
	store  promptActionStore
	logger *slog.Logger
}

type PromptActionInput struct {
	WorkspaceID string
	UserID      string
	SiteID      string
	Kind        JobKind
	Prompt      string
	Payload     any
}

func NewPromptActionManager(store promptActionStore, logger *slog.Logger) *PromptActionManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &PromptActionManager{
		store:  store,
		logger: logger,
	}
}

func NewPromptActionManagerFromDB(db PromptActionDB, logger *slog.Logger) *PromptActionManager {
	if db == nil {
		return nil
	}
	return NewPromptActionManager(promptActionDBAdapter{db: db}, logger)
}

func (m *PromptActionManager) CreateJob(ctx context.Context, input PromptActionInput) (string, error) {
	if m == nil || m.store == nil {
		return "", fmt.Errorf("generation jobs are not configured")
	}
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		return "", fmt.Errorf("workspace id is required")
	}
	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return "", fmt.Errorf("encode generation payload: %w", err)
	}

	tx, err := m.store.BeginPromptTx(ctx)
	if err != nil {
		return "", fmt.Errorf("begin generation job admission: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := m.enforcePromptQuota(ctx, tx, workspaceID); err != nil {
		return "", err
	}

	var jobID string
	if err := tx.QueryRow(ctx, `
		insert into generation_jobs (site_id, workspace_id, kind, state, status, prompt, input_context, payload, created_by)
		values (nullif($1, '')::uuid, $2::uuid, $3, 'pending', 'queued', $4, $5, $5, nullif($6, '')::uuid)
		returning id::text
	`, strings.TrimSpace(input.SiteID), workspaceID, input.Kind, strings.TrimSpace(input.Prompt), payloadJSON, strings.TrimSpace(input.UserID)).Scan(&jobID); err != nil {
		return "", fmt.Errorf("create generation job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit generation job admission: %w", err)
	}
	return jobID, nil
}

func (m *PromptActionManager) UpdateProgress(ctx context.Context, jobID string, kind JobKind, stepName string) error {
	if m == nil || m.store == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	step := StepForJob(kind, stepName)
	if step == nil {
		return nil
	}
	if _, err := m.store.Exec(ctx, `
		update generation_jobs
		set kind = $1,
		    state = 'running',
		    status = 'running',
		    current_step = $2,
		    error_reason = null,
		    started_at = coalesce(started_at, now()),
		    completed_at = null,
		    updated_at = now()
		where id = $3::uuid
	`, kind, step.Name, jobID); err != nil {
		return fmt.Errorf("update generation job progress: %w", err)
	}
	return nil
}

func (m *PromptActionManager) CompleteJob(ctx context.Context, jobID string, siteID string, output any) error {
	if m == nil || m.store == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("encode generation output: %w", err)
	}

	tx, err := m.store.BeginPromptTx(ctx)
	if err != nil {
		return fmt.Errorf("begin generation completion: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var workspaceID string
	if err := tx.QueryRow(ctx, `
		update generation_jobs
		set site_id = nullif($1, '')::uuid,
		    state = 'succeeded',
		    status = 'completed',
		    output_plan = $2,
		    current_step = 'persist',
		    error = null,
		    error_reason = null,
		    completed_at = now(),
		    updated_at = now()
		where id = $3::uuid
		  and state <> 'succeeded'
		returning workspace_id::text
	`, strings.TrimSpace(siteID), outputJSON, jobID).Scan(&workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("complete generation job: %w", err)
	}

	if err := m.incrementTrialPromptUsage(ctx, tx, workspaceID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit generation completion: %w", err)
	}
	return nil
}

func (m *PromptActionManager) FailJob(ctx context.Context, jobID string, cause error) error {
	if m == nil || m.store == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	payload := map[string]any{
		"message": cause.Error(),
	}
	var validationErr siteconfig.ValidationError
	if errors.As(cause, &validationErr) {
		payload["issues"] = validationErr.Issues
	}
	errorJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode generation error: %w", err)
	}
	if _, err := m.store.Exec(ctx, `
		update generation_jobs
		set state = 'failed',
		    status = 'failed',
		    error = $1,
		    error_reason = $2,
		    completed_at = now(),
		    updated_at = now()
		where id = $3::uuid
	`, errorJSON, generationFailureReason(cause), jobID); err != nil {
		return fmt.Errorf("mark generation job failed: %w", err)
	}
	return nil
}

func (m *PromptActionManager) enforcePromptQuota(ctx context.Context, tx promptActionTx, workspaceID string) error {
	var entitlement billing.Entitlement
	var siteLimit *int
	var promptLimit *int
	var assetBytes *int64
	err := tx.QueryRow(ctx, `
		select workspace_id::text,
		       plan,
		       status,
		       subscription_live,
		       custom_domains_enabled,
		       active_site_limit,
		       monthly_prompt_limit,
		       asset_storage_limit_bytes,
		       updated_at
		from billing_entitlements
		where workspace_id = $1::uuid
		for update
	`, workspaceID).Scan(
		&entitlement.WorkspaceID,
		&entitlement.Plan,
		&entitlement.Status,
		&entitlement.SubscriptionLive,
		&entitlement.CustomDomainsEnabled,
		&siteLimit,
		&promptLimit,
		&assetBytes,
		&entitlement.UpdatedAt,
	)
	switch {
	case err == nil:
		entitlement.ActiveSiteLimit = siteLimit
		entitlement.MonthlyPromptLimit = promptLimit
		entitlement.AssetStorageLimitBytes = assetBytes
	case errors.Is(err, pgx.ErrNoRows):
		entitlement.WorkspaceID = workspaceID
	default:
		return fmt.Errorf("load billing entitlement: %w", err)
	}

	if entitlement.SubscriptionLive {
		return m.enforcePaidPromptQuota(ctx, tx, workspaceID, entitlement)
	}
	return m.enforceTrialPromptQuota(ctx, tx, workspaceID)
}

func (m *PromptActionManager) enforceTrialPromptQuota(ctx context.Context, tx promptActionTx, workspaceID string) error {
	rows, err := tx.Query(ctx, `
		select prompts_used
		from guest_sessions
		where workspace_id = $1::uuid
		for update
	`, workspaceID)
	if err != nil {
		return fmt.Errorf("lock trial prompt usage: %w", err)
	}
	defer rows.Close()

	promptsUsed := 0
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			return fmt.Errorf("scan trial prompt usage: %w", err)
		}
		promptsUsed += value
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate trial prompt usage: %w", err)
	}

	activeReservations, err := promptReservationCount(ctx, tx, workspaceID, nil, false)
	if err != nil {
		return err
	}
	if promptsUsed+activeReservations >= trialPromptLimit {
		return &PromptQuotaExceededError{
			Code:    "trial_exhausted",
			Message: "your trial has reached its prompt limit",
		}
	}
	return nil
}

func (m *PromptActionManager) enforcePaidPromptQuota(ctx context.Context, tx promptActionTx, workspaceID string, entitlement billing.Entitlement) error {
	limit := entitlement.MonthlyPromptLimit
	if limit == nil || *limit <= 0 {
		return nil
	}

	var periodStart *time.Time
	var periodEnd *time.Time
	err := tx.QueryRow(ctx, `
		select current_period_start, current_period_end
		from billing_subscriptions
		where workspace_id = $1::uuid
		  and status in ('active', 'trialing')
		order by updated_at desc
		limit 1
	`, workspaceID).Scan(&periodStart, &periodEnd)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load billing period: %w", err)
	}
	if periodStart == nil || periodEnd == nil {
		return nil
	}

	reservedAndCompleted, err := promptReservationCount(ctx, tx, workspaceID, []time.Time{periodStart.UTC(), periodEnd.UTC()}, true)
	if err != nil {
		return err
	}
	if reservedAndCompleted >= *limit {
		return &PromptQuotaExceededError{
			Code: "plan_limit_exceeded",
			Message: fmt.Sprintf(
				"your %s plan has used its current prompt allowance; upgrade or wait for the next billing period",
				humanPromptPlanName(entitlement.Plan),
			),
		}
	}
	return nil
}

func promptReservationCount(ctx context.Context, store interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, workspaceID string, period []time.Time, includeSucceeded bool) (int, error) {
	statuses := []string{"pending", "running"}
	if includeSucceeded {
		statuses = append(statuses, "succeeded")
	}
	query := `
		select count(*)
		from generation_jobs
		where workspace_id = $1::uuid
		  and kind = any($2::text[])
		  and state = any($3::text[])
	`
	args := []any{workspaceID, promptCountingJobKinds(), statuses}
	if len(period) == 2 {
		query += `
		  and created_at >= $4
		  and created_at < $5
		`
		args = append(args, period[0], period[1])
	}
	var count int
	if err := store.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count prompt reservations: %w", err)
	}
	return count, nil
}

func (m *PromptActionManager) incrementTrialPromptUsage(ctx context.Context, tx promptActionTx, workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil
	}
	var subscriptionLive bool
	err := tx.QueryRow(ctx, `
		select subscription_live
		from billing_entitlements
		where workspace_id = $1::uuid
	`, workspaceID).Scan(&subscriptionLive)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load prompt accounting entitlement: %w", err)
	}
	if subscriptionLive {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		update guest_sessions
		set prompts_used = prompts_used + 1,
		    last_seen_at = now()
		where workspace_id = $1::uuid
	`, workspaceID); err != nil {
		return fmt.Errorf("increment trial prompt usage: %w", err)
	}
	return nil
}

func promptCountingJobKinds() []string {
	return []string{
		string(JobKindSite),
		string(JobKindPageReprompt),
		string(JobKindSiteReprompt),
		string(JobKindThemeRegenerate),
		string(JobKindBlockSuggest),
		string(JobKindCollectionDraft),
		string(JobKindEntryDraft),
	}
}

func humanPromptPlanName(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "pro":
		return "Pro"
	default:
		return "Basic"
	}
}
