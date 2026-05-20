package sites

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/slugs"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSiteNameRequired        = errors.New("site name is required")
	ErrSiteSlugInvalid         = errors.New("site slug is invalid")
	ErrSiteSlugConflict        = errors.New("site slug is already in use")
	ErrNoSiteChanges           = errors.New("site update requires at least one change")
	ErrPageTitleRequired       = errors.New("page title is required")
	ErrPageSlugInvalid         = errors.New("page slug is invalid")
	ErrPageSlugConflict        = errors.New("page slug is already in use")
	ErrPageNotFound            = errors.New("page was not found")
	ErrNoPageChanges           = errors.New("page update requires at least one change")
	ErrPageLimitReached        = errors.New("site already has the maximum number of pages")
	ErrPageOrderInvalid        = errors.New("page reorder must include every page exactly once")
	ErrHomepageSlugLocked      = errors.New("homepage slug cannot be changed")
	ErrHomepageDeleteForbidden = errors.New("homepage cannot be deleted")
	ErrMinimumPagesRequired    = errors.New("site must keep at least one page")
	ErrBlockNotFound           = errors.New("block was not found")
	ErrNoBlockChanges          = errors.New("block update requires at least one change")
	ErrBlockTypeRequired       = errors.New("block type is required")
	ErrBlockOrderInvalid       = errors.New("block reorder must include every block exactly once")
	ErrNavigationOrderInvalid  = errors.New("navigation reorder must include every visible navigation page exactly once")
	ErrNavigationLabelRequired = errors.New("navigation item label is required")
	ErrNavigationItemInvalid   = errors.New("navigation item must reference a page or include an href")
	ErrNavigationPageUnknown   = errors.New("navigation item references a page that does not exist")
	ErrNavigationHrefInvalid   = errors.New("navigation item href is invalid")
	ErrNavigationLabelTooLong  = errors.New("navigation item label is too long")

	ErrPageTypeUnsupported       = errors.New("page type is not supported")
	ErrPageCollectionRequired    = errors.New("collection page must reference a collection")
	ErrPageCollectionUnsupported = errors.New("static pages cannot reference a collection")
	ErrPageCollectionNotFound    = errors.New("page references a collection that does not exist")
	ErrPageTypeChangeForbidden   = errors.New("page type cannot be changed after creation")
)

const navigationLabelMaxLength = 60

type mutationDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Mutator interface {
	CreateSite(ctx context.Context, workspaceID string, input CreateSiteInput) (siteconfig.SiteDraft, error)
	UpdateSite(ctx context.Context, workspaceID string, siteID string, input UpdateSiteInput) (siteconfig.SiteDraft, error)
	CreatePage(ctx context.Context, workspaceID string, siteID string, input CreatePageInput) (siteconfig.SiteDraft, error)
	UpdatePage(ctx context.Context, workspaceID string, siteID string, pageID string, input UpdatePageInput) (siteconfig.SiteDraft, error)
	DeletePage(ctx context.Context, workspaceID string, siteID string, pageID string) (siteconfig.SiteDraft, error)
	ReorderPages(ctx context.Context, workspaceID string, siteID string, pageIDs []string) (siteconfig.SiteDraft, error)
	ReorderNavigation(ctx context.Context, workspaceID string, siteID string, pageIDs []string) (siteconfig.SiteDraft, error)
	UpdateNavigation(ctx context.Context, workspaceID string, siteID string, items []siteconfig.NavigationItem) (siteconfig.SiteDraft, error)
	CreateBlock(ctx context.Context, workspaceID string, siteID string, pageID string, input CreateBlockInput) (siteconfig.SiteDraft, error)
	UpdateBlock(ctx context.Context, workspaceID string, siteID string, pageID string, blockID string, input UpdateBlockInput) (siteconfig.SiteDraft, error)
	DeleteBlock(ctx context.Context, workspaceID string, siteID string, pageID string, blockID string) (siteconfig.SiteDraft, error)
	DuplicateBlock(ctx context.Context, workspaceID string, siteID string, pageID string, blockID string) (siteconfig.SiteDraft, error)
	ReorderBlocks(ctx context.Context, workspaceID string, siteID string, pageID string, blockIDs []string) (siteconfig.SiteDraft, error)
	DeleteSite(ctx context.Context, workspaceID string, siteID string) error
}

