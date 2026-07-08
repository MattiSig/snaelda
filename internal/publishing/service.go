package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
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
	SiteSlug      string                `json:"siteSlug"`
	Hostname      string                `json:"hostname,omitempty"`
	DefaultLocale string                `json:"defaultLocale,omitempty"`
	Version       VersionSummary        `json:"version"`
	PagePath      string                `json:"pagePath"`
	Page          PublishedPageArtifact `json:"page"`
}

type PublishedPageArtifact struct {
	PageID              string         `json:"pageId,omitempty"`
	PagePath            string         `json:"pagePath"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	CanonicalURL        string         `json:"canonicalUrl"`
	OGImageURL          string         `json:"ogImageUrl,omitempty"`
	LocalBusinessJSONLD map[string]any `json:"localBusinessJsonLd,omitempty"`
	HTML                string         `json:"html"`
	// DefaultLocale carries the published site's content locale through the
	// page cache so the API response can drive `<html lang>`/`og:locale`; it is
	// promoted to PublishedSiteResult.DefaultLocale and not serialized here.
	DefaultLocale string `json:"-"`
}

type PublishedArtifactResult struct {
	SiteSlug string         `json:"siteSlug"`
	Hostname string         `json:"hostname,omitempty"`
	Version  VersionSummary `json:"version"`
	File     ArtifactFile   `json:"file"`
}

type ServiceConfig struct {
	AppBaseURL       string
	PublicBaseURL    string
	APIBaseURL       string
	PublicBaseDomain string
	ArtifactsDir     string
	Renderer         ArtifactRenderer
	Store            ArtifactStore
	Cache            PublishedSiteCache
	AssetProvenance  AssetProvenanceLookup
	CDNPurger        CDNPurger
	Logger           *slog.Logger
}

// CDNPurger releases cached responses at the edge after a publish, rollback,
// or domain change. The default implementation is nil (no CDN); production
// deployments inject a Fastly/Cloudflare/etc. backed implementation here.
type CDNPurger interface {
	PurgeSite(ctx context.Context, siteID string, hostname string) error
}

// AssetProvenanceLookup retrieves provenance metadata (provider, author,
// source url, etc.) for assets referenced in the published snapshot so the
// publisher can attach attribution credits at publish time.
type AssetProvenanceLookup interface {
	LookupCredits(ctx context.Context, assetIDs []string) ([]siteconfig.ImageCredit, error)
}

type Service struct {
	db               DB
	reader           sites.Reader
	renderer         ArtifactRenderer
	store            ArtifactStore
	cache            PublishedSiteCache
	publicBaseURL    string
	publicBaseDomain string
	assetProvenance  AssetProvenanceLookup
	cdnPurger        CDNPurger
	logger           *slog.Logger
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
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	renderer := cfg.Renderer
	if renderer == nil {
		renderer = NewWorkerArtifactRenderer(WorkerRendererConfig{
			PublicBaseURL: cfg.PublicBaseURL,
			APIBaseURL:    cfg.APIBaseURL,
			Logger:        logger,
		})
	}
	store := cfg.Store
	if store == nil {
		store = newLocalArtifactStore(cfg.ArtifactsDir)
	}
	cache := cfg.Cache
	if cache == nil {
		cache = NewPublishedSiteCache()
	}

	return &Service{
		db:               db,
		reader:           sites.NewPostgresReader(db),
		renderer:         renderer,
		store:            store,
		cache:            cache,
		publicBaseURL:    strings.TrimSpace(cfg.PublicBaseURL),
		publicBaseDomain: normalizeHostname(cfg.PublicBaseDomain),
		assetProvenance:  cfg.AssetProvenance,
		cdnPurger:        cfg.CDNPurger,
		logger:           logger,
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
	if err := s.enrichSnapshotCredits(ctx, &snapshot); err != nil {
		return PublishResult{}, err
	}
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

	hostname, err := ensureSubdomain(ctx, tx, siteID, metadata.SiteSlug, s.publicBaseDomain)
	if err != nil {
		return PublishResult{}, err
	}

	artifacts, err := s.renderer.Render(ctx, ArtifactRenderInput{
		PublicBaseURL: s.publicBaseURL,
		SiteSlug:      metadata.SiteSlug,
		Hostname:      hostname,
		Version:       version,
		Snapshot:      snapshot,
	})
	if err != nil {
		return PublishResult{}, err
	}
	if err := validateArtifactBundle(artifacts, snapshot, metadata.SiteSlug, hostname, version); err != nil {
		return PublishResult{}, err
	}
	if err := s.store.Save(ctx, siteID, version.ID, artifacts); err != nil {
		return PublishResult{}, fmt.Errorf("store published artifacts: %w", err)
	}
	// If anything after this point fails before commit succeeds, the artifacts
	// would be orphans referenced by no row in site_versions. Clean them up.
	publishCommitted := false
	defer func() {
		if publishCommitted {
			return
		}
		if err := s.store.Delete(context.Background(), siteID, version.ID); err != nil {
			s.logger.Warn("clean up orphan published artifacts after failed publish", "siteId", siteID, "versionId", version.ID, "error", err.Error())
		}
	}()

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
	publishCommitted = true
	s.invalidatePublishedSiteCache(siteID)
	s.purgeCDN(ctx, siteID, hostname)

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

	// Refuse to promote a version whose rendered artifacts are missing —
	// without them the public site would serve 404s on every page.
	if s.store != nil {
		if _, err := s.store.Load(ctx, siteID, versionID, "manifest.json"); err != nil {
			if errors.Is(err, ErrArtifactNotFound) {
				return RollbackResult{}, ErrVersionNotFound
			}
			return RollbackResult{}, fmt.Errorf("verify rollback artifacts: %w", err)
		}
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
	s.purgeCDN(ctx, siteID, result.Hostname)

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
	lookup, err := s.loadPublishedSiteLookup(ctx, `
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
		       coalesce(sv.publish_note, '')
		from sites s
		join site_versions sv on sv.id = s.published_version_id
		where s.slug = $1
		order by s.updated_at desc
		limit 1
	`, strings.TrimSpace(siteSlug))
	if err != nil {
		return PublishedSiteResult{}, err
	}
	s.storePublishedLookupCaches(lookup)
	return s.resolvePublishedSiteResult(ctx, lookup, pagePath)
}

func (s *Service) LoadPublishedSiteByHostname(ctx context.Context, hostname string, pagePath string) (PublishedSiteResult, error) {
	normalizedHostname := normalizeHostname(hostname)
	if lookup, ok := s.loadCachedDomainLookup(normalizedHostname); ok {
		return s.resolvePublishedSiteResult(ctx, lookup, pagePath)
	}

	lookup, err := s.loadPublishedSiteLookup(ctx, `
		select s.slug,
		       d.hostname,
		       sv.id::text,
		       sv.site_id::text,
		       sv.version_number,
		       sv.created_at,
		       coalesce(sv.publish_note, '')
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
	s.storePublishedLookupCaches(lookup)
	return s.resolvePublishedSiteResult(ctx, lookup, pagePath)
}

