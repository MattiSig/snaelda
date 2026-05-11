package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLocalArtifactStoreWritesVersionFiles(t *testing.T) {
	rootDir := t.TempDir()
	store := newLocalArtifactStore(rootDir)

	err := store.Save(context.Background(), "site-1", "version-1", ArtifactBundle{
		SchemaVersion: "published_artifacts.v1",
		Files: []ArtifactFile{
			{
				Path:        "pages/index.html",
				ContentType: "text/html; charset=utf-8",
				Body:        "<div>home</div>",
			},
			{
				Path:        "manifest.json",
				ContentType: "application/json; charset=utf-8",
				Body:        "{\"schemaVersion\":\"published_artifacts.v1\"}\n",
			},
		},
	})
	if err != nil {
		t.Fatalf("save artifacts: %v", err)
	}

	pageArtifact, err := os.ReadFile(filepath.Join(rootDir, "site-1", "version-1", "pages", "index.html"))
	if err != nil {
		t.Fatalf("read page artifact: %v", err)
	}
	if string(pageArtifact) != "<div>home</div>" {
		t.Fatalf("expected page artifact body, got %q", string(pageArtifact))
	}
}

func TestLocalArtifactStoreRejectsEscapingPaths(t *testing.T) {
	rootDir := t.TempDir()
	store := newLocalArtifactStore(rootDir)

	err := store.Save(context.Background(), "site-1", "version-1", ArtifactBundle{
		SchemaVersion: "published_artifacts.v1",
		Files: []ArtifactFile{{
			Path:        "../escape.txt",
			ContentType: "text/plain; charset=utf-8",
			Body:        "nope",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "escapes the artifact root") {
		t.Fatalf("expected path escape error, got %v", err)
	}
}

func TestPublishRendersAndStoresArtifactsBeforeCommit(t *testing.T) {
	db := &fakeArtifactPublishDB{
		tx: &fakeArtifactPublishTx{
			siteID:      "00000000-0000-4000-8000-000000000201",
			workspaceID: "00000000-0000-4000-8000-000000000101",
			siteSlug:    "nordic-studio",
			versionID:   "00000000-0000-4000-8000-000000000701",
			createdAt:   time.Date(2026, 5, 11, 9, 30, 0, 0, time.UTC),
			nextVersion: 3,
		},
	}
	renderer := &fakeArtifactRenderer{
		bundle: ArtifactBundle{
			SchemaVersion: "published_artifacts.v1",
			Files: []ArtifactFile{{
				Path:        "pages/index.html",
				ContentType: "text/html; charset=utf-8",
				Body:        "<div>published</div>",
			}},
		},
	}
	store := &fakeArtifactStore{}
	service := Service{
		db:       db,
		reader:   fakeDraftReader{draft: buildArtifactDraft()},
		renderer: renderer,
		store:    store,
	}

	result, err := service.Publish(
		context.Background(),
		"00000000-0000-4000-8000-000000000201",
		"00000000-0000-4000-8000-000000000001",
		PublishInput{PublishNote: "Launch day"},
	)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if result.Version.ID != "00000000-0000-4000-8000-000000000701" {
		t.Fatalf("expected returned version id, got %#v", result.Version)
	}
	if renderer.input.Hostname != "nordic-studio.localhost" {
		t.Fatalf("expected renderer hostname, got %#v", renderer.input)
	}
	if renderer.input.SiteSlug != "nordic-studio" {
		t.Fatalf("expected renderer site slug, got %#v", renderer.input)
	}
	if store.savedSiteID != "00000000-0000-4000-8000-000000000201" || store.savedVersionID != result.Version.ID {
		t.Fatalf("expected store save to receive site/version, got site=%q version=%q", store.savedSiteID, store.savedVersionID)
	}
	if db.tx.liveVersionID != result.Version.ID {
		t.Fatalf("expected live version pointer to update, got %q", db.tx.liveVersionID)
	}
	if !db.tx.committed {
		t.Fatal("expected publish transaction to commit")
	}
}

func TestPublishDoesNotMarkSiteLiveWhenArtifactStorageFails(t *testing.T) {
	db := &fakeArtifactPublishDB{
		tx: &fakeArtifactPublishTx{
			siteID:      "00000000-0000-4000-8000-000000000201",
			workspaceID: "00000000-0000-4000-8000-000000000101",
			siteSlug:    "nordic-studio",
			versionID:   "00000000-0000-4000-8000-000000000701",
			createdAt:   time.Date(2026, 5, 11, 9, 30, 0, 0, time.UTC),
			nextVersion: 3,
		},
	}
	service := Service{
		db:       db,
		reader:   fakeDraftReader{draft: buildArtifactDraft()},
		renderer: &fakeArtifactRenderer{bundle: ArtifactBundle{SchemaVersion: "published_artifacts.v1"}},
		store:    &fakeArtifactStore{err: errors.New("disk full")},
	}

	_, err := service.Publish(
		context.Background(),
		"00000000-0000-4000-8000-000000000201",
		"00000000-0000-4000-8000-000000000001",
		PublishInput{PublishNote: "Launch day"},
	)
	if err == nil || !strings.Contains(err.Error(), "store published artifacts") {
		t.Fatalf("expected artifact storage error, got %v", err)
	}
	if db.tx.liveVersionID != "" {
		t.Fatalf("expected live version pointer to stay empty, got %q", db.tx.liveVersionID)
	}
	if db.tx.committed {
		t.Fatal("expected publish transaction not to commit")
	}
}

type fakeDraftReader struct {
	draft siteconfig.SiteDraft
	err   error
}

func (r fakeDraftReader) ListSites(context.Context, string) ([]sites.Summary, error) {
	return nil, errors.New("not implemented")
}

func (r fakeDraftReader) LoadDraft(context.Context, string) (siteconfig.SiteDraft, error) {
	return r.draft, r.err
}

type fakeArtifactRenderer struct {
	input  ArtifactRenderInput
	bundle ArtifactBundle
	err    error
}

func (r *fakeArtifactRenderer) Render(_ context.Context, input ArtifactRenderInput) (ArtifactBundle, error) {
	r.input = input
	return r.bundle, r.err
}

type fakeArtifactStore struct {
	savedSiteID    string
	savedVersionID string
	savedBundle    ArtifactBundle
	err            error
}

func (s *fakeArtifactStore) Save(_ context.Context, siteID string, versionID string, bundle ArtifactBundle) error {
	s.savedSiteID = siteID
	s.savedVersionID = versionID
	s.savedBundle = bundle
	return s.err
}

type fakeArtifactPublishDB struct {
	tx *fakeArtifactPublishTx
}

func (db *fakeArtifactPublishDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakeArtifactPublishDB")
}

func (db *fakeArtifactPublishDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeArtifactRow{err: errors.New("query row is not implemented in fakeArtifactPublishDB")}
}

func (db *fakeArtifactPublishDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("exec is not implemented in fakeArtifactPublishDB")
}

func (db *fakeArtifactPublishDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return db.tx, nil
}

type fakeArtifactPublishTx struct {
	siteID        string
	workspaceID   string
	siteSlug      string
	versionID     string
	createdAt     time.Time
	nextVersion   int
	hostname      string
	liveVersionID string
	committed     bool
}

func (tx *fakeArtifactPublishTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not implemented in fakeArtifactPublishTx")
}

func (tx *fakeArtifactPublishTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *fakeArtifactPublishTx) Rollback(context.Context) error {
	return nil
}

func (tx *fakeArtifactPublishTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("copy is not implemented in fakeArtifactPublishTx")
}

func (tx *fakeArtifactPublishTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeArtifactPublishTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeArtifactPublishTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("prepare is not implemented in fakeArtifactPublishTx")
}

func (tx *fakeArtifactPublishTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "insert into site_domains"):
		tx.hostname = arguments[1].(string)
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.Contains(sql, "update site_domains"):
		tx.hostname = arguments[len(arguments)-1].(string)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "update sites") && strings.Contains(sql, "published_version_id"):
		tx.liveVersionID = arguments[1].(string)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "insert into audit_events"):
		var metadata map[string]any
		if err := json.Unmarshal(arguments[4].([]byte), &metadata); err != nil {
			return pgconn.CommandTag{}, err
		}
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
}

