package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/email"
	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/jackc/pgx/v5"
)

const maxOnceOverDeliverySteps = 5

var (
	ErrOnceOverRequestNotFound  = errors.New("once-over request was not found")
	ErrOnceOverNotReady         = errors.New("once-over request is not ready for delivery")
	ErrOnceOverDeliveryMismatch = errors.New("once-over request was already delivered with different details")
)

type PendingOnceOverRequest struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspaceId"`
	WorkspaceName     string    `json:"workspaceName"`
	OwnerName         string    `json:"ownerName,omitempty"`
	OwnerEmail        string    `json:"ownerEmail,omitempty"`
	PaidAt            time.Time `json:"paidAt"`
	IntakeSubmittedAt time.Time `json:"intakeSubmittedAt"`
	IntakeBusiness    string    `json:"intakeBusiness"`
	IntakeVisitor     string    `json:"intakeVisitor"`
	IntakeOutcome     string    `json:"intakeOutcome"`
	IntakeStuckOn     string    `json:"intakeStuckOn,omitempty"`
}

type DeliverOnceOverInput struct {
	RequestID         string
	VideoURL          string
	DeliveryNextSteps []string
	DeliveredByUserID string
}

func (s *Service) ListPendingOnceOvers(ctx context.Context) ([]PendingOnceOverRequest, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("billing is not configured")
	}

	rows, err := s.store.Query(ctx, `
		select r.id::text,
		       r.workspace_id::text,
		       w.name,
		       coalesce(u.name, ''),
		       coalesce(u.email, bc.email, ''),
		       r.paid_at,
		       r.intake_submitted_at,
		       coalesce(r.intake_business, ''),
		       coalesce(r.intake_visitor, ''),
		       coalesce(r.intake_outcome, ''),
		       coalesce(r.intake_stuck_on, '')
		from once_over_requests r
		join workspaces w on w.id = r.workspace_id
		left join users u on u.id = w.created_by
		left join billing_customers bc on bc.workspace_id = w.id
		where r.intake_submitted_at is not null
		  and r.delivered_at is null
		  and w.once_over_status = $1
		order by r.intake_submitted_at asc, r.created_at asc
	`, onceOverStatusPending)
	if err != nil {
		return nil, fmt.Errorf("list pending once-over requests: %w", err)
	}
	defer rows.Close()

	pending := []PendingOnceOverRequest{}
	for rows.Next() {
		var request PendingOnceOverRequest
		if err := rows.Scan(
			&request.ID,
			&request.WorkspaceID,
			&request.WorkspaceName,
			&request.OwnerName,
			&request.OwnerEmail,
			&request.PaidAt,
			&request.IntakeSubmittedAt,
			&request.IntakeBusiness,
			&request.IntakeVisitor,
			&request.IntakeOutcome,
			&request.IntakeStuckOn,
		); err != nil {
			return nil, fmt.Errorf("scan pending once-over request: %w", err)
		}
		pending = append(pending, request)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending once-over requests: %w", err)
	}
	return pending, nil
}

