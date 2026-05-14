package sites

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakePreviewTokenDB struct {
	queryRow func(context.Context, string, ...any) pgx.Row
	exec     func(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (db fakePreviewTokenDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (db fakePreviewTokenDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRow(ctx, sql, args...)
}

func (db fakePreviewTokenDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return db.exec(ctx, sql, args...)
}

func (db fakePreviewTokenDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, nil
}

type previewTokenRow struct {
	values []string
	err    error
}

func (r previewTokenRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		target := dest[index].(*string)
		*target = value
	}
	return nil
}

func TestIssuePreviewTokenRevokesExistingAndStoresHash(t *testing.T) {
	var execCalls []struct {
		sql  string
		args []any
	}
	db := fakePreviewTokenDB{
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return previewTokenRow{err: pgx.ErrNoRows}
		},
		exec: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execCalls = append(execCalls, struct {
				sql  string
				args []any
			}{sql: sql, args: args})
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	service := NewPostgresPreviewTokenService(db, 48*time.Hour)
	service.now = func() time.Time {
		return time.Date(2026, time.May, 14, 10, 0, 0, 0, time.UTC)
	}
	service.newToken = func() (string, error) {
		return "preview-secret", nil
	}

	token, err := service.Issue(context.Background(), "site-1", "user-1")
	if err != nil {
		t.Fatalf("issue preview token: %v", err)
	}

	if token.Token != "preview-secret" {
		t.Fatalf("expected preview token value, got %q", token.Token)
	}
	if want := service.now().Add(48 * time.Hour); !token.ExpiresAt.Equal(want) {
		t.Fatalf("expected expiry %s, got %s", want, token.ExpiresAt)
	}
	if len(execCalls) != 2 {
		t.Fatalf("expected two exec calls, got %d", len(execCalls))
	}
	if !strings.Contains(execCalls[0].sql, "update site_preview_tokens") {
		t.Fatalf("expected first call to revoke old tokens, got %q", execCalls[0].sql)
	}
	if !strings.Contains(execCalls[1].sql, "insert into site_preview_tokens") {
		t.Fatalf("expected second call to insert token, got %q", execCalls[1].sql)
	}
	if got := execCalls[1].args[2].(string); got != previewTokenHash("preview-secret") {
		t.Fatalf("expected hashed preview token, got %q", got)
	}
}

func TestLoadDraftByPreviewTokenReturnsDraftAndTouchesUsage(t *testing.T) {
	var touchedHash string
	db := fakePreviewTokenDB{
		queryRow: func(_ context.Context, sql string, args ...any) pgx.Row {
			if strings.Contains(sql, "from site_preview_tokens") {
				return previewTokenRow{values: []string{"site-1"}}
			}
			return previewTokenRow{err: pgx.ErrNoRows}
		},
		exec: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "set last_used_at = now()") {
				touchedHash = args[0].(string)
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	wantDraft := validHandlerDraft()
	service := NewPostgresPreviewTokenService(db, DefaultPreviewTokenTTL)
	service.reader = fakeReader{draft: wantDraft}

	draft, err := service.LoadDraft(context.Background(), "preview-secret")
	if err != nil {
		t.Fatalf("load preview draft: %v", err)
	}

	if draft.Site.ID != wantDraft.Site.ID {
		t.Fatalf("expected draft %q, got %q", wantDraft.Site.ID, draft.Site.ID)
	}
	if touchedHash != previewTokenHash("preview-secret") {
		t.Fatalf("expected touched hash %q, got %q", previewTokenHash("preview-secret"), touchedHash)
	}
}

func TestLoadDraftByPreviewTokenRejectsUnknownToken(t *testing.T) {
	db := fakePreviewTokenDB{
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return previewTokenRow{err: pgx.ErrNoRows}
		},
		exec: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	service := NewPostgresPreviewTokenService(db, DefaultPreviewTokenTTL)

	_, err := service.LoadDraft(context.Background(), "missing-token")
	if err == nil {
		t.Fatal("expected missing preview token error")
	}
	if err != ErrPreviewTokenNotFound {
		t.Fatalf("expected ErrPreviewTokenNotFound, got %v", err)
	}
}

func TestRevokePreviewTokenRejectsMissingSite(t *testing.T) {
	service := NewPostgresPreviewTokenService(fakePreviewTokenDB{
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return previewTokenRow{err: pgx.ErrNoRows}
		},
		exec: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}, DefaultPreviewTokenTTL)

	if err := service.Revoke(context.Background(), " "); err != ErrPreviewTokenInvalid {
		t.Fatalf("expected ErrPreviewTokenInvalid, got %v", err)
	}
}

func TestPreviewTokenHashIsStable(t *testing.T) {
	if previewTokenHash("abc") != previewTokenHash("abc") {
		t.Fatal("expected stable preview token hash")
	}
	if previewTokenHash("abc") == previewTokenHash("def") {
		t.Fatal("expected distinct preview token hashes")
	}
}
