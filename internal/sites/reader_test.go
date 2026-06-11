package sites

import (
	"context"
	"encoding/json"
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
			Revision:      7,
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
	if draft.Revision != 7 {
		t.Fatalf("expected draft revision 7, got %d", draft.Revision)
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

func TestAssembleDraftPreservesExplicitStoredNavigation(t *testing.T) {
	rows := NormalizedDraftRows{
		Site: siteRow{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
			Navigation: siteconfig.NavigationConfig{
				Primary: []siteconfig.NavigationItem{
					{Label: "Start here", PageID: "page_home"},
					{Label: "Book", Href: "https://example.com/book"},
				},
			},
			HasNavigation: true,
		},
		Theme: themeRow{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#f8f7f4",
					"foreground": "#1d2520",
					"primary":    "#315c4f",
				},
			},
		},
		Pages: []pageRow{
			{ID: "page_contact", Title: "Contact", Slug: "/contact", Sort: 1},
			{ID: "page_home", Title: "Home", Slug: "/", Sort: 0},
		},
	}

	draft := AssembleDraft(rows)
	if got := draft.Navigation.Primary[0].Label; got != "Start here" {
		t.Fatalf("expected explicit navigation label to survive assembly, got %q", got)
	}
	if got := draft.Navigation.Primary[1].Href; got != "https://example.com/book" {
		t.Fatalf("expected explicit external navigation link to survive assembly, got %q", got)
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
	if rows.Site.Revision != 0 {
		t.Fatalf("expected zero draft revision for a new draft, got %d", rows.Site.Revision)
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
	if !rows.Site.HasNavigation {
		t.Fatal("expected explicit navigation to be marked for persistence")
	}
	if len(rows.Site.Navigation.Primary) != 1 || rows.Site.Navigation.Primary[0].PageID != "00000000-0000-4000-8000-000000000501" {
		t.Fatalf("expected navigation to persist, got %#v", rows.Site.Navigation.Primary)
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
	if len(tx.execs) != 9 {
		t.Fatalf("expected site, theme, delete entries, delete collections, delete blocks, delete pages, page, and two block writes; got %d", len(tx.execs))
	}
	if !strings.Contains(tx.execs[0].sql, "insert into sites") {
		t.Fatalf("expected first write to save site, got %s", tx.execs[0].sql)
	}
	if !strings.Contains(tx.execs[2].sql, "delete from collection_entries") {
		t.Fatalf("expected entries cleanup before pages, got %s", tx.execs[2].sql)
	}
	if !strings.Contains(tx.execs[3].sql, "delete from collections") {
		t.Fatalf("expected collections cleanup before pages, got %s", tx.execs[3].sql)
	}
	if !strings.Contains(tx.execs[5].sql, "update pages") || !strings.Contains(tx.execs[5].sql, "set in_draft = false") {
		t.Fatalf("expected removed pages to be archived, got %s", tx.execs[5].sql)
	}
	pageIDs, ok := tx.execs[5].args[1].([]string)
	if !ok || len(pageIDs) != 1 || pageIDs[0] != "00000000-0000-4000-8000-000000000501" {
		t.Fatalf("expected canonical page IDs in archive pass, got %#v", tx.execs[5].args[1])
	}
	if !strings.Contains(tx.execs[6].sql, "in_draft") {
		t.Fatalf("expected active page upsert to restore in_draft, got %s", tx.execs[6].sql)
	}
	if hidden, ok := tx.execs[8].args[8].(bool); !ok || !hidden {
		t.Fatalf("expected hidden block flag in is_hidden argument, got %#v", tx.execs[8].args[8])
	}
	siteSettingsJSON, ok := tx.execs[0].args[7].([]byte)
	if !ok {
		t.Fatalf("expected site settings JSON bytes, got %#v", tx.execs[0].args[7])
	}
	var siteSettings struct {
		Navigation siteconfig.NavigationConfig `json:"navigation"`
	}
	if err := json.Unmarshal(siteSettingsJSON, &siteSettings); err != nil {
		t.Fatalf("decode saved site settings: %v", err)
	}
	if len(siteSettings.Navigation.Primary) != 1 || siteSettings.Navigation.Primary[0].PageID != "00000000-0000-4000-8000-000000000501" {
		t.Fatalf("expected saved site settings to include navigation, got %#v", siteSettings.Navigation.Primary)
	}
	if revision, ok := tx.execs[0].args[5].(int64); !ok || revision != 1 {
		t.Fatalf("expected initial persisted draft revision 1, got %#v", tx.execs[0].args[5])
	}
}

func TestLoadPagesReadsOnlyDraftRows(t *testing.T) {
	db := &capturingReaderDB{}
	reader := NewPostgresReader(db)

	pages, err := reader.loadPages(context.Background(), "site-demo")
	if err != nil {
		t.Fatalf("load pages: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("expected no pages from empty fake rows, got %#v", pages)
	}
	if len(db.queries) != 1 {
		t.Fatalf("expected one query, got %d", len(db.queries))
	}
	if !strings.Contains(db.queries[0], "in_draft = true") {
		t.Fatalf("expected page load query to filter archived rows, got %s", db.queries[0])
	}
}

func TestListSitesCountsOnlyDraftPages(t *testing.T) {
	db := &capturingReaderDB{}
	reader := NewPostgresReader(db)

	sites, err := reader.ListSites(context.Background(), "workspace-demo")
	if err != nil {
		t.Fatalf("list sites: %v", err)
	}
	if len(sites) != 0 {
		t.Fatalf("expected no sites from empty fake rows, got %#v", sites)
	}
	if len(db.queries) != 1 {
		t.Fatalf("expected one query, got %d", len(db.queries))
	}
	if !strings.Contains(db.queries[0], "p.in_draft = true") {
		t.Fatalf("expected site list query to count only draft pages, got %s", db.queries[0])
	}
}

func TestSaveDraftRejectsStaleRevision(t *testing.T) {
	tx := &fakeDraftTx{
		execResults: []pgconn.CommandTag{
			pgconn.NewCommandTag("INSERT 0 0"),
		},
		queryRows: []pgx.Row{
			fakeDraftRow{boolValue: true},
		},
	}
	db := &fakeDraftDB{tx: tx}
	writer := NewPostgresWriter(db)
	draft := validPersistenceDraft()
	draft.Revision = 3

	err := writer.SaveDraft(context.Background(), "00000000-0000-4000-8000-000000000101", draft)
	if !errors.Is(err, ErrDraftConflict) {
		t.Fatalf("expected draft conflict, got %v", err)
	}
	if tx.committed {
		t.Fatal("expected conflicting transaction not to commit")
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

func TestSaveDraftRejectsAssetFromDifferentSite(t *testing.T) {
	tx := &fakeDraftTx{}
	db := &fakeDraftDB{
		tx:       tx,
		queryRow: fakeDraftRow{json: []byte(`[{"id":"asset-1","siteId":"site-2"}]`)},
	}
	writer := NewPostgresWriter(db)
	draft := validPersistenceDraft()
	draft.Pages[0].Blocks[0].Props["image"] = map[string]any{
		"assetId": "asset-1",
		"alt":     "Hero",
	}

	err := writer.SaveDraft(context.Background(), "00000000-0000-4000-8000-000000000101", draft)
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !validationErr.Has("invalid_asset_reference") {
		t.Fatalf("expected invalid asset reference issue, got %#v", validationErr.Issues)
	}
	if db.beginCount != 0 {
		t.Fatalf("expected asset validation before transaction, got %d begins", db.beginCount)
	}
}

func TestSaveDraftRejectsBrandLogoFromDifferentSite(t *testing.T) {
	tx := &fakeDraftTx{}
	db := &fakeDraftDB{
		tx:       tx,
		queryRow: fakeDraftRow{json: []byte(`[{"id":"asset-logo","siteId":"site-2"}]`)},
	}
	writer := NewPostgresWriter(db)
	draft := validPersistenceDraft()
	draft.Brand = siteconfig.BrandConfig{
		BusinessName: "Nordic Studio",
		PrimaryColor: "#315c4f",
		Logo: &siteconfig.BrandLogo{
			AssetID: "asset-logo",
			Alt:     "Nordic Studio logo",
		},
	}

	err := writer.SaveDraft(context.Background(), "00000000-0000-4000-8000-000000000101", draft)
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !validationErr.Has("invalid_asset_reference") {
		t.Fatalf("expected invalid asset reference issue, got %#v", validationErr.Issues)
	}
	if db.beginCount != 0 {
		t.Fatalf("expected asset validation before transaction, got %d begins", db.beginCount)
	}
}

func TestSaveDraftRejectsCollectionEntryAssetFromDifferentSite(t *testing.T) {
	tx := &fakeDraftTx{}
	db := &fakeDraftDB{
		tx:       tx,
		queryRow: fakeDraftRow{json: []byte(`[{"id":"asset-entry","siteId":"site-2"}]`)},
	}
	writer := NewPostgresWriter(db)
	draft := validPersistenceDraft()
	draft.Collections = []siteconfig.Collection{{
		ID:            "collection-1",
		Slug:          "services",
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []siteconfig.FieldDefinition{
			{Key: "image", Label: "Image", Type: siteconfig.FieldTypeAsset},
		},
		Entries: []siteconfig.CollectionEntry{{
			ID:        "entry-1",
			Slug:      "scaffolding",
			Status:    siteconfig.EntryStatusDraft,
			SortOrder: 0,
			Fields: map[string]any{
				"image": map[string]any{"assetId": "asset-entry", "alt": "Scaffolding"},
			},
		}},
	}}

	err := writer.SaveDraft(context.Background(), "00000000-0000-4000-8000-000000000101", draft)
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !validationErr.Has("invalid_asset_reference") {
		t.Fatalf("expected invalid asset reference issue, got %#v", validationErr.Issues)
	}
	if db.beginCount != 0 {
		t.Fatalf("expected asset validation before transaction, got %d begins", db.beginCount)
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
	queryRow   pgx.Row
}

func (db *fakeDraftDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	db.beginCount++
	return db.tx, nil
}

func (db *fakeDraftDB) QueryRow(context.Context, string, ...any) pgx.Row {
	if db.queryRow != nil {
		return db.queryRow
	}
	return fakeDraftRow{json: []byte(`[]`)}
}

type fakeExec struct {
	sql  string
	args []any
}

type fakeDraftTx struct {
	execs       []fakeExec
	execResults []pgconn.CommandTag
	queryRows   []pgx.Row
	committed   bool
	rolledBack  bool
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
	if len(tx.execResults) > 0 {
		tag := tx.execResults[0]
		tx.execResults = tx.execResults[1:]
		return tag, nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (tx *fakeDraftTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakeDraftTx")
}

func (tx *fakeDraftTx) QueryRow(context.Context, string, ...any) pgx.Row {
	if len(tx.queryRows) == 0 {
		return fakeDraftRow{boolValue: false}
	}
	row := tx.queryRows[0]
	tx.queryRows = tx.queryRows[1:]
	return row
}

func (tx *fakeDraftTx) Conn() *pgx.Conn {
	return nil
}

type fakeDraftRow struct {
	json      []byte
	boolValue bool
	err       error
}

func (r fakeDraftRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	switch target := dest[0].(type) {
	case *[]byte:
		*target = r.json
	case *bool:
		*target = r.boolValue
	default:
		return errors.New("unsupported scan destination")
	}
	return nil
}

type capturingReaderDB struct {
	queries []string
}

func (db *capturingReaderDB) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	db.queries = append(db.queries, sql)
	return &emptyRows{}, nil
}

func (db *capturingReaderDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeDraftRow{err: pgx.ErrNoRows}
}

func (db *capturingReaderDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("exec is not implemented in capturingReaderDB")
}

func (db *capturingReaderDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("transactions are not implemented in capturingReaderDB")
}

type emptyRows struct{}

func (r *emptyRows) Close()                                       {}
func (r *emptyRows) Err() error                                   { return nil }
func (r *emptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *emptyRows) Conn() *pgx.Conn                              { return nil }
func (r *emptyRows) RawValues() [][]byte                          { return nil }
func (r *emptyRows) Values() ([]any, error)                       { return nil, nil }
func (r *emptyRows) Next() bool                                   { return false }
func (r *emptyRows) Scan(...any) error                            { return errors.New("scan should not be called on emptyRows") }
