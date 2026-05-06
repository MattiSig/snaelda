package sites

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("site not found")

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Reader interface {
	ListSites(ctx context.Context, workspaceID string) ([]Summary, error)
	LoadDraft(ctx context.Context, siteID string) (siteconfig.SiteDraft, error)
}

type Summary struct {
	ID                 string `json:"id"`
	WorkspaceID        string `json:"workspaceId"`
	Name               string `json:"name"`
	Slug               string `json:"slug"`
	Status             string `json:"status"`
	DefaultLocale      string `json:"defaultLocale"`
	PublishedVersionID string `json:"publishedVersionId,omitempty"`
	PageCount          int    `json:"pageCount"`
}

type PostgresReader struct {
	db DB
}

type siteRow struct {
	ID            string
	Name          string
	Slug          string
	Status        string
	DefaultLocale string
	SEO           siteconfig.SEOConfig
}

type themeRow struct {
	Version string
	Tokens  siteconfig.ThemeTokens
}

type pageRow struct {
	ID       string
	Title    string
	Slug     string
	Sort     int
	SEO      siteconfig.SEOConfig
	Settings map[string]any
}

type blockRow struct {
	ID       string
	PageID   string
	Type     string
	Version  string
	Sort     int
	Props    map[string]any
	Settings siteconfig.BlockSettings
	Hidden   bool
}

type NormalizedDraftRows struct {
	Site   siteRow
	Theme  themeRow
	Pages  []pageRow
	Blocks []blockRow
}

func NewPostgresReader(db DB) *PostgresReader {
	return &PostgresReader{db: db}
}

