package audit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	testWorkspaceID = "00000000-0000-4000-8000-000000000101"
	testSiteID      = "00000000-0000-4000-8000-000000000201"
	testUserID      = "00000000-0000-4000-8000-000000000001"
)

type fakeAuditStore struct {
	sql  string
	args []any
	err  error
}

func (s *fakeAuditStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.sql = sql
	s.args = args
	return pgconn.NewCommandTag("INSERT 0 1"), s.err
}

func TestRecordInsertsAuditEvent(t *testing.T) {
	store := &fakeAuditStore{}
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	recorder := NewRecorder(store).WithClock(timestamps.ClockFunc(func() time.Time { return now }))

	err := recorder.Record(context.Background(), Event{
		WorkspaceID: testWorkspaceID,
		SiteID:      testSiteID,
		UserID:      testUserID,
		Action:      "site.create",
		Metadata: map[string]any{
			"slug": "nordic-studio",
		},
	})
	if err != nil {
		t.Fatalf("record audit event: %v", err)
	}

	if !strings.Contains(store.sql, "insert into audit_events") {
		t.Fatalf("expected audit insert, got %q", store.sql)
	}
	if store.args[0] != testWorkspaceID || store.args[1] != testSiteID || store.args[2] != testUserID {
		t.Fatalf("unexpected resource args %#v", store.args[:3])
	}
	if store.args[3] != "site.create" {
		t.Fatalf("unexpected action %q", store.args[3])
	}
	if string(store.args[4].([]byte)) != `{"slug":"nordic-studio"}` {
		t.Fatalf("unexpected metadata %s", store.args[4])
	}
	if !store.args[5].(time.Time).Equal(now) {
		t.Fatalf("unexpected created_at %v", store.args[5])
	}
}

func TestRecordUsesNullsForOptionalIDs(t *testing.T) {
	store := &fakeAuditStore{}
	recorder := NewRecorder(store)

	err := recorder.Record(context.Background(), Event{Action: "auth.login"})
	if err != nil {
		t.Fatalf("record audit event: %v", err)
	}

	if store.args[0] != nil || store.args[1] != nil || store.args[2] != nil {
		t.Fatalf("expected nil optional ids, got %#v", store.args[:3])
	}
	if string(store.args[4].([]byte)) != `{}` {
		t.Fatalf("expected empty metadata object, got %s", store.args[4])
	}
}

func TestRecordValidatesEvent(t *testing.T) {
	recorder := NewRecorder(&fakeAuditStore{})

	tests := []Event{
		{},
		{Action: "site.create", WorkspaceID: "workspace-1"},
		{Action: "site.create", SiteID: "site-1"},
		{Action: "site.create", UserID: "user-1"},
		{Action: "site.create", Metadata: make(chan int)},
	}

	for _, event := range tests {
		if err := recorder.Record(context.Background(), event); !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("expected invalid event error for %#v, got %v", event, err)
		}
	}
}

func TestRecordRequiresStore(t *testing.T) {
	err := NewRecorder(nil).Record(context.Background(), Event{Action: "site.create"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}
