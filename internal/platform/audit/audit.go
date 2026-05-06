package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/timestamps"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidEvent = errors.New("audit event is invalid")
	ErrUnavailable  = errors.New("audit store is not configured")
)

type Store interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Event struct {
	WorkspaceID string
	SiteID      string
	UserID      string
	Action      string
	Metadata    any
}

type Recorder struct {
	store Store
	clock timestamps.Clock
}

func NewRecorder(store Store) *Recorder {
	return &Recorder{
		store: store,
		clock: timestamps.SystemClock{},
	}
}

func (r *Recorder) WithClock(clock timestamps.Clock) *Recorder {
	if clock == nil {
		return r
	}
	return &Recorder{
		store: r.store,
		clock: clock,
	}
}

func (r *Recorder) Record(ctx context.Context, event Event) error {
	if r == nil || r.store == nil {
		return ErrUnavailable
	}

	action := strings.TrimSpace(event.Action)
	if action == "" {
		return fmt.Errorf("%w: action is required", ErrInvalidEvent)
	}
	if err := validateOptionalID("workspace id", event.WorkspaceID); err != nil {
		return err
	}
	if err := validateOptionalID("site id", event.SiteID); err != nil {
		return err
	}
	if err := validateOptionalID("user id", event.UserID); err != nil {
		return err
	}

	metadata, err := marshalMetadata(event.Metadata)
	if err != nil {
		return err
	}

	_, err = r.store.Exec(ctx, `
		insert into audit_events (workspace_id, site_id, user_id, action, metadata, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`, nullString(event.WorkspaceID), nullString(event.SiteID), nullString(event.UserID), action, metadata, r.clock.Now())
	if err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

func marshalMetadata(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}

	metadata, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: metadata must be JSON serializable", ErrInvalidEvent)
	}
	if string(metadata) == "null" {
		return []byte("{}"), nil
	}
	return metadata, nil
}

func validateOptionalID(label string, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if !ids.IsValid(value) {
		return fmt.Errorf("%w: %s must be a valid UUID", ErrInvalidEvent, label)
	}
	return nil
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
