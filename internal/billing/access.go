package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AccessStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var ErrSubscriptionRequired = errors.New("subscription required")

type WorkspaceState struct {
	Entitlement Entitlement     `json:"entitlement"`
	Usage       Usage           `json:"usage"`
	OnceOver    OnceOverState   `json:"onceOver"`
	Catalog     CatalogResponse `json:"catalog"`
}

type Usage struct {
	ActiveSiteCount    int        `json:"activeSiteCount"`
	PeriodPromptCount  int        `json:"periodPromptCount"`
	UploadedAssetBytes int64      `json:"uploadedAssetBytes"`
	CurrentPeriodStart *time.Time `json:"currentPeriodStart,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"currentPeriodEnd,omitempty"`
}

type LimitExceededError struct {
	Resource string
	Message  string
}

func (e *LimitExceededError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return fmt.Sprintf("%s limit exceeded", e.Resource)
}

func (e *LimitExceededError) Is(target error) bool {
	other, ok := target.(*LimitExceededError)
	if !ok {
		return false
	}
	if other.Resource == "" {
		return true
	}
	return e.Resource == other.Resource
}

var ErrPlanLimitExceeded = &LimitExceededError{}

func LoadWorkspaceState(ctx context.Context, store AccessStore, workspaceID string) (WorkspaceState, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if store == nil || workspaceID == "" {
		return WorkspaceState{}, fmt.Errorf("billing access is not configured")
	}

	entitlement, err := loadEntitlement(ctx, store, workspaceID)
	if err != nil {
		return WorkspaceState{}, err
	}
	usage, err := loadUsage(ctx, store, workspaceID, entitlement.SubscriptionLive)
	if err != nil {
		return WorkspaceState{}, err
	}
	onceOver, err := loadOnceOverState(ctx, store, workspaceID)
	if err != nil {
		return WorkspaceState{}, err
	}

	return WorkspaceState{
		Entitlement: entitlement,
		Usage:       usage,
		OnceOver:    onceOver,
		Catalog:     NewCatalog(nil, nil, nil).Response(),
	}, nil
}

func EnforceSiteLimit(ctx context.Context, store AccessStore, workspaceID string) error {
	state, err := LoadWorkspaceState(ctx, store, workspaceID)
	if err != nil {
		return err
	}
	limit := state.Entitlement.ActiveSiteLimit
	if !state.Entitlement.SubscriptionLive || limit == nil || *limit <= 0 {
		return nil
	}
	if state.Usage.ActiveSiteCount >= *limit {
		return &LimitExceededError{
			Resource: "sites",
			Message:  fmt.Sprintf("your %s plan includes %d active sites; upgrade or remove a site before creating another", humanPlanName(state.Entitlement.Plan), *limit),
		}
	}
	return nil
}

func EnforcePromptLimit(ctx context.Context, store AccessStore, workspaceID string) error {
	state, err := LoadWorkspaceState(ctx, store, workspaceID)
	if err != nil {
		return err
	}
	limit := state.Entitlement.MonthlyPromptLimit
	if !state.Entitlement.SubscriptionLive || limit == nil || *limit <= 0 {
		return nil
	}
	if state.Usage.PeriodPromptCount >= *limit {
		return &LimitExceededError{
			Resource: "prompts",
			Message:  fmt.Sprintf("your %s plan has used its current prompt allowance; upgrade or wait for the next billing period", humanPlanName(state.Entitlement.Plan)),
		}
	}
	return nil
}

func EnforceCustomDomains(ctx context.Context, store AccessStore, workspaceID string) error {
	state, err := LoadWorkspaceState(ctx, store, workspaceID)
	if err != nil {
		return err
	}
	if state.Entitlement.SubscriptionLive && state.Entitlement.CustomDomainsEnabled {
		return nil
	}
	return fmt.Errorf("%w: upgrade to a paid plan to attach and verify custom domains", ErrSubscriptionRequired)
}

func EnforceAssetStorageLimit(ctx context.Context, store AccessStore, workspaceID string, additionalBytes int64) error {
	state, err := LoadWorkspaceState(ctx, store, workspaceID)
	if err != nil {
		return err
	}
	limit := state.Entitlement.AssetStorageLimitBytes
	if !state.Entitlement.SubscriptionLive || limit == nil || *limit <= 0 {
		return nil
	}
	if state.Usage.UploadedAssetBytes+additionalBytes > *limit {
		return &LimitExceededError{
			Resource: "assets",
			Message:  "this upload would exceed your plan's asset storage allowance; upgrade or remove existing assets first",
		}
	}
	return nil
}