func (s *Service) LoadPublishedArtifactBySlug(ctx context.Context, siteSlug string, artifactPath string) (PublishedArtifactResult, error) {
	lookup, err := s.loadPublishedSiteLookup(ctx, `
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
		       coalesce(sv.publish_note, '')
		from sites s
		join site_versions sv on sv.id = s.published_version_id
		where s.slug = $1
		order by s.updated_at desc
		limit 1
	`, strings.TrimSpace(siteSlug))
	if err != nil {
		return PublishedArtifactResult{}, err
	}
	s.storePublishedLookupCaches(lookup)
	return s.loadPublishedArtifact(ctx, lookup, artifactPath)
}

func (s *Service) LoadPublishedArtifactByHostname(ctx context.Context, hostname string, artifactPath string) (PublishedArtifactResult, error) {
	normalizedHostname := normalizeHostname(hostname)
	if lookup, ok := s.loadCachedDomainLookup(normalizedHostname); ok {
		return s.loadPublishedArtifact(ctx, lookup, artifactPath)
	}

	lookup, err := s.loadPublishedSiteLookup(ctx, `
		select s.slug,
		       d.hostname,
		       sv.id::text,
		       sv.site_id::text,
		       sv.version_number,
		       sv.created_at,
		       coalesce(sv.publish_note, '')
		from site_domains d
		join sites s on s.id = d.site_id
		join site_versions sv on sv.id = s.published_version_id
		where lower(d.hostname) = lower($1)
		  and d.status = 'active'
		order by d.updated_at desc, d.created_at desc
		limit 1
	`, normalizedHostname)
	if err != nil {
		return PublishedArtifactResult{}, err
	}
	s.storePublishedLookupCaches(lookup)
	return s.loadPublishedArtifact(ctx, lookup, artifactPath)
}

