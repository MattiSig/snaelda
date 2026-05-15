package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeAssetProvenance struct {
	credits     []siteconfig.ImageCredit
	requestedID []string
}

func (f *fakeAssetProvenance) LookupCredits(_ context.Context, assetIDs []string) ([]siteconfig.ImageCredit, error) {
	f.requestedID = append(f.requestedID, assetIDs...)
	return f.credits, nil
}

func TestEnrichSnapshotCreditsAttachesImageCredits(t *testing.T) {
	lookup := &fakeAssetProvenance{credits: []siteconfig.ImageCredit{
		{Provider: "pexels", Author: "Test Photographer", AuthorURL: "https://www.pexels.com/@test", SourceURL: "https://www.pexels.com/photo/1", License: "Pexels License"},
		{Provider: "pexels", Author: "Test Photographer", AuthorURL: "https://www.pexels.com/@test", SourceURL: "https://www.pexels.com/photo/1", License: "Pexels License"},
	}}
	service := &Service{assetProvenance: lookup}

	snapshot := &siteconfig.PublishedSnapshot{
		Pages: []siteconfig.PageDraft{{
			ID: "p1", Slug: "/", Title: "Home",
			Blocks: []siteconfig.BlockInstance{{
				ID: "b1", Type: "hero", Version: siteconfig.BlockVersionV1,
				Props: map[string]any{"image": map[string]any{"assetId": "asset-1"}},
			}},
		}},
	}

	if err := service.enrichSnapshotCredits(context.Background(), snapshot); err != nil {
		t.Fatalf("enrich credits: %v", err)
	}
	if len(snapshot.ImageCredits) != 1 {
		t.Fatalf("expected dedup to leave one credit, got %#v", snapshot.ImageCredits)
	}
	if snapshot.ImageCredits[0].Provider != "pexels" {
		t.Fatalf("expected pexels provider, got %q", snapshot.ImageCredits[0].Provider)
	}
	if len(lookup.requestedID) != 1 || lookup.requestedID[0] != "asset-1" {
		t.Fatalf("expected lookup with asset-1, got %#v", lookup.requestedID)
	}
}

func TestEnrichSnapshotCreditsSkipsWhenNoAssets(t *testing.T) {
	lookup := &fakeAssetProvenance{}
	service := &Service{assetProvenance: lookup}
	snapshot := &siteconfig.PublishedSnapshot{
		Pages: []siteconfig.PageDraft{{ID: "p1", Slug: "/", Title: "Home", Blocks: []siteconfig.BlockInstance{{ID: "b1", Type: "text_section", Props: map[string]any{"heading": "Hi"}}}}},
	}
	if err := service.enrichSnapshotCredits(context.Background(), snapshot); err != nil {
		t.Fatalf("enrich credits: %v", err)
	}
	if snapshot.ImageCredits != nil {
		t.Fatalf("expected nil credits, got %#v", snapshot.ImageCredits)
	}
	if len(lookup.requestedID) != 0 {
		t.Fatalf("expected no lookup, got %#v", lookup.requestedID)
	}
}

func TestBuildPublishedSnapshotAddsSEOFallbacks(t *testing.T) {
	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#151215",
					"foreground": "#f6f2ec",
					"primary":    "#8fc6ff",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block_hero",
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"headline":    "Clear websites for focused teams",
							"subheadline": "Structured sites from maintained blocks.",
							"layout":      "centered",
						},
					},
				},
			},
			{
				ID:    "page_contact",
				Title: "Contact",
				Slug:  "/contact",
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      "block_text",
						Type:    "text_section",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading": "Get in touch",
							"body":    "Send a note to plan your next launch.",
						},
					},
				},
			},
		},
	}

	snapshot := buildPublishedSnapshot(draft)
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		t.Fatalf("validate snapshot: %v", err)
	}
	if snapshot.Site.SEO.Title != "Nordic Studio" {
		t.Fatalf("expected site title fallback, got %q", snapshot.Site.SEO.Title)
	}
	if snapshot.Site.SEO.Description != "Structured sites from maintained blocks." {
		t.Fatalf("expected site description fallback, got %q", snapshot.Site.SEO.Description)
	}
	if snapshot.Pages[1].SEO.Title != "Contact | Nordic Studio" {
		t.Fatalf("expected page title fallback, got %q", snapshot.Pages[1].SEO.Title)
	}
	if snapshot.Pages[1].SEO.Description != "Send a note to plan your next launch." {
		t.Fatalf("expected page description fallback, got %q", snapshot.Pages[1].SEO.Description)
	}
}

