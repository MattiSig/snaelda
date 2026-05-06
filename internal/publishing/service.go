package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound         = errors.New("published site not found")
	ErrHostnameConflict = errors.New("published hostname is already in use")
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

type PublishedSiteResult struct {
	SiteSlug string                       `json:"siteSlug"`
	Hostname string                       `json:"hostname,omitempty"`
	Version  VersionSummary               `json:"version"`
	Snapshot siteconfig.PublishedSnapshot `json:"snapshot"`
}

type Service struct {
	db     DB
	reader sites.Reader
}

func NewService(db DB) *Service {
	return &Service{
		db:     db,
		reader: sites.NewPostgresReader(db),
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

	hostname, err := ensureSubdomain(ctx, tx, siteID, draft.Site.Slug)
	if err != nil {
		return PublishResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PublishResult{}, fmt.Errorf("commit publish transaction: %w", err)
	}

	return PublishResult{
		Version:  version,
		SiteSlug: draft.Site.Slug,
		Hostname: hostname,
		Snapshot: snapshot,
	}, nil
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

func (s *Service) LoadPublishedSiteBySlug(ctx context.Context, siteSlug string) (PublishedSiteResult, error) {
	var result PublishedSiteResult
	var snapshotJSON []byte
	err := s.db.QueryRow(ctx, `
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
	`, strings.TrimSpace(siteSlug)).Scan(
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
		return PublishedSiteResult{}, ErrNotFound
	}
	if err != nil {
		return PublishedSiteResult{}, fmt.Errorf("load published site: %w", err)
	}

	result.Version.IsCurrent = true
	if err := json.Unmarshal(snapshotJSON, &result.Snapshot); err != nil {
		return PublishedSiteResult{}, fmt.Errorf("decode published snapshot: %w", err)
	}
	if err := siteconfig.ValidatePublishedSnapshot(result.Snapshot); err != nil {
		return PublishedSiteResult{}, fmt.Errorf("published snapshot is invalid: %w", err)
	}
	return result, nil
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
