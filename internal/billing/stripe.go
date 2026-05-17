package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/webhook"
)

type CheckoutSessionRequest struct {
	WorkspaceID   string
	WorkspaceName string
	Plan          string
	PurchaseType  string
	Mode          string
	PriceID       string
	CustomerID    string
	CustomerEmail string
	SuccessURL    string
	CancelURL     string
}

type CheckoutSessionResult struct {
	URL        string
	CustomerID string
}

type PortalSessionRequest struct {
	CustomerID string
	ReturnURL  string
}

type PortalSessionResult struct {
	URL string
}

type WebhookEvent struct {
	ID              string
	Type            string
	CheckoutSession CheckoutCompletedData
	Subscription    SubscriptionEventData
	Invoice         InvoiceEventData
}

type CheckoutCompletedData struct {
	SessionID       string
	WorkspaceID     string
	CustomerID      string
	CustomerEmail   string
	Plan            string
	PurchaseType    string
	Mode            string
	PaymentIntentID string
	CompletedAt     time.Time
}

const (
	checkoutModeSubscription = "subscription"
	checkoutModePayment      = "payment"
)

type SubscriptionEventData struct {
	WorkspaceID        string
	SubscriptionID     string
	CustomerID         string
	Status             string
	Plan               string
	PriceID            string
	ProductID          string
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	CancelAtPeriodEnd  bool
	CanceledAt         *time.Time
}

type InvoiceEventData struct {
	CustomerID       string
	CustomerEmail    string
	AmountPaid       int64
	Currency         string
	HostedInvoiceURL string
	Plan             string
}

type stripeSDK interface {
	V1CheckoutSessions_Create(ctx context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error)
	V1BillingPortalSessions_Create(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error)
}

type stripeWrapper struct {
	client sdkBridge
	secret string
}

type sdkBridge interface {
	CreateCheckoutSession(ctx context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error)
	CreatePortalSession(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error)
}

type stripeClientBridge struct {
	client *stripe.Client
}

func (b stripeClientBridge) CreateCheckoutSession(ctx context.Context, params *stripe.CheckoutSessionCreateParams) (*stripe.CheckoutSession, error) {
	return b.client.V1CheckoutSessions.Create(ctx, params)
}

func (b stripeClientBridge) CreatePortalSession(ctx context.Context, params *stripe.BillingPortalSessionCreateParams) (*stripe.BillingPortalSession, error) {
	return b.client.V1BillingPortalSessions.Create(ctx, params)
}

func NewStripeClient(secret string, webhookSecret string) stripeClient {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	return &stripeWrapper{
		client: stripeClientBridge{client: stripe.NewClient(secret)},
		secret: strings.TrimSpace(webhookSecret),
	}
}

func (s *stripeWrapper) CreateCheckoutSession(ctx context.Context, req CheckoutSessionRequest) (CheckoutSessionResult, error) {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = checkoutModeSubscription
	}
	params := &stripe.CheckoutSessionCreateParams{
		Mode:              stripe.String(mode),
		SuccessURL:        stripe.String(req.SuccessURL),
		CancelURL:         stripe.String(req.CancelURL),
		ClientReferenceID: stripe.String(req.WorkspaceID),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(req.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"workspace_id":  req.WorkspaceID,
			"plan":          normalizePlan(req.Plan),
			"purchase_type": normalizePurchaseType(req.PurchaseType),
		},
	}
	if strings.TrimSpace(req.CustomerID) != "" {
		params.Customer = stripe.String(req.CustomerID)
	}
	if strings.TrimSpace(req.CustomerEmail) != "" {
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}

	session, err := s.client.CreateCheckoutSession(ctx, params)
	if err != nil {
		return CheckoutSessionResult{}, err
	}

	customerID := ""
	if session.Customer != nil {
		customerID = session.Customer.ID
	}
	return CheckoutSessionResult{
		URL:        session.URL,
		CustomerID: customerID,
	}, nil
}

func (s *stripeWrapper) CreatePortalSession(ctx context.Context, req PortalSessionRequest) (PortalSessionResult, error) {
	params := &stripe.BillingPortalSessionCreateParams{
		Customer:  stripe.String(req.CustomerID),
		ReturnURL: stripe.String(req.ReturnURL),
	}
	session, err := s.client.CreatePortalSession(ctx, params)
	if err != nil {
		return PortalSessionResult{}, err
	}
	return PortalSessionResult{URL: session.URL}, nil
}