func (r *PostgresReader) ListSites(ctx context.Context, workspaceID string) ([]Summary, error) {
	rows, err := r.db.Query(ctx, `
		select s.id::text,
		       s.workspace_id::text,
		       s.name,
		       s.slug,
		       case
		         when s.published_version_id is not null then 'published'
		         else s.status
		       end as status,
		       s.default_locale,
		       coalesce(s.published_version_id::text, '') as published_version_id,
		       count(p.id)::int as page_count
		from sites s
		left join pages p on p.site_id = s.id
		where s.workspace_id = $1
		group by s.id
		order by s.updated_at desc, s.created_at desc
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	defer rows.Close()

	var summaries []Summary
	for rows.Next() {
		var summary Summary
		if err := rows.Scan(
			&summary.ID,
			&summary.WorkspaceID,
			&summary.Name,
			&summary.Slug,
			&summary.Status,
			&summary.DefaultLocale,
			&summary.PublishedVersionID,
			&summary.PageCount,
		); err != nil {
			return nil, fmt.Errorf("scan site summary: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate site summaries: %w", err)
	}
	return summaries, nil
}

func (r *PostgresReader) LoadDraft(ctx context.Context, siteID string) (siteconfig.SiteDraft, error) {
	normalized, err := r.loadNormalizedDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	draft := AssembleDraft(normalized)
	if err := siteconfig.ValidateDraft(draft); err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("assembled draft is invalid: %w", err)
	}
	return draft, nil
}

func (r *PostgresReader) loadNormalizedDraft(ctx context.Context, siteID string) (NormalizedDraftRows, error) {
	var site siteRow
	var siteSettingsJSON []byte
	err := r.db.QueryRow(ctx, `
		select id::text, name, slug, status, default_locale, settings
		from sites
		where id = $1
	`, siteID).Scan(&site.ID, &site.Name, &site.Slug, &site.Status, &site.DefaultLocale, &siteSettingsJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return NormalizedDraftRows{}, ErrNotFound
	}
	if err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("load site: %w", err)
	}
	if err := decodeNestedSEO(siteSettingsJSON, &site.SEO); err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("decode site settings: %w", err)
	}

	var theme themeRow
	var tokensJSON []byte
	err = r.db.QueryRow(ctx, `
		select version, tokens
		from themes
		where site_id = $1
		order by created_at asc
		limit 1
	`, siteID).Scan(&theme.Version, &tokensJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return NormalizedDraftRows{}, fmt.Errorf("load theme: %w", ErrNotFound)
	}
	if err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("load theme: %w", err)
	}
	if err := decodeJSON(tokensJSON, &theme.Tokens); err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("decode theme tokens: %w", err)
	}

	pages, err := r.loadPages(ctx, siteID)
	if err != nil {
		return NormalizedDraftRows{}, err
	}
	blocks, err := r.loadBlocks(ctx, siteID)
	if err != nil {
		return NormalizedDraftRows{}, err
	}

	return NormalizedDraftRows{
		Site:   site,
		Theme:  theme,
		Pages:  pages,
		Blocks: blocks,
	}, nil
}

func (r *PostgresReader) loadPages(ctx context.Context, siteID string) ([]pageRow, error) {
	rows, err := r.db.Query(ctx, `
		select id::text, title, slug, sort_order, seo, settings
		from pages
		where site_id = $1
		order by sort_order asc, created_at asc
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("load pages: %w", err)
	}
	defer rows.Close()

	var pages []pageRow
	for rows.Next() {
		var page pageRow
		var seoJSON []byte
		var settingsJSON []byte
		if err := rows.Scan(&page.ID, &page.Title, &page.Slug, &page.Sort, &seoJSON, &settingsJSON); err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		if err := decodeJSON(seoJSON, &page.SEO); err != nil {
			return nil, fmt.Errorf("decode page seo: %w", err)
		}
		if err := decodeJSON(settingsJSON, &page.Settings); err != nil {
			return nil, fmt.Errorf("decode page settings: %w", err)
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages: %w", err)
	}
	return pages, nil
}

func (r *PostgresReader) loadBlocks(ctx context.Context, siteID string) ([]blockRow, error) {
	rows, err := r.db.Query(ctx, `
		select id::text, page_id::text, type, version, sort_order, props, settings, is_hidden
		from block_instances
		where site_id = $1
		order by page_id asc, sort_order asc, created_at asc
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("load blocks: %w", err)
	}
	defer rows.Close()

	var blocks []blockRow
	for rows.Next() {
		var block blockRow
		var propsJSON []byte
		var settingsJSON []byte
		if err := rows.Scan(
			&block.ID,
			&block.PageID,
			&block.Type,
			&block.Version,
			&block.Sort,
			&propsJSON,
			&settingsJSON,
			&block.Hidden,
		); err != nil {
			return nil, fmt.Errorf("scan block: %w", err)
		}
		if err := decodeJSON(propsJSON, &block.Props); err != nil {
			return nil, fmt.Errorf("decode block props: %w", err)
		}
		if err := decodeJSON(settingsJSON, &block.Settings); err != nil {
			return nil, fmt.Errorf("decode block settings: %w", err)
		}
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blocks: %w", err)
	}
	return blocks, nil
}

func AssembleDraft(rows NormalizedDraftRows) siteconfig.SiteDraft {
	blocksByPage := map[string][]blockRow{}
	for _, block := range rows.Blocks {
		blocksByPage[block.PageID] = append(blocksByPage[block.PageID], block)
	}
	for pageID := range blocksByPage {
		sort.SliceStable(blocksByPage[pageID], func(i int, j int) bool {
			return blocksByPage[pageID][i].Sort < blocksByPage[pageID][j].Sort
		})
	}

	pages := append([]pageRow(nil), rows.Pages...)
	sort.SliceStable(pages, func(i int, j int) bool {
		return pages[i].Sort < pages[j].Sort
	})

	draftPages := make([]siteconfig.PageDraft, 0, len(pages))
	navigation := siteconfig.NavigationConfig{
		Primary: make([]siteconfig.NavigationItem, 0, len(pages)),
	}
	for _, page := range pages {
		draftBlocks := make([]siteconfig.BlockInstance, 0, len(blocksByPage[page.ID]))
		for _, block := range blocksByPage[page.ID] {
			settings := block.Settings
			if block.Hidden {
				settings.Hidden = true
			}
			draftBlocks = append(draftBlocks, siteconfig.BlockInstance{
				ID:       block.ID,
				Type:     block.Type,
				Version:  block.Version,
				Props:    block.Props,
				Settings: settings,
			})
		}
		draftPages = append(draftPages, siteconfig.PageDraft{
			ID:       page.ID,
			Title:    page.Title,
			Slug:     page.Slug,
			SEO:      page.SEO,
			Blocks:   draftBlocks,
			Settings: page.Settings,
		})
		navigation.Primary = append(navigation.Primary, siteconfig.NavigationItem{
			Label:  page.Title,
			PageID: page.ID,
		})
	}

	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            rows.Site.ID,
			Name:          rows.Site.Name,
			Slug:          rows.Site.Slug,
			Status:        rows.Site.Status,
			DefaultLocale: rows.Site.DefaultLocale,
			SEO:           rows.Site.SEO,
		},
		Theme:      siteconfig.ThemeConfig{Version: rows.Theme.Version, Tokens: rows.Theme.Tokens},
		Navigation: navigation,
		Pages:      draftPages,
	}
}

func decodeNestedSEO(raw []byte, dest *siteconfig.SEOConfig) error {
	var settings struct {
		SEO siteconfig.SEOConfig `json:"seo"`
	}
	if err := decodeJSON(raw, &settings); err != nil {
		return err
	}
	*dest = settings.SEO
	return nil
}

func decodeJSON(raw []byte, dest any) error {
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	return json.Unmarshal(raw, dest)
}