func (tx *fakeArtifactPublishTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakeArtifactPublishTx")
}

func (tx *fakeArtifactPublishTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "select workspace_id::text, slug"):
		return fakeArtifactRow{values: []any{tx.workspaceID, tx.siteSlug}}
	case strings.Contains(sql, "select coalesce(max(version_number), 0) + 1"):
		return fakeArtifactRow{values: []any{tx.nextVersion}}
	case strings.Contains(sql, "insert into site_versions"):
		return fakeArtifactRow{values: []any{tx.versionID, tx.createdAt}}
	case strings.Contains(sql, "from site_domains") && strings.Contains(sql, "type = 'subdomain'"):
		return fakeArtifactRow{err: pgx.ErrNoRows}
	default:
		return fakeArtifactRow{err: pgx.ErrNoRows}
	}
}

func (tx *fakeArtifactPublishTx) Conn() *pgx.Conn {
	return nil
}

type fakeArtifactRow struct {
	values []any
	err    error
}

func (r fakeArtifactRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *int:
			*target = value.(int)
		case *time.Time:
			*target = value.(time.Time)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func buildArtifactDraft() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            "00000000-0000-4000-8000-000000000201",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#151215",
					"foreground": "#f6f2ec",
					"primary":    "#8fc6ff",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{
				{Label: "Home", PageID: "page_home"},
				{Label: "Contact", PageID: "page_contact"},
			},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{{
					ID:      "block_hero",
					Type:    "hero",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{
						"headline": "Clear websites for focused teams",
						"layout":   "centered",
					},
				}},
			},
			{
				ID:    "page_contact",
				Title: "Contact",
				Slug:  "/contact",
				Blocks: []siteconfig.BlockInstance{{
					ID:      "block_text_contact",
					Type:    "text_section",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{
						"heading": "Say hello",
						"body":    "Send a note with your launch timeline.",
					},
				}},
			},
		},
	}
}