func (s *Service) loadPublishedSiteLookup(ctx context.Context, query string, lookup string) (publishedSiteLookup, error) {
	var result publishedSiteLookup
	err := s.db.QueryRow(ctx, query, lookup).Scan(
		&result.SiteSlug,
		&result.Hostname,
		&result.Version.ID,
		&result.Version.SiteID,
		&result.Version.VersionNumber,
		&result.Version.CreatedAt,
		&result.Version.PublishNote,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return publishedSiteLookup{}, ErrNotFound
	}
	if err != nil {
		return publishedSiteLookup{}, fmt.Errorf("load published site: %w", err)
	}

	result.Version.IsCurrent = true
	return result, nil
}

func (s *Service) resolvePublishedSiteResult(ctx context.Context, lookup publishedSiteLookup, pagePath string) (PublishedSiteResult, error) {
	result := PublishedSiteResult{
		SiteSlug: lookup.SiteSlug,
		Hostname: lookup.Hostname,
		Version:  lookup.Version,
	}

	normalizedPath := normalizePublishedPagePath(pagePath)
	if page, ok := s.loadCachedPage(lookup.Version.SiteID, lookup.Version.ID, normalizedPath); ok {
		result.PagePath = normalizedPath
		result.DefaultLocale = page.DefaultLocale
		result.Page = page
		return result, nil
	}

	page, err := s.loadPublishedArtifactPage(ctx, lookup, normalizedPath)
	if err != nil {
		return PublishedSiteResult{}, err
	}
	result.PagePath = normalizedPath
	result.DefaultLocale = page.DefaultLocale
	result.Page = page
	s.storeCachedPage(lookup.Version.SiteID, lookup.Version.ID, normalizedPath, page)
	return result, nil
}

func (s *Service) loadCachedDomainLookup(hostname string) (publishedSiteLookup, bool) {
	if s.cache == nil {
		return publishedSiteLookup{}, false
	}
	return s.cache.LoadDomain(hostname)
}

func (s *Service) loadCachedPage(siteID string, versionID string, pagePath string) (PublishedPageArtifact, bool) {
	if s.cache == nil {
		return PublishedPageArtifact{}, false
	}
	return s.cache.LoadPage(siteID, versionID, pagePath)
}

func (s *Service) storePublishedLookupCaches(lookup publishedSiteLookup) {
	if lookup.Hostname != "" {
		s.storeCachedDomainLookup(lookup.Hostname, lookup)
	}
}

func (s *Service) storeCachedDomainLookup(hostname string, lookup publishedSiteLookup) {
	if s.cache == nil {
		return
	}
	s.cache.StoreDomain(hostname, lookup)
}

