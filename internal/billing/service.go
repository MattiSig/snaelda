package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/email"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	planTrial = "trial"
	planBasic = "basic"
	planPro   = "pro"
)

type DB interface {
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
	Stripe             stripeClient
	SuccessURL         string
	CancelURL          string
	PortalReturnURL    string
	BasicPriceID       string
	ProPriceID         string
	ProductName        string
	EmailSender        email.Sender
	DefaultSiteLimit   int
	DefaultPromptLimit int
	DefaultAssetBytes  int64
}

type Service struct {
	store              DB
	stripe             stripeClient
	successURL         string
	cancelURL          string
	portalReturnURL    string
	priceByPlan        map[string]string
	productName        string
	emailSender        email.Sender
	defaultSiteLimit   int
	defaultPromptLimit int
	defaultAssetBytes  int64
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
	UpdatedAt              time.Time `json:"updatedAt"`
}

type CheckoutInput struct {
	Session auth.Session
	Plan    string
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
	if cfg.DefaultSiteLimit == 0 {
		cfg.DefaultSiteLimit = 3
	}
	if cfg.DefaultPromptLimit == 0 {
		cfg.DefaultPromptLimit = 250
	}
	if cfg.DefaultAssetBytes == 0 {
		cfg.DefaultAssetBytes = 5 << 30
	}

	priceByPlan := map[string]string{}
	if strings.TrimSpace(cfg.BasicPriceID) != "" {
		priceByPlan[planBasic] = strings.TrimSpace(cfg.BasicPriceID)
	}
	if strings.TrimSpace(cfg.ProPriceID) != "" {
		priceByPlan[planPro] = strings.TrimSpace(cfg.ProPriceID)
	}

	return &Service{
		store:              store,
		stripe:             cfg.Stripe,
		successURL:         strings.TrimSpace(cfg.SuccessURL),
		cancelURL:          strings.TrimSpace(cfg.CancelURL),
		portalReturnURL:    strings.TrimSpace(cfg.PortalReturnURL),
		priceByPlan:        priceByPlan,
		productName:        cfg.ProductName,
		emailSender:        cfg.EmailSender,
		defaultSiteLimit:   cfg.DefaultSiteLimit,
		defaultPromptLimit: cfg.DefaultPromptLimit,
		defaultAssetBytes:  cfg.DefaultAssetBytes,
	}
}

func (s *Service) CreateCheckoutSession(ctx context.Context, input CheckoutInput) (string, error) {
	if s == nil || s.store == nil || s.stripe == nil {
		return "", fmt.Errorf("billing is not configured")
	}

	plan := normalizePlan(input.Plan)
	priceID, ok := s.priceByPlan[plan]
	if !ok {
		return "", fmt.Errorf("unknown billing plan %q", plan)
	}

	contact, err := s.lookupBillingContact(ctx, input.Session.WorkspaceID)
	if err != nil {
		return "", err
	}

	customerID := strings.TrimSpace(contact.StripeCustomerID)
	if customerID == "" {
		customerID = s.lookupCustomerID(ctx, input.Session.WorkspaceID)
	}

	result, err := s.stripe.CreateCheckoutSession(ctx, CheckoutSessionRequest{
		WorkspaceID:   input.Session.WorkspaceID,
		WorkspaceName: contact.WorkspaceName,
		Plan:          plan,
		PriceID:       priceID,
		CustomerID:    customerID,
		CustomerEmail: contact.UserEmail,
		SuccessURL:    s.successURL,
		CancelURL:     s.cancelURL,
	})
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(result.CustomerID) != "" {
		if err := s.upsertCustomer(ctx, input.Session.WorkspaceID, result.CustomerID, contact.UserEmail); err != nil {
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
	err := s.store.QueryRow(ctx, `
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
		&entitlement.UpdatedAt,
	)
	if err == nil {
		entitlement.ActiveSiteLimit = siteLimit
		entitlement.MonthlyPromptLimit = promptLimit
		entitlement.AssetStorageLimitBytes = assetBytes
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
		if err := s.handleCheckoutCompleted(ctx, tx, event); err != nil {
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

func (s *Service) handleCheckoutCompleted(ctx context.Context, tx pgx.Tx, event WebhookEvent) error {
	workspaceID := strings.TrimSpace(event.CheckoutSession.WorkspaceID)
	if workspaceID == "" {
		return nil
	}
	if event.CheckoutSession.CustomerID != "" {
		if err := s.upsertCustomerTx(ctx, tx, workspaceID, event.CheckoutSession.CustomerID, event.CheckoutSession.CustomerEmail); err != nil {
			return err
		}
	}
	if event.CheckoutSession.CustomerEmail != "" {
		if err := claimWorkspaceByEmail(ctx, tx, workspaceID, event.CheckoutSession.CustomerEmail); err != nil {
			return err
		}
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

	plan := normalizePlan(subscription.Plan)
	live := subscription.Status == "active" || subscription.Status == "trialing"
	if plan == "" {
		if live {
			plan = planBasic
		} else {
			plan = planTrial
		}
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
	if _, err := tx.Exec(ctx, `
		insert into billing_entitlements (
			workspace_id, plan, status, subscription_live, custom_domains_enabled,
			active_site_limit, monthly_prompt_limit, asset_storage_limit_bytes
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (workspace_id) do update
		set plan = excluded.plan,
		    status = excluded.status,
		    subscription_live = excluded.subscription_live,
		    custom_domains_enabled = excluded.custom_domains_enabled,
		    active_site_limit = excluded.active_site_limit,
		    monthly_prompt_limit = excluded.monthly_prompt_limit,
		    asset_storage_limit_bytes = excluded.asset_storage_limit_bytes,
		    updated_at = now()
	`, workspaceID, map[bool]string{true: plan, false: planTrial}[live], status, live, live, s.defaultSiteLimit, s.defaultPromptLimit, s.defaultAssetBytes); err != nil {
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
	_, err = s.emailSender.SendBillingReceipt(ctx, email.Address{Email: contact.UserEmail, Name: contact.UserName}, email.BillingReceiptTemplateData{
		ProductName:   s.productName,
		WorkspaceName: contact.WorkspaceName,
		Amount:        amount,
		Currency:      currency,
		ReceiptURL:    invoice.HostedInvoiceURL,
		PlanName:      humanPlanName(invoice.Plan),
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

	_, err = s.emailSender.SendBillingPaymentFailed(ctx, email.Address{Email: contact.UserEmail, Name: contact.UserName}, email.BillingPaymentFailedTemplateData{
		ProductName:   s.productName,
		WorkspaceName: contact.WorkspaceName,
		PlanName:      humanPlanName(invoice.Plan),
		PortalURL:     portalURL,
	}, "billing_payment_failed:"+eventID)
	return err
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
		       coalesce(u.email, ''),
		       coalesce(u.name, ''),
		       coalesce(w.stripe_customer_id, '')
		from workspaces w
		left join users u on u.id = w.created_by
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
	switch normalizePlan(plan) {
	case planPro:
		return "Pro"
	default:
		return "Basic"
	}
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