type CreateSiteInput struct {
	Name   string
	Slug   string
	Prompt string
}

type UpdateSiteInput struct {
	Name *string
	Slug *string
}

type CreatePageInput struct {
	Title               string
	Slug                string
	Type                string
	CollectionID        string
	IncludeInNavigation *bool
}

type UpdatePageInput struct {
	Title               *string
	Slug                *string
	Type                *string
	CollectionID        *string
	SEO                 *siteconfig.SEOConfig
	IncludeInNavigation *bool
}

type CreateBlockInput struct {
	Type    string
	Version string
}

type UpdateBlockInput struct {
	Props  map[string]any
	Hidden *bool
}

type PostgresMutator struct {
	db       mutationDB
	reader   Reader
	writer   Writer
	recorder *audit.Recorder
	logger   *slog.Logger
}

// MutatorOption customizes the PostgresMutator constructed by
// NewPostgresMutator.
type MutatorOption func(*PostgresMutator)

// WithAuditRecorder attaches an audit recorder so authoring-lifecycle events
// (site create, destructive page/block/site deletes) are written to
// audit_events.
func WithAuditRecorder(recorder *audit.Recorder) MutatorOption {
	return func(m *PostgresMutator) {
		m.recorder = recorder
	}
}

// WithLogger sets the structured logger used to report best-effort audit
// recording failures.
func WithLogger(logger *slog.Logger) MutatorOption {
	return func(m *PostgresMutator) {
		m.logger = logger
	}
}

func NewPostgresMutator(db DB, options ...MutatorOption) *PostgresMutator {
	mutator := &PostgresMutator{
		db:     db,
		reader: NewPostgresReader(db),
		writer: NewPostgresWriter(db),
	}
	for _, option := range options {
		if option != nil {
			option(mutator)
		}
	}
	return mutator
}

func (m *PostgresMutator) recordAudit(ctx context.Context, event audit.Event) {
	if m == nil || m.recorder == nil {
		return
	}
	if event.UserID == "" {
		if user, ok := auth.UserFromContext(ctx); ok {
			event.UserID = user.ID
		}
	}
	if err := m.recorder.Record(ctx, event); err != nil {
		if m.logger != nil {
			m.logger.Warn("record audit event",
				"action", event.Action,
				"siteId", event.SiteID,
				"workspaceId", event.WorkspaceID,
				"error", err.Error(),
			)
		}
	}
}

func (m *PostgresMutator) CreateSite(ctx context.Context, workspaceID string, input CreateSiteInput) (siteconfig.SiteDraft, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return siteconfig.SiteDraft{}, ErrSiteNameRequired
	}

	slugValue, err := m.createSlug(ctx, workspaceID, input.Slug, name)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	draft, err := starterDraft(name, slugValue, input.Prompt)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	if err := m.savePrompt(ctx, workspaceID, draft.Site.ID, input.Prompt); err != nil {
		return siteconfig.SiteDraft{}, err
	}

	savedDraft, err := m.reader.LoadDraft(ctx, draft.Site.ID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	m.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      savedDraft.Site.ID,
		Action:      "site.create",
		Metadata: map[string]any{
			"name":       savedDraft.Site.Name,
			"slug":       savedDraft.Site.Slug,
			"hasPrompt":  strings.TrimSpace(input.Prompt) != "",
		},
	})
	return savedDraft, nil
}

func (m *PostgresMutator) UpdateSite(ctx context.Context, workspaceID string, siteID string, input UpdateSiteInput) (siteconfig.SiteDraft, error) {
	if input.Name == nil && input.Slug == nil {
		return siteconfig.SiteDraft{}, ErrNoSiteChanges
	}

	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return siteconfig.SiteDraft{}, ErrSiteNameRequired
		}
		draft.Site.Name = name
	}

	if input.Slug != nil {
		slugValue := strings.TrimSpace(*input.Slug)
		if !slugs.IsValid(slugValue) {
			return siteconfig.SiteDraft{}, ErrSiteSlugInvalid
		}
		taken, err := m.siteSlugExists(ctx, workspaceID, slugValue, siteID)
		if err != nil {
			return siteconfig.SiteDraft{}, err
		}
		if taken {
			return siteconfig.SiteDraft{}, ErrSiteSlugConflict
		}
		draft.Site.Slug = slugValue
	}

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}

	savedDraft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return savedDraft, nil
}