func (s *Service) storeCachedPage(siteID string, versionID string, pagePath string, page PublishedPageArtifact) {
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

// InvalidateSite drops the public render cache for a site. External callers
// (e.g., the domains write API, slug changes, billing entitlement flips)
// should call this when a site's public addressability changes.
func (s *Service) InvalidateSite(ctx context.Context, siteID string) {
	if s == nil {
		return
	}
	s.invalidatePublishedSiteCache(siteID)
	s.purgeCDN(ctx, siteID, "")
}

// InvalidateHostname drops the public render cache for a single hostname.
// Use this when a custom domain is activated, deactivated, verified, or
// renamed so cached lookups do not point at stale site/version data.
func (s *Service) InvalidateHostname(ctx context.Context, hostname string) {
	if s == nil {
		return
	}
	if s.cache != nil {
		s.cache.InvalidateHostname(hostname)
	}
	s.purgeCDN(ctx, "", hostname)
}

// purgeCDN releases the public cache for a site at the edge. It is a best
// effort signal — failures are logged, not returned, because the public cache
// will eventually catch up via ETag revalidation regardless.
func (s *Service) purgeCDN(ctx context.Context, siteID string, hostname string) {
	if s == nil || s.cdnPurger == nil {
		return
	}
	if err := s.cdnPurger.PurgeSite(ctx, siteID, hostname); err != nil {
		logger := s.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("cdn purge failed", "siteId", siteID, "hostname", hostname, "error", err.Error())
	}
}

func validateArtifactBundle(bundle ArtifactBundle, snapshot siteconfig.PublishedSnapshot, siteSlug string, hostname string, version VersionSummary) error {
	if strings.TrimSpace(bundle.SchemaVersion) != "published_artifacts.v1" {
		return siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path:    "publish.artifacts.schemaVersion",
			Code:    "invalid_artifact_bundle",
			Message: "publish artifact bundle must use schema version published_artifacts.v1",
		}}}
	}

	filesByPath := make(map[string]ArtifactFile, len(bundle.Files))
	for _, file := range bundle.Files {
		cleanPath := filepath.ToSlash(strings.TrimSpace(file.Path))
		if cleanPath == "" {
			return siteconfig.ValidationError{Issues: []siteconfig.Issue{{
				Path:    "publish.artifacts.files",
				Code:    "missing_artifact_path",
				Message: "publish artifact bundle includes a file with no path",
			}}}
		}
		filesByPath[cleanPath] = file
	}

	requiredPaths := []string{"manifest.json", "robots.txt", "sitemap.xml", "assets/theme.css"}
	issues := []siteconfig.Issue{}
	for _, requiredPath := range requiredPaths {
		if _, ok := filesByPath[requiredPath]; !ok {
			issues = append(issues, siteconfig.Issue{
				Path:    "publish.artifacts." + requiredPath,
				Code:    "missing_artifact",
				Message: "required publish artifact is missing",
			})
		}
	}

	manifestFile, ok := filesByPath["manifest.json"]
	if !ok {
		return siteconfig.ValidationError{Issues: issues}
	}

	manifest, err := decodeArtifactManifest([]byte(manifestFile.Body))
	if err != nil {
		return siteconfig.ValidationError{Issues: append(issues, siteconfig.Issue{
			Path:    "publish.artifacts.manifest",
			Code:    "invalid_artifact_manifest",
			Message: err.Error(),
		})}
	}

	if manifest.SiteSlug != siteSlug {
		issues = append(issues, siteconfig.Issue{
			Path:    "publish.artifacts.manifest.siteSlug",
			Code:    "invalid_artifact_manifest",
			Message: "publish artifact manifest site slug does not match the published site",
		})
	}
	if normalizeHostname(manifest.Hostname) != normalizeHostname(hostname) {
		issues = append(issues, siteconfig.Issue{
			Path:    "publish.artifacts.manifest.hostname",
			Code:    "invalid_artifact_manifest",
			Message: "publish artifact manifest hostname does not match the published hostname",
		})
	}
	if manifest.Version.ID != version.ID || manifest.Version.VersionNumber != version.VersionNumber {
		issues = append(issues, siteconfig.Issue{
			Path:    "publish.artifacts.manifest.version",
			Code:    "invalid_artifact_manifest",
			Message: "publish artifact manifest version does not match the published version",
		})
	}

	manifestPages := make(map[string]ArtifactManifestPage, len(manifest.Pages))
	for _, page := range manifest.Pages {
		manifestPages[normalizePublishedPagePath(page.PagePath)] = page
		if _, ok := filesByPath[filepath.ToSlash(page.FilePath)]; !ok {
			issues = append(issues, siteconfig.Issue{
				Path:    "publish.artifacts." + page.FilePath,
				Code:    "missing_artifact",
				Message: "manifest references a page artifact that was not rendered",
			})
		}
	}

	collectionsByID := map[string]siteconfig.Collection{}
	for _, collection := range snapshot.Collections {
		collectionsByID[collection.ID] = collection
	}

	requiredPagePaths := expectedPagePaths(snapshot, collectionsByID)
	for _, expected := range requiredPagePaths {
		manifestPage, ok := manifestPages[expected.pagePath]
		if !ok {
			issues = append(issues, siteconfig.Issue{
				Path:    "publish.artifacts.pages." + expected.pagePath,
				Code:    "missing_artifact",
				Message: "publish artifact manifest is missing a rendered page",
			})
			continue
		}
		if strings.TrimSpace(manifestPage.Title) == "" || strings.TrimSpace(manifestPage.Description) == "" || strings.TrimSpace(manifestPage.CanonicalURL) == "" {
			issues = append(issues, siteconfig.Issue{
				Path:    "publish.artifacts.pages." + expected.pagePath,
				Code:    "invalid_artifact_manifest",
				Message: "publish artifact manifest page metadata is incomplete",
			})
		}

		pageFile, ok := filesByPath[filepath.ToSlash(manifestPage.FilePath)]
		if !ok {
			// Already reported above via "missing_artifact"; do not double-report.
			continue
		}
		if reason := htmlBodyIncompleteReason(pageFile.Body); reason != "" {
			issues = append(issues, siteconfig.Issue{
				Path:    "publish.artifacts." + manifestPage.FilePath,
				Code:    "incomplete_artifact_html",
				Message: "rendered page artifact is incomplete: " + reason,
			})
		}
	}

	if len(issues) > 0 {
		return siteconfig.ValidationError{Issues: issues}
	}
	return nil
}

