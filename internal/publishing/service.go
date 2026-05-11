package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound         = errors.New("published site not found")
	ErrPageNotFound     = errors.New("published page not found")
	ErrHostnameConflict = errors.New("published hostname is already in use")
	ErrVersionNotFound  = errors.New("published version not found")
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type PublishInput struct {
	PublishNote string
}

type VersionSummary struct {
	ID            string    `json:"id"`
	SiteID        string    `json:"siteId"`
	VersionNumber int       `json:"versionNumber"`
	CreatedAt     time.Time `json:"createdAt"`
	PublishNote   string    `json:"publishNote,omitempty"`
	IsCurrent     bool      `json:"isCurrent"`
}

type PublishResult struct {
	Version  VersionSummary               `json:"version"`
	SiteSlug string                       `json:"siteSlug"`
	Hostname string                       `json:"hostname"`
	Snapshot siteconfig.PublishedSnapshot `json:"snapshot"`
}

type RollbackResult struct {
	Version  VersionSummary `json:"version"`
	SiteSlug string         `json:"siteSlug"`
	Hostname string         `json:"hostname"`
}

type PublishedSiteResult struct {
	SiteSlug string                       `json:"siteSlug"`
	Hostname string                       `json:"hostname,omitempty"`
	Version  VersionSummary               `json:"version"`
	PagePath string                       `json:"pagePath"`
	Page     siteconfig.PageDraft         `json:"page"`
	Snapshot siteconfig.PublishedSnapshot `json:"snapshot"`
}

type ServiceConfig struct {
	AppBaseURL   string
	ArtifactsDir string
	Renderer     ArtifactRenderer
	Store        ArtifactStore
	Cache        publishedSiteCache
}

type Service struct {
	db       DB
	reader   sites.Reader
	renderer ArtifactRenderer
	store    ArtifactStore
	cache    publishedSiteCache
}

type siteMetadata struct {
	WorkspaceID string
	SiteSlug    string
}

type publishedSiteLookup struct {
	SiteSlug string
	Hostname string
	Version  VersionSummary
}

func NewService(db DB, cfg ServiceConfig) *Service {
	renderer := cfg.Renderer
	if renderer == nil {
		renderer = newCommandArtifactRenderer(cfg.AppBaseURL)
	}
	store := cfg.Store
	if store == nil {
		store = newLocalArtifactStore(cfg.ArtifactsDir)
	}
	cache := cfg.Cache
	if cache == nil {
		cache = newMemoryPublishedSiteCache()
	}

	return &Service{
		db:       db,
		reader:   sites.NewPostgresReader(db),
		renderer: renderer,
		store:    store,
		cache:    cache,
	}
}

