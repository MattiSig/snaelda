package generation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type recorderStore struct {
	captures        []recorderCaptureCall
	history         recorderHistoryCall
	queryRowErr     error
	historyRowErr   error
	recordHistoryFn func(args []any)
}

type recorderCaptureCall struct {
	siteID      string
	workspaceID string
	scope       string
	pageID      string
	prompt      string
	draftJSON   []byte
	summaryJSON []byte
	createdBy   string
}

type recorderHistoryCall struct {
	siteID             string
	workspaceID        string
	scope              string
	targetID           string
	prompt             string
	previousRevisionID string
	resultRevisionID   string
	jobID              string
	changeSummary      string
	createdBy          string
}

func (s *recorderStore) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

func (s *recorderStore) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (s *recorderStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}

func (s *recorderStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "insert into draft_revisions"):
		if s.queryRowErr != nil {
			return scalarRow{err: s.queryRowErr}
		}
		call := recorderCaptureCall{
			siteID:      args[0].(string),
			workspaceID: args[1].(string),
			scope:       args[2].(string),
			pageID:      args[3].(string),
			prompt:      args[4].(string),
			draftJSON:   args[5].([]byte),
			createdBy:   args[8].(string),
		}
		call.summaryJSON = args[7].([]byte)
		s.captures = append(s.captures, call)
		return scalarRow{value: "rev-" + itoa(len(s.captures))}
	case strings.Contains(sql, "insert into reprompt_history"):
		if s.historyRowErr != nil {
			return scalarRow{err: s.historyRowErr}
		}
		if s.recordHistoryFn != nil {
			s.recordHistoryFn(args)
		}
		s.history = recorderHistoryCall{
			siteID:             args[0].(string),
			workspaceID:        args[1].(string),
			scope:              args[2].(string),
			targetID:           args[3].(string),
			prompt:             args[4].(string),
			previousRevisionID: args[5].(string),
			resultRevisionID:   args[6].(string),
			jobID:              args[7].(string),
			changeSummary:      args[8].(string),
			createdBy:          args[9].(string),
		}
		return scalarRow{value: "history-1"}
	}
	return scalarRow{err: pgx.ErrNoRows}
}

type scalarRow struct {
	value string
	err   error
}

func (r scalarRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 0 {
		return nil
	}
	switch target := dest[0].(type) {
	case *string:
		*target = r.value
	default:
		return errors.New("unsupported scan target")
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	if n < 0 {
		digits = append(digits, '-')
		n = -n
	}
	stack := []byte{}
	for n > 0 {
		stack = append(stack, byte('0'+n%10))
		n /= 10
	}
	for i := len(stack) - 1; i >= 0; i-- {
		digits = append(digits, stack[i])
	}
	return string(digits)
}

func TestPromptHistoryRecorderCapturesPreAndPostRevisions(t *testing.T) {
	store := &recorderStore{}
	recorder := NewPromptHistoryRecorder(store, nil)

	prev := siteconfig.SiteDraft{Site: siteconfig.DraftSite{ID: "site-1", Name: "Before"}}
	next := siteconfig.SiteDraft{Site: siteconfig.DraftSite{ID: "site-1", Name: "After"}}

	result, err := recorder.Record(context.Background(), PromptHistoryInput{
		WorkspaceID:   "workspace-1",
		SiteID:        "site-1",
		UserID:        "user-1",
		JobID:         "job-1",
		Scope:         "collection",
		TargetID:      "collection-1",
		Prompt:        "Make a services list",
		ChangeSummary: "Drafted the Services collection.",
		PreviousDraft: prev,
		NextDraft:     next,
		Summary:       map[string]any{"fieldCount": 3},
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	if len(store.captures) != 2 {
		t.Fatalf("expected two captures, got %d", len(store.captures))
	}
	if store.captures[0].scope != "collection" {
		t.Fatalf("expected scope collection, got %q", store.captures[0].scope)
	}
	if store.captures[0].pageID != "" {
		t.Fatalf("collection-scope captures should not carry page_id, got %q", store.captures[0].pageID)
	}
	var summary map[string]any
	if err := json.Unmarshal(store.captures[0].summaryJSON, &summary); err != nil {
		t.Fatalf("summary json: %v", err)
	}
	if summary["fieldCount"].(float64) != 3 {
		t.Fatalf("expected summary preserved, got %v", summary["fieldCount"])
	}

	var beforeDraft siteconfig.SiteDraft
	if err := json.Unmarshal(store.captures[0].draftJSON, &beforeDraft); err != nil {
		t.Fatalf("decode before: %v", err)
	}
	if beforeDraft.Site.Name != "Before" {
		t.Fatalf("expected before snapshot, got %q", beforeDraft.Site.Name)
	}

	var afterDraft siteconfig.SiteDraft
	if err := json.Unmarshal(store.captures[1].draftJSON, &afterDraft); err != nil {
		t.Fatalf("decode after: %v", err)
	}
	if afterDraft.Site.Name != "After" {
		t.Fatalf("expected after snapshot, got %q", afterDraft.Site.Name)
	}

	if store.history.previousRevisionID != "rev-1" || store.history.resultRevisionID != "rev-2" {
		t.Fatalf("expected history to reference captured revisions, got %+v", store.history)
	}
	if store.history.targetID != "collection-1" {
		t.Fatalf("expected target id collection-1, got %q", store.history.targetID)
	}
	if store.history.jobID != "job-1" {
		t.Fatalf("expected job linkage, got %q", store.history.jobID)
	}
	if store.history.changeSummary != "Drafted the Services collection." {
		t.Fatalf("expected change summary, got %q", store.history.changeSummary)
	}
	if result.PreviousRevisionID != "rev-1" || result.ResultRevisionID != "rev-2" || result.HistoryID == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestPromptHistoryRecorderRejectsMissingIdentifiers(t *testing.T) {
	store := &recorderStore{}
	recorder := NewPromptHistoryRecorder(store, nil)

	if _, err := recorder.Record(context.Background(), PromptHistoryInput{
		WorkspaceID: "",
		SiteID:      "site-1",
		Scope:       "collection",
	}); err == nil {
		t.Fatal("expected error when workspace missing")
	}

	if _, err := recorder.Record(context.Background(), PromptHistoryInput{
		WorkspaceID: "workspace-1",
		SiteID:      "site-1",
		Scope:       "",
	}); err == nil {
		t.Fatal("expected error when scope missing")
	}
}

func TestPromptHistoryRecorderNilRecorderIsNoOp(t *testing.T) {
	var recorder *PromptHistoryRecorder
	if _, err := recorder.Record(context.Background(), PromptHistoryInput{
		WorkspaceID: "workspace-1",
		SiteID:      "site-1",
		Scope:       "collection",
	}); err != nil {
		t.Fatalf("nil recorder should no-op, got %v", err)
	}
}
