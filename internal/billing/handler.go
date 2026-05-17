package billing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/email"
	"github.com/jackc/pgx/v5"
)

type HandlerConfig struct {
	StripeSecretKey        string
	StripeWebhookSecret    string
	BasicPriceID           string
	ProPriceID             string
	BillingSuccessURL      string
	BillingCancelURL       string
	BillingPortalReturnURL string
	ProductName            string
	EmailSender            email.Sender
}

type Handler struct {
	service *Service
}

func NewHandler(store DB, cfg HandlerConfig) *Handler {
	return &Handler{
		service: NewService(store, ServiceConfig{
			Stripe:             NewStripeClient(cfg.StripeSecretKey, cfg.StripeWebhookSecret),
			SuccessURL:         cfg.BillingSuccessURL,
			CancelURL:          cfg.BillingCancelURL,
			PortalReturnURL:    cfg.BillingPortalReturnURL,
			BasicPriceID:       cfg.BasicPriceID,
			ProPriceID:         cfg.ProPriceID,
			ProductName:        cfg.ProductName,
			EmailSender:        cfg.EmailSender,
			DefaultSiteLimit:   3,
			DefaultPromptLimit: 250,
			DefaultAssetBytes:  5 << 30,
		}),
	}
}

func NewHandlerWithService(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Mount(mux *http.ServeMux, requireSession func(http.Handler) http.Handler) {
	mux.Handle("GET /api/billing/entitlements", requireSession(http.HandlerFunc(h.entitlements)))
	mux.Handle("POST /api/billing/checkout", requireSession(http.HandlerFunc(h.checkout)))
	mux.Handle("POST /api/billing/portal", requireSession(http.HandlerFunc(h.portal)))
	mux.HandleFunc("POST /api/billing/webhook", h.webhook)
}

type checkoutRequest struct {
	Plan string `json:"plan"`
}

func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		writeBillingError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}

	var payload checkoutRequest
	_ = json.NewDecoder(r.Body).Decode(&payload)
	url, err := h.service.CreateCheckoutSession(r.Context(), CheckoutInput{
		Session: session,
		Plan:    payload.Plan,
	})
	if err != nil {
		writeBillingError(w, http.StatusBadRequest, "billing_checkout_failed", err.Error())
		return
	}
	writeBillingJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *Handler) portal(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		writeBillingError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	url, err := h.service.CreatePortalSession(r.Context(), session.WorkspaceID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "billing_portal_failed"
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusConflict
			code = "billing_portal_unavailable"
		}
		writeBillingError(w, status, code, err.Error())
		return
	}
	writeBillingJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *Handler) entitlements(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		writeBillingError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	state, err := LoadWorkspaceState(r.Context(), h.service.store, session.WorkspaceID)
	if err != nil {
		writeBillingError(w, http.StatusInternalServerError, "billing_entitlements_failed", err.Error())
		return
	}
	writeBillingJSON(w, http.StatusOK, state)
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeBillingError(w, http.StatusBadRequest, "invalid_request", "request body is required")
		return
	}
	if err := h.service.HandleWebhook(r.Context(), payload, r.Header.Get("Stripe-Signature")); err != nil {
		writeBillingError(w, http.StatusBadRequest, "billing_webhook_failed", err.Error())
		return
	}
	writeBillingJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeBillingJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeBillingError(w http.ResponseWriter, status int, code string, message string) {
	writeBillingJSON(w, status, map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}
