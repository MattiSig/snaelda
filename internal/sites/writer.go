package sites

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const defaultThemeName = "Default"

type transactionStarter interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Writer interface {
	SaveDraft(ctx context.Context, workspaceID string, draft siteconfig.SiteDraft) error
}

type PostgresWriter struct {
	db transactionStarter
}

func NewPostgresWriter(db transactionStarter) *PostgresWriter {
	return &PostgresWriter{db: db}
}

func (w *PostgresWriter) SaveDraft(ctx context.Context, workspaceID string, draft siteconfig.SiteDraft) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace id is required")
	}
	rows, err := normalizeDraft(draft)
	if err != nil {
		return err
	}

	tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin draft persistence transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := saveSite(ctx, tx, workspaceID, rows.Site); err != nil {
		return err
	}
	if err := saveTheme(ctx, tx, rows.Site.ID, rows.Theme); err != nil {
		return err
	}
	if err := replacePagesAndBlocks(ctx, tx, rows.Site.ID, rows.Pages, rows.Blocks); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit draft persistence transaction: %w", err)
	}
	return nil
}

func normalizeDraft(draft siteconfig.SiteDraft) (NormalizedDraftRows, error) {
	if err := siteconfig.ValidateDraft(draft); err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("validate draft before persistence: %w", err)
	}

	siteStatus := draft.Site.Status
	if siteStatus == "" {
		siteStatus = "draft"
	}
	defaultLocale := draft.Site.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	pages := make([]pageRow, 0, len(draft.Pages))
	blocks := make([]blockRow, 0, countBlocks(draft.Pages))
	for pageIndex, page := range draft.Pages {
		settings := page.Settings
		if settings == nil {
			settings = map[string]any{}
		}
		pages = append(pages, pageRow{
			ID:       page.ID,
			Title:    page.Title,
			Slug:     page.Slug,
			Sort:     pageIndex,
			SEO:      page.SEO,
			Settings: settings,
		})
		for blockIndex, block := range page.Blocks {
			props := block.Props
			if props == nil {
				props = map[string]any{}
			}
			blocks = append(blocks, blockRow{
				ID:      block.ID,
				PageID:  page.ID,
				Type:    block.Type,
				Version: block.Version,
				Sort:    blockIndex,
				Props:   props,
				Settings: siteconfig.BlockSettings{
					AnchorID: block.Settings.AnchorID,
				},
				Hidden: block.Settings.Hidden,
			})
		}
	}

	return NormalizedDraftRows{
		Site: siteRow{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			Slug:          draft.Site.Slug,
			Status:        siteStatus,
			DefaultLocale: defaultLocale,
			SEO:           draft.Site.SEO,
		},
		Theme: themeRow{
			Version: draft.Theme.Version,
			Tokens:  draft.Theme.Tokens,
		},
		Pages:  pages,
		Blocks: blocks,
	}, nil
}

func countBlocks(pages []siteconfig.PageDraft) int {
	count := 0
	for _, page := range pages {
		count += len(page.Blocks)
	}
	return count
}

func saveSite(ctx context.Context, tx pgx.Tx, workspaceID string, site siteRow) error {
	settingsJSON, err := marshalJSON(map[string]any{
		"seo": site.SEO,
	})
	if err != nil {
		return fmt.Errorf("encode site settings: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		insert into sites (id, workspace_id, name, slug, status, default_locale, settings)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (id) do update
		set name = excluded.name,
		    slug = excluded.slug,
		    status = excluded.status,
		    default_locale = excluded.default_locale,
		    settings = excluded.settings,
		    updated_at = now()
		where sites.workspace_id = excluded.workspace_id
	`, site.ID, workspaceID, site.Name, site.Slug, site.Status, site.DefaultLocale, settingsJSON)
	if err != nil {
		return fmt.Errorf("save site row: %w", err)
	}
	if err := requireRowsAffected(tag, "save site row"); err != nil {
		return err
	}
	return nil
}

func saveTheme(ctx context.Context, tx pgx.Tx, siteID string, theme themeRow) error {
	tokensJSON, err := marshalJSON(theme.Tokens)
	if err != nil {
		return fmt.Errorf("encode theme tokens: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		insert into themes (site_id, name, version, tokens)
		values ($1, $2, $3, $4)
		on conflict (site_id, name) do update
		set version = excluded.version,
		    tokens = excluded.tokens,
		    updated_at = now()
	`, siteID, defaultThemeName, theme.Version, tokensJSON)
	if err != nil {
		return fmt.Errorf("save theme row: %w", err)
	}
	if err := requireRowsAffected(tag, "save theme row"); err != nil {
		return err
	}
	return nil
}

func replacePagesAndBlocks(ctx context.Context, tx pgx.Tx, siteID string, pages []pageRow, blocks []blockRow) error {
	if _, err := tx.Exec(ctx, `delete from block_instances where site_id = $1`, siteID); err != nil {
		return fmt.Errorf("delete existing block rows: %w", err)
	}

	pageIDs := make([]string, 0, len(pages))
	for _, page := range pages {
		pageIDs = append(pageIDs, page.ID)
	}
	if _, err := tx.Exec(ctx, `
		delete from pages
		where site_id = $1
		  and not (id::text = any($2))
	`, siteID, pageIDs); err != nil {
		return fmt.Errorf("delete removed page rows: %w", err)
	}

	for _, page := range pages {
		seoJSON, err := marshalJSON(page.SEO)
		if err != nil {
			return fmt.Errorf("encode page seo %s: %w", page.ID, err)
		}
		settingsJSON, err := marshalJSON(page.Settings)
		if err != nil {
			return fmt.Errorf("encode page settings %s: %w", page.ID, err)
		}
		tag, err := tx.Exec(ctx, `
			insert into pages (id, site_id, title, slug, sort_order, status, seo, settings)
			values ($1, $2, $3, $4, $5, 'draft', $6, $7)
			on conflict (id) do update
			set title = excluded.title,
			    slug = excluded.slug,
			    sort_order = excluded.sort_order,
			    status = excluded.status,
			    seo = excluded.seo,
			    settings = excluded.settings,
			    updated_at = now()
			where pages.site_id = excluded.site_id
		`, page.ID, siteID, page.Title, page.Slug, page.Sort, seoJSON, settingsJSON)
		if err != nil {
			return fmt.Errorf("save page row %s: %w", page.ID, err)
		}
		if err := requireRowsAffected(tag, "save page row "+page.ID); err != nil {
			return err
		}
	}

	for _, block := range blocks {
		propsJSON, err := marshalJSON(block.Props)
		if err != nil {
			return fmt.Errorf("encode block props %s: %w", block.ID, err)
		}
		settingsJSON, err := marshalJSON(block.Settings)
		if err != nil {
			return fmt.Errorf("encode block settings %s: %w", block.ID, err)
		}
		tag, err := tx.Exec(ctx, `
			insert into block_instances (id, page_id, site_id, type, version, sort_order, props, settings, is_hidden)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, block.ID, block.PageID, siteID, block.Type, block.Version, block.Sort, propsJSON, settingsJSON, block.Hidden)
		if err != nil {
			return fmt.Errorf("save block row %s: %w", block.ID, err)
		}
		if err := requireRowsAffected(tag, "save block row "+block.ID); err != nil {
			return err
		}
	}
	return nil
}

func marshalJSON(value any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return []byte(`{}`), nil
	}
	return raw, nil
}

func requireRowsAffected(tag pgconn.CommandTag, operation string) error {
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: no rows affected", operation)
	}
	return nil
}