func TestRollbackSetsLiveVersionAndRecordsAuditEvent(t *testing.T) {
	store := newFakePublishingStore()
	service := Service{
		db:               store,
		reader:           fakePublishingReader{},
		publicBaseURL:    "http://localhost:3000",
		publicBaseDomain: "localhost",
	}

	result, err := service.Rollback(
		context.Background(),
		"00000000-0000-4000-8000-000000000201",
		"00000000-0000-4000-8000-000000000701",
		"00000000-0000-4000-8000-000000000001",
	)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if result.Version.VersionNumber != 1 || !result.Version.IsCurrent {
		t.Fatalf("expected rolled back current version, got %#v", result.Version)
	}
	if result.SiteSlug != "nordic-studio" {
		t.Fatalf("expected site slug, got %q", result.SiteSlug)
	}
	if store.tx.liveVersionID != "00000000-0000-4000-8000-000000000701" {
		t.Fatalf("expected live version pointer to update, got %q", store.tx.liveVersionID)
	}
	if !store.tx.committed {
		t.Fatal("expected rollback transaction to commit")
	}
	if len(store.tx.auditEvents) != 1 {
		t.Fatalf("expected one audit event, got %#v", store.tx.auditEvents)
	}
	if store.tx.auditEvents[0].Action != "site.rollback" {
		t.Fatalf("expected rollback audit action, got %#v", store.tx.auditEvents[0])
	}
	if store.tx.auditEvents[0].Metadata["versionNumber"] != float64(1) {
		t.Fatalf("expected rollback metadata to include version number, got %#v", store.tx.auditEvents[0].Metadata)
	}
}

func TestRollbackRejectsUnknownVersion(t *testing.T) {
	store := newFakePublishingStore()
	service := Service{
		db:               store,
		reader:           fakePublishingReader{},
		publicBaseURL:    "http://localhost:3000",
		publicBaseDomain: "localhost",
	}

	_, err := service.Rollback(
		context.Background(),
		"00000000-0000-4000-8000-000000000201",
		"00000000-0000-4000-8000-000000000799",
		"00000000-0000-4000-8000-000000000001",
	)
	if !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("expected version not found error, got %v", err)
	}
}

type fakePublishingReader struct{}

func (fakePublishingReader) ListSites(context.Context, string) ([]sites.Summary, error) {
	return nil, errors.New("not implemented")
}

func (fakePublishingReader) LoadDraft(context.Context, string) (siteconfig.SiteDraft, error) {
	return siteconfig.SiteDraft{}, errors.New("not implemented")
}

func (fakePublishingReader) LoadGenerationMetadata(context.Context, string) (sites.GenerationMetadata, error) {
	return sites.GenerationMetadata{}, errors.New("not implemented")
}

type fakePublishingStore struct {
	tx                  *fakePublishingTx
	publishedSiteSlug   string
	publishedHostname   string
	publishedVersion    VersionSummary
	slugLookupCount     int
	hostnameLookupCount int
	artifactFiles       map[string]ArtifactFile
}

func newFakePublishingStore() *fakePublishingStore {
	return &fakePublishingStore{
		tx: &fakePublishingTx{
			siteID:      "00000000-0000-4000-8000-000000000201",
			workspaceID: "00000000-0000-4000-8000-000000000101",
			siteSlug:    "nordic-studio",
			hostname:    "nordic-studio.localhost",
			liveVersion: "00000000-0000-4000-8000-000000000702",
			versions: map[string]VersionSummary{
				"00000000-0000-4000-8000-000000000701": {
					ID:            "00000000-0000-4000-8000-000000000701",
					SiteID:        "00000000-0000-4000-8000-000000000201",
					VersionNumber: 1,
					CreatedAt:     time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
					PublishNote:   "Initial launch",
				},
				"00000000-0000-4000-8000-000000000702": {
					ID:            "00000000-0000-4000-8000-000000000702",
					SiteID:        "00000000-0000-4000-8000-000000000201",
					VersionNumber: 2,
					CreatedAt:     time.Date(2026, 5, 7, 8, 0, 0, 0, time.UTC),
					PublishNote:   "Refined hero copy",
				},
			},
		},
		publishedSiteSlug: "nordic-studio",
		publishedHostname: "nordic-studio.localhost",
		publishedVersion: VersionSummary{
			ID:            "00000000-0000-4000-8000-000000000702",
			SiteID:        "00000000-0000-4000-8000-000000000201",
			VersionNumber: 2,
			CreatedAt:     time.Date(2026, 5, 7, 8, 0, 0, 0, time.UTC),
			PublishNote:   "Refined hero copy",
			IsCurrent:     true,
		},
	}
}

