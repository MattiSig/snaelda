package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/email"
	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	planTrial = "trial"
	planBasic = "basic"
	planPro   = "pro"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type txBeginner interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type stripeClient interface {
	CreateCheckoutSession(ctx context.Context, req CheckoutSessionRequest) (CheckoutSessionResult, error)
	CreatePortalSession(ctx context.Context, req PortalSessionRequest) (PortalSessionResult, error)
	ConstructWebhookEvent(payload []byte, header string) (WebhookEvent, error)
}

type ServiceConfig struct {
	Stripe          stripeClient
	SuccessURL      string
	CancelURL       string
	PortalReturnURL string
	AppBaseURL      string
	BasicPriceID    string
	ProPriceID      string
	OnceOverPriceID string
	ProductName     string
	EmailSender     email.Sender
	AuditRecorder   *audit.Recorder
}

type Service struct {
	store           DB
	stripe          stripeClient
	successURL      string
	cancelURL       string
	portalReturnURL string
	appBaseURL      string
	catalog         Catalog
	onceOverPriceID string
	productName     string
	emailSender     email.Sender
	auditRecorder   *audit.Recorder
}

type Entitlement struct {
	WorkspaceID            string    `json:"workspaceId"`
	Plan                   string    `json:"plan"`
	Status                 string    `json:"status"`
	SubscriptionLive       bool      `json:"subscriptionLive"`
	CustomDomainsEnabled   bool      `json:"customDomainsEnabled"`
	ActiveSiteLimit        *int      `json:"activeSiteLimit,omitempty"`
	MonthlyPromptLimit     *int      `json:"monthlyPromptLimit,omitempty"`
	AssetStorageLimitBytes *int64    `json:"assetStorageLimitBytes,omitempty"`
	CollectionLimit        *int      `json:"collectionLimit,omitempty"`
	CollectionEntryLimit   *int      `json:"collectionEntryLimit,omitempty"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type CheckoutInput struct {
	Session      auth.Session
	Plan         string
	PurchaseType string
	// Email lets an unclaimed trial session set the customer email
	// up-front so Stripe pre-fills it and the post-payment claim binds
	// the workspace to a known address instead of whatever the customer
	// types at Stripe.
	Email string
}

type BillingContact struct {
	WorkspaceID      string
	WorkspaceName    string
	UserID           string
	UserEmail        string
	UserName         string
	StripeCustomerID string
}

func NewService(store DB, cfg ServiceConfig) *Service {
	if cfg.ProductName == "" {
		cfg.ProductName = "Snaelda"
	}

	return &Service{
		store:           store,
		stripe:          cfg.Stripe,
		successURL:      strings.TrimSpace(cfg.SuccessURL),
		cancelURL:       strings.TrimSpace(cfg.CancelURL),
		portalReturnURL: strings.TrimSpace(cfg.PortalReturnURL),
		appBaseURL:      strings.TrimSpace(cfg.AppBaseURL),
		catalog:         NewCatalog(cfg.BasicPriceID, cfg.ProPriceID),
		onceOverPriceID: strings.TrimSpace(cfg.OnceOverPriceID),
		productName:     cfg.ProductName,
		emailSender:     cfg.EmailSender,
		auditRecorder:   cfg.AuditRecorder,
	}
}

func (s *Service) CreateCheckoutSession(ctx context.Context, input CheckoutInput) (string, error) {
	if s == nil || s.store == nil || s.stripe == nil {
		return "", fmt.Errorf("billing is not configured")
	}

	purchaseType := normalizePurchaseType(input.PurchaseType)
	plan := normalizePlan(input.Plan)
	priceID := ""
	mode := checkoutModeSubscription

	switch purchaseType {
	case onceOverPurchaseType:
		mode = checkoutModePayment
		priceID = strings.TrimSpace(s.onceOverPriceID)
		if priceID == "" {
			return "", fmt.Errorf("once-over checkout is not configured")
		}
		state, err := s.GetOnceOverState(ctx, input.Session.WorkspaceID)
		if err != nil {
			return "", err
		}
		if state.Status == onceOverStatusAwaitingIntake || state.Status == onceOverStatusPending {
			return "", fmt.Errorf("finish the current once-over request before buying another")
		}
	default:
		var ok bool
		priceID, ok = s.catalog.PriceIDForPlan(plan)
		if !ok {
			return "", fmt.Errorf("unknown billing plan %q", plan)
		}
	}

	contact, err := s.lookupBillingContact(ctx, input.Session.WorkspaceID)
	if err != nil {
		return "", err
	}

	customerID := strings.TrimSpace(contact.StripeCustomerID)
	if customerID == "" {
		customerID = s.lookupCustomerID(ctx, input.Session.WorkspaceID)
	}

	customerEmail := contact.UserEmail
	if customerEmail == "" {
		customerEmail = strings.TrimSpace(input.Email)
	}

	result, err := s.stripe.CreateCheckoutSession(ctx, CheckoutSessionRequest{
		WorkspaceID:   input.Session.WorkspaceID,
		WorkspaceName: contact.WorkspaceName,
		Plan:          plan,
		PurchaseType:  purchaseType,
		Mode:          mode,
		PriceID:       priceID,
		CustomerID:    customerID,
		CustomerEmail: customerEmail,
		SuccessURL:    s.successURL,
		CancelURL:     s.cancelURL,
	})
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(result.CustomerID) != "" {
		if err := s.upsertCustomer(ctx, input.Session.WorkspaceID, result.CustomerID, customerEmail); err != nil {
			return "", err
		}
	}

	return result.URL, nil
}

func (s *Service) CreatePortalSession(ctx context.Context, workspaceID string) (string, error) {
	if s == nil || s.store == nil || s.stripe == nil {
		return "", fmt.Errorf("billing is not configured")
	}
	customerID := s.lookupCustomerID(ctx, workspaceID)
	if customerID == "" {
		return "", pgx.ErrNoRows
	}
	result, err := s.stripe.CreatePortalSession(ctx, PortalSessionRequest{
		CustomerID: customerID,
		ReturnURL:  s.portalReturnURL,
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s *Service) GetEntitlement(ctx context.Context, workspaceID string) (Entitlement, error) {
	if s == nil || s.store == nil {
		return Entitlement{}, fmt.Errorf("billing is not configured")
	}

	var entitlement Entitlement
	var siteLimit *int
	var promptLimit *int
	var assetBytes *int64
	var collectionLimit *int
	var collectionEntryLimit *int
	err := s.store.QueryRow(ctx, `
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
	if err != pgx.ErrNoRows {
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

func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if s == nil || s.store == nil || s.stripe == nil {
		return fmt.Errorf("billing is not configured")
	}
	event, err := s.stripe.ConstructWebhookEvent(payload, signature)
	if err != nil {
		return err
	}

	tx, err := beginTx(ctx, s.store)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var processed bool
	if err := tx.QueryRow(ctx, `
		select exists(select 1 from billing_events where stripe_event_id = $1)
	`, event.ID).Scan(&processed); err != nil {
		return err
	}
	if processed {
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `
		insert into billing_events (stripe_event_id, event_type, payload)
		values ($1, $2, $3::jsonb)
	`, event.ID, event.Type, payload); err != nil {
		return err
	}

	switch event.Type {
	case "checkout.session.completed":
		if event.CheckoutSession.Mode == checkoutModePayment || event.CheckoutSession.PurchaseType == onceOverPurchaseType {
			if err := s.handleOnceOverCheckoutCompleted(ctx, tx, event); err != nil {
				return err
			}
			break
		}
		if err := s.handleCheckoutCompleted(ctx, tx, event, event.ID); err != nil {
			return err
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		if err := s.handleSubscriptionEvent(ctx, tx, event.Subscription); err != nil {
			return err
		}
	case "invoice.paid":
		if err := s.handleInvoicePaid(ctx, tx, event.Invoice, event.ID); err != nil {
			return err
		}
	case "invoice.payment_failed":
		if err := s.handleInvoicePaymentFailed(ctx, tx, event.Invoice, event.ID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func normalizePurchaseType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", subscriptionPurchaseType:
		return subscriptionPurchaseType
	case onceOverPurchaseType:
		return onceOverPurchaseType
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func (s *Service) handleCheckoutCompleted(ctx context.Context, tx pgx.Tx, event WebhookEvent, eventID string) error {
	workspaceID := strings.TrimSpace(event.CheckoutSession.WorkspaceID)
	if workspaceID == "" {
		return nil
	}
	if event.CheckoutSession.CustomerID != "" {
		if err := s.upsertCustomerTx(ctx, tx, workspaceID, event.CheckoutSession.CustomerID, event.CheckoutSession.CustomerEmail); err != nil {
			return err
		}
	}
	claimedEmail := strings.TrimSpace(event.CheckoutSession.CustomerEmail)
	if claimedEmail == "" {
		return nil
	}
	if err := claimWorkspaceByEmail(ctx, tx, workspaceID, claimedEmail); err != nil {
		return err
	}
	if s.emailSender.Mailer == nil {
		return nil
	}
	contact, err := lookupBillingContactTx(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	loginURL := s.appBaseURL
	if loginURL != "" {
		loginURL = strings.TrimRight(loginURL, "/") + "/login"
	}
	if _, err := s.emailSender.SendWorkspaceClaimed(ctx,
		email.Address{Email: claimedEmail, Name: contact.UserName},
		email.WorkspaceClaimedTemplateData{
			ProductName:   s.productName,
			WorkspaceName: contact.WorkspaceName,
			LoginURL:      loginURL,
		},
		"workspace_claimed:"+eventID,
	); err != nil {
		return err
	}
	return nil
}

func (s *Service) handleSubscriptionEvent(ctx context.Context, tx pgx.Tx, subscription SubscriptionEventData) error {
	workspaceID := strings.TrimSpace(subscription.WorkspaceID)
	if workspaceID == "" && subscription.CustomerID != "" {
		workspaceID = lookupWorkspaceIDByCustomer(ctx, tx, subscription.CustomerID)
	}
	if workspaceID == "" {
		return nil
	}

	live := subscription.Status == "active" || subscription.Status == "trialing"
	plan, planDef, err := s.resolveSubscriptionPlan(ctx, tx, subscription)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into billing_subscriptions (
			workspace_id, stripe_customer_id, stripe_subscription_id, plan, status, price_id, product_id,
			current_period_start, current_period_end, cancel_at_period_end, canceled_at
		)
		values ($1, $2, $3, $4, $5, nullif($6, ''), nullif($7, ''), $8, $9, $10, $11)
		on conflict (stripe_subscription_id) do update
		set workspace_id = excluded.workspace_id,
		    stripe_customer_id = excluded.stripe_customer_id,
		    plan = excluded.plan,
		    status = excluded.status,
		    price_id = excluded.price_id,
		    product_id = excluded.product_id,
		    current_period_start = excluded.current_period_start,
		    current_period_end = excluded.current_period_end,
		    cancel_at_period_end = excluded.cancel_at_period_end,
		    canceled_at = excluded.canceled_at,
		    updated_at = now()
	`, workspaceID, subscription.CustomerID, subscription.SubscriptionID, plan, subscription.Status, subscription.PriceID, subscription.ProductID, subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd, subscription.CancelAtPeriodEnd, subscription.CanceledAt); err != nil {
		return err
	}

	status := subscription.Status
	if status == "" {
		status = "inactive"
	}
	var entitlementPlan string
	var customDomainsEnabled bool
	var activeSiteLimit any
	var monthlyPromptLimit any
	var assetStorageLimitBytes any
	var collectionLimit any
	var collectionEntryLimit any
	if live {
		entitlementPlan = plan
		customDomainsEnabled = planDef.CustomDomainsEnabled
		activeSiteLimit = planDef.ActiveSiteLimit
		monthlyPromptLimit = planDef.MonthlyPromptLimit
		assetStorageLimitBytes = planDef.AssetStorageLimitBytes
		collectionLimit = planDef.CollectionLimit
		collectionEntryLimit = planDef.CollectionEntryLimit
	} else {
		entitlementPlan = planTrial
	}
	if _, err := tx.Exec(ctx, `
		insert into billing_entitlements (
			workspace_id, plan, status, subscription_live, custom_domains_enabled,
			active_site_limit, monthly_prompt_limit, asset_storage_limit_bytes,
			collection_limit, collection_entry_limit
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		on conflict (workspace_id) do update
		set plan = excluded.plan,
		    status = excluded.status,
		    subscription_live = excluded.subscription_live,
		    custom_domains_enabled = excluded.custom_domains_enabled,
		    active_site_limit = excluded.active_site_limit,
		    monthly_prompt_limit = excluded.monthly_prompt_limit,
		    asset_storage_limit_bytes = excluded.asset_storage_limit_bytes,
		    collection_limit = excluded.collection_limit,
		    collection_entry_limit = excluded.collection_entry_limit,
		    updated_at = now()
	`, workspaceID, entitlementPlan, status, live, customDomainsEnabled, activeSiteLimit, monthlyPromptLimit, assetStorageLimitBytes, collectionLimit, collectionEntryLimit); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		update workspaces
		set plan = $2,
		    stripe_customer_id = nullif($3, ''),
		    updated_at = now()
		where id = $1
	`, workspaceID, map[bool]string{true: plan, false: planTrial}[live], subscription.CustomerID); err != nil {
		return err
	}
	return nil
}

func (s *Service) handleInvoicePaid(ctx context.Context, tx pgx.Tx, invoice InvoiceEventData, eventID string) error {
	workspaceID := lookupWorkspaceIDByCustomer(ctx, tx, invoice.CustomerID)
	if workspaceID == "" {
		return nil
	}
	contact, err := lookupBillingContactTx(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	if contact.UserEmail == "" {
		contact.UserEmail = strings.TrimSpace(invoice.CustomerEmail)
	}
	if contact.UserEmail == "" || s.emailSender.Mailer == nil {
		return nil
	}
	amount, currency := formatAmount(invoice.AmountPaid, invoice.Currency)
	planName := humanPlanName(invoice.Plan)
	if plan, ok := s.catalog.PlanByPriceID(invoice.PriceID); ok {
		planName = plan.Name
	}
	_, err = s.emailSender.SendBillingReceipt(ctx, email.Address{Email: contact.UserEmail, Name: contact.UserName}, email.BillingReceiptTemplateData{
		ProductName:   s.productName,
		WorkspaceName: contact.WorkspaceName,
		Amount:        amount,
		Currency:      currency,
		ReceiptURL:    invoice.HostedInvoiceURL,
		PlanName:      planName,
	}, "billing_receipt:"+eventID)
	return err
}

func (s *Service) handleInvoicePaymentFailed(ctx context.Context, tx pgx.Tx, invoice InvoiceEventData, eventID string) error {
	workspaceID := lookupWorkspaceIDByCustomer(ctx, tx, invoice.CustomerID)
	if workspaceID == "" {
		return nil
	}
	contact, err := lookupBillingContactTx(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	if contact.UserEmail == "" {
		contact.UserEmail = strings.TrimSpace(invoice.CustomerEmail)
	}
	if contact.UserEmail == "" || s.emailSender.Mailer == nil {
		return nil
	}

	portalURL := s.portalReturnURL
	if invoice.CustomerID != "" {
		if portal, portalErr := s.stripe.CreatePortalSession(ctx, PortalSessionRequest{
			CustomerID: invoice.CustomerID,
			ReturnURL:  s.portalReturnURL,
		}); portalErr == nil && strings.TrimSpace(portal.URL) != "" {
			portalURL = portal.URL
		}
	}

	planName := humanPlanName(invoice.Plan)
	if plan, ok := s.catalog.PlanByPriceID(invoice.PriceID); ok {
		planName = plan.Name
	}

	_, err = s.emailSender.SendBillingPaymentFailed(ctx, email.Address{Email: contact.UserEmail, Name: contact.UserName}, email.BillingPaymentFailedTemplateData{
		ProductName:   s.productName,
		WorkspaceName: contact.WorkspaceName,
		PlanName:      planName,
		PortalURL:     portalURL,
	}, "billing_payment_failed:"+eventID)
	return err
}

func (s *Service) Catalog() CatalogResponse {
	return s.catalog.Response()
}

func (s *Service) lookupBillingContact(ctx context.Context, workspaceID string) (BillingContact, error) {
	return lookupBillingContactTx(ctx, s.store, workspaceID)
}

func lookupBillingContactTx(ctx context.Context, store interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, workspaceID string) (BillingContact, error) {
	var contact BillingContact
	err := store.QueryRow(ctx, `
		select w.id::text,
		       w.name,
		       coalesce(u.id::text, ''),
		       coalesce(u.email, bc.email, ''),
		       coalesce(u.name, ''),
		       coalesce(w.stripe_customer_id, '')
		from workspaces w
		left join users u on u.id = w.created_by
		left join billing_customers bc on bc.workspace_id = w.id
		where w.id = $1
	`, workspaceID).Scan(&contact.WorkspaceID, &contact.WorkspaceName, &contact.UserID, &contact.UserEmail, &contact.UserName, &contact.StripeCustomerID)
	return contact, err
}

func (s *Service) lookupCustomerID(ctx context.Context, workspaceID string) string {
	var customerID string
	_ = s.store.QueryRow(ctx, `
		select stripe_customer_id
		from billing_customers
		where workspace_id = $1
	`, workspaceID).Scan(&customerID)
	return strings.TrimSpace(customerID)
}

func lookupWorkspaceIDByCustomer(ctx context.Context, store interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, customerID string) string {
	if strings.TrimSpace(customerID) == "" {
		return ""
	}
	var workspaceID string
	_ = store.QueryRow(ctx, `
		select workspace_id::text
		from billing_customers
		where stripe_customer_id = $1
	`, customerID).Scan(&workspaceID)
	return workspaceID
}

func (s *Service) upsertCustomer(ctx context.Context, workspaceID string, customerID string, emailAddress string) error {
	return s.upsertCustomerTx(ctx, s.store, workspaceID, customerID, emailAddress)
}

func (s *Service) upsertCustomerTx(ctx context.Context, store interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}, workspaceID string, customerID string, emailAddress string) error {
	if strings.TrimSpace(customerID) == "" {
		return nil
	}
	if _, err := store.Exec(ctx, `
		insert into billing_customers (workspace_id, stripe_customer_id, email)
		values ($1, $2, nullif($3, ''))
		on conflict (workspace_id) do update
		set stripe_customer_id = excluded.stripe_customer_id,
		    email = coalesce(excluded.email, billing_customers.email),
		    updated_at = now()
	`, workspaceID, customerID, emailAddress); err != nil {
		return err
	}
	if _, err := store.Exec(ctx, `
		update workspaces
		set stripe_customer_id = $2,
		    updated_at = now()
		where id = $1
	`, workspaceID, customerID); err != nil {
		return err
	}
	return nil
}

func normalizePlan(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", planTrial:
		return planBasic
	case planBasic:
		return planBasic
	case planPro:
		return planPro
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func humanPlanName(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case planTrial:
		return "Trial"
	case planPro:
		return "Pro"
	default:
		return "Basic"
	}
}

func (s *Service) resolveSubscriptionPlan(ctx context.Context, tx pgx.Tx, subscription SubscriptionEventData) (string, PlanDefinition, error) {
	if plan, ok := s.catalog.PlanByPriceID(subscription.PriceID); ok {
		return plan.ID, plan, nil
	}

	if strings.TrimSpace(subscription.SubscriptionID) != "" {
		var existingPlan string
		if err := tx.QueryRow(ctx, `
			select plan
			from billing_subscriptions
			where stripe_subscription_id = $1
		`, subscription.SubscriptionID).Scan(&existingPlan); err == nil {
			if plan, ok := s.catalog.PlanByID(existingPlan); ok {
				return plan.ID, plan, nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", PlanDefinition{}, err
		}
	}

	if plan, ok := s.catalog.PlanByID(subscription.Plan); ok {
		return plan.ID, plan, nil
	}

	return "", PlanDefinition{}, fmt.Errorf("stripe subscription price %q is not configured to a billing plan", strings.TrimSpace(subscription.PriceID))
}

func formatAmount(amountMinor int64, currency string) (string, string) {
	ccy := strings.ToUpper(strings.TrimSpace(currency))
	if ccy == "" {
		ccy = "USD"
	}
	return fmt.Sprintf("%.2f", float64(amountMinor)/100), ccy
}

func claimWorkspaceByEmail(ctx context.Context, tx pgx.Tx, workspaceID string, emailAddress string) error {
	emailAddress = strings.ToLower(strings.TrimSpace(emailAddress))
	if emailAddress == "" {
		return nil
	}

	var userID string
	var userName string
	if err := tx.QueryRow(ctx, `
		insert into users (email)
		values ($1)
		on conflict (email) do update
		set updated_at = now()
		returning id::text, coalesce(name, '')
	`, emailAddress).Scan(&userID, &userName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into workspace_members (workspace_id, user_id, role)
		values ($1, $2, 'owner')
		on conflict (workspace_id, user_id) do update
		set role = excluded.role
	`, workspaceID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update workspaces
		set created_by = coalesce(created_by, $2),
		    updated_at = now()
		where id = $1
	`, workspaceID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update guest_sessions
		set claimed_by_user_id = coalesce(claimed_by_user_id, $2),
		    claimed_at = coalesce(claimed_at, now()),
		    recovery_key_hash = null,
		    last_seen_at = now()
		where workspace_id = $1
	`, workspaceID, userID); err != nil {
		return err
	}
	return nil
}

func beginTx(ctx context.Context, store DB) (pgx.Tx, error) {
	beginner, ok := store.(txBeginner)
	if !ok {
		return nil, fmt.Errorf("transaction support is not configured")
	}
	return beginner.BeginTx(ctx, pgx.TxOptions{})
}

func encodeJSON(value any) []byte {
	payload, _ := json.Marshal(value)
	return payload
}
