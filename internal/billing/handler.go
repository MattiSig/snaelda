package billing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/email"
	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/jackc/pgx/v5"
)

type HandlerConfig struct {
	AppBaseURL             string
	StripeSecretKey        string
	StripeWebhookSecret    string
	BasicPriceID           string
	ProPriceID             string
	OnceOverPriceID        string
	BillingSuccessURL      string
	BillingCancelURL       string
	BillingPortalReturnURL string
	ProductName            string
	EmailSender            email.Sender
	AuditRecorder          *audit.Recorder
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
			AppBaseURL:         cfg.AppBaseURL,
			BasicPriceID:       cfg.BasicPriceID,
			ProPriceID:         cfg.ProPriceID,
			OnceOverPriceID:    cfg.OnceOverPriceID,
			ProductName:        cfg.ProductName,
			EmailSender:        cfg.EmailSender,
			AuditRecorder:      cfg.AuditRecorder,
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
	mux.Handle("PUT /api/billing/once-over", requireSession(http.HandlerFunc(h.updateOnceOver)))
	mux.Handle("GET /api/billing/once-over/pending", requireSession(http.HandlerFunc(h.listPendingOnceOvers)))
	mux.Handle("POST /api/billing/once-over/{requestId}/deliver", requireSession(http.HandlerFunc(h.deliverOnceOver)))
	mux.Handle("POST /api/billing/portal", requireSession(http.HandlerFunc(h.portal)))
	mux.HandleFunc("POST /api/billing/webhook", h.webhook)
}

type checkoutRequest struct {
	Plan         string `json:"plan"`
	PurchaseType string `json:"purchaseType"`
}

type onceOverUpdateRequest struct {
	IntakeBusiness string `json:"intakeBusiness"`
	IntakeVisitor  string `json:"intakeVisitor"`
	IntakeOutcome  string `json:"intakeOutcome"`
	IntakeStuckOn  string `json:"intakeStuckOn"`
	ReadyForReview bool   `json:"readyForReview"`
}

type onceOverDeliveryRequest struct {
	VideoURL          string   `json:"videoUrl"`
	DeliveryNextSteps []string `json:"deliveryNextSteps"`
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
		Session:      session,
		Plan:         payload.Plan,
		PurchaseType: payload.PurchaseType,
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

func (h *Handler) updateOnceOver(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		writeBillingError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}

	var payload onceOverUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeBillingError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	state, err := h.service.UpdateOnceOver(r.Context(), UpdateOnceOverInput{
		WorkspaceID:    session.WorkspaceID,
		IntakeBusiness: payload.IntakeBusiness,
		IntakeVisitor:  payload.IntakeVisitor,
		IntakeOutcome:  payload.IntakeOutcome,
		IntakeStuckOn:  payload.IntakeStuckOn,
		ReadyForReview: payload.ReadyForReview,
	})
	if err != nil {
		status := http.StatusBadRequest
		code := "once_over_update_failed"
		switch {
		case errors.Is(err, ErrOnceOverUnavailable):
			status = http.StatusConflict
			code = "once_over_unavailable"
		}
		writeBillingError(w, status, code, err.Error())
		return
	}

	writeBillingJSON(w, http.StatusOK, map[string]OnceOverState{"onceOver": state})
}

func (h *Handler) listPendingOnceOvers(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok || !session.IsOperator {
		writeBillingError(w, http.StatusForbidden, "forbidden", "operator access is required")
		return
	}

	requests, err := h.service.ListPendingOnceOvers(r.Context())
	if err != nil {
		writeBillingError(w, http.StatusInternalServerError, "once_over_pending_failed", err.Error())
		return
	}
	writeBillingJSON(w, http.StatusOK, map[string]any{"requests": requests})
}

func (h *Handler) deliverOnceOver(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok || !session.IsOperator || session.User == nil {
		writeBillingError(w, http.StatusForbidden, "forbidden", "operator access is required")
		return
	}

	requestID := r.PathValue("requestId")
	if requestID == "" {
		writeBillingError(w, http.StatusBadRequest, "invalid_request", "request id is required")
		return
	}

	var payload onceOverDeliveryRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeBillingError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	state, err := h.service.DeliverOnceOver(r.Context(), DeliverOnceOverInput{
		RequestID:         requestID,
		VideoURL:          payload.VideoURL,
		DeliveryNextSteps: payload.DeliveryNextSteps,
		DeliveredByUserID: session.User.ID,
	})
	if err != nil {
		status := http.StatusBadRequest
		code := "once_over_delivery_failed"
		switch {
		case errors.Is(err, ErrOnceOverRequestNotFound):
			status = http.StatusNotFound
			code = "once_over_request_not_found"
		case errors.Is(err, ErrOnceOverNotReady), errors.Is(err, ErrOnceOverDeliveryMismatch):
			status = http.StatusConflict
			code = "once_over_delivery_conflict"
		}
		writeBillingError(w, status, code, err.Error())
		return
	}

	writeBillingJSON(w, http.StatusOK, map[string]OnceOverState{"onceOver": state})
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