func (s *fakePublishingStore) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakePublishingStore")
}

func (s *fakePublishingStore) QueryRow(_ context.Context, sql string, arguments ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "where s.slug = $1"):
		s.slugLookupCount++
		lookup := strings.TrimSpace(arguments[0].(string))
		if lookup != s.publishedSiteSlug {
			return fakePublishingRow{err: pgx.ErrNoRows}
		}
		return fakePublishingRow{values: []any{
			s.publishedSiteSlug,
			s.publishedHostname,
			s.publishedVersion.ID,
			s.publishedVersion.SiteID,
			s.publishedVersion.VersionNumber,
			s.publishedVersion.CreatedAt,
			s.publishedVersion.PublishNote,
		}}
	case strings.Contains(sql, "from site_domains d") && strings.Contains(sql, "join site_versions sv on sv.id = s.published_version_id"):
		s.hostnameLookupCount++
		lookup := strings.TrimSpace(arguments[0].(string))
		if lookup != s.publishedHostname {
			return fakePublishingRow{err: pgx.ErrNoRows}
		}
		return fakePublishingRow{values: []any{
			s.publishedSiteSlug,
			s.publishedHostname,
			s.publishedVersion.ID,
			s.publishedVersion.SiteID,
			s.publishedVersion.VersionNumber,
			s.publishedVersion.CreatedAt,
			s.publishedVersion.PublishNote,
		}}
	default:
		return fakePublishingRow{err: errors.New("query row is not implemented in fakePublishingStore")}
	}
}

func (s *fakePublishingStore) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("exec is not implemented in fakePublishingStore")
}

func (s *fakePublishingStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return s.tx, nil
}

func (s *fakePublishingStore) Save(context.Context, string, string, ArtifactBundle) error {
	return errors.New("save is not implemented in fakePublishingStore")
}

func (s *fakePublishingStore) Load(_ context.Context, siteID string, versionID string, path string) (ArtifactFile, error) {
	if siteID != s.publishedVersion.SiteID || versionID != s.publishedVersion.ID {
		return ArtifactFile{}, ErrArtifactNotFound
	}
	if len(s.artifactFiles) == 0 {
		s.artifactFiles = buildFakePublishedArtifacts(s.publishedSiteSlug, s.publishedHostname, s.publishedVersion)
	}
	file, ok := s.artifactFiles[path]
	if !ok {
		return ArtifactFile{}, ErrArtifactNotFound
	}
	return file, nil
}

type recordedAuditEvent struct {
	Action   string
	Metadata map[string]any
}

type fakePublishingTx struct {
	siteID        string
	workspaceID   string
	siteSlug      string
	hostname      string
	liveVersion   string
	liveVersionID string
	versions      map[string]VersionSummary
	auditEvents   []recordedAuditEvent
	committed     bool
	rolledBack    bool
}

func (tx *fakePublishingTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *fakePublishingTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *fakePublishingTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("copy is not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakePublishingTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakePublishingTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("prepare is not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "update sites") && strings.Contains(sql, "published_version_id"):
		tx.liveVersion = arguments[1].(string)
		tx.liveVersionID = arguments[1].(string)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "insert into audit_events"):
		var metadata map[string]any
		if err := json.Unmarshal(arguments[4].([]byte), &metadata); err != nil {
			return pgconn.CommandTag{}, err
		}
		tx.auditEvents = append(tx.auditEvents, recordedAuditEvent{
			Action:   arguments[3].(string),
			Metadata: metadata,
		})
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
}

func (tx *fakePublishingTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented in fakePublishingTx")
}