func EnforceCollectionLimit(ctx context.Context, store AccessStore, workspaceID string, additionalCollections int) error {
	if additionalCollections <= 0 {
		return nil
	}
	state, err := LoadWorkspaceState(ctx, store, workspaceID)
	if err != nil {
		return err
	}
	limit := state.Entitlement.CollectionLimit
	if !state.Entitlement.SubscriptionLive || limit == nil || *limit <= 0 {
		return nil
	}

	var collectionCount int
	if err := store.QueryRow(ctx, `
		select count(*)
		from collections c
		join sites s on s.id = c.site_id
		where s.workspace_id = $1
	`, workspaceID).Scan(&collectionCount); err != nil {
		return err
	}
	if collectionCount+additionalCollections > *limit {
		return &LimitExceededError{
			Resource: "collections",
			Message:  fmt.Sprintf("your %s plan includes %d collections; upgrade before adding more", humanPlanName(state.Entitlement.Plan), *limit),
		}
	}
	return nil
}

func EnforceCollectionEntryLimit(ctx context.Context, store AccessStore, workspaceID string, additionalEntries int) error {
	if additionalEntries <= 0 {
		return nil
	}
	state, err := LoadWorkspaceState(ctx, store, workspaceID)
	if err != nil {
		return err
	}
	limit := state.Entitlement.CollectionEntryLimit
	if !state.Entitlement.SubscriptionLive || limit == nil || *limit <= 0 {
		return nil
	}

	var entryCount int
	if err := store.QueryRow(ctx, `
		select count(*)
		from collection_entries ce
		join sites s on s.id = ce.site_id
		where s.workspace_id = $1
	`, workspaceID).Scan(&entryCount); err != nil {
		return err
	}
	if entryCount+additionalEntries > *limit {
		return &LimitExceededError{
			Resource: "collection_entries",
			Message:  fmt.Sprintf("your %s plan includes %d collection entries/detail URLs; upgrade before adding more", humanPlanName(state.Entitlement.Plan), *limit),
		}
	}
	return nil
}

func loadEntitlement(ctx context.Context, store AccessStore, workspaceID string) (Entitlement, error) {
	var entitlement Entitlement
	var siteLimit *int
	var promptLimit *int
	var assetBytes *int64
	var collectionLimit *int
	var collectionEntryLimit *int
	err := store.QueryRow(ctx, `
		select workspace_id::text,
		       plan,
		       status,
		       subscription_live,
		       custom_domains_enabled,
		       active_site_limit,
		       monthly_prompt_limit,
		       asset_storage_limit_bytes,
		       collection_limit,
		       collection_entry_limit,
		       updated_at
		from billing_entitlements
		where workspace_id = $1
	`, workspaceID).Scan(
		&entitlement.WorkspaceID,
		&entitlement.Plan,
		&entitlement.Status,
		&entitlement.SubscriptionLive,
		&entitlement.CustomDomainsEnabled,
		&siteLimit,
		&promptLimit,
		&assetBytes,
		&collectionLimit,
		&collectionEntryLimit,
		&entitlement.UpdatedAt,
	)
	if err == nil {
		entitlement.ActiveSiteLimit = siteLimit
		entitlement.MonthlyPromptLimit = promptLimit
		entitlement.AssetStorageLimitBytes = assetBytes
		entitlement.CollectionLimit = collectionLimit
		entitlement.CollectionEntryLimit = collectionEntryLimit
		return entitlement, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Entitlement{}, err
	}

	now := time.Now().UTC()
	return Entitlement{
		WorkspaceID:          workspaceID,
		Plan:                 planTrial,
		Status:               "trial",
		SubscriptionLive:     false,
		CustomDomainsEnabled: false,
		UpdatedAt:            now,
	}, nil
}

func loadUsage(ctx context.Context, store AccessStore, workspaceID string, subscriptionLive bool) (Usage, error) {
	var usage Usage
	if err := store.QueryRow(ctx, `
		select count(*)
		from sites
		where workspace_id = $1
	`, workspaceID).Scan(&usage.ActiveSiteCount); err != nil {
		return Usage{}, err
	}
	if err := store.QueryRow(ctx, `
		select coalesce(sum(
			case
				when coalesce(metadata->>'uploadStatus', '') = 'uploaded'
				then coalesce(nullif(metadata->>'sizeBytes', '')::bigint, 0)
				else 0
			end
		), 0)
		from assets
		where workspace_id = $1
	`, workspaceID).Scan(&usage.UploadedAssetBytes); err != nil {
		return Usage{}, err
	}

	if !subscriptionLive {
		return usage, nil
	}

	var periodStart *time.Time
	var periodEnd *time.Time
	err := store.QueryRow(ctx, `
		select current_period_start, current_period_end
		from billing_subscriptions
		where workspace_id = $1
		  and status in ('active', 'trialing')
		order by updated_at desc
		limit 1
	`, workspaceID).Scan(&periodStart, &periodEnd)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Usage{}, err
	}
	usage.CurrentPeriodStart = periodStart
	usage.CurrentPeriodEnd = periodEnd

	if periodStart == nil || periodEnd == nil {
		return usage, nil
	}

	if err := store.QueryRow(ctx, `
		select count(*)
		from generation_jobs
		where workspace_id = $1
		  and status = 'completed'
		  and created_at >= $2
		  and created_at < $3
	`, workspaceID, *periodStart, *periodEnd).Scan(&usage.PeriodPromptCount); err != nil {
		return Usage{}, err
	}

	return usage, nil
}