func (m *PostgresMutator) CreatePage(ctx context.Context, workspaceID string, siteID string, input CreatePageInput) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	if len(draft.Pages) >= siteconfig.MaxPagesPerSite {
		return siteconfig.SiteDraft{}, ErrPageLimitReached
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return siteconfig.SiteDraft{}, ErrPageTitleRequired
	}

	pageID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate page id: %w", err)
	}
	slugValue, err := createPageSlug(input.Slug, title, draft.Pages, "")
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	pageType := strings.TrimSpace(input.Type)
	if pageType == "" {
		pageType = siteconfig.PageTypeStatic
	}
	switch pageType {
	case siteconfig.PageTypeStatic:
		if strings.TrimSpace(input.CollectionID) != "" {
			return siteconfig.SiteDraft{}, ErrPageCollectionUnsupported
		}
	case siteconfig.PageTypeCollectionIndex, siteconfig.PageTypeCollectionDetail:
		if strings.TrimSpace(input.CollectionID) == "" {
			return siteconfig.SiteDraft{}, ErrPageCollectionRequired
		}
		if !collectionExists(draft.Collections, input.CollectionID) {
			return siteconfig.SiteDraft{}, ErrPageCollectionNotFound
		}
	default:
		return siteconfig.SiteDraft{}, ErrPageTypeUnsupported
	}

	page := siteconfig.PageDraft{
		ID:    pageID,
		Title: title,
		Slug:  slugValue,
		SEO: siteconfig.SEOConfig{
			Title:       title,
			Description: draft.Site.SEO.Description,
		},
		Blocks:   []siteconfig.BlockInstance{},
		Settings: pageSettingsValue(input.IncludeInNavigation),
	}
	if pageType != siteconfig.PageTypeStatic {
		page.Type = pageType
		page.CollectionID = strings.TrimSpace(input.CollectionID)
	}
	draft.Pages = append(draft.Pages, page)
	draft.Navigation = syncNavigationWithPages(draft.Navigation, draft.Pages)

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) UpdatePage(ctx context.Context, workspaceID string, siteID string, pageID string, input UpdatePageInput) (siteconfig.SiteDraft, error) {
	if input.Title == nil && input.Slug == nil && input.SEO == nil && input.IncludeInNavigation == nil && input.Type == nil && input.CollectionID == nil {
		return siteconfig.SiteDraft{}, ErrNoPageChanges
	}

	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	pageIndex := findPageIndex(draft.Pages, pageID)
	if pageIndex == -1 {
		return siteconfig.SiteDraft{}, ErrPageNotFound
	}

	page := draft.Pages[pageIndex]
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return siteconfig.SiteDraft{}, ErrPageTitleRequired
		}
		page.Title = title
		if page.SEO.Title == "" {
			page.SEO.Title = title
		}
	}
	if input.Slug != nil {
		slugValue, err := createPageSlug(*input.Slug, page.Title, draft.Pages, pageID)
		if err != nil {
			return siteconfig.SiteDraft{}, err
		}
		if page.Slug == "/" && slugValue != "/" {
			return siteconfig.SiteDraft{}, ErrHomepageSlugLocked
		}
		page.Slug = slugValue
	}
	if input.SEO != nil {
		page.SEO = *input.SEO
	}
	if input.Type != nil {
		requestedType := strings.TrimSpace(*input.Type)
		if requestedType == "" {
			requestedType = siteconfig.PageTypeStatic
		}
		currentType := page.Type
		if currentType == "" {
			currentType = siteconfig.PageTypeStatic
		}
		if requestedType != currentType {
			return siteconfig.SiteDraft{}, ErrPageTypeChangeForbidden
		}
	}
	if input.CollectionID != nil {
		desired := strings.TrimSpace(*input.CollectionID)
		switch page.Type {
		case "", siteconfig.PageTypeStatic:
			if desired != "" {
				return siteconfig.SiteDraft{}, ErrPageCollectionUnsupported
			}
		case siteconfig.PageTypeCollectionIndex, siteconfig.PageTypeCollectionDetail:
			if desired == "" {
				return siteconfig.SiteDraft{}, ErrPageCollectionRequired
			}
			if !collectionExists(draft.Collections, desired) {
				return siteconfig.SiteDraft{}, ErrPageCollectionNotFound
			}
			page.CollectionID = desired
		}
	}
	page.Settings = pageSettingsValue(input.IncludeInNavigation, page.Settings)

	draft.Pages[pageIndex] = page
	draft.Navigation = syncNavigationWithPages(draft.Navigation, draft.Pages)
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func collectionExists(collections []siteconfig.Collection, id string) bool {
	for _, collection := range collections {
		if collection.ID == id {
			return true
		}
	}
	return false
}