func (s *stripeWrapper) ConstructWebhookEvent(payload []byte, header string) (WebhookEvent, error) {
	if s.secret == "" {
		return WebhookEvent{}, fmt.Errorf("stripe webhook secret is not configured")
	}
	event, err := webhook.ConstructEvent(payload, header, s.secret)
	if err != nil {
		return WebhookEvent{}, err
	}

	result := WebhookEvent{
		ID:   event.ID,
		Type: string(event.Type),
	}

	switch event.Type {
	case "checkout.session.completed":
		var data struct {
			ID                string `json:"id"`
			ClientReferenceID string `json:"client_reference_id"`
			Customer          string `json:"customer"`
			PaymentIntent     string `json:"payment_intent"`
			Mode              string `json:"mode"`
			Created           int64  `json:"created"`
			CustomerDetails   struct {
				Email string `json:"email"`
			} `json:"customer_details"`
			CustomerEmail string            `json:"customer_email"`
			Metadata      map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(event.Data.Raw, &data); err != nil {
			return WebhookEvent{}, err
		}
		result.CheckoutSession = CheckoutCompletedData{
			SessionID:       strings.TrimSpace(data.ID),
			WorkspaceID:     firstNonEmpty(data.Metadata["workspace_id"], data.ClientReferenceID),
			CustomerID:      strings.TrimSpace(data.Customer),
			CustomerEmail:   firstNonEmpty(data.CustomerDetails.Email, data.CustomerEmail),
			Plan:            strings.TrimSpace(data.Metadata["plan"]),
			PurchaseType:    normalizePurchaseType(data.Metadata["purchase_type"]),
			Mode:            strings.TrimSpace(data.Mode),
			PaymentIntentID: strings.TrimSpace(data.PaymentIntent),
		}
		if data.Created > 0 {
			result.CheckoutSession.CompletedAt = time.Unix(data.Created, 0).UTC()
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var data struct {
			ID                 string            `json:"id"`
			Customer           string            `json:"customer"`
			Status             string            `json:"status"`
			CancelAtPeriodEnd  bool              `json:"cancel_at_period_end"`
			CanceledAt         int64             `json:"canceled_at"`
			CurrentPeriodStart int64             `json:"current_period_start"`
			CurrentPeriodEnd   int64             `json:"current_period_end"`
			Metadata           map[string]string `json:"metadata"`
			Items              struct {
				Data []struct {
					Price struct {
						ID      string `json:"id"`
						Product string `json:"product"`
					} `json:"price"`
				} `json:"data"`
			} `json:"items"`
		}
		if err := json.Unmarshal(event.Data.Raw, &data); err != nil {
			return WebhookEvent{}, err
		}
		result.Subscription = SubscriptionEventData{
			WorkspaceID:       strings.TrimSpace(data.Metadata["workspace_id"]),
			SubscriptionID:    data.ID,
			CustomerID:        data.Customer,
			Status:            data.Status,
			Plan:              strings.TrimSpace(data.Metadata["plan"]),
			CancelAtPeriodEnd: data.CancelAtPeriodEnd,
		}
		if len(data.Items.Data) > 0 {
			result.Subscription.PriceID = data.Items.Data[0].Price.ID
			result.Subscription.ProductID = data.Items.Data[0].Price.Product
		}
		if data.CurrentPeriodStart > 0 {
			at := time.Unix(data.CurrentPeriodStart, 0).UTC()
			result.Subscription.CurrentPeriodStart = &at
		}
		if data.CurrentPeriodEnd > 0 {
			at := time.Unix(data.CurrentPeriodEnd, 0).UTC()
			result.Subscription.CurrentPeriodEnd = &at
		}
		if data.CanceledAt > 0 {
			at := time.Unix(data.CanceledAt, 0).UTC()
			result.Subscription.CanceledAt = &at
		}
	case "invoice.paid", "invoice.payment_failed":
		var data struct {
			Customer         string `json:"customer"`
			CustomerEmail    string `json:"customer_email"`
			AmountPaid       int64  `json:"amount_paid"`
			Currency         string `json:"currency"`
			HostedInvoiceURL string `json:"hosted_invoice_url"`
			Lines            struct {
				Data []struct {
					Metadata map[string]string `json:"metadata"`
					Plan     struct {
						ID string `json:"id"`
					} `json:"plan"`
				} `json:"data"`
			} `json:"lines"`
		}
		if err := json.Unmarshal(event.Data.Raw, &data); err != nil {
			return WebhookEvent{}, err
		}
		result.Invoice = InvoiceEventData{
			CustomerID:       data.Customer,
			CustomerEmail:    data.CustomerEmail,
			AmountPaid:       data.AmountPaid,
			Currency:         data.Currency,
			HostedInvoiceURL: data.HostedInvoiceURL,
		}
		if len(data.Lines.Data) > 0 {
			result.Invoice.Plan = strings.TrimSpace(data.Lines.Data[0].Metadata["plan"])
			if result.Invoice.Plan == "" {
				result.Invoice.Plan = data.Lines.Data[0].Plan.ID
			}
		}
	}

	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