// expectedPagePath captures one URL the publish artifact bundle must render.
// collection_detail templates contribute one entry per published entry rather
// than a single rendered URL for the template itself.
type expectedPagePath struct {
	pagePath string
	source   string
}

func expectedPagePaths(snapshot siteconfig.PublishedSnapshot, collectionsByID map[string]siteconfig.Collection) []expectedPagePath {
	out := make([]expectedPagePath, 0, len(snapshot.Pages))
	seen := map[string]bool{}
	for _, page := range snapshot.Pages {
		if page.Status == siteconfig.PageStatusDraft {
			continue
		}
		if page.Type == siteconfig.PageTypeCollectionDetail {
			collection, ok := collectionsByID[page.CollectionID]
			if !ok {
				continue
			}
			// Honor the per-collection opt-out: when settings explicitly
			// disable detail URLs, the template still exists in the editor
			// but emits no public artifacts.
			if !collectionExposesDetailURLs(collection, snapshot.Pages) {
				continue
			}
			for _, entry := range collection.Entries {
				if entry.Status != "" && entry.Status != siteconfig.EntryStatusPublished {
					continue
				}
				path := collectionEntryPagePath(collection, entry)
				if seen[path] {
					continue
				}
				seen[path] = true
				out = append(out, expectedPagePath{
					pagePath: path,
					source:   "collection_detail:" + page.ID + ":" + entry.ID,
				})
			}
			continue
		}
		path := normalizePublishedPagePath(page.Slug)
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, expectedPagePath{
			pagePath: path,
			source:   "page:" + page.ID,
		})
	}
	return out
}

// collectionExposesDetailURLs reports whether the collection should produce
// public detail-page artifacts. The canonical rule is: a collection exposes
// detail URLs when either the explicit settings flag is true OR a
// collection_detail template binds to it (legacy behavior preserved for
// existing draft/published data). To opt out, the user must clear all
// detail templates AND set settings.exposeDetailUrls=false.
func collectionExposesDetailURLs(collection siteconfig.Collection, pages []siteconfig.PageDraft) bool {
	if collection.Settings.ExposeDetailURLs {
		return true
	}
	for _, page := range pages {
		if page.Type == siteconfig.PageTypeCollectionDetail && page.CollectionID == collection.ID {
			return true
		}
	}
	return false
}

// collectionEntryPagePath is the canonical URL for a single collection entry.
// Per spec 19 the default URL pattern is /{collection.slug}/{entry.slug}; we
// emit one rendered HTML page per published entry under that path.
func collectionEntryPagePath(collection siteconfig.Collection, entry siteconfig.CollectionEntry) string {
	return normalizePublishedPagePath("/" + collection.Slug + "/" + entry.Slug)
}

