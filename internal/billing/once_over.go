package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/email"
	"github.com/jackc/pgx/v5"
)

const (
	onceOverStatusNone           = "none"
	onceOverStatusAwaitingIntake = "awaiting_intake"
	onceOverStatusPending        = "pending"
	onceOverStatusDelivered      = "delivered"
	onceOverPurchaseType         = "once_over"
	subscriptionPurchaseType     = "subscription"
	maxOnceOverIntakeFieldLength = 500
	maxOnceOverVideoURLLength    = 2048
)

var ErrOnceOverUnavailable = errors.New("once-over is not available for this workspace")

type OnceOverState struct {
	Status  string           `json:"status"`
	Request *OnceOverRequest `json:"request,omitempty"`
}

type OnceOverRequest struct {
	ID                string     `json:"id"`
	PaidAt            time.Time  `json:"paidAt"`
	IntakeBusiness    string     `json:"intakeBusiness,omitempty"`
	IntakeVisitor     string     `json:"intakeVisitor,omitempty"`
	IntakeOutcome     string     `json:"intakeOutcome,omitempty"`
	IntakeStuckOn     string     `json:"intakeStuckOn,omitempty"`
	IntakeSubmittedAt *time.Time `json:"intakeSubmittedAt,omitempty"`
	VideoURL          string     `json:"videoUrl,omitempty"`
	DeliveryNextSteps []string   `json:"deliveryNextSteps,omitempty"`
	DeliveredAt       *time.Time `json:"deliveredAt,omitempty"`
}

type UpdateOnceOverInput struct {
	WorkspaceID    string
	IntakeBusiness string
	IntakeVisitor  string
	IntakeOutcome  string
	IntakeStuckOn  string
	ReadyForReview bool
}

func (s *Service) GetOnceOverState(ctx context.Context, workspaceID string) (OnceOverState, error) {
	return loadOnceOverState(ctx, s.store, workspaceID)
}

func (s *Service) UpdateOnceOver(ctx context.Context, input UpdateOnceOverInput) (OnceOverState, error) {
	if s == nil || s.store == nil {
		return OnceOverState{}, fmt.Errorf("billing is not configured")
	}

	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		return OnceOverState{}, ErrOnceOverUnavailable
	}

	intakeBusiness, err := sanitizeOnceOverField(input.IntakeBusiness)
	if err != nil {
		return OnceOverState{}, err
	}
	intakeVisitor, err := sanitizeOnceOverField(input.IntakeVisitor)
	if err != nil {
		return OnceOverState{}, err
	}
	intakeOutcome, err := sanitizeOnceOverField(input.IntakeOutcome)
	if err != nil {
		return OnceOverState{}, err
	}
	intakeStuckOn, err := sanitizeOnceOverField(input.IntakeStuckOn)
	if err != nil {
		return OnceOverState{}, err
	}

	if input.ReadyForReview {
		switch {
		case intakeBusiness == "":
			return OnceOverState{}, fmt.Errorf("business summary is required before marking the once-over ready")
		case intakeVisitor == "":
			return OnceOverState{}, fmt.Errorf("visitor summary is required before marking the once-over ready")
		case intakeOutcome == "":
			return OnceOverState{}, fmt.Errorf("primary outcome is required before marking the once-over ready")
		}
	}

	tx, err := beginTx(ctx, s.store)
	if err != nil {
		return OnceOverState{}, err
	}
	defer tx.Rollback(ctx)

	state, err := loadOnceOverState(ctx, tx, workspaceID)
	if err != nil {
		return OnceOverState{}, err
	}
	if state.Status == onceOverStatusNone || state.Request == nil {
		return OnceOverState{}, ErrOnceOverUnavailable
	}
	if state.Status == onceOverStatusDelivered {
		return OnceOverState{}, fmt.Errorf("this once-over has already been delivered")
	}

	var requestID string
	if err := tx.QueryRow(ctx, `
		select id::text
		from once_over_requests
		where workspace_id = $1
		order by created_at desc
		limit 1
	`, workspaceID).Scan(&requestID); err != nil {
		return OnceOverState{}, err
	}

	nextStatus := state.Status
	submittedAtArg := any(nil)
	if input.ReadyForReview {
		nextStatus = onceOverStatusPending
		now := time.Now().UTC()
		submittedAtArg = &now
	}

	if _, err := tx.Exec(ctx, `
		update once_over_requests
		set intake_business = $2,
		    intake_visitor = $3,
		    intake_outcome = $4,
		    intake_stuck_on = $5,
		    intake_submitted_at = coalesce(intake_submitted_at, $6)
		where id = $1::uuid
	`, requestID, intakeBusiness, intakeVisitor, intakeOutcome, intakeStuckOn, submittedAtArg); err != nil {
		return OnceOverState{}, err
	}

	if _, err := tx.Exec(ctx, `
		update workspaces
		set once_over_status = $2,
		    updated_at = now()
		where id = $1
	`, workspaceID, nextStatus); err != nil {
		return OnceOverState{}, err
	}

	nextState, err := loadOnceOverState(ctx, tx, workspaceID)
	if err != nil {
		return OnceOverState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OnceOverState{}, err
	}
	return nextState, nil
}

