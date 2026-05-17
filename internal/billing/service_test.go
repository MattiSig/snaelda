package billing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/email"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCreateCheckoutSessionPersistsCustomerMapping(t *testing.T) {
	store := newFakeBillingStore()
	store.workspaces["workspace-1"] = fakeWorkspace{
		id:   "workspace-1",
		name: "Wool Shop",
	}

	stripe := &fakeStripeClient{
		checkoutResult: CheckoutSessionResult{
			URL:        "https://checkout.stripe.test/session",
			CustomerID: "cus_123",
		},
	}

	service := NewService(store, ServiceConfig{
		Stripe:          stripe,
		SuccessURL:      "https://app.test/success",
		CancelURL:       "https://app.test/cancel",
		PortalReturnURL: "https://app.test/billing",
		BasicPriceID:    "price_basic",
	})

	url, err := service.CreateCheckoutSession(context.Background(), CheckoutInput{
		Session: authSession("workspace-1"),
		Plan:    "basic",
	})
	if err != nil {
		t.Fatalf("create checkout session: %v", err)
	}
	if url != "https://checkout.stripe.test/session" {
		t.Fatalf("expected checkout url, got %q", url)
	}
	if store.customersByWorkspace["workspace-1"].customerID != "cus_123" {
		t.Fatalf("expected persisted customer mapping, got %+v", store.customersByWorkspace["workspace-1"])
	}
	if stripe.lastCheckout.PriceID != "price_basic" {
		t.Fatalf("expected basic price id, got %q", stripe.lastCheckout.PriceID)
	}
}

func TestHandleWebhookUpdatesEntitlements(t *testing.T) {
	store := newFakeBillingStore()
	store.workspaces["workspace-1"] = fakeWorkspace{id: "workspace-1", name: "Wool Shop"}
	store.workspaceByCustomer["cus_123"] = "workspace-1"

	service := NewService(store, ServiceConfig{
		Stripe:          &fakeStripeClient{event: WebhookEvent{ID: "evt_1", Type: "customer.subscription.updated", Subscription: SubscriptionEventData{WorkspaceID: "workspace-1", SubscriptionID: "sub_123", CustomerID: "cus_123", Status: "active", Plan: "pro"}}},
		SuccessURL:      "https://app.test/success",
		CancelURL:       "https://app.test/cancel",
		PortalReturnURL: "https://app.test/billing",
		BasicPriceID:    "price_basic",
		ProPriceID:      "price_pro",
	})

	if err := service.HandleWebhook(context.Background(), []byte(`{}`), "sig"); err != nil {
		t.Fatalf("handle webhook: %v", err)
	}

	entitlement, err := service.GetEntitlement(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("get entitlement: %v", err)
	}
	if !entitlement.SubscriptionLive {
		t.Fatal("expected live entitlement")
	}
	if entitlement.Plan != "pro" {
		t.Fatalf("expected pro plan, got %q", entitlement.Plan)
	}
}

func TestHandleWebhookSendsBillingReceipt(t *testing.T) {
	store := newFakeBillingStore()
	store.workspaces["workspace-1"] = fakeWorkspace{
		id:              "workspace-1",
		name:            "Wool Shop",
		createdByUserID: "user-1",
	}
	store.users["user-1"] = fakeUser{id: "user-1", email: "owner@example.com", name: "Owner"}
	store.workspaceByCustomer["cus_123"] = "workspace-1"
	store.customersByWorkspace["workspace-1"] = fakeCustomer{customerID: "cus_123", email: "owner@example.com"}

	mailer := email.NewMemoryMailer()
	service := NewService(store, ServiceConfig{
		Stripe: &fakeStripeClient{event: WebhookEvent{
			ID:   "evt_2",
			Type: "invoice.paid",
			Invoice: InvoiceEventData{
				CustomerID:       "cus_123",
				CustomerEmail:    "owner@example.com",
				AmountPaid:       2000,
				Currency:         "usd",
				HostedInvoiceURL: "https://stripe.test/invoices/inv_123",
				Plan:             "basic",
			},
		}},
		SuccessURL:      "https://app.test/success",
		CancelURL:       "https://app.test/cancel",
		PortalReturnURL: "https://app.test/billing",
		BasicPriceID:    "price_basic",
		EmailSender: email.Sender{
			Mailer:      mailer,
			DefaultFrom: email.Address{Email: "hi@snaelda.app", Name: "Snaelda"},
		},
	})

	if err := service.HandleWebhook(context.Background(), []byte(`{}`), "sig"); err != nil {
		t.Fatalf("handle webhook: %v", err)
	}

	if len(mailer.Messages) != 1 {
		t.Fatalf("expected one email, got %d", len(mailer.Messages))
	}
	msg := mailer.Messages[0]
	if msg.To[0].Email != "owner@example.com" {
		t.Fatalf("expected owner email, got %q", msg.To[0].Email)
	}
	if !strings.Contains(msg.Subject, "receipt") {
		t.Fatalf("expected receipt subject, got %q", msg.Subject)
	}
}

