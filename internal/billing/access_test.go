package billing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEnforceCollectionLimitBlocksAtPlanCap(t *testing.T) {
	store := accessStoreStub{
		entitlement: Entitlement{
			WorkspaceID:          "workspace-1",
			Plan:                 planBasic,
			Status:               "active",
			SubscriptionLive:     true,
			CollectionLimit:      intPtr(5),
			CollectionEntryLimit: intPtr(100),
			UpdatedAt:            time.Now().UTC(),
		},
		collectionCount: 5,
	}

	err := EnforceCollectionLimit(context.Background(), store, "workspace-1", 1)
	if !errors.Is(err, ErrPlanLimitExceeded) {
		t.Fatalf("expected plan limit error, got %v", err)
	}
	if !strings.Contains(err.Error(), "5 collections") {
		t.Fatalf("expected collection limit in message, got %q", err.Error())
	}
}

func TestEnforceCollectionEntryLimitBlocksAtPlanCap(t *testing.T) {
	store := accessStoreStub{
		entitlement: Entitlement{
			WorkspaceID:          "workspace-1",
			Plan:                 planPro,
			Status:               "active",
			SubscriptionLive:     true,
			CollectionEntryLimit: intPtr(1000),
			UpdatedAt:            time.Now().UTC(),
		},
		collectionEntryCount: 1000,
	}

	err := EnforceCollectionEntryLimit(context.Background(), store, "workspace-1", 1)
	if !errors.Is(err, ErrPlanLimitExceeded) {
		t.Fatalf("expected plan limit error, got %v", err)
	}
	if !strings.Contains(err.Error(), "1000 collection entries/detail URLs") {
		t.Fatalf("expected entry limit in message, got %q", err.Error())
	}
}

func TestEnforceCollectionLimitsIgnoreTrialWorkspaces(t *testing.T) {
	store := accessStoreStub{
		entitlement: Entitlement{
			WorkspaceID:          "workspace-1",
			Plan:                 planTrial,
			Status:               "trial",
			SubscriptionLive:     false,
			CollectionLimit:      intPtr(1),
			CollectionEntryLimit: intPtr(1),
			UpdatedAt:            time.Now().UTC(),
		},
		collectionCount:      10,
		collectionEntryCount: 100,
	}

	if err := EnforceCollectionLimit(context.Background(), store, "workspace-1", 1); err != nil {
		t.Fatalf("expected no error for trial workspace collection limit, got %v", err)
	}
	if err := EnforceCollectionEntryLimit(context.Background(), store, "workspace-1", 1); err != nil {
		t.Fatalf("expected no error for trial workspace entry limit, got %v", err)
	}
}

type accessStoreStub struct {
	entitlement          Entitlement
	activeSiteCount      int
	assetBytes           int64
	periodPromptCount    int
	collectionCount      int
	collectionEntryCount int
}

func (s accessStoreStub) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "from billing_entitlements"):
		return accessRowStub{values: []any{
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
	case strings.Contains(sql, "from collections c"):
		return accessRowStub{values: []any{s.collectionCount}}
	case strings.Contains(sql, "from collection_entries ce"):
		return accessRowStub{values: []any{s.collectionEntryCount}}
	case strings.Contains(sql, "from sites"):
		return accessRowStub{values: []any{s.activeSiteCount}}
	case strings.Contains(sql, "from assets"):
		return accessRowStub{values: []any{s.assetBytes}}
	case strings.Contains(sql, "from billing_subscriptions"):
		return accessRowStub{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from workspaces"):
		return accessRowStub{values: []any{onceOverStatusNone}}
	case strings.Contains(sql, "from once_over_requests"):
		return accessRowStub{err: pgx.ErrNoRows}
	default:
		return accessRowStub{err: pgx.ErrNoRows}
	}
}

type accessRowStub struct {
	values []any
	err    error
}

func (r accessRowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case *bool:
			*target = r.values[i].(bool)
		case *int:
			*target = r.values[i].(int)
		case *int64:
			*target = r.values[i].(int64)
		case *time.Time:
			*target = r.values[i].(time.Time)
		case **int:
			if r.values[i] == nil {
				*target = nil
			} else {
				*target = r.values[i].(*int)
			}
		case **int64:
			if r.values[i] == nil {
				*target = nil
			} else {
				*target = r.values[i].(*int64)
			}
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func intPtr(value int) *int {
	return &value
}