// htmlBodyIncompleteReason returns a non-empty reason string when a rendered
// page artifact looks empty, malformed, or contains a known render-failure
// marker. An empty return value means the body passes the structural sanity
// checks performed at publish time.
//
// These checks are intentionally cheap and structural — they are not a full
// HTML validator. The goal is to fail loudly when the renderer emits stub or
// truncated content (which would otherwise ship to production).
func htmlBodyIncompleteReason(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "body is empty"
	}
	// React's renderToStaticMarkup never emits "[object Object]"; its presence
	// indicates a value leaked through a missing renderer for a prop. Treat
	// these as render failures so they cannot be published.
	if strings.Contains(trimmed, "[object Object]") {
		return "body contains \"[object Object]\" render leak"
	}
	if strings.Contains(trimmed, "<!-- error:") {
		return "body contains an error comment"
	}
	// Sanity-check that the body has at least one element with matching open
	// and close brackets. Catches truncated streams where the renderer cut
	// off mid-tag.
	openCount := strings.Count(trimmed, "<")
	closeCount := strings.Count(trimmed, ">")
	if openCount == 0 || closeCount == 0 {
		return "body has no HTML elements"
	}
	if openCount != closeCount {
		return "body has unbalanced angle brackets"
	}
	// Require at least one closing tag, which proves at least one element was
	// rendered through to completion (rather than leaving dangling text).
	if !strings.Contains(trimmed, "</") {
		return "body has no closing HTML tag"
	}
	return ""
}

func decodeArtifactManifest(body []byte) (ArtifactManifest, error) {
	var manifest ArtifactManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return ArtifactManifest{}, fmt.Errorf("decode publish artifact manifest: %w", err)
	}
	if strings.TrimSpace(manifest.SchemaVersion) != "published_artifacts.v1" {
		return ArtifactManifest{}, fmt.Errorf("publish artifact manifest must use schema version published_artifacts.v1")
	}
	return manifest, nil
}

func (s *Service) loadPublishedArtifactPage(ctx context.Context, lookup publishedSiteLookup, pagePath string) (PublishedPageArtifact, error) {
	manifestFile, err := s.store.Load(ctx, lookup.Version.SiteID, lookup.Version.ID, "manifest.json")
	if errors.Is(err, ErrArtifactNotFound) {
		return PublishedPageArtifact{}, ErrNotFound
	}
	if err != nil {
		return PublishedPageArtifact{}, fmt.Errorf("load published artifact manifest: %w", err)
	}

	manifest, err := decodeArtifactManifest([]byte(manifestFile.Body))
	if err != nil {
		return PublishedPageArtifact{}, fmt.Errorf("load published artifact manifest: %w", err)
	}

	var manifestPage ArtifactManifestPage
	found := false
	for _, candidate := range manifest.Pages {
		if normalizePublishedPagePath(candidate.PagePath) == pagePath {
			manifestPage = candidate
			found = true
			break
		}
	}
	if !found {
		return PublishedPageArtifact{}, ErrPageNotFound
	}

	pageFile, err := s.store.Load(ctx, lookup.Version.SiteID, lookup.Version.ID, manifestPage.FilePath)
	if errors.Is(err, ErrArtifactNotFound) {
		return PublishedPageArtifact{}, ErrPageNotFound
	}
	if err != nil {
		return PublishedPageArtifact{}, fmt.Errorf("load published page artifact: %w", err)
	}

	return PublishedPageArtifact{
		PageID:              manifestPage.PageID,
		PagePath:            pagePath,
		Title:               manifestPage.Title,
		Description:         manifestPage.Description,
		CanonicalURL:        manifestPage.CanonicalURL,
		OGImageURL:          manifestPage.OGImageURL,
		LocalBusinessJSONLD: manifestPage.LocalBusinessJSONLD,
		HTML:                pageFile.Body,
		DefaultLocale:       firstNonEmpty(manifest.DefaultLocale, "en"),
	}, nil
}

