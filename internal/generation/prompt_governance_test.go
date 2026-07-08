package generation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type promptGovernanceStore struct {
	entitlement          *billing.Entitlement
	periodStart          *time.Time
	periodEnd            *time.Time
	activeReservations   int
	periodPromptCount    int
	trialPromptsUsed     int
	jobWorkspaceID       string
	nextJobID            string
	insertedJobPrompt    string
	completedJobOutput   []byte
	guestPromptsIncrHits int
}

func (s *promptGovernanceStore) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "from guest_sessions") {
		rows := [][]any{}
		if s.trialPromptsUsed > 0 {
			rows = append(rows, []any{s.trialPromptsUsed})
		}
		return &promptGovernanceRows{rows: rows}, nil
	}
	return &promptGovernanceRows{err: errors.New("not implemented")}, nil
}

func (s *promptGovernanceStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "select subscription_live"):
		if s.entitlement == nil {
			return promptGovernanceRow{err: pgx.ErrNoRows}
		}
		return promptGovernanceRow{values: []any{s.entitlement.SubscriptionLive}}
	case strings.Contains(sql, "from billing_entitlements"):
		if s.entitlement == nil {
			return promptGovernanceRow{err: pgx.ErrNoRows}
		}
		return promptGovernanceRow{values: []any{
			s.entitlement.WorkspaceID,
			s.entitlement.Plan,
			s.entitlement.Status,
			s.entitlement.SubscriptionLive,
			s.entitlement.CustomDomainsEnabled,
			s.entitlement.ActiveSiteLimit,
			s.entitlement.MonthlyPromptLimit,
			s.entitlement.AssetStorageLimitBytes,
			s.entitlement.CollectionLimit,
			s.entitlement.CollectionEntryLimit,
			s.entitlement.UpdatedAt,
		}}
	case strings.Contains(sql, "from billing_subscriptions"):
		if s.periodStart == nil || s.periodEnd == nil {
			return promptGovernanceRow{err: pgx.ErrNoRows}
		}
		return promptGovernanceRow{values: []any{s.periodStart, s.periodEnd}}
	case strings.Contains(sql, "from generation_jobs"):
		count := s.activeReservations
		if len(args) >= 5 {
			count = s.periodPromptCount
		}
		return promptGovernanceRow{values: []any{count}}
	case strings.Contains(sql, "insert into generation_jobs"):
		jobID := s.nextJobID
		if jobID == "" {
			jobID = "job-1"
		}
		s.jobWorkspaceID = args[1].(string)
		s.insertedJobPrompt = args[3].(string)
		return promptGovernanceRow{values: []any{jobID}}
	case strings.Contains(sql, "returning workspace_id::text"):
		s.completedJobOutput = args[1].([]byte)
		return promptGovernanceRow{values: []any{s.jobWorkspaceID}}
	default:
		return promptGovernanceRow{err: pgx.ErrNoRows}
	}
}

func (s *promptGovernanceStore) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "update guest_sessions") {
		s.guestPromptsIncrHits++
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (s *promptGovernanceStore) BeginPromptTx(context.Context) (promptActionTx, error) {
	return &promptGovernanceTx{store: s}, nil
}

type promptGovernanceTx struct {
	store *promptGovernanceStore
}

func (tx *promptGovernanceTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return tx.store.Query(ctx, sql, args...)
}

func (tx *promptGovernanceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.store.QueryRow(ctx, sql, args...)
}

func (tx *promptGovernanceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.store.Exec(ctx, sql, args...)
}

func (tx *promptGovernanceTx) Commit(context.Context) error   { return nil }
func (tx *promptGovernanceTx) Rollback(context.Context) error { return nil }

type promptGovernanceRow struct {
	values []any
	err    error
}

func (r promptGovernanceRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *bool:
			*target = value.(bool)
		case *int:
			*target = value.(int)
		case **int:
			*target = value.(*int)
		case **int64:
			*target = value.(*int64)
		case *time.Time:
			*target = value.(time.Time)
		case **time.Time:
			*target = value.(*time.Time)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

type promptGovernanceRows struct {
	rows  [][]any
	index int
	err   error
}

func (r *promptGovernanceRows) Close()                                       {}
func (r *promptGovernanceRows) Err() error                                   { return r.err }
func (r *promptGovernanceRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *promptGovernanceRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *promptGovernanceRows) RawValues() [][]byte                          { return nil }
func (r *promptGovernanceRows) Conn() *pgx.Conn                              { return nil }
func (r *promptGovernanceRows) Values() ([]any, error) {
	if r.index == 0 || r.index > len(r.rows) {
		return nil, errors.New("values called without row")
	}
	return r.rows[r.index-1], nil
}

func (r *promptGovernanceRows) Next() bool {
	if r.err != nil || r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}

func (r *promptGovernanceRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) {
		return errors.New("scan called without row")
	}
	return promptGovernanceRow{values: r.rows[r.index-1]}.Scan(dest...)
}

func TestPromptActionManagerCreateJobRejectsPaidQuotaOverflow(t *testing.T) {
	limit := 2
	now := time.Now().UTC()
	store := &promptGovernanceStore{
		entitlement: &billing.Entitlement{
			WorkspaceID:        "workspace-1",
			Plan:               "site",
			Status:             "active",
			SubscriptionLive:   true,
			MonthlyPromptLimit: &limit,
			UpdatedAt:          now,
		},
		periodStart:       ptrTime(now.Add(-time.Hour)),
		periodEnd:         ptrTime(now.Add(time.Hour)),
		periodPromptCount: 2,
	}
	manager := NewPromptActionManager(store, nil)

	_, err := manager.CreateJob(context.Background(), PromptActionInput{
		WorkspaceID: "workspace-1",
		Kind:        JobKindThemeRegenerate,
		Prompt:      "Refresh the palette",
		Payload:     map[string]any{"scope": "theme"},
	})
	var quotaErr *PromptQuotaExceededError
	if !errors.As(err, &quotaErr) {
		t.Fatalf("expected prompt quota error, got %v", err)
	}
	if quotaErr.Code != "plan_limit_exceeded" {
		t.Fatalf("expected plan_limit_exceeded, got %q", quotaErr.Code)
	}
}

func TestPromptActionManagerCompleteJobCountsTrialSuccessOnlyWithoutSubscription(t *testing.T) {
	now := time.Now().UTC()
	store := &promptGovernanceStore{
		entitlement: &billing.Entitlement{
			WorkspaceID:      "workspace-1",
			Plan:             "trial",
			Status:           "trial",
			SubscriptionLive: false,
			UpdatedAt:        now,
		},
		jobWorkspaceID: "workspace-1",
	}
	manager := NewPromptActionManager(store, nil)

	if err := manager.CompleteJob(context.Background(), "job-1", "site-1", map[string]any{"ok": true}); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	if store.guestPromptsIncrHits != 1 {
		t.Fatalf("expected trial prompt usage increment, got %d", store.guestPromptsIncrHits)
	}

	store.entitlement.SubscriptionLive = true
	if err := manager.CompleteJob(context.Background(), "job-2", "site-1", map[string]any{"ok": true}); err != nil {
		t.Fatalf("complete job for subscriber: %v", err)
	}
	if store.guestPromptsIncrHits != 1 {
		t.Fatalf("expected subscribed completion not to increment trial usage, got %d", store.guestPromptsIncrHits)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
