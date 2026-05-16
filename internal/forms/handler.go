package forms

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/email"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type FormService interface {
	Submit(ctx context.Context, input SubmitInput) (SubmitResult, error)
	ListBySite(ctx context.Context, siteID string) ([]Submission, error)
	UpdateStatus(ctx context.Context, submissionID string, input UpdateSubmissionInput) (Submission, error)
}

type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
	RequireFormSubmission(ctx context.Context, submissionID string, allowedRoles ...string) (authorization.Scope, error)
}

type submissionRateLimiter interface {
	Allow(ctx context.Context, siteID string, blockID string, clientIPHash string) bool
}

type Handler struct {
	service    FormService
	authorizer Authorizer
	limiter    submissionRateLimiter
}

type HandlerConfig struct {
	EmailSender      email.Sender
	EmailRateLimiter *email.RateLimiter
	Logger           *slog.Logger
	ProductName      string
}

type submitFormRequest struct {
	Payload map[string]any `json:"payload"`
}

type updateSubmissionRequest struct {
	Status *string `json:"status,omitempty"`
}

func NewHandler(db DB) *Handler {
	return NewHandlerWithConfig(db, HandlerConfig{})
}

func NewHandlerWithConfig(db DB, cfg HandlerConfig) *Handler {
	return &Handler{
		service: NewServiceWithConfig(db, ServiceConfig{
			EmailSender:      cfg.EmailSender,
			EmailRateLimiter: cfg.EmailRateLimiter,
			Logger:           cfg.Logger,
			ProductName:      cfg.ProductName,
		}),
		authorizer: authorization.New(db),
		limiter:    NewDurableSubmissionRateLimiter(db, 5, 10*time.Minute, nil),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.HandleFunc("POST /api/public/forms/{siteId}/{blockId}/submit", h.submit)
	mux.Handle("GET /api/sites/{siteId}/form-submissions", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("PATCH /api/form-submissions/{submissionId}", requireUser(http.HandlerFunc(h.update)))
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	blockID := strings.TrimSpace(r.PathValue("blockId"))
	if siteID == "" || blockID == "" {
		writeError(w, http.StatusBadRequest, "invalid_form_target", "site id and block id are required")
		return
	}

	clientIPHash := HashClientIP(clientIPFromRequest(r))
	if h.limiter != nil {
		if !h.limiter.Allow(r.Context(), siteID, blockID, clientIPHash) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "please wait before submitting this form again")
			return
		}
	}

	var payload submitFormRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	result, err := h.service.Submit(r.Context(), SubmitInput{
		SiteID:       siteID,
		BlockID:      blockID,
		Payload:      payload.Payload,
		ClientIPHash: clientIPHash,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": result.SuccessMessage,
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	submissions, err := h.service.ListBySite(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_form_submissions_failed", "could not load form submissions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"submissions": submissions,
	})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	submissionID := strings.TrimSpace(r.PathValue("submissionId"))
	if _, err := h.authorizer.RequireFormSubmission(r.Context(), submissionID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload updateSubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	submission, err := h.service.UpdateStatus(r.Context(), submissionID, UpdateSubmissionInput{
		Status: payload.Status,
	})
	if err != nil {
		writeFormError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"submission": submission,
	})
}

func writeFormError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSiteRequired):
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
	case errors.Is(err, ErrBlockRequired):
		writeError(w, http.StatusBadRequest, "invalid_block_id", "block id is required")
	case errors.Is(err, ErrSiteNotFound):
		writeError(w, http.StatusNotFound, "site_not_found", "site was not found")
	case errors.Is(err, ErrSiteNotPublished):
		writeError(w, http.StatusNotFound, "site_not_published", "site has not been published")
	case errors.Is(err, ErrFormBlockNotFound):
		writeError(w, http.StatusNotFound, "form_not_found", "contact form was not found")
	case errors.Is(err, ErrFormBlockInvalid):
		writeError(w, http.StatusBadRequest, "invalid_form_block", "block is not a contact form")
	case errors.Is(err, ErrSubmissionNotFound):
		writeError(w, http.StatusNotFound, "submission_not_found", "form submission was not found")
	case errors.Is(err, ErrNoSubmissionChanges):
		writeError(w, http.StatusBadRequest, "no_submission_changes", "submission update requires a status change")
	case errors.Is(err, ErrSubmissionStatusInvalid):
		writeError(w, http.StatusBadRequest, "invalid_submission_status", "submission status is not supported")
	default:
		var validationErr siteconfig.ValidationError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"code":    "validation_failed",
					"message": "form submission is invalid",
				},
				"issues": validationErr.Issues,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "form_processing_failed", "could not process form submission")
	}
}

func writeAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authorization.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
	case errors.Is(err, authorization.ErrInvalidResourceID):
		writeError(w, http.StatusBadRequest, "invalid_resource", "resource id is required")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access is not allowed")
	case errors.Is(err, authorization.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "authorization_unavailable", "authorization is not configured")
	default:
		writeError(w, http.StatusInternalServerError, "authorization_failed", "authorization failed")
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}

func clientIPFromRequest(r *http.Request) string {
	for _, value := range []string{
		r.Header.Get("X-Forwarded-For"),
		r.Header.Get("X-Real-IP"),
		r.RemoteAddr,
	} {
		if value == "" {
			continue
		}
		candidate := strings.TrimSpace(strings.Split(value, ",")[0])
		host, _, err := net.SplitHostPort(candidate)
		if err == nil && host != "" {
			return host
		}
		return candidate
	}
	return "unknown"
}