func (s *Service) Publish(ctx context.Context, siteID string, userID string, input PublishInput) (PublishResult, error) {
	draft, err := s.reader.LoadDraft(ctx, siteID)
	if errors.Is(err, sites.ErrNotFound) {
		return PublishResult{}, ErrNotFound
	}
	if err != nil {
		return PublishResult{}, fmt.Errorf("load draft for publish: %w", err)
	}

	snapshot := buildPublishedSnapshot(draft)
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		return PublishResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PublishResult{}, fmt.Errorf("begin publish transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	metadata, err := loadSiteMetadata(ctx, tx, siteID)
	if err != nil {
		return PublishResult{}, err
	}

	var nextVersion int
	if err := tx.QueryRow(ctx, `
		select coalesce(max(version_number), 0) + 1
		from site_versions
		where site_id = $1
	`, siteID).Scan(&nextVersion); err != nil {
		return PublishResult{}, fmt.Errorf("allocate next version: %w", err)
	}

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return PublishResult{}, fmt.Errorf("encode published snapshot: %w", err)
	}

	version := VersionSummary{
		SiteID:        siteID,
		VersionNumber: nextVersion,
		IsCurrent:     true,
		PublishNote:   strings.TrimSpace(input.PublishNote),
	}
	if err := tx.QueryRow(ctx, `
		insert into site_versions (site_id, version_number, snapshot, created_by, publish_note)
		values ($1, $2, $3, nullif($4, '')::uuid, nullif($5, ''))
		returning id::text, created_at
	`, siteID, nextVersion, snapshotJSON, userID, version.PublishNote).Scan(&version.ID, &version.CreatedAt); err != nil {
		return PublishResult{}, fmt.Errorf("insert published version: %w", err)
	}

	hostname, err := ensureSubdomain(ctx, tx, siteID, metadata.SiteSlug)
	if err != nil {
		return PublishResult{}, err
	}

	artifacts, err := s.renderer.Render(ctx, ArtifactRenderInput{
		SiteSlug: metadata.SiteSlug,
		Hostname: hostname,
		Version:  version,
		Snapshot: snapshot,
	})
	if err != nil {
		return PublishResult{}, err
	}
	if err := s.store.Save(ctx, siteID, version.ID, artifacts); err != nil {
		return PublishResult{}, fmt.Errorf("store published artifacts: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		update sites
		set published_version_id = $2::uuid,
		    updated_at = now()
		where id = $1
	`, siteID, version.ID)
	if err != nil {
		return PublishResult{}, fmt.Errorf("mark site published: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return PublishResult{}, ErrNotFound
	}

	if err := recordAuditEvent(ctx, tx, audit.Event{
		WorkspaceID: metadata.WorkspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "site.publish",
		Metadata: map[string]any{
			"siteSlug":      metadata.SiteSlug,
			"versionId":     version.ID,
			"versionNumber": version.VersionNumber,
			"publishNote":   version.PublishNote,
			"hostname":      hostname,
		},
	}); err != nil {
		return PublishResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PublishResult{}, fmt.Errorf("commit publish transaction: %w", err)
	}
	s.invalidatePublishedSiteCache(siteID)

	return PublishResult{
		Version:  version,
		SiteSlug: metadata.SiteSlug,
		Hostname: hostname,
		Snapshot: snapshot,
	}, nil
}

func (s *Service) Rollback(ctx context.Context, siteID string, versionID string, userID string) (RollbackResult, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RollbackResult{}, fmt.Errorf("begin rollback transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	metadata, err := loadSiteMetadata(ctx, tx, siteID)
	if err != nil {
		return RollbackResult{}, err
	}

	result := RollbackResult{
		SiteSlug: metadata.SiteSlug,
	}
	if err := tx.QueryRow(ctx, `
		select sv.id::text,
		       sv.site_id::text,
		       sv.version_number,
		       sv.created_at,
		       coalesce(sv.publish_note, ''),
		       coalesce((
		         select hostname
		         from site_domains
		         where site_id = sv.site_id
		           and type = 'subdomain'
		         order by created_at asc
		         limit 1
		       ), '')
		from site_versions sv
		where sv.site_id = $1
		  and sv.id = $2::uuid
	`, siteID, versionID).Scan(
		&result.Version.ID,
		&result.Version.SiteID,
		&result.Version.VersionNumber,
		&result.Version.CreatedAt,
		&result.Version.PublishNote,
		&result.Hostname,
	); errors.Is(err, pgx.ErrNoRows) {
		return RollbackResult{}, ErrVersionNotFound
	} else if err != nil {
		return RollbackResult{}, fmt.Errorf("load published version for rollback: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		update sites
		set published_version_id = $2::uuid,
		    updated_at = now()
		where id = $1
	`, siteID, versionID)
	if err != nil {
		return RollbackResult{}, fmt.Errorf("set live published version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return RollbackResult{}, ErrNotFound
	}

	result.Version.IsCurrent = true
	if err := recordAuditEvent(ctx, tx, audit.Event{
		WorkspaceID: metadata.WorkspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "site.rollback",
		Metadata: map[string]any{
			"siteSlug":      metadata.SiteSlug,
			"versionId":     result.Version.ID,
			"versionNumber": result.Version.VersionNumber,
			"publishNote":   result.Version.PublishNote,
			"hostname":      result.Hostname,
		},
	}); err != nil {
		return RollbackResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RollbackResult{}, fmt.Errorf("commit rollback transaction: %w", err)
	}
	s.invalidatePublishedSiteCache(siteID)

	return result, nil
}

func (s *Service) ListVersions(ctx context.Context, siteID string) ([]VersionSummary, error) {
	rows, err := s.db.Query(ctx, `
		select sv.id::text,
		       sv.site_id::text,
		       sv.version_number,
		       sv.created_at,
		       coalesce(sv.publish_note, ''),
		       sv.id = s.published_version_id as is_current
		from site_versions sv
		join sites s on s.id = sv.site_id
		where sv.site_id = $1
		order by sv.version_number desc
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list published versions: %w", err)
	}
	defer rows.Close()

	versions := []VersionSummary{}
	for rows.Next() {
		var version VersionSummary
		if err := rows.Scan(
			&version.ID,
			&version.SiteID,
			&version.VersionNumber,
			&version.CreatedAt,
			&version.PublishNote,
			&version.IsCurrent,
		); err != nil {
			return nil, fmt.Errorf("scan published version: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published versions: %w", err)
	}
	return versions, nil
}

func (s *Service) LoadPublishedSiteBySlug(ctx context.Context, siteSlug string, pagePath string) (PublishedSiteResult, error) {
	lookup, snapshot, err := s.loadPublishedSiteLookup(ctx, `
		select s.slug,
		       coalesce((
		         select hostname
		         from site_domains
		         where site_id = s.id
		           and type = 'subdomain'
		         order by created_at asc
		         limit 1
		       ), ''),
		       sv.id::text,
		       sv.site_id::text,
		       sv.version_number,
		       sv.created_at,
		       coalesce(sv.publish_note, ''),
		       sv.snapshot
		from sites s
		join site_versions sv on sv.id = s.published_version_id
		where s.slug = $1
		order by s.updated_at desc
		limit 1
	`, strings.TrimSpace(siteSlug))
	if err != nil {
		return PublishedSiteResult{}, err
	}
	s.storePublishedLookupCaches(lookup, snapshot)
	return s.resolvePublishedSiteResult(ctx, lookup, &snapshot, pagePath)
}

func (s *Service) LoadPublishedSiteByHostname(ctx context.Context, hostname string, pagePath string) (PublishedSiteResult, error) {
	normalizedHostname := normalizeHostname(hostname)
	if lookup, ok := s.loadCachedDomainLookup(normalizedHostname); ok {
		return s.resolvePublishedSiteResult(ctx, lookup, nil, pagePath)
	}

	lookup, snapshot, err := s.loadPublishedSiteLookup(ctx, `
		select s.slug,
		       d.hostname,
		       sv.id::text,
		       sv.site_id::text,
		       sv.version_number,
		       sv.created_at,
		       coalesce(sv.publish_note, ''),
		       sv.snapshot
		from site_domains d
		join sites s on s.id = d.site_id
		join site_versions sv on sv.id = s.published_version_id
		where lower(d.hostname) = lower($1)
		  and d.status = 'active'
		order by d.updated_at desc, d.created_at desc
		limit 1
	`, normalizedHostname)
	if err != nil {
		return PublishedSiteResult{}, err
	}
	s.storePublishedLookupCaches(lookup, snapshot)
	return s.resolvePublishedSiteResult(ctx, lookup, &snapshot, pagePath)
}

func (s *Service) loadPublishedSiteLookup(ctx context.Context, query string, lookup string) (publishedSiteLookup, siteconfig.PublishedSnapshot, error) {
	var result publishedSiteLookup
	var snapshotJSON []byte
	err := s.db.QueryRow(ctx, query, lookup).Scan(
		&result.SiteSlug,
		&result.Hostname,
		&result.Version.ID,
		&result.Version.SiteID,
		&result.Version.VersionNumber,
		&result.Version.CreatedAt,
		&result.Version.PublishNote,
		&snapshotJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return publishedSiteLookup{}, siteconfig.PublishedSnapshot{}, ErrNotFound
	}
	if err != nil {
		return publishedSiteLookup{}, siteconfig.PublishedSnapshot{}, fmt.Errorf("load published site: %w", err)
	}

	result.Version.IsCurrent = true
	snapshot, err := decodePublishedSnapshot(snapshotJSON)
	if err != nil {
		return publishedSiteLookup{}, siteconfig.PublishedSnapshot{}, err
	}
	return result, snapshot, nil
}

func (s *Service) resolvePublishedSiteResult(ctx context.Context, lookup publishedSiteLookup, initialSnapshot *siteconfig.PublishedSnapshot, pagePath string) (PublishedSiteResult, error) {
	snapshot, err := s.loadPublishedSnapshot(ctx, lookup, initialSnapshot)
	if err != nil {
		return PublishedSiteResult{}, err
	}

	result := PublishedSiteResult{
		SiteSlug: lookup.SiteSlug,
		Hostname: lookup.Hostname,
		Version:  lookup.Version,
		Snapshot: snapshot,
	}

	normalizedPath := normalizePublishedPagePath(pagePath)
	if page, ok := s.loadCachedPage(lookup.Version.SiteID, lookup.Version.ID, normalizedPath); ok {
		result.PagePath = normalizedPath
		result.Page = page
		return result, nil
	}

	page, err := resolvePublishedPage(snapshot, normalizedPath)
	if err != nil {
		return PublishedSiteResult{}, err
	}
	result.PagePath = normalizedPath
	result.Page = page
	s.storeCachedPage(lookup.Version.SiteID, lookup.Version.ID, normalizedPath, page)
	return result, nil
}

func (s *Service) loadPublishedSnapshot(ctx context.Context, lookup publishedSiteLookup, initialSnapshot *siteconfig.PublishedSnapshot) (siteconfig.PublishedSnapshot, error) {
	if snapshot, ok := s.loadCachedSnapshot(lookup.Version.SiteID, lookup.Version.ID); ok {
		return snapshot, nil
	}
	if initialSnapshot != nil {
		s.storeCachedSnapshot(lookup.Version.SiteID, lookup.Version.ID, *initialSnapshot)
		return *initialSnapshot, nil
	}

	var snapshotJSON []byte
	if err := s.db.QueryRow(ctx, `
		select snapshot
		from site_versions
		where site_id = $1::uuid
		  and id = $2::uuid
	`, lookup.Version.SiteID, lookup.Version.ID).Scan(&snapshotJSON); errors.Is(err, pgx.ErrNoRows) {
		return siteconfig.PublishedSnapshot{}, ErrNotFound
	} else if err != nil {
		return siteconfig.PublishedSnapshot{}, fmt.Errorf("load published snapshot: %w", err)
	}

	snapshot, err := decodePublishedSnapshot(snapshotJSON)
	if err != nil {
		return siteconfig.PublishedSnapshot{}, err
	}
	s.storeCachedSnapshot(lookup.Version.SiteID, lookup.Version.ID, snapshot)
	return snapshot, nil
}

func decodePublishedSnapshot(snapshotJSON []byte) (siteconfig.PublishedSnapshot, error) {
	var snapshot siteconfig.PublishedSnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return siteconfig.PublishedSnapshot{}, fmt.Errorf("decode published snapshot: %w", err)
	}
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		return siteconfig.PublishedSnapshot{}, fmt.Errorf("published snapshot is invalid: %w", err)
	}
	return snapshot, nil
}

func (s *Service) loadCachedDomainLookup(hostname string) (publishedSiteLookup, bool) {
	if s.cache == nil {
		return publishedSiteLookup{}, false
	}
	return s.cache.LoadDomain(hostname)
}

func (s *Service) loadCachedSnapshot(siteID string, versionID string) (siteconfig.PublishedSnapshot, bool) {
	if s.cache == nil {
		return siteconfig.PublishedSnapshot{}, false
	}
	return s.cache.LoadSnapshot(siteID, versionID)
}

func (s *Service) loadCachedPage(siteID string, versionID string, pagePath string) (siteconfig.PageDraft, bool) {
	if s.cache == nil {
		return siteconfig.PageDraft{}, false
	}
	return s.cache.LoadPage(siteID, versionID, pagePath)
}

func (s *Service) storePublishedLookupCaches(lookup publishedSiteLookup, snapshot siteconfig.PublishedSnapshot) {
	if lookup.Hostname != "" {
		s.storeCachedDomainLookup(lookup.Hostname, lookup)
	}
	s.storeCachedSnapshot(lookup.Version.SiteID, lookup.Version.ID, snapshot)
}

func (s *Service) storeCachedDomainLookup(hostname string, lookup publishedSiteLookup) {
	if s.cache == nil {
		return
	}
	s.cache.StoreDomain(hostname, lookup)
}

func (s *Service) storeCachedSnapshot(siteID string, versionID string, snapshot siteconfig.PublishedSnapshot) {
	if s.cache == nil {
		return
	}
	s.cache.StoreSnapshot(siteID, versionID, snapshot)
}

func (s *Service) storeCachedPage(siteID string, versionID string, pagePath string, page siteconfig.PageDraft) {
	if s.cache == nil {
		return
	}
	s.cache.StorePage(siteID, versionID, pagePath, page)
}

func (s *Service) invalidatePublishedSiteCache(siteID string) {
	if s.cache == nil {
		return
	}
	s.cache.InvalidateSite(siteID)
}

func normalizeHostname(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	} else if strings.Count(value, ":") == 1 {
		host, port, found := strings.Cut(value, ":")
		if found && host != "" && port != "" {
			value = host
		}
	}
	return strings.TrimSuffix(value, ".")
}

func normalizePublishedPagePath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "/" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	value = strings.ReplaceAll(value, "//", "/")
	value = strings.TrimRight(value, "/")
	if value == "" {
		return "/"
	}
	return value
}

func resolvePublishedPage(snapshot siteconfig.PublishedSnapshot, pagePath string) (siteconfig.PageDraft, error) {
	for _, page := range snapshot.Pages {
		if page.Slug == pagePath {
			return page, nil
		}
	}
	return siteconfig.PageDraft{}, ErrPageNotFound
}

func loadSiteMetadata(ctx context.Context, rower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, siteID string) (siteMetadata, error) {
	var metadata siteMetadata
	err := rower.QueryRow(ctx, `
		select workspace_id::text, slug
		from sites
		where id = $1
	`, siteID).Scan(&metadata.WorkspaceID, &metadata.SiteSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return siteMetadata{}, ErrNotFound
	}
	if err != nil {
		return siteMetadata{}, fmt.Errorf("load site metadata: %w", err)
	}
	return metadata, nil
}

func recordAuditEvent(ctx context.Context, store audit.Store, event audit.Event) error {
	if err := audit.NewRecorder(store).Record(ctx, event); err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

func ensureSubdomain(ctx context.Context, tx pgx.Tx, siteID string, siteSlug string) (string, error) {
	hostname := siteSlug + ".localhost"

	var domainID string
	var currentHostname string
	err := tx.QueryRow(ctx, `
		select id::text, hostname
		from site_domains
		where site_id = $1
		  and type = 'subdomain'
		order by created_at asc
		limit 1
	`, siteID).Scan(&domainID, &currentHostname)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := tx.Exec(ctx, `
			insert into site_domains (site_id, hostname, type, status)
			values ($1, $2, 'subdomain', 'active')
		`, siteID, hostname); err != nil {
			if isUniqueViolation(err) {
				return "", ErrHostnameConflict
			}
			return "", fmt.Errorf("create subdomain record: %w", err)
		}
	case err != nil:
		return "", fmt.Errorf("load existing subdomain record: %w", err)
	case currentHostname != hostname:
		if _, err := tx.Exec(ctx, `
			update site_domains
			set hostname = $2,
			    status = 'active',
			    updated_at = now()
			where id = $1
		`, domainID, hostname); err != nil {
			if isUniqueViolation(err) {
				return "", ErrHostnameConflict
			}
			return "", fmt.Errorf("update subdomain record: %w", err)
		}
	default:
		if _, err := tx.Exec(ctx, `
			update site_domains
			set status = 'active',
			    updated_at = now()
			where id = $1
		`, domainID); err != nil {
			return "", fmt.Errorf("refresh subdomain record: %w", err)
		}
	}

	return hostname, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func buildPublishedSnapshot(draft siteconfig.SiteDraft) siteconfig.PublishedSnapshot {
	siteDescription := draft.Site.SEO.Description
	if siteDescription == "" {
		siteDescription = firstNonEmpty(
			pageDescription(firstPageBySlug(draft.Pages, "/")),
			firstNonEmptyPageDescription(draft.Pages),
			"Discover "+draft.Site.Name+".",
		)
	}

	siteSEO := siteconfig.SEOConfig{
		Title:       clampText(firstNonEmpty(draft.Site.SEO.Title, draft.Site.Name), 70),
		Description: clampText(siteDescription, 180),
	}

	pages := make([]siteconfig.PageDraft, 0, len(draft.Pages))
	for _, page := range draft.Pages {
		pageSEO := page.SEO
		if pageSEO.Title == "" {
			if page.Slug == "/" {
				pageSEO.Title = draft.Site.Name
			} else {
				pageSEO.Title = strings.TrimSpace(page.Title + " | " + draft.Site.Name)
			}
		}
		if pageSEO.Description == "" {
			pageSEO.Description = firstNonEmpty(pageDescription(page), siteSEO.Description)
		}
		pageSEO.Title = clampText(pageSEO.Title, 70)
		pageSEO.Description = clampText(pageSEO.Description, 180)

		pages = append(pages, siteconfig.PageDraft{
			ID:       page.ID,
			Title:    page.Title,
			Slug:     page.Slug,
			SEO:      pageSEO,
			Blocks:   page.Blocks,
			Settings: page.Settings,
		})
	}

	defaultLocale := draft.Site.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	return siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site: siteconfig.PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: defaultLocale,
			SEO:           siteSEO,
		},
		Theme:      draft.Theme,
		Navigation: draft.Navigation,
		Pages:      pages,
	}
}

func firstPageBySlug(pages []siteconfig.PageDraft, slug string) siteconfig.PageDraft {
	for _, page := range pages {
		if page.Slug == slug {
			return page
		}
	}
	if len(pages) == 0 {
		return siteconfig.PageDraft{}
	}
	return pages[0]
}

func firstNonEmptyPageDescription(pages []siteconfig.PageDraft) string {
	for _, page := range pages {
		if description := pageDescription(page); description != "" {
			return description
		}
	}
	return ""
}

func pageDescription(page siteconfig.PageDraft) string {
	if page.SEO.Description != "" {
		return strings.TrimSpace(page.SEO.Description)
	}
	for _, block := range page.Blocks {
		switch block.Type {
		case "hero":
			if value := asString(block.Props["subheadline"]); value != "" {
				return value
			}
			if value := asString(block.Props["headline"]); value != "" {
				return value
			}
		case "text_section", "image_text", "cta_band":
			if value := asString(block.Props["body"]); value != "" {
				return value
			}
		case "features_grid":
			if value := asString(block.Props["intro"]); value != "" {
				return value
			}
			for _, item := range asSlice(block.Props["items"]) {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if value := asString(itemMap["body"]); value != "" {
					return value
				}
			}
		}
	}
	return ""
}

func asString(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	return normalizeWhitespace(raw)
}

func asSlice(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = normalizeWhitespace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func clampText(value string, limit int) string {
	value = normalizeWhitespace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit]))
}
