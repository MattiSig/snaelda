package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

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
	db writerDB
}

type writerDB interface {
	transactionStarter
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewPostgresWriter(db writerDB) *PostgresWriter {
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
	if err := w.validateAssetReferences(ctx, workspaceID, draft.Site.ID, draft.Pages); err != nil {
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
	if err := replaceCollectionsAndEntries(ctx, tx, rows.Site.ID, rows.Collections, rows.Entries); err != nil {
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
		pageType := page.Type
		if pageType == "" {
			pageType = siteconfig.PageTypeStatic
		}
		pages = append(pages, pageRow{
			ID:           page.ID,
			Title:        page.Title,
			Slug:         page.Slug,
			Sort:         pageIndex,
			Type:         pageType,
			CollectionID: page.CollectionID,
			SEO:          page.SEO,
			Settings:     settings,
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
				Hidden:   block.Settings.Hidden,
				Bindings: block.Bindings,
			})
		}
	}

	collections := make([]collectionRow, 0, len(draft.Collections))
	entries := make([]collectionEntryRow, 0)
	for index, collection := range draft.Collections {
		schema := collection.Schema
		if schema == nil {
			schema = []siteconfig.FieldDefinition{}
		}
		sortOrder := collection.SortOrder
		if sortOrder == 0 {
			sortOrder = index
		}
		collections = append(collections, collectionRow{
			ID:            collection.ID,
			Slug:          collection.Slug,
			SingularLabel: collection.SingularLabel,
			PluralLabel:   collection.PluralLabel,
			Schema:        schema,
			Settings:      collection.Settings,
			SortOrder:     sortOrder,
		})
		for entryIndex, entry := range collection.Entries {
			status := entry.Status
			if status == "" {
				status = siteconfig.EntryStatusDraft
			}
			fields := entry.Fields
			if fields == nil {
				fields = map[string]any{}
			}
			entrySort := entry.SortOrder
			if entrySort == 0 {
				entrySort = entryIndex
			}
			entries = append(entries, collectionEntryRow{
				ID:           entry.ID,
				CollectionID: collection.ID,
				Slug:         entry.Slug,
				Fields:       fields,
				SEO:          entry.SEO,
				Status:       status,
				SortOrder:    entrySort,
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
			Navigation:    draft.Navigation,
			HasNavigation: true,
		},
		Theme: themeRow{
			Version: draft.Theme.Version,
			Tokens:  draft.Theme.Tokens,
		},
		Pages:       pages,
		Blocks:      blocks,
		Collections: collections,
		Entries:     entries,
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
		"seo":        site.SEO,
		"navigation": site.Navigation,
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
		pageType := page.Type
		if pageType == "" {
			pageType = siteconfig.PageTypeStatic
		}
		var collectionID any
		if page.CollectionID != "" {
			collectionID = page.CollectionID
		}
		tag, err := tx.Exec(ctx, `
			insert into pages (id, site_id, title, slug, sort_order, status, type, collection_id, seo, settings)
			values ($1, $2, $3, $4, $5, 'draft', $6, $7, $8, $9)
			on conflict (id) do update
			set title = excluded.title,
			    slug = excluded.slug,
			    sort_order = excluded.sort_order,
			    status = excluded.status,
			    type = excluded.type,
			    collection_id = excluded.collection_id,
			    seo = excluded.seo,
			    settings = excluded.settings,
			    updated_at = now()
			where pages.site_id = excluded.site_id
		`, page.ID, siteID, page.Title, page.Slug, page.Sort, pageType, collectionID, seoJSON, settingsJSON)
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
		bindingsJSON, err := marshalJSON(block.Bindings)
		if err != nil {
			return fmt.Errorf("encode block bindings %s: %w", block.ID, err)
		}
		tag, err := tx.Exec(ctx, `
			insert into block_instances (id, page_id, site_id, type, version, sort_order, props, settings, is_hidden, bindings)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, block.ID, block.PageID, siteID, block.Type, block.Version, block.Sort, propsJSON, settingsJSON, block.Hidden, bindingsJSON)
		if err != nil {
			return fmt.Errorf("save block row %s: %w", block.ID, err)
		}
		if err := requireRowsAffected(tag, "save block row "+block.ID); err != nil {
			return err
		}
	}
	return nil
}

func replaceCollectionsAndEntries(ctx context.Context, tx pgx.Tx, siteID string, collections []collectionRow, entries []collectionEntryRow) error {
	if _, err := tx.Exec(ctx, `delete from collection_entries where site_id = $1`, siteID); err != nil {
		return fmt.Errorf("delete existing collection entry rows: %w", err)
	}

	collectionIDs := make([]string, 0, len(collections))
	for _, collection := range collections {
		collectionIDs = append(collectionIDs, collection.ID)
	}
	if _, err := tx.Exec(ctx, `
		delete from collections
		where site_id = $1
		  and not (id::text = any($2))
	`, siteID, collectionIDs); err != nil {
		return fmt.Errorf("delete removed collection rows: %w", err)
	}

	for _, collection := range collections {
		schemaJSON, err := marshalJSON(collection.Schema)
		if err != nil {
			return fmt.Errorf("encode collection schema %s: %w", collection.ID, err)
		}
		settingsJSON, err := marshalJSON(collection.Settings)
		if err != nil {
			return fmt.Errorf("encode collection settings %s: %w", collection.ID, err)
		}
		tag, err := tx.Exec(ctx, `
			insert into collections (id, site_id, slug, singular_label, plural_label, schema, settings, sort_order)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (id) do update
			set slug = excluded.slug,
			    singular_label = excluded.singular_label,
			    plural_label = excluded.plural_label,
			    schema = excluded.schema,
			    settings = excluded.settings,
			    sort_order = excluded.sort_order,
			    updated_at = now()
			where collections.site_id = excluded.site_id
		`, collection.ID, siteID, collection.Slug, collection.SingularLabel, collection.PluralLabel, schemaJSON, settingsJSON, collection.SortOrder)
		if err != nil {
			return fmt.Errorf("save collection row %s: %w", collection.ID, err)
		}
		if err := requireRowsAffected(tag, "save collection row "+collection.ID); err != nil {
			return err
		}
	}

	for _, entry := range entries {
		fieldsJSON, err := marshalJSON(entry.Fields)
		if err != nil {
			return fmt.Errorf("encode collection entry fields %s: %w", entry.ID, err)
		}
		seoJSON, err := marshalJSON(entry.SEO)
		if err != nil {
			return fmt.Errorf("encode collection entry seo %s: %w", entry.ID, err)
		}
		tag, err := tx.Exec(ctx, `
			insert into collection_entries (id, collection_id, site_id, slug, fields, seo, status, sort_order)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
		`, entry.ID, entry.CollectionID, siteID, entry.Slug, fieldsJSON, seoJSON, entry.Status, entry.SortOrder)
		if err != nil {
			return fmt.Errorf("save collection entry row %s: %w", entry.ID, err)
		}
		if err := requireRowsAffected(tag, "save collection entry row "+entry.ID); err != nil {
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

type assetReference struct {
	ID   string
	Path string
}

func (w *PostgresWriter) validateAssetReferences(ctx context.Context, workspaceID string, siteID string, pages []siteconfig.PageDraft) error {
	references := collectAssetReferences(pages)
	if len(references) == 0 {
		return nil
	}

	assetIDs := []string{}
	for _, reference := range references {
		if !slices.Contains(assetIDs, reference.ID) {
			assetIDs = append(assetIDs, reference.ID)
		}
	}

	var matchesJSON []byte
	if err := w.db.QueryRow(ctx, `
		select coalesce(
			jsonb_agg(
				jsonb_build_object(
					'id', id::text,
					'siteId', coalesce(site_id::text, '')
				)
			),
			'[]'::jsonb
		)
		from assets
		where workspace_id = $1
		  and id::text = any($2)
	`, workspaceID, assetIDs).Scan(&matchesJSON); err != nil {
		return fmt.Errorf("validate draft asset references: %w", err)
	}

	type assetMatch struct {
		ID     string `json:"id"`
		SiteID string `json:"siteId"`
	}
	matches := []assetMatch{}
	if err := json.Unmarshal(matchesJSON, &matches); err != nil {
		return fmt.Errorf("decode asset reference validation: %w", err)
	}

	allowed := map[string]string{}
	for _, match := range matches {
		allowed[match.ID] = match.SiteID
	}

	issues := []siteconfig.Issue{}
	for _, reference := range references {
		referencedSiteID, ok := allowed[reference.ID]
		if !ok {
			issues = append(issues, siteconfig.Issue{
				Path:    reference.Path,
				Code:    "invalid_asset_reference",
				Message: "referenced asset does not belong to this workspace",
			})
			continue
		}
		if referencedSiteID != "" && referencedSiteID != siteID {
			issues = append(issues, siteconfig.Issue{
				Path:    reference.Path,
				Code:    "invalid_asset_reference",
				Message: "referenced asset belongs to a different site",
			})
		}
	}
	if len(issues) > 0 {
		return siteconfig.ValidationError{Issues: issues}
	}
	return nil
}

func collectAssetReferences(pages []siteconfig.PageDraft) []assetReference {
	references := []assetReference{}
	for pageIndex, page := range pages {
		for blockIndex, block := range page.Blocks {
			path := fmt.Sprintf("pages[%d].blocks[%d].props", pageIndex, blockIndex)
			references = append(references, collectAssetReferencesFromValue(path, block.Props)...)
		}
	}
	return references
}

func collectAssetReferencesFromValue(path string, value any) []assetReference {
	references := []assetReference{}
	switch typed := value.(type) {
	case map[string]any:
		if assetID, ok := typed["assetId"].(string); ok && assetID != "" {
			references = append(references, assetReference{
				ID:   assetID,
				Path: path + ".assetId",
			})
		}
		for key, nested := range typed {
			references = append(references, collectAssetReferencesFromValue(path+"."+key, nested)...)
		}
	case []any:
		for index, nested := range typed {
			references = append(references, collectAssetReferencesFromValue(fmt.Sprintf("%s[%d]", path, index), nested)...)
		}
	}
	return references
}