func (m *PostgresMutator) DeletePage(ctx context.Context, workspaceID string, siteID string, pageID string) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	pageIndex := findPageIndex(draft.Pages, pageID)
	if pageIndex == -1 {
		return siteconfig.SiteDraft{}, ErrPageNotFound
	}
	if len(draft.Pages) == 1 {
		return siteconfig.SiteDraft{}, ErrMinimumPagesRequired
	}
	if draft.Pages[pageIndex].Slug == "/" {
		return siteconfig.SiteDraft{}, ErrHomepageDeleteForbidden
	}

	deletedPage := draft.Pages[pageIndex]
	draft.Pages = append(draft.Pages[:pageIndex], draft.Pages[pageIndex+1:]...)
	draft.Navigation = syncNavigationWithPages(draft.Navigation, draft.Pages)
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	m.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		Action:      "page.delete",
		Metadata: map[string]any{
			"pageId": deletedPage.ID,
			"title":  deletedPage.Title,
			"slug":   deletedPage.Slug,
		},
	})
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) ReorderPages(ctx context.Context, workspaceID string, siteID string, pageIDs []string) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	reorderedPages, err := reorderPages(draft.Pages, pageIDs)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	draft.Pages = reorderedPages
	draft.Navigation = syncNavigationWithPages(draft.Navigation, draft.Pages)

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) ReorderNavigation(ctx context.Context, workspaceID string, siteID string, pageIDs []string) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	navigation, err := reorderNavigation(draft.Navigation, draft.Pages, pageIDs)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	draft.Navigation = navigation

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) UpdateNavigation(ctx context.Context, workspaceID string, siteID string, items []siteconfig.NavigationItem) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	navigation, includedPageIDs, err := normalizeNavigationItems(items, draft.Pages)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	draft.Navigation = navigation
	draft.Pages = applyNavigationInclusion(draft.Pages, includedPageIDs)

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) DeleteSite(ctx context.Context, workspaceID string, siteID string) error {
	var siteName, siteSlug string
	if draft, err := m.reader.LoadDraft(ctx, siteID); err == nil {
		siteName = draft.Site.Name
		siteSlug = draft.Site.Slug
	}

	tag, err := m.db.Exec(ctx, `
		delete from sites
		where id = $1
		  and workspace_id = $2
	`, siteID, workspaceID)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	m.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		Action:      "site.delete",
		Metadata: map[string]any{
			"name": siteName,
			"slug": siteSlug,
		},
	})
	return nil
}

func (m *PostgresMutator) CreateBlock(ctx context.Context, workspaceID string, siteID string, pageID string, input CreateBlockInput) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	pageIndex := findPageIndex(draft.Pages, pageID)
	if pageIndex == -1 {
		return siteconfig.SiteDraft{}, ErrPageNotFound
	}

	blockType := strings.TrimSpace(input.Type)
	if blockType == "" {
		return siteconfig.SiteDraft{}, ErrBlockTypeRequired
	}
	version := strings.TrimSpace(input.Version)
	if version == "" {
		version = siteconfig.BlockVersionV1
	}
	definition, err := siteconfig.DefaultBlockRegistry().Lookup(blockType, version)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	blockID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate block id: %w", err)
	}

	draft.Pages[pageIndex].Blocks = append(draft.Pages[pageIndex].Blocks, siteconfig.BlockInstance{
		ID:      blockID,
		Type:    definition.Type,
		Version: definition.Version,
		Props:   deepCloneMap(definition.DefaultProps),
	})
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) UpdateBlock(ctx context.Context, workspaceID string, siteID string, pageID string, blockID string, input UpdateBlockInput) (siteconfig.SiteDraft, error) {
	if input.Props == nil && input.Hidden == nil {
		return siteconfig.SiteDraft{}, ErrNoBlockChanges
	}

	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	pageIndex := -1
	blockIndex := -1
	for index, page := range draft.Pages {
		if page.ID != pageID {
			continue
		}
		pageIndex = index
		for candidateIndex, block := range page.Blocks {
			if block.ID == blockID {
				blockIndex = candidateIndex
				break
			}
		}
		break
	}

	if pageIndex == -1 {
		return siteconfig.SiteDraft{}, ErrPageNotFound
	}
	if blockIndex == -1 {
		return siteconfig.SiteDraft{}, ErrBlockNotFound
	}

	block := draft.Pages[pageIndex].Blocks[blockIndex]
	if input.Props != nil {
		block.Props = input.Props
	}
	if input.Hidden != nil {
		block.Settings.Hidden = *input.Hidden
	}
	draft.Pages[pageIndex].Blocks[blockIndex] = block

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}

	savedDraft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return savedDraft, nil
}