func (tx *fakePublishingTx) QueryRow(_ context.Context, sql string, arguments ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "select workspace_id::text, slug"):
		return fakePublishingRow{values: []any{tx.workspaceID, tx.siteSlug}}
	case strings.Contains(sql, "from site_versions sv") && strings.Contains(sql, "where sv.site_id = $1"):
		versionID := arguments[1].(string)
		version, ok := tx.versions[versionID]
		if !ok {
			return fakePublishingRow{err: pgx.ErrNoRows}
		}
		return fakePublishingRow{values: []any{
			version.ID,
			version.SiteID,
			version.VersionNumber,
			version.CreatedAt,
			version.PublishNote,
			tx.hostname,
		}}
	default:
		return fakePublishingRow{err: pgx.ErrNoRows}
	}
}

func (tx *fakePublishingTx) Conn() *pgx.Conn {
	return nil
}

type fakePublishingRow struct {
	values []any
	err    error
}

func (r fakePublishingRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *int:
			*target = value.(int)
		case *time.Time:
			*target = value.(time.Time)
		case *[]byte:
			*target = value.([]byte)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func TestLoadPublishedSiteBySlugResolvesRequestedPage(t *testing.T) {
	store := newFakePublishingStore()
	service := Service{db: store, store: store}

	result, err := service.LoadPublishedSiteBySlug(context.Background(), "nordic-studio", "/contact")
	if err != nil {
		t.Fatalf("load published site: %v", err)
	}

	if result.PagePath != "/contact" {
		t.Fatalf("expected page path, got %q", result.PagePath)
	}
	if result.Page.Title != "Contact | Nordic Studio" {
		t.Fatalf("expected contact page, got %#v", result.Page)
	}
}

func TestLoadPublishedSiteBySlugRejectsUnknownPagePath(t *testing.T) {
	store := newFakePublishingStore()
	service := Service{db: store, store: store}

	_, err := service.LoadPublishedSiteBySlug(context.Background(), "nordic-studio", "/missing")
	if !errors.Is(err, ErrPageNotFound) {
		t.Fatalf("expected page not found, got %v", err)
	}
}

func TestLoadPublishedSiteByHostnameResolvesRequestedPage(t *testing.T) {
	store := newFakePublishingStore()
	service := Service{db: store, store: store}

	result, err := service.LoadPublishedSiteByHostname(context.Background(), "nordic-studio.localhost:3000", "/contact")
	if err != nil {
		t.Fatalf("load published site by hostname: %v", err)
	}

	if result.Hostname != "nordic-studio.localhost" {
		t.Fatalf("expected normalized hostname, got %q", result.Hostname)
	}
	if result.Page.Title != "Contact | Nordic Studio" {
		t.Fatalf("expected contact page, got %#v", result.Page)
	}
}

func TestLoadPublishedSiteByHostnameWarmsCachesAndAvoidsSecondLookup(t *testing.T) {
	store := newFakePublishingStore()
	cache := newMemoryPublishedSiteCache()
	service := Service{
		db:    store,
		store: store,
		cache: cache,
	}

	first, err := service.LoadPublishedSiteByHostname(context.Background(), "nordic-studio.localhost", "/contact")
	if err != nil {
		t.Fatalf("first hosted lookup: %v", err)
	}
	second, err := service.LoadPublishedSiteByHostname(context.Background(), "nordic-studio.localhost", "/contact")
	if err != nil {
		t.Fatalf("second hosted lookup: %v", err)
	}

	if first.Page.HTML != second.Page.HTML || first.Page.Title != second.Page.Title {
		t.Fatalf("expected cached hosted lookup to return same page, got first=%#v second=%#v", first.Page, second.Page)
	}
	if store.hostnameLookupCount != 1 {
		t.Fatalf("expected one hostname lookup query, got %d", store.hostnameLookupCount)
	}
	if _, ok := cache.LoadDomain("nordic-studio.localhost"); !ok {
		t.Fatal("expected domain lookup cache entry")
	}
	if _, ok := cache.LoadPage(first.Version.SiteID, first.Version.ID, "/contact"); !ok {
		t.Fatal("expected page cache entry")
	}
}

func TestRollbackInvalidatesPublishedSiteCache(t *testing.T) {
	store := newFakePublishingStore()
	cache := newMemoryPublishedSiteCache()
	cache.StoreDomain(store.publishedHostname, publishedSiteLookup{
		SiteSlug: store.publishedSiteSlug,
		Hostname: store.publishedHostname,
		Version:  store.publishedVersion,
	})
	cache.StorePage(store.publishedVersion.SiteID, store.publishedVersion.ID, "/contact", PublishedPageArtifact{
		PagePath:     "/contact",
		Title:        "Contact | Nordic Studio",
		Description:  "Send a note to plan your next launch.",
		CanonicalURL: "http://nordic-studio.localhost:3000/contact",
		HTML:         "<div>contact</div>",
	})
	service := Service{
		db:               store,
		store:            store,
		reader:           fakePublishingReader{},
		cache:            cache,
		publicBaseURL:    "http://localhost:3000",
		publicBaseDomain: "localhost",
	}

	_, err := service.Rollback(
		context.Background(),
		"00000000-0000-4000-8000-000000000201",
		"00000000-0000-4000-8000-000000000701",
		"00000000-0000-4000-8000-000000000001",
	)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if _, ok := cache.LoadDomain(store.publishedHostname); ok {
		t.Fatal("expected rollback to invalidate domain cache")
	}
	if _, ok := cache.LoadPage(store.publishedVersion.SiteID, store.publishedVersion.ID, "/contact"); ok {
		t.Fatal("expected rollback to invalidate page cache")
	}
}

func buildFakePublishedArtifacts(siteSlug string, hostname string, version VersionSummary) map[string]ArtifactFile {
	manifest := ArtifactManifest{
		SchemaVersion: "published_artifacts.v1",
		SiteSlug:      siteSlug,
		Hostname:      hostname,
		Version:       version,
		Pages: []ArtifactManifestPage{
			{
				PagePath:     "/",
				FilePath:     "pages/index.html",
				Title:        "Nordic Studio",
				Description:  "Calm design systems for focused teams.",
				CanonicalURL: "http://nordic-studio.localhost:3000/",
			},
			{
				PagePath:     "/contact",
				FilePath:     "pages/contact/index.html",
				Title:        "Contact | Nordic Studio",
				Description:  "Send a note to plan your next launch.",
				CanonicalURL: "http://nordic-studio.localhost:3000/contact",
			},
		},
	}
	manifestJSON, _ := json.Marshal(manifest)

	return map[string]ArtifactFile{
		"manifest.json": {
			Path:        "manifest.json",
			ContentType: "application/json; charset=utf-8",
			Body:        string(manifestJSON),
		},
		"pages/index.html": {
			Path:        "pages/index.html",
			ContentType: "text/html; charset=utf-8",
			Body:        "<div><h1>Home</h1></div>",
		},
		"pages/contact/index.html": {
			Path:        "pages/contact/index.html",
			ContentType: "text/html; charset=utf-8",
			Body:        "<div><h1>Contact</h1></div>",
		},
		"robots.txt": {
			Path:        "robots.txt",
			ContentType: "text/plain; charset=utf-8",
			Body:        "User-agent: *\nAllow: /\n",
		},
		"sitemap.xml": {
			Path:        "sitemap.xml",
			ContentType: "application/xml; charset=utf-8",
			Body:        "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n",
		},
		"assets/theme.css": {
			Path:        "assets/theme.css",
			ContentType: "text/css; charset=utf-8",
			Body:        ":root {}\n",
		},
	}
}

func validPublishedSnapshotWithContact() siteconfig.PublishedSnapshot {
	return siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site: siteconfig.PublishedSite{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			DefaultLocale: "en",
			SEO: siteconfig.SEOConfig{
				Title:       "Nordic Studio",
				Description: "Calm design systems for focused teams.",
			},
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "#151215",
					"foreground": "#f6f2ec",
					"primary":    "#8fc6ff",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{
				{Label: "Home", PageID: "page_home"},
				{Label: "Contact", PageID: "page_contact"},
			},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				SEO: siteconfig.SEOConfig{
					Title:       "Nordic Studio",
					Description: "Calm design systems for focused teams.",
				},
				Blocks: []siteconfig.BlockInstance{{
					ID:      "block_hero",
					Type:    "hero",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{
						"headline": "Clear websites for focused teams",
						"layout":   "centered",
					},
				}},
			},
			{
				ID:    "page_contact",
				Title: "Contact",
				Slug:  "/contact",
				SEO: siteconfig.SEOConfig{
					Title:       "Contact | Nordic Studio",
					Description: "Start a new project conversation.",
				},
				Blocks: []siteconfig.BlockInstance{{
					ID:      "block_text_contact",
					Type:    "text_section",
					Version: siteconfig.BlockVersionV1,
					Props: map[string]any{
						"heading": "Say hello",
						"body":    "Send a note with your launch timeline.",
					},
				}},
			},
		},
	}
}