type fakeStripeClient struct {
	checkoutResult CheckoutSessionResult
	portalResult   PortalSessionResult
	event          WebhookEvent
	lastCheckout   CheckoutSessionRequest
}

func (f *fakeStripeClient) CreateCheckoutSession(_ context.Context, req CheckoutSessionRequest) (CheckoutSessionResult, error) {
	f.lastCheckout = req
	return f.checkoutResult, nil
}

func (f *fakeStripeClient) CreatePortalSession(context.Context, PortalSessionRequest) (PortalSessionResult, error) {
	if f.portalResult.URL == "" {
		return PortalSessionResult{URL: "https://billing.stripe.test/portal"}, nil
	}
	return f.portalResult, nil
}

func (f *fakeStripeClient) ConstructWebhookEvent([]byte, string) (WebhookEvent, error) {
	if f.event.ID == "" {
		return WebhookEvent{}, errors.New("missing event")
	}
	return f.event, nil
}

type fakeBillingStore struct {
	workspaces           map[string]fakeWorkspace
	users                map[string]fakeUser
	customersByWorkspace map[string]fakeCustomer
	workspaceByCustomer  map[string]string
	entitlements         map[string]Entitlement
	processedEvents      map[string]string
	subscriptions        map[string]SubscriptionEventData
}

type fakeWorkspace struct {
	id               string
	name             string
	createdByUserID  string
	stripeCustomerID string
	plan             string
}

type fakeUser struct {
	id    string
	email string
	name  string
}

type fakeCustomer struct {
	customerID string
	email      string
}

func newFakeBillingStore() *fakeBillingStore {
	return &fakeBillingStore{
		workspaces:           map[string]fakeWorkspace{},
		users:                map[string]fakeUser{},
		customersByWorkspace: map[string]fakeCustomer{},
		workspaceByCustomer:  map[string]string{},
		entitlements:         map[string]Entitlement{},
		processedEvents:      map[string]string{},
		subscriptions:        map[string]SubscriptionEventData{},
	}
}

func (s *fakeBillingStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return &fakeBillingTx{store: s}, nil
}

func (s *fakeBillingStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	return s.queryRow(sql, args...)
}

func (s *fakeBillingStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return s.exec(sql, args...)
}

func (s *fakeBillingStore) queryRow(sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "from workspaces w"):
		workspace := s.workspaces[args[0].(string)]
		user := s.users[workspace.createdByUserID]
		return fakeRow{values: []any{workspace.id, workspace.name, user.id, user.email, user.name, workspace.stripeCustomerID}}
	case strings.Contains(sql, "from billing_customers") && strings.Contains(sql, "where workspace_id = $1"):
		customer := s.customersByWorkspace[args[0].(string)]
		if customer.customerID == "" {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []any{customer.customerID}}
	case strings.Contains(sql, "from billing_entitlements"):
		entitlement, ok := s.entitlements[args[0].(string)]
		if !ok {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []any{
			entitlement.WorkspaceID,
			entitlement.Plan,
			entitlement.Status,
			entitlement.SubscriptionLive,
			entitlement.CustomDomainsEnabled,
			entitlement.ActiveSiteLimit,
			entitlement.MonthlyPromptLimit,
			entitlement.AssetStorageLimitBytes,
			entitlement.UpdatedAt,
		}}
	case strings.Contains(sql, "select exists(select 1 from billing_events"):
		_, ok := s.processedEvents[args[0].(string)]
		return fakeRow{values: []any{ok}}
	case strings.Contains(sql, "from billing_customers") && strings.Contains(sql, "where stripe_customer_id = $1"):
		workspaceID := s.workspaceByCustomer[args[0].(string)]
		if workspaceID == "" {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []any{workspaceID}}
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