func (s *Service) DeliverOnceOver(ctx context.Context, input DeliverOnceOverInput) (OnceOverState, error) {
	if s == nil || s.store == nil {
		return OnceOverState{}, fmt.Errorf("billing is not configured")
	}

	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		return OnceOverState{}, ErrOnceOverRequestNotFound
	}
	videoURL, err := sanitizeOnceOverVideoURL(input.VideoURL)
	if err != nil {
		return OnceOverState{}, err
	}
	nextSteps, err := sanitizeOnceOverNextSteps(input.DeliveryNextSteps)
	if err != nil {
		return OnceOverState{}, err
	}

	tx, err := beginTx(ctx, s.store)
	if err != nil {
		return OnceOverState{}, err
	}
	defer tx.Rollback(ctx)

	var (
		workspaceID       string
		status            string
		intakeSubmittedAt *time.Time
		videoURLValue     *string
		nextStepsJSON     string
		deliveredAt       *time.Time
	)
	err = tx.QueryRow(ctx, `
		select r.workspace_id::text,
		       w.once_over_status,
		       r.intake_submitted_at,
		       r.video_url,
		       coalesce(r.delivery_next_steps, '[]'::jsonb)::text,
		       r.delivered_at
		from once_over_requests r
		join workspaces w on w.id = r.workspace_id
		where r.id = $1::uuid
		for update of r, w
	`, requestID).Scan(
		&workspaceID,
		&status,
		&intakeSubmittedAt,
		&videoURLValue,
		&nextStepsJSON,
		&deliveredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OnceOverState{}, ErrOnceOverRequestNotFound
	}
	if err != nil {
		return OnceOverState{}, fmt.Errorf("load once-over delivery request: %w", err)
	}
	if intakeSubmittedAt == nil || (status != onceOverStatusPending && deliveredAt == nil) {
		return OnceOverState{}, ErrOnceOverNotReady
	}

	contact, err := lookupBillingContactTx(ctx, tx, workspaceID)
	if err != nil {
		return OnceOverState{}, fmt.Errorf("load once-over delivery contact: %w", err)
	}

	if deliveredAt != nil {
		currentVideoURL := ""
		if videoURLValue != nil {
			currentVideoURL = strings.TrimSpace(*videoURLValue)
		}
		var currentNextSteps []string
		if err := json.Unmarshal([]byte(nextStepsJSON), &currentNextSteps); err != nil {
			return OnceOverState{}, fmt.Errorf("decode stored once-over delivery steps: %w", err)
		}
		if currentVideoURL != videoURL || !slices.Equal(currentNextSteps, nextSteps) {
			return OnceOverState{}, ErrOnceOverDeliveryMismatch
		}
	} else {
		nextStepsPayload, err := json.Marshal(nextSteps)
		if err != nil {
			return OnceOverState{}, fmt.Errorf("encode once-over delivery steps: %w", err)
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, `
			update once_over_requests
			set video_url = $2,
			    delivery_next_steps = $3::jsonb,
			    delivered_at = $4,
			    updated_at = now()
			where id = $1::uuid
		`, requestID, videoURL, nextStepsPayload, now); err != nil {
			return OnceOverState{}, fmt.Errorf("persist once-over delivery: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			update workspaces
			set once_over_status = $2,
			    updated_at = now()
			where id = $1::uuid
		`, workspaceID, onceOverStatusDelivered); err != nil {
			return OnceOverState{}, fmt.Errorf("mark once-over delivered: %w", err)
		}
		if err := audit.NewRecorder(tx).Record(ctx, audit.Event{
			WorkspaceID: workspaceID,
			UserID:      strings.TrimSpace(input.DeliveredByUserID),
			Action:      "once_over.delivered",
			Metadata: map[string]any{
				"requestId":  requestID,
				"videoUrl":   videoURL,
				"nextSteps":  nextSteps,
				"ownerEmail": contact.UserEmail,
			},
		}); err != nil {
			return OnceOverState{}, err
		}
	}

	state, err := loadOnceOverState(ctx, tx, workspaceID)
	if err != nil {
		return OnceOverState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OnceOverState{}, err
	}

	if contact.UserEmail != "" && s.emailSender.Mailer != nil {
		if _, err := s.emailSender.SendOnceOverDelivered(ctx, email.Address{
			Email: contact.UserEmail,
			Name:  contact.UserName,
		}, contact.Locale, email.OnceOverDeliveredTemplateData{
			ProductName:   s.productName,
			WorkspaceName: contact.WorkspaceName,
			DeliveryURL:   videoURL,
			NextSteps:     nextSteps,
		}, "once_over_delivered:"+requestID); err != nil {
			return OnceOverState{}, err
		}
	}

	return state, nil
}

func sanitizeOnceOverNextSteps(values []string) ([]string, error) {
	nextSteps := make([]string, 0, len(values))
	for _, value := range values {
		trimmed, err := sanitizeOnceOverField(value)
		if err != nil {
			return nil, err
		}
		if trimmed == "" {
			continue
		}
		nextSteps = append(nextSteps, trimmed)
	}
	switch {
	case len(nextSteps) == 0:
		return nil, fmt.Errorf("at least one next step is required before delivering the once-over")
	case len(nextSteps) > maxOnceOverDeliverySteps:
		return nil, fmt.Errorf("once-over delivery supports at most %d next steps", maxOnceOverDeliverySteps)
	default:
		return nextSteps, nil
	}
}