func (m *PostgresMutator) DeleteBlock(ctx context.Context, workspaceID string, siteID string, pageID string, blockID string) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	pageIndex := findPageIndex(draft.Pages, pageID)
	if pageIndex == -1 {
		return siteconfig.SiteDraft{}, ErrPageNotFound
	}
	blockIndex := findBlockIndex(draft.Pages[pageIndex].Blocks, blockID)
	if blockIndex == -1 {
		return siteconfig.SiteDraft{}, ErrBlockNotFound
	}

	deletedBlock := draft.Pages[pageIndex].Blocks[blockIndex]
	draft.Pages[pageIndex].Blocks = append(
		draft.Pages[pageIndex].Blocks[:blockIndex],
		draft.Pages[pageIndex].Blocks[blockIndex+1:]...,
	)
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	m.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		Action:      "block.delete",
		Metadata: map[string]any{
			"pageId":  pageID,
			"blockId": deletedBlock.ID,
			"type":    deletedBlock.Type,
			"version": deletedBlock.Version,
		},
	})
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) DuplicateBlock(ctx context.Context, workspaceID string, siteID string, pageID string, blockID string) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	pageIndex := findPageIndex(draft.Pages, pageID)
	if pageIndex == -1 {
		return siteconfig.SiteDraft{}, ErrPageNotFound
	}
	blockIndex := findBlockIndex(draft.Pages[pageIndex].Blocks, blockID)
	if blockIndex == -1 {
		return siteconfig.SiteDraft{}, ErrBlockNotFound
	}
	newBlockID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate duplicated block id: %w", err)
	}

	block := draft.Pages[pageIndex].Blocks[blockIndex]
	duplicate := siteconfig.BlockInstance{
		ID:       newBlockID,
		Type:     block.Type,
		Version:  block.Version,
		Props:    deepCloneMap(block.Props),
		Settings: deepCloneBlockSettings(block.Settings),
	}
	blocks := draft.Pages[pageIndex].Blocks
	blocks = append(blocks[:blockIndex+1], append([]siteconfig.BlockInstance{duplicate}, blocks[blockIndex+1:]...)...)
	draft.Pages[pageIndex].Blocks = blocks

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) ReorderBlocks(ctx context.Context, workspaceID string, siteID string, pageID string, blockIDs []string) (siteconfig.SiteDraft, error) {
	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	pageIndex := findPageIndex(draft.Pages, pageID)
	if pageIndex == -1 {
		return siteconfig.SiteDraft{}, ErrPageNotFound
	}
	reorderedBlocks, err := reorderBlocks(draft.Pages[pageIndex].Blocks, blockIDs)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	draft.Pages[pageIndex].Blocks = reorderedBlocks

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return m.reader.LoadDraft(ctx, siteID)
}

func (m *PostgresMutator) createSlug(ctx context.Context, workspaceID string, requested string, name string) (string, error) {
	if value := strings.TrimSpace(requested); value != "" {
		if !slugs.IsValid(value) {
			return "", ErrSiteSlugInvalid
		}
		taken, err := m.siteSlugExists(ctx, workspaceID, value, "")
		if err != nil {
			return "", err
		}
		if taken {
			return "", ErrSiteSlugConflict
		}
		return value, nil
	}

	value, err := slugs.EnsureUnique(name, func(candidate string) (bool, error) {
		return m.siteSlugExists(ctx, workspaceID, candidate, "")
	})
	if err != nil {
		return "", fmt.Errorf("generate site slug: %w", err)
	}
	return value, nil
}

