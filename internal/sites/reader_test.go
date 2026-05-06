package sites

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAssembleDraftFromNormalizedRows(t *testing.T) {
	rows := NormalizedDraftRows{
		Site: siteRow{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: themeRow{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#f8f7f4",
					"foreground": "#1d2520",
					"primary":    "#315c4f",
					"accent":     "#c2774b",
				},
				Typography: map[string]any{
					"heading": "Inter",
					"body":    "Inter",
				},
				Layout: map[string]any{
					"maxWidth": "1120px",
				},
				Shape: map[string]any{
					"radius": "8px",
				},
			},
		},
		Pages: []pageRow{
			{
				ID:    "page_contact",
				Title: "Contact",
				Slug:  "/contact",
				Sort:  1,
				SEO: siteconfig.SEOConfig{
					Title:       "Contact Nordic Studio",
					Description: "Start a focused site project.",
				},
				Settings: map[string]any{},
			},
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				Sort:  0,
				SEO: siteconfig.SEOConfig{
					Title:       "Nordic Studio",
					Description: "Calm design systems for focused teams.",
				},
				Settings: map[string]any{},
			},
		},
		Blocks: []blockRow{
			{
				ID:      "block_text",
				PageID:  "page_home",
				Type:    "text_section",
				Version: siteconfig.BlockVersionV1,
				Sort:    1,
				Props: map[string]any{
					"heading":   "A structured seed draft",
					"body":      "Stored as validated application data.",
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				ID:      "block_hero",
				PageID:  "page_home",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Sort:    0,
				Props: map[string]any{
					"eyebrow":     "Nordic Studio",
					"headline":    "Clear websites for focused teams",
					"subheadline": "Structured sites from maintained blocks.",
					"primaryCta": map[string]any{
						"label": "Start a project",
						"href":  "/contact",
					},
					"layout": "centered",
				},
			},
			{
				ID:      "block_cta",
				PageID:  "page_contact",
				Type:    "cta_band",
				Version: siteconfig.BlockVersionV1,
				Sort:    0,
				Props: map[string]any{
					"heading": "Ready to begin?",
					"body":    "Send a concise note.",
					"variant": "primary",
				},
				Settings: siteconfig.BlockSettings{AnchorID: "contact"},
				Hidden:   true,
			},
		},
	}

	draft := AssembleDraft(rows)
	if err := siteconfig.ValidateDraft(draft); err != nil {
		t.Fatalf("validate assembled draft: %v", err)
	}
	if draft.Pages[0].ID != "page_home" {
		t.Fatalf("expected home page first, got %q", draft.Pages[0].ID)
	}
	if draft.Pages[0].Blocks[0].ID != "block_hero" {
		t.Fatalf("expected hero block first, got %q", draft.Pages[0].Blocks[0].ID)
	}
	if draft.Navigation.Primary[1].PageID != "page_contact" {
		t.Fatalf("expected navigation to follow page order, got %#v", draft.Navigation.Primary)
	}
	if !draft.Pages[1].Blocks[0].Settings.Hidden {
		t.Fatal("expected hidden block setting to be preserved from normalized row")
	}
	if draft.Pages[1].Blocks[0].Settings.AnchorID != "contact" {
		t.Fatalf("expected anchor setting to be preserved, got %q", draft.Pages[1].Blocks[0].Settings.AnchorID)
	}
}

func TestNormalizeDraftForPersistence(t *testing.T) {
	draft := validPersistenceDraft()

	rows, err := normalizeDraft(draft)
	if err != nil {
		t.Fatalf("normalize draft: %v", err)
	}

	if rows.Site.DefaultLocale != "en" {
		t.Fatalf("expected default locale fallback, got %q", rows.Site.DefaultLocale)
	}
	if len(rows.Pages) != 1 || rows.Pages[0].Sort != 0 {
		t.Fatalf("expected one normalized page with sort order 0, got %#v", rows.Pages)
	}
	if len(rows.Blocks) != 2 {
		t.Fatalf("expected two normalized blocks, got %d", len(rows.Blocks))
	}
	if rows.Blocks[1].Sort != 1 {
		t.Fatalf("expected block sort order from draft order, got %d", rows.Blocks[1].Sort)
	}
	if !rows.Blocks[1].Hidden {
		t.Fatal("expected hidden block to persist through is_hidden")
	}
	if rows.Blocks[1].Settings.Hidden {
		t.Fatal("expected hidden flag to be split out of block settings JSON")
	}
	if rows.Blocks[1].Settings.AnchorID != "details" {
		t.Fatalf("expected anchor setting to persist, got %q", rows.Blocks[1].Settings.AnchorID)
	}
}

