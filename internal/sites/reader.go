package sites

import (
	"bytes"
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
	LoadGenerationMetadata(ctx context.Context, siteID string) (GenerationMetadata, error)
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

type GenerationMetadata struct {
	Prompt               string   `json:"prompt"`
	ThemePreset          string   `json:"themePreset,omitempty"`
	AssetsNeeded         []string `json:"assetsNeeded,omitempty"`
	Assumptions          []string `json:"assumptions,omitempty"`
	ValidationRetryCount int      `json:"validationRetryCount,omitempty"`
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
	Navigation    siteconfig.NavigationConfig
	HasNavigation bool
	Brand         siteconfig.BrandConfig
}

type themeRow struct {
	Version string
	Tokens  siteconfig.ThemeTokens
}

type pageRow struct {
	ID           string
	Title        string
	Slug         string
	Sort         int
	Type         string
	CollectionID string
	SEO          siteconfig.SEOConfig
	Settings     map[string]any
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
	Bindings map[string]siteconfig.BlockBinding
}

type collectionRow struct {
	ID            string
	Slug          string
	SingularLabel string
	PluralLabel   string
	Schema        []siteconfig.FieldDefinition
	Settings      siteconfig.CollectionSettings
	SortOrder     int
}

type collectionEntryRow struct {
	ID           string
	CollectionID string
	Slug         string
	Fields       map[string]any
	SEO          siteconfig.SEOConfig
	Status       string
	SortOrder    int
}

type NormalizedDraftRows struct {
	Site        siteRow
	Theme       themeRow
	Pages       []pageRow
	Blocks      []blockRow
	Collections []collectionRow
	Entries     []collectionEntryRow
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

func (r *PostgresReader) LoadGenerationMetadata(ctx context.Context, siteID string) (GenerationMetadata, error) {
	var prompt string
	var summaryJSON []byte
	err := r.db.QueryRow(ctx, `
		select coalesce(generation_prompt, ''),
		       coalesce(generation_summary, '{}'::jsonb)
		from sites
		where id = $1
	`, siteID).Scan(&prompt, &summaryJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationMetadata{}, ErrNotFound
	}
	if err != nil {
		return GenerationMetadata{}, fmt.Errorf("load generation metadata: %w", err)
	}

	summary := map[string]any{}
	if len(summaryJSON) > 0 {
		if err := decodeJSON(summaryJSON, &summary); err != nil {
			return GenerationMetadata{}, fmt.Errorf("decode generation metadata: %w", err)
		}
	}

	return GenerationMetadata{
		Prompt:               prompt,
		ThemePreset:          asStringValue(summary["themePreset"]),
		AssetsNeeded:         asStringSlice(summary["assetsNeeded"]),
		Assumptions:          asStringSlice(summary["assumptions"]),
		ValidationRetryCount: asIntValue(summary["validationRetryCount"]),
	}, nil
}

func (r *PostgresReader) loadNormalizedDraft(ctx context.Context, siteID string) (NormalizedDraftRows, error) {
	var site siteRow
	var siteSettingsJSON []byte
	var brandJSON []byte
	err := r.db.QueryRow(ctx, `
		select id::text, name, slug, status, default_locale, settings, coalesce(brand, '{}'::jsonb)
		from sites
		where id = $1
	`, siteID).Scan(&site.ID, &site.Name, &site.Slug, &site.Status, &site.DefaultLocale, &siteSettingsJSON, &brandJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return NormalizedDraftRows{}, ErrNotFound
	}
	if err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("load site: %w", err)
	}
	if err := decodeSiteSettings(siteSettingsJSON, &site); err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("decode site settings: %w", err)
	}
	if err := decodeJSON(brandJSON, &site.Brand); err != nil {
		return NormalizedDraftRows{}, fmt.Errorf("decode site brand: %w", err)
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
	collections, err := r.loadCollections(ctx, siteID)
	if err != nil {
		return NormalizedDraftRows{}, err
	}
	entries, err := r.loadCollectionEntries(ctx, siteID)
	if err != nil {
		return NormalizedDraftRows{}, err
	}

	return NormalizedDraftRows{
		Site:        site,
		Theme:       theme,
		Pages:       pages,
		Blocks:      blocks,
		Collections: collections,
		Entries:     entries,
	}, nil
}

func (r *PostgresReader) loadPages(ctx context.Context, siteID string) ([]pageRow, error) {
	rows, err := r.db.Query(ctx, `
		select id::text,
		       title,
		       slug,
		       sort_order,
		       coalesce(type, 'static'),
		       coalesce(collection_id::text, ''),
		       seo,
		       settings
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
		if err := rows.Scan(
			&page.ID,
			&page.Title,
			&page.Slug,
			&page.Sort,
			&page.Type,
			&page.CollectionID,
			&seoJSON,
			&settingsJSON,
		); err != nil {
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
		select id::text,
		       page_id::text,
		       type,
		       version,
		       sort_order,
		       props,
		       settings,
		       is_hidden,
		       coalesce(bindings, '{}'::jsonb)
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
		var bindingsJSON []byte
		if err := rows.Scan(
			&block.ID,
			&block.PageID,
			&block.Type,
			&block.Version,
			&block.Sort,
			&propsJSON,
			&settingsJSON,
			&block.Hidden,
			&bindingsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan block: %w", err)
		}
		if err := decodeJSON(propsJSON, &block.Props); err != nil {
			return nil, fmt.Errorf("decode block props: %w", err)
		}
		if err := decodeJSON(settingsJSON, &block.Settings); err != nil {
			return nil, fmt.Errorf("decode block settings: %w", err)
		}
		block.Bindings = map[string]siteconfig.BlockBinding{}
		if err := decodeJSON(bindingsJSON, &block.Bindings); err != nil {
			return nil, fmt.Errorf("decode block bindings: %w", err)
		}
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blocks: %w", err)
	}
	return blocks, nil
}

func (r *PostgresReader) loadCollections(ctx context.Context, siteID string) ([]collectionRow, error) {
	rows, err := r.db.Query(ctx, `
		select id::text,
		       slug,
		       singular_label,
		       plural_label,
		       schema,
		       settings,
		       sort_order
		from collections
		where site_id = $1
		order by sort_order asc, created_at asc
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("load collections: %w", err)
	}
	defer rows.Close()

	var collections []collectionRow
	for rows.Next() {
		var collection collectionRow
		var schemaJSON []byte
		var settingsJSON []byte
		if err := rows.Scan(
			&collection.ID,
			&collection.Slug,
			&collection.SingularLabel,
			&collection.PluralLabel,
			&schemaJSON,
			&settingsJSON,
			&collection.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("scan collection: %w", err)
		}
		collection.Schema = []siteconfig.FieldDefinition{}
		if len(schemaJSON) > 0 {
			if err := decodeJSON(schemaJSON, &collection.Schema); err != nil {
				return nil, fmt.Errorf("decode collection schema: %w", err)
			}
		}
		if err := decodeJSON(settingsJSON, &collection.Settings); err != nil {
			return nil, fmt.Errorf("decode collection settings: %w", err)
		}
		collections = append(collections, collection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collections: %w", err)
	}
	return collections, nil
}

func (r *PostgresReader) loadCollectionEntries(ctx context.Context, siteID string) ([]collectionEntryRow, error) {
	rows, err := r.db.Query(ctx, `
		select id::text,
		       collection_id::text,
		       slug,
		       fields,
		       seo,
		       status,
		       sort_order
		from collection_entries
		where site_id = $1
		order by collection_id asc, sort_order asc, created_at asc
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("load collection entries: %w", err)
	}
	defer rows.Close()

	var entries []collectionEntryRow
	for rows.Next() {
		var entry collectionEntryRow
		var fieldsJSON []byte
		var seoJSON []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.CollectionID,
			&entry.Slug,
			&fieldsJSON,
			&seoJSON,
			&entry.Status,
			&entry.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("scan collection entry: %w", err)
		}
		entry.Fields = map[string]any{}
		if err := decodeJSON(fieldsJSON, &entry.Fields); err != nil {
			return nil, fmt.Errorf("decode collection entry fields: %w", err)
		}
		if err := decodeJSON(seoJSON, &entry.SEO); err != nil {
			return nil, fmt.Errorf("decode collection entry seo: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collection entries: %w", err)
	}
	return entries, nil
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
	for _, page := range pages {
		draftBlocks := make([]siteconfig.BlockInstance, 0, len(blocksByPage[page.ID]))
		for _, block := range blocksByPage[page.ID] {
			settings := block.Settings
			if block.Hidden {
				settings.Hidden = true
			}
			instance := siteconfig.BlockInstance{
				ID:       block.ID,
				Type:     block.Type,
				Version:  block.Version,
				Props:    block.Props,
				Settings: settings,
			}
			if len(block.Bindings) > 0 {
				instance.Bindings = block.Bindings
			}
			draftBlocks = append(draftBlocks, instance)
		}
		pageType := page.Type
		if pageType == "" {
			pageType = siteconfig.PageTypeStatic
		}
		draft := siteconfig.PageDraft{
			ID:       page.ID,
			Title:    page.Title,
			Slug:     page.Slug,
			SEO:      page.SEO,
			Blocks:   draftBlocks,
			Settings: page.Settings,
		}
		if pageType != siteconfig.PageTypeStatic {
			draft.Type = pageType
			draft.CollectionID = page.CollectionID
		} else if page.CollectionID != "" {
			draft.CollectionID = page.CollectionID
		}
		draftPages = append(draftPages, draft)
	}

	navigation := rows.Site.Navigation
	if !rows.Site.HasNavigation {
		navigation = navigationFromPageRows(pages)
	}

	entriesByCollection := map[string][]siteconfig.CollectionEntry{}
	for _, entry := range rows.Entries {
		entriesByCollection[entry.CollectionID] = append(entriesByCollection[entry.CollectionID], siteconfig.CollectionEntry{
			ID:        entry.ID,
			Slug:      entry.Slug,
			Fields:    entry.Fields,
			SEO:       entry.SEO,
			Status:    entry.Status,
			SortOrder: entry.SortOrder,
		})
	}
	collections := make([]siteconfig.Collection, 0, len(rows.Collections))
	for _, row := range rows.Collections {
		collections = append(collections, siteconfig.Collection{
			ID:            row.ID,
			Slug:          row.Slug,
			SingularLabel: row.SingularLabel,
			PluralLabel:   row.PluralLabel,
			Schema:        row.Schema,
			Settings:      row.Settings,
			SortOrder:     row.SortOrder,
			Entries:       entriesByCollection[row.ID],
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
		Brand:       rows.Site.Brand,
		Theme:       siteconfig.ThemeConfig{Version: rows.Theme.Version, Tokens: rows.Theme.Tokens},
		Navigation:  navigation,
		Pages:       draftPages,
		Collections: collections,
	}
}

func asStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func asStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok && text != "" {
			result = append(result, text)
		}
	}
	return result
}

func asIntValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func navigationFromPageRows(pages []pageRow) siteconfig.NavigationConfig {
	navigation := siteconfig.NavigationConfig{
		Primary: make([]siteconfig.NavigationItem, 0, len(pages)),
	}
	for _, page := range pages {
		if !pageIncludedInNavigation(page.Settings) {
			continue
		}
		navigation.Primary = append(navigation.Primary, siteconfig.NavigationItem{
			Label:  page.Title,
			PageID: page.ID,
		})
	}
	return navigation
}

func pageIncludedInNavigation(settings map[string]any) bool {
	if settings == nil {
		return true
	}
	value, ok := settings["includeInNavigation"]
	if !ok {
		return true
	}
	include, ok := value.(bool)
	if !ok {
		return true
	}
	return include
}

func decodeSiteSettings(raw []byte, site *siteRow) error {
	var settings struct {
		SEO        siteconfig.SEOConfig `json:"seo"`
		Navigation json.RawMessage      `json:"navigation"`
	}
	if err := decodeJSON(raw, &settings); err != nil {
		return err
	}
	site.SEO = settings.SEO
	navigationRaw := bytes.TrimSpace(settings.Navigation)
	if len(navigationRaw) == 0 || string(navigationRaw) == "null" {
		site.Navigation = siteconfig.NavigationConfig{}
		site.HasNavigation = false
		return nil
	}
	if err := json.Unmarshal(navigationRaw, &site.Navigation); err != nil {
		return err
	}
	site.HasNavigation = true
	return nil
}

func decodeJSON(raw []byte, dest any) error {
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	return json.Unmarshal(raw, dest)
}