func (m *PostgresMutator) siteSlugExists(ctx context.Context, workspaceID string, slugValue string, excludeSiteID string) (bool, error) {
	var exists bool
	err := m.db.QueryRow(ctx, `
		select exists(
			select 1
			from sites
			where workspace_id = $1
			  and slug = $2
			  and ($3 = '' or id::text <> $3)
		)
	`, workspaceID, slugValue, excludeSiteID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check site slug: %w", err)
	}
	return exists, nil
}

func (m *PostgresMutator) savePrompt(ctx context.Context, workspaceID string, siteID string, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}

	tag, err := m.db.Exec(ctx, `
		update sites
		set generation_prompt = $1,
		    updated_at = now()
		where id = $2
		  and workspace_id = $3
	`, prompt, siteID, workspaceID)
	if err != nil {
		return fmt.Errorf("save site prompt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func createPageSlug(requested string, title string, pages []siteconfig.PageDraft, excludePageID string) (string, error) {
	value := strings.TrimSpace(requested)
	if value != "" {
		if !siteconfigPathValid(value) {
			return "", ErrPageSlugInvalid
		}
		if pageSlugExists(pages, value, excludePageID) {
			return "", ErrPageSlugConflict
		}
		return value, nil
	}

	basePath := slugs.PagePath(title)
	if !pageSlugExists(pages, basePath, excludePageID) {
		return basePath, nil
	}
	base := strings.TrimPrefix(basePath, "/")
	if base == "" {
		base = "home"
	}
	unique, err := slugs.EnsureUnique(base, func(candidate string) (bool, error) {
		return pageSlugExists(pages, "/"+candidate, excludePageID), nil
	})
	if err != nil {
		return "", fmt.Errorf("generate page slug: %w", err)
	}
	return "/" + unique, nil
}

func pageSlugExists(pages []siteconfig.PageDraft, slugValue string, excludePageID string) bool {
	for _, page := range pages {
		if page.ID == excludePageID {
			continue
		}
		if page.Slug == slugValue {
			return true
		}
	}
	return false
}

func siteconfigPathValid(value string) bool {
	return value == "/" || strings.HasPrefix(value, "/") && slugs.IsValid(strings.TrimPrefix(value, "/"))
}

func pageSettingsValue(includeInNavigation *bool, existing ...map[string]any) map[string]any {
	settings := map[string]any{}
	if len(existing) > 0 && existing[0] != nil {
		settings = deepCloneMap(existing[0])
	}
	if includeInNavigation != nil {
		settings["includeInNavigation"] = *includeInNavigation
	}
	return settings
}

func syncNavigationWithPages(
	existing siteconfig.NavigationConfig,
	pages []siteconfig.PageDraft,
) siteconfig.NavigationConfig {
	pageByID := make(map[string]siteconfig.PageDraft, len(pages))
	includedPageIDs := make(map[string]bool, len(pages))
	for _, page := range pages {
		pageByID[page.ID] = page
		if pageIncludedInNavigation(page.Settings) {
			includedPageIDs[page.ID] = true
		}
	}

	primary := make([]siteconfig.NavigationItem, 0, len(existing.Primary)+len(pages))
	seenPageIDs := make(map[string]bool, len(existing.Primary))
	for _, item := range existing.Primary {
		if item.PageID != "" {
			if !includedPageIDs[item.PageID] {
				continue
			}
			seenPageIDs[item.PageID] = true
			primary = append(primary, siteconfig.NavigationItem{
				Label:  item.Label,
				PageID: item.PageID,
			})
			continue
		}
		if item.Href != "" {
			primary = append(primary, siteconfig.NavigationItem{
				Label: item.Label,
				Href:  item.Href,
			})
		}
	}

	for _, page := range pages {
		if !includedPageIDs[page.ID] || seenPageIDs[page.ID] {
			continue
		}
		primary = append(primary, siteconfig.NavigationItem{
			Label:  page.Title,
			PageID: page.ID,
		})
	}

	return siteconfig.NavigationConfig{Primary: primary}
}

func findPageIndex(pages []siteconfig.PageDraft, pageID string) int {
	for index, page := range pages {
		if page.ID == pageID {
			return index
		}
	}
	return -1
}

func findBlockIndex(blocks []siteconfig.BlockInstance, blockID string) int {
	for index, block := range blocks {
		if block.ID == blockID {
			return index
		}
	}
	return -1
}

func reorderPages(pages []siteconfig.PageDraft, pageIDs []string) ([]siteconfig.PageDraft, error) {
	if len(pages) != len(pageIDs) {
		return nil, ErrPageOrderInvalid
	}
	byID := map[string]siteconfig.PageDraft{}
	for _, page := range pages {
		byID[page.ID] = page
	}
	reordered := make([]siteconfig.PageDraft, 0, len(pageIDs))
	seen := map[string]bool{}
	for _, pageID := range pageIDs {
		page, ok := byID[pageID]
		if !ok || seen[pageID] {
			return nil, ErrPageOrderInvalid
		}
		seen[pageID] = true
		reordered = append(reordered, page)
	}
	return reordered, nil
}

func reorderBlocks(blocks []siteconfig.BlockInstance, blockIDs []string) ([]siteconfig.BlockInstance, error) {
	if len(blocks) != len(blockIDs) {
		return nil, ErrBlockOrderInvalid
	}
	byID := map[string]siteconfig.BlockInstance{}
	for _, block := range blocks {
		byID[block.ID] = block
	}
	reordered := make([]siteconfig.BlockInstance, 0, len(blockIDs))
	seen := map[string]bool{}
	for _, blockID := range blockIDs {
		block, ok := byID[blockID]
		if !ok || seen[blockID] {
			return nil, ErrBlockOrderInvalid
		}
		seen[blockID] = true
		reordered = append(reordered, block)
	}
	return reordered, nil
}

func normalizeNavigationItems(
	items []siteconfig.NavigationItem,
	pages []siteconfig.PageDraft,
) (siteconfig.NavigationConfig, map[string]bool, error) {
	pageIDs := make(map[string]bool, len(pages))
	for _, page := range pages {
		pageIDs[page.ID] = true
	}

	normalized := make([]siteconfig.NavigationItem, 0, len(items))
	seenPageIDs := make(map[string]bool, len(items))
	includedPageIDs := make(map[string]bool, len(items))
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			return siteconfig.NavigationConfig{}, nil, ErrNavigationLabelRequired
		}
		if len(label) > navigationLabelMaxLength {
			return siteconfig.NavigationConfig{}, nil, ErrNavigationLabelTooLong
		}

		hasPageID := strings.TrimSpace(item.PageID) != ""
		hasHref := strings.TrimSpace(item.Href) != ""
		if hasPageID == hasHref {
			return siteconfig.NavigationConfig{}, nil, ErrNavigationItemInvalid
		}

		if hasPageID {
			pageID := strings.TrimSpace(item.PageID)
			if !pageIDs[pageID] {
				return siteconfig.NavigationConfig{}, nil, ErrNavigationPageUnknown
			}
			if seenPageIDs[pageID] {
				return siteconfig.NavigationConfig{}, nil, ErrNavigationItemInvalid
			}
			seenPageIDs[pageID] = true
			includedPageIDs[pageID] = true
			normalized = append(normalized, siteconfig.NavigationItem{
				Label:  label,
				PageID: pageID,
			})
			continue
		}

		href := strings.TrimSpace(item.Href)
		if err := siteconfig.ValidateURL(href); err != nil {
			return siteconfig.NavigationConfig{}, nil, ErrNavigationHrefInvalid
		}
		normalized = append(normalized, siteconfig.NavigationItem{
			Label: label,
			Href:  href,
		})
	}

	return siteconfig.NavigationConfig{Primary: normalized}, includedPageIDs, nil
}