func (s *Service) loadPublishedArtifact(ctx context.Context, lookup publishedSiteLookup, artifactPath string) (PublishedArtifactResult, error) {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(artifactPath))
	if normalizedPath == "" {
		return PublishedArtifactResult{}, ErrNotFound
	}

	allowed, err := s.allowedArtifactPaths(ctx, lookup)
	if err != nil {
		return PublishedArtifactResult{}, err
	}
	if _, ok := allowed[normalizedPath]; !ok {
		return PublishedArtifactResult{}, ErrNotFound
	}

	file, err := s.store.Load(ctx, lookup.Version.SiteID, lookup.Version.ID, normalizedPath)
	if errors.Is(err, ErrArtifactNotFound) {
		return PublishedArtifactResult{}, ErrNotFound
	}
	if err != nil {
		return PublishedArtifactResult{}, fmt.Errorf("load published artifact: %w", err)
	}

	return PublishedArtifactResult{
		SiteSlug: lookup.SiteSlug,
		Hostname: lookup.Hostname,
		Version:  lookup.Version,
		File:     file,
	}, nil
}

// allowedArtifactPaths returns the set of artifact paths the manifest declares
// for the active published version. Public artifact reads are restricted to
// this set so callers cannot fetch arbitrary keys from the artifact store.
func (s *Service) allowedArtifactPaths(ctx context.Context, lookup publishedSiteLookup) (map[string]struct{}, error) {
	manifestFile, err := s.store.Load(ctx, lookup.Version.SiteID, lookup.Version.ID, "manifest.json")
	if errors.Is(err, ErrArtifactNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load published artifact manifest: %w", err)
	}
	manifest, err := decodeArtifactManifest([]byte(manifestFile.Body))
	if err != nil {
		return nil, fmt.Errorf("load published artifact manifest: %w", err)
	}

	allowed := map[string]struct{}{
		"manifest.json": {},
	}
	for _, entry := range manifest.Files {
		key := filepath.ToSlash(strings.TrimSpace(entry))
		if key == "" {
			continue
		}
		allowed[key] = struct{}{}
	}
	// Pre-v1.1 manifests omit Files; fall back to the well-known set so
	// existing publishes keep serving until they are re-rendered.
	if len(manifest.Files) == 0 {
		for _, path := range []string{"robots.txt", "sitemap.xml", "assets/theme.css"} {
			allowed[path] = struct{}{}
		}
		for _, page := range manifest.Pages {
			if key := filepath.ToSlash(strings.TrimSpace(page.FilePath)); key != "" {
				allowed[key] = struct{}{}
			}
		}
	}
	return allowed, nil
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

func ensureSubdomain(ctx context.Context, tx pgx.Tx, siteID string, siteSlug string, publicBaseDomain string) (string, error) {
	normalizedBaseDomain := normalizeHostname(publicBaseDomain)
	if normalizedBaseDomain == "" {
		return "", fmt.Errorf("public base domain is required")
	}

	hostname := normalizeHostname(siteSlug + "." + normalizedBaseDomain)

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

// enrichSnapshotCredits walks the snapshot pages, resolves provenance for
// every referenced asset, and writes the credit summary into the snapshot
// so the public renderer can attribute backend-imported imagery.
func (s *Service) enrichSnapshotCredits(ctx context.Context, snapshot *siteconfig.PublishedSnapshot) error {
	if s == nil || s.assetProvenance == nil || snapshot == nil {
		return nil
	}

	references := siteconfig.CollectSnapshotAssetIDs(*snapshot)
	if len(references) == 0 {
		snapshot.ImageCredits = nil
		return nil
	}
	assetIDs := make([]string, 0, len(references))
	for id := range references {
		assetIDs = append(assetIDs, id)
	}

	credits, err := s.assetProvenance.LookupCredits(ctx, assetIDs)
	if err != nil {
		return fmt.Errorf("lookup asset provenance for snapshot: %w", err)
	}
	snapshot.ImageCredits = dedupeImageCredits(credits)
	return nil
}

func dedupeImageCredits(credits []siteconfig.ImageCredit) []siteconfig.ImageCredit {
	if len(credits) == 0 {
		return nil
	}
	seen := map[string]bool{}
	output := make([]siteconfig.ImageCredit, 0, len(credits))
	for _, credit := range credits {
		key := strings.ToLower(strings.TrimSpace(credit.Provider)) + "|" + strings.ToLower(strings.TrimSpace(credit.Author)) + "|" + strings.ToLower(strings.TrimSpace(credit.SourceURL))
		if seen[key] {
			continue
		}
		seen[key] = true
		output = append(output, credit)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func buildPublishedSnapshot(draft siteconfig.SiteDraft) siteconfig.PublishedSnapshot {
	publishedPages := publishedSnapshotPages(draft.Pages)

	siteDescription := draft.Site.SEO.Description
	if siteDescription == "" {
		siteDescription = firstNonEmpty(
			pageDescription(firstPageBySlug(publishedPages, "/")),
			firstNonEmptyPageDescription(publishedPages),
			"Discover "+draft.Site.Name+".",
		)
	}

	siteSEO := siteconfig.SEOConfig{
		Title:       clampText(firstNonEmpty(draft.Site.SEO.Title, draft.Site.Name), 70),
		Description: clampText(siteDescription, 180),
	}

	pages := make([]siteconfig.PageDraft, 0, len(publishedPages))
	for _, page := range publishedPages {
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
			ID:           page.ID,
			Title:        page.Title,
			Slug:         page.Slug,
			Status:       siteconfig.PageStatusPublished,
			Type:         page.Type,
			CollectionID: page.CollectionID,
			SEO:          pageSEO,
			Blocks:       page.Blocks,
			Settings:     page.Settings,
		})
	}

	defaultLocale := draft.Site.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	collections := publishedCollections(draft.Collections)

	return siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site: siteconfig.PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: defaultLocale,
			SEO:           siteSEO,
		},
		Brand:       publishedBrand(draft),
		Theme:       draft.Theme,
		Navigation:  publishedNavigation(draft.Navigation, pages),
		Pages:       pages,
		Collections: collections,
	}
}

