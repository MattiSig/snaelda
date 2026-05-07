package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBuildPublishedSnapshotAddsSEOFallbacks(t *testing.T) {
	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            "site_demo",
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
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block_hero",
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"headline":    "Clear websites for focused teams",
							"subheadline": "Structured sites from maintained blocks.",
							"layout":      "centered",
						},
					},
				},
			},
			{
				ID:    "page_contact",
				Title: "Contact",
				Slug:  "/contact",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block_text",
						Type:    "text_section",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading": "Get in touch",
							"body":    "Send a note to plan your next launch.",
						},
					},
				},
			},
		},
	}

	snapshot := buildPublishedSnapshot(draft)
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		t.Fatalf("validate snapshot: %v", err)
	}
	if snapshot.Site.SEO.Title != "Nordic Studio" {
		t.Fatalf("expected site title fallback, got %q", snapshot.Site.SEO.Title)
	}
	if snapshot.Site.SEO.Description != "Structured sites from maintained blocks." {
		t.Fatalf("expected site description fallback, got %q", snapshot.Site.SEO.Description)
	}
	if snapshot.Pages[1].SEO.Title != "Contact | Nordic Studio" {
		t.Fatalf("expected page title fallback, got %q", snapshot.Pages[1].SEO.Title)
	}
	if snapshot.Pages[1].SEO.Description != "Send a note to plan your next launch." {
		t.Fatalf("expected page description fallback, got %q", snapshot.Pages[1].SEO.Description)
	}
}

func TestRollbackSetsLiveVersionAndRecordsAuditEvent(t *testing.T) {
	store := newFakePublishingStore()
	service := Service{
		db:     store,
		reader: fakePublishingReader{},
	}

	result, err := service.Rollback(
		context.Background(),
		"00000000-0000-4000-8000-000000000201",
		"00000000-0000-4000-8000-000000000701",
		"00000000-0000-4000-8000-000000000001",
	)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if result.Version.VersionNumber != 1 || !result.Version.IsCurrent {
		t.Fatalf("expected rolled back current version, got %#v", result.Version)
	}
	if result.SiteSlug != "nordic-studio" {
		t.Fatalf("expected site slug, got %q", result.SiteSlug)
	}
	if store.tx.liveVersionID != "00000000-0000-4000-8000-000000000701" {
		t.Fatalf("expected live version pointer to update, got %q", store.tx.liveVersionID)
	}
	if !store.tx.committed {
		t.Fatal("expected rollback transaction to commit")
	}
	if len(store.tx.auditEvents) != 1 {
		t.Fatalf("expected one audit event, got %#v", store.tx.auditEvents)
	}
	if store.tx.auditEvents[0].Action != "site.rollback" {
		t.Fatalf("expected rollback audit action, got %#v", store.tx.auditEvents[0])
	}
	if store.tx.auditEvents[0].Metadata["versionNumber"] != float64(1) {
		t.Fatalf("expected rollback metadata to include version number, got %#v", store.tx.auditEvents[0].Metadata)
	}
}

func TestRollbackRejectsUnknownVersion(t *testing.T) {
	store := newFakePublishingStore()
	service := Service{
		db:     store,
		reader: fakePublishingReader{},
	}

	_, err := service.Rollback(
		context.Background(),
		"00000000-0000-4000-8000-000000000201",
		"00000000-0000-4000-8000-000000000799",
		"00000000-0000-4000-8000-000000000001",
	)
	if !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("expected version not found error, got %v", err)
	}
}

type fakePublishingReader struct{}

func (fakePublishingReader) ListSites(context.Context, string) ([]sites.Summary, error) {
	return nil, errors.New("not implemented")
}

func (fakePublishingReader) LoadDraft(context.Context, string) (siteconfig.SiteDraft, error) {
	return siteconfig.SiteDraft{}, errors.New("not implemented")
}

type fakePublishingStore struct {
	tx *fakePublishingTx
}

func newFakePublishingStore() *fakePublishingStore {
	return &fakePublishingStore{
		tx: &fakePublishingTx{
			siteID:      "00000000-0000-4000-8000-000000000201",
			workspaceID: "00000000-0000-4000-8000-000000000101",
			siteSlug:    "nordic-studio",
			hostname:    "nordic-studio.localhost",
			liveVersion: "00000000-0000-4000-8000-000000000702",
			versions: map[string]VersionSummary{
				"00000000-0000-4000-8000-000000000701": {
					ID:            "00000000-0000-4000-8000-000000000701",
					SiteID:        "00000000-0000-4000-8000-000000000201",
					VersionNumber: 1,
					CreatedAt:     time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
					PublishNote:   "Initial launch",
				},
				"00000000-0000-4000-8000-000000000702": {
					ID:            "00000000-0000-4000-8000-000000000702",
					SiteID:        "00000000-0000-4000-8000-000000000201",
					VersionNumber: 2,
					CreatedAt:     time.Date(2026, 5, 7, 8, 0, 0, 0, time.UTC),
					PublishNote:   "Refined hero copy",
				},
			},
		},
	}
}

func (s *fakePublishingStore) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakePublishingStore")
}

func (s *fakePublishingStore) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakePublishingRow{err: errors.New("query row is not implemented in fakePublishingStore")}
}

func (s *fakePublishingStore) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("exec is not implemented in fakePublishingStore")
}

func (s *fakePublishingStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return s.tx, nil
}

type recordedAuditEvent struct {
	Action   string
	Metadata map[string]any
}

type fakePublishingTx struct {
	siteID        string
	workspaceID   string
	siteSlug      string
	hostname      string
	liveVersion   string
	liveVersionID string
	versions      map[string]VersionSummary
	auditEvents   []recordedAuditEvent
	committed     bool
	rolledBack    bool
}

func (tx *fakePublishingTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *fakePublishingTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *fakePublishingTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("copy is not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakePublishingTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakePublishingTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("prepare is not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "update sites") && strings.Contains(sql, "published_version_id"):
		tx.liveVersion = arguments[1].(string)
		tx.liveVersionID = arguments[1].(string)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "insert into audit_events"):
		var metadata map[string]any
		if err := json.Unmarshal(arguments[4].([]byte), &metadata); err != nil {
			return pgconn.CommandTag{}, err
		}
		tx.auditEvents = append(tx.auditEvents, recordedAuditEvent{
			Action:   arguments[3].(string),
			Metadata: metadata,
		})
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
}

func (tx *fakePublishingTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) QueryRow(_ context.Context, sql string, arguments ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "select workspace_id::text, slug"):
		return fakePublishingRow{values: []any{tx.workspaceID, tx.siteSlug}}
	case strings.Contains(sql, "from site_versions sv") && strings.Contains(sql, "where sv.site_id = $1"):
		versionID := arguments[1].(string)
		version, ok := tx.versions[versionID]
		if !ok {
			return fakePublishingRow{err: pgx.ErrNoRows}
		}
		return fakePublishingRow{values: []any{
			version.ID,
			version.SiteID,
			version.VersionNumber,
			version.CreatedAt,
			version.PublishNote,
			tx.hostname,
		}}
	default:
		return fakePublishingRow{err: pgx.ErrNoRows}
	}
}

func (tx *fakePublishingTx) Conn() *pgx.Conn {
	return nil
}

type fakePublishingRow struct {
	values []any
	err    error
}

func (r fakePublishingRow) Scan(dest ...any) error {
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