func (s *Service) handleOnceOverCheckoutCompleted(ctx context.Context, tx pgx.Tx, event WebhookEvent) error {
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

	paymentID := strings.TrimSpace(event.CheckoutSession.PaymentIntentID)
	if paymentID == "" {
		paymentID = strings.TrimSpace(event.CheckoutSession.SessionID)
	}
	if paymentID == "" {
		return nil
	}

	paidAt := event.CheckoutSession.CompletedAt
	if paidAt.IsZero() {
		paidAt = time.Now().UTC()
	}
	if _, err := tx.Exec(ctx, `
		insert into once_over_requests (
			workspace_id,
			stripe_payment_id,
			stripe_checkout_session_id,
			paid_at
		)
		values ($1, $2, nullif($3, ''), $4)
		on conflict (stripe_payment_id) do nothing
	`, workspaceID, paymentID, strings.TrimSpace(event.CheckoutSession.SessionID), paidAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update workspaces
		set once_over_status = $2,
		    updated_at = now()
		where id = $1
	`, workspaceID, onceOverStatusAwaitingIntake); err != nil {
		return err
	}

	if s.emailSender.Mailer == nil {
		return nil
	}
	contact, err := lookupBillingContactTx(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	if contact.UserEmail == "" {
		contact.UserEmail = strings.TrimSpace(event.CheckoutSession.CustomerEmail)
	}
	if contact.UserEmail == "" {
		return nil
	}

	_, err = s.emailSender.SendOnceOverIntakeReady(ctx, email.Address{
		Email: contact.UserEmail,
		Name:  contact.UserName,
	}, contact.Locale, email.OnceOverIntakeReadyTemplateData{
		ProductName:   s.productName,
		WorkspaceName: contact.WorkspaceName,
		IntakeURL:     buildOnceOverIntakeURL(s.appBaseURL),
	}, "once_over_intake_ready:"+strings.TrimSpace(event.ID))
	return err
}

func loadOnceOverState(ctx context.Context, store interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, workspaceID string) (OnceOverState, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return OnceOverState{}, ErrOnceOverUnavailable
	}

	var state OnceOverState
	if err := store.QueryRow(ctx, `
		select coalesce(once_over_status, 'none')
		from workspaces
		where id = $1
	`, workspaceID).Scan(&state.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OnceOverState{Status: onceOverStatusNone}, nil
		}
		return OnceOverState{}, err
	}

	var request OnceOverRequest
	var intakeSubmittedAt *time.Time
	var deliveredAt *time.Time
	var videoURL *string
	var nextStepsJSON string
	err := store.QueryRow(ctx, `
		select id::text,
		       paid_at,
		       coalesce(intake_business, ''),
		       coalesce(intake_visitor, ''),
		       coalesce(intake_outcome, ''),
		       coalesce(intake_stuck_on, ''),
		       intake_submitted_at,
		       video_url,
		       coalesce(delivery_next_steps, '[]'::jsonb)::text,
		       delivered_at
		from once_over_requests
		where workspace_id = $1
		order by created_at desc
		limit 1
	`, workspaceID).Scan(
		&request.ID,
		&request.PaidAt,
		&request.IntakeBusiness,
		&request.IntakeVisitor,
		&request.IntakeOutcome,
		&request.IntakeStuckOn,
		&intakeSubmittedAt,
		&videoURL,
		&nextStepsJSON,
		&deliveredAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if state.Status == "" {
				state.Status = onceOverStatusNone
			}
			return state, nil
		}
		return OnceOverState{}, err
	}

	request.IntakeSubmittedAt = intakeSubmittedAt
	request.DeliveredAt = deliveredAt
	if videoURL != nil {
		request.VideoURL = strings.TrimSpace(*videoURL)
	}
	if err := json.Unmarshal([]byte(nextStepsJSON), &request.DeliveryNextSteps); err != nil {
		return OnceOverState{}, fmt.Errorf("decode once-over delivery steps: %w", err)
	}
	state.Request = &request
	if state.Status == "" {
		state.Status = onceOverStatusNone
	}
	return state, nil
}

func sanitizeOnceOverField(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maxOnceOverIntakeFieldLength {
		return "", fmt.Errorf("once-over intake fields must be %d characters or fewer", maxOnceOverIntakeFieldLength)
	}
	return value, nil
}

func sanitizeOnceOverVideoURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > maxOnceOverVideoURLLength {
		return "", fmt.Errorf("once-over delivery URLs must be %d characters or fewer", maxOnceOverVideoURLLength)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("once-over delivery URL must be a valid absolute URL")
	}
	return parsed.String(), nil
}

func buildOnceOverIntakeURL(appBaseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if base == "" {
		return "/app/billing"
	}
	return base + "/app/billing"
}