func (s *fakeBillingStore) exec(sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "insert into billing_customers"):
		workspaceID := args[0].(string)
		customerID := args[1].(string)
		emailAddress := args[2].(string)
		s.customersByWorkspace[workspaceID] = fakeCustomer{customerID: customerID, email: emailAddress}
		s.workspaceByCustomer[customerID] = workspaceID
	case strings.Contains(sql, "update workspaces") && strings.Contains(sql, "stripe_customer_id = $2"):
		workspace := s.workspaces[args[0].(string)]
		workspace.stripeCustomerID = args[1].(string)
		s.workspaces[workspace.id] = workspace
	case strings.Contains(sql, "insert into billing_events"):
		s.processedEvents[args[0].(string)] = args[1].(string)
	case strings.Contains(sql, "insert into billing_subscriptions"):
		subscription := SubscriptionEventData{
			WorkspaceID:       args[0].(string),
			CustomerID:        args[1].(string),
			SubscriptionID:    args[2].(string),
			Plan:              args[3].(string),
			Status:            args[4].(string),
			CancelAtPeriodEnd: args[9].(bool),
		}
		s.subscriptions[subscription.SubscriptionID] = subscription
	case strings.Contains(sql, "insert into billing_entitlements"):
		workspaceID := args[0].(string)
		siteLimit := args[5].(int)
		promptLimit := args[6].(int)
		assetBytes := args[7].(int64)
		now := time.Now().UTC()
		s.entitlements[workspaceID] = Entitlement{
			WorkspaceID:            workspaceID,
			Plan:                   args[1].(string),
			Status:                 args[2].(string),
			SubscriptionLive:       args[3].(bool),
			CustomDomainsEnabled:   args[4].(bool),
			ActiveSiteLimit:        &siteLimit,
			MonthlyPromptLimit:     &promptLimit,
			AssetStorageLimitBytes: &assetBytes,
			UpdatedAt:              now,
		}
	case strings.Contains(sql, "update workspaces") && strings.Contains(sql, "set plan = $2"):
		workspace := s.workspaces[args[0].(string)]
		workspace.plan = args[1].(string)
		workspace.stripeCustomerID = args[2].(string)
		s.workspaces[workspace.id] = workspace
	case strings.Contains(sql, "insert into users"):
		emailAddress := strings.ToLower(strings.TrimSpace(args[0].(string)))
		for _, user := range s.users {
			if user.email == emailAddress {
				return pgconn.CommandTag{}, nil
			}
		}
		s.users["user-claimed"] = fakeUser{id: "user-claimed", email: emailAddress}
	case strings.Contains(sql, "insert into workspace_members"),
		strings.Contains(sql, "update guest_sessions"),
		strings.Contains(sql, "update workspaces") && strings.Contains(sql, "created_by = coalesce"):
	default:
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, nil
}

type fakeBillingTx struct {
	store *fakeBillingStore
}

func (tx *fakeBillingTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}
func (tx *fakeBillingTx) Commit(context.Context) error   { return nil }
func (tx *fakeBillingTx) Rollback(context.Context) error { return nil }
func (tx *fakeBillingTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (tx *fakeBillingTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (tx *fakeBillingTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (tx *fakeBillingTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (tx *fakeBillingTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.store.exec(sql, args...)
}
func (tx *fakeBillingTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (tx *fakeBillingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.store.queryRow(sql, args...)
}
func (tx *fakeBillingTx) Conn() *pgx.Conn { return nil }

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			switch value := r.values[i].(type) {
			case string:
				*target = value
			case *string:
				if value != nil {
					*target = *value
				}
			}
		case *bool:
			*target = r.values[i].(bool)
		case *time.Time:
			*target = r.values[i].(time.Time)
		case **int:
			if r.values[i] == nil {
				*target = nil
			} else {
				value := r.values[i].(*int)
				*target = value
			}
		case **int64:
			if r.values[i] == nil {
				*target = nil
			} else {
				value := r.values[i].(*int64)
				*target = value
			}
		case *int:
			*target = r.values[i].(int)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func authSession(workspaceID string) auth.Session {
	return auth.Session{WorkspaceID: workspaceID}
}