func TestSaveDraftPersistsCanonicalDraftInTransaction(t *testing.T) {
	tx := &fakeDraftTx{}
	db := &fakeDraftDB{tx: tx}
	writer := NewPostgresWriter(db)

	err := writer.SaveDraft(context.Background(), "00000000-0000-4000-8000-000000000101", validPersistenceDraft())
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}

	if db.beginCount != 1 {
		t.Fatalf("expected one transaction, got %d", db.beginCount)
	}
	if !tx.committed {
		t.Fatal("expected transaction to commit")
	}
	if len(tx.execs) != 7 {
		t.Fatalf("expected site, theme, delete blocks, delete pages, page, and two block writes; got %d", len(tx.execs))
	}
	if !strings.Contains(tx.execs[0].sql, "insert into sites") {
		t.Fatalf("expected first write to save site, got %s", tx.execs[0].sql)
	}
	if !strings.Contains(tx.execs[3].sql, "delete from pages") {
		t.Fatalf("expected removed pages cleanup, got %s", tx.execs[3].sql)
	}
	pageIDs, ok := tx.execs[3].args[1].([]string)
	if !ok || len(pageIDs) != 1 || pageIDs[0] != "00000000-0000-4000-8000-000000000501" {
		t.Fatalf("expected canonical page IDs in cleanup, got %#v", tx.execs[3].args[1])
	}
	if hidden, ok := tx.execs[6].args[8].(bool); !ok || !hidden {
		t.Fatalf("expected hidden block flag in is_hidden argument, got %#v", tx.execs[6].args[8])
	}
}

func TestSaveDraftRejectsMoreThanTenPagesBeforeWriting(t *testing.T) {
	tx := &fakeDraftTx{}
	db := &fakeDraftDB{tx: tx}
	writer := NewPostgresWriter(db)
	draft := validPersistenceDraft()
	draft.Pages = make([]siteconfig.PageDraft, 0, siteconfig.MaxPagesPerSite+1)
	for i := 0; i < siteconfig.MaxPagesPerSite+1; i++ {
		page := validPersistenceDraft().Pages[0]
		page.ID = "page_" + strconv.Itoa(i)
		page.Slug = "/page-" + strconv.Itoa(i)
		draft.Pages = append(draft.Pages, page)
	}
	draft.Navigation.Primary = []siteconfig.NavigationItem{{Label: "Page 0", PageID: "page_0"}}

	err := writer.SaveDraft(context.Background(), "00000000-0000-4000-8000-000000000101", draft)
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) || !validationErr.Has("too_many_pages") {
		t.Fatalf("expected too_many_pages validation error, got %v", err)
	}
	if db.beginCount != 0 {
		t.Fatalf("expected validation to stop before opening a transaction, got %d transactions", db.beginCount)
	}
	if len(tx.execs) != 0 {
		t.Fatalf("expected no writes for invalid draft, got %d", len(tx.execs))
	}
}

func validPersistenceDraft() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:     "00000000-0000-4000-8000-000000000201",
			Name:   "Nordic Studio",
			Slug:   "nordic-studio",
			Status: "draft",
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#f8f7f4",
					"foreground": "#1d2520",
					"primary":    "#315c4f",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "00000000-0000-4000-8000-000000000501"}},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "00000000-0000-4000-8000-000000000501",
				Title: "Home",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "00000000-0000-4000-8000-000000000601",
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"headline": "Clear websites for focused teams",
							"layout":   "centered",
						},
					},
					{
						ID:      "00000000-0000-4000-8000-000000000602",
						Type:    "text_section",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading":   "A structured seed draft",
							"body":      "Stored as validated application data.",
							"alignment": "left",
							"width":     "default",
						},
						Settings: siteconfig.BlockSettings{
							Hidden:   true,
							AnchorID: "details",
						},
					},
				},
			},
		},
	}
}

type fakeDraftDB struct {
	tx         *fakeDraftTx
	beginCount int
}

func (db *fakeDraftDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	db.beginCount++
	return db.tx, nil
}

type fakeExec struct {
	sql  string
	args []any
}

type fakeDraftTx struct {
	execs      []fakeExec
	committed  bool
	rolledBack bool
}

func (tx *fakeDraftTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not implemented in fakeDraftTx")
}

func (tx *fakeDraftTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *fakeDraftTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *fakeDraftTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("copy is not implemented in fakeDraftTx")
}

func (tx *fakeDraftTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeDraftTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeDraftTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("prepare is not implemented in fakeDraftTx")
}

func (tx *fakeDraftTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.execs = append(tx.execs, fakeExec{sql: sql, args: arguments})
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (tx *fakeDraftTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakeDraftTx")
}

func (tx *fakeDraftTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (tx *fakeDraftTx) Conn() *pgx.Conn {
	return nil
}