func publishedSnapshotPages(pages []siteconfig.PageDraft) []siteconfig.PageDraft {
	published := make([]siteconfig.PageDraft, 0, len(pages))
	for _, page := range pages {
		if page.Status == siteconfig.PageStatusDraft {
			continue
		}
		published = append(published, page)
	}
	return published
}

func publishedNavigation(navigation siteconfig.NavigationConfig, pages []siteconfig.PageDraft) siteconfig.NavigationConfig {
	pageIDs := make(map[string]struct{}, len(pages))
	for _, page := range pages {
		pageIDs[page.ID] = struct{}{}
	}

	filter := func(items []siteconfig.NavigationItem) []siteconfig.NavigationItem {
		if len(items) == 0 {
			return nil
		}
		filtered := make([]siteconfig.NavigationItem, 0, len(items))
		for _, item := range items {
			if item.PageID == "" {
				filtered = append(filtered, item)
				continue
			}
			if _, ok := pageIDs[item.PageID]; ok {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}

	return siteconfig.NavigationConfig{
		Primary: filter(navigation.Primary),
		Footer:  filter(navigation.Footer),
	}
}

// publishedBrand guarantees the snapshot carries the minimum brand fields the
// published validator requires. If the draft has no brand stored yet, fall back
// to site.name for businessName and the theme's primary color so older drafts
// authored before the brand column existed still pass publish validation.
func publishedBrand(draft siteconfig.SiteDraft) siteconfig.BrandConfig {
	brand := draft.Brand
	if strings.TrimSpace(brand.BusinessName) == "" {
		brand.BusinessName = draft.Site.Name
	}
	if strings.TrimSpace(brand.PrimaryColor) == "" {
		brand.PrimaryColor = draft.Theme.Tokens.Colors["primary"]
	}
	return brand
}

// publishedCollections drops draft entries from the snapshot so only
// published entries are publicly routable. Empty collections survive so the
// schema is preserved.
func publishedCollections(collections []siteconfig.Collection) []siteconfig.Collection {
	if len(collections) == 0 {
		return nil
	}
	out := make([]siteconfig.Collection, 0, len(collections))
	for _, collection := range collections {
		published := make([]siteconfig.CollectionEntry, 0, len(collection.Entries))
		for _, entry := range collection.Entries {
			if entry.Status != siteconfig.EntryStatusPublished {
				continue
			}
			published = append(published, entry)
		}
		out = append(out, siteconfig.Collection{
			ID:            collection.ID,
			Slug:          collection.Slug,
			SingularLabel: collection.SingularLabel,
			PluralLabel:   collection.PluralLabel,
			Schema:        collection.Schema,
			Settings:      collection.Settings,
			SortOrder:     collection.SortOrder,
			Entries:       published,
		})
	}
	return out
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