func applyNavigationInclusion(pages []siteconfig.PageDraft, includedPageIDs map[string]bool) []siteconfig.PageDraft {
	updated := make([]siteconfig.PageDraft, len(pages))
	for index, page := range pages {
		include := includedPageIDs[page.ID]
		page.Settings = pageSettingsValue(&include, page.Settings)
		updated[index] = page
	}
	return updated
}

func reorderNavigation(
	existing siteconfig.NavigationConfig,
	pages []siteconfig.PageDraft,
	pageIDs []string,
) (siteconfig.NavigationConfig, error) {
	internalPages := make(map[string]siteconfig.PageDraft, len(pages))
	internalCount := 0
	for _, page := range pages {
		if !pageIncludedInNavigation(page.Settings) {
			continue
		}
		internalPages[page.ID] = page
		internalCount++
	}
	if len(pageIDs) != internalCount {
		return siteconfig.NavigationConfig{}, ErrNavigationOrderInvalid
	}

	existingLabels := make(map[string]string, len(existing.Primary))
	for _, item := range existing.Primary {
		if item.PageID != "" {
			existingLabels[item.PageID] = item.Label
		}
	}

	seen := make(map[string]bool, len(pageIDs))
	reordered := make([]siteconfig.NavigationItem, 0, len(existing.Primary))
	for _, pageID := range pageIDs {
		page, ok := internalPages[pageID]
		if !ok || seen[pageID] {
			return siteconfig.NavigationConfig{}, ErrNavigationOrderInvalid
		}
		seen[pageID] = true
		label := existingLabels[pageID]
		if label == "" {
			label = page.Title
		}
		reordered = append(reordered, siteconfig.NavigationItem{
			Label:  label,
			PageID: page.ID,
		})
	}

	for _, item := range existing.Primary {
		if item.PageID == "" && item.Href != "" {
			reordered = append(reordered, item)
		}
	}

	return siteconfig.NavigationConfig{Primary: reordered}, nil
}

func deepCloneMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return mapsClone(value)
	}
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return mapsClone(value)
	}
	return cloned
}

func mapsClone(value map[string]any) map[string]any {
	return maps.Clone(value)
}

func deepCloneBlockSettings(value siteconfig.BlockSettings) siteconfig.BlockSettings {
	return siteconfig.BlockSettings{
		Hidden:   value.Hidden,
		AnchorID: value.AnchorID,
	}
}

func starterDraft(name string, slugValue string, prompt string) (siteconfig.SiteDraft, error) {
	siteID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate site id: %w", err)
	}
	pageID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate page id: %w", err)
	}
	heroID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate hero block id: %w", err)
	}
	textID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate text block id: %w", err)
	}
	featuresID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate features block id: %w", err)
	}
	ctaID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate cta block id: %w", err)
	}

	subheadline := strings.TrimSpace(prompt)
	if subheadline == "" {
		subheadline = "Start from a structured draft with real pages, editable sections, and a preview route that stays on the same site contract."
	}
	if len(subheadline) > 280 {
		subheadline = subheadline[:277] + "..."
	}

	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            siteID,
			Name:          name,
			Slug:          slugValue,
			Status:        "draft",
			DefaultLocale: "en",
			SEO: siteconfig.SEOConfig{
				Title:       name,
				Description: subheadline,
			},
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens:  siteconfig.ThemePreset(siteconfig.ThemePaletteMeanerDark).Tokens,
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{
				{Label: "Home", PageID: pageID},
			},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    pageID,
				Title: "Home",
				Slug:  "/",
				SEO: siteconfig.SEOConfig{
					Title:       name,
					Description: subheadline,
				},
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      heroID,
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"eyebrow":     name,
							"headline":    "A welcoming first draft for " + name,
							"subheadline": subheadline,
							"primaryCta": map[string]any{
								"label": "Review the draft",
								"href":  "#next-step",
							},
							"layout": "split-left",
						},
					},
					{
						ID:      textID,
						Type:    "text_section",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading":   "What this starter gives you",
							"body":      "A single-page site scaffold with validated blocks, a saved draft, and room to tune the copy before generation and publishing are wired in.",
							"alignment": "left",
							"width":     "default",
						},
					},
					{
						ID:      featuresID,
						Type:    "features_grid",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading": "Ready for the next loop",
							"intro":   "The prototype keeps each section inside the maintained registry so preview and publish can stay consistent later.",
							"columns": 3,
							"items": []any{
								map[string]any{
									"title": "Structured draft",
									"body":  "Every page and block is validated application data.",
								},
								map[string]any{
									"title": "Builder-friendly",
									"body":  "Site metadata can be edited without breaking the stored draft.",
								},
								map[string]any{
									"title": "Preview-ready",
									"body":  "The React preview reads the same draft shape the API serves.",
								},
							},
						},
					},
					{
						ID:      ctaID,
						Type:    "cta_band",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading": "Next step",
							"body":    "Refine the name or slug now, then move into richer page and block editing.",
							"cta": map[string]any{
								"label": "Stay in the builder",
								"href":  "/app/sites/" + siteID,
							},
							"variant": "accent",
						},
						Settings: siteconfig.BlockSettings{
							AnchorID: "next-step",
						},
					},
				},
			},
		},
	}, nil
}
