package assets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCreateUploadStoresPendingAsset(t *testing.T) {
	store := newFakeAssetStore()
	storage := &fakeStorage{
		upload: PresignedUpload{URL: "http://upload.test", Method: "PUT"},
	}
	service := NewService(store, storage)

	result, err := service.CreateUpload(context.Background(), CreateUploadInput{
		WorkspaceID: "workspace-1",
		SiteID:      "site-1",
		UserID:      "user-1",
		FileName:    "hero.png",
		ContentType: "image/png",
		SizeBytes:   2048,
		AltText:     "Hero image",
	})
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}

	if result.Asset.ID == "" {
		t.Fatal("expected generated asset id")
	}
	if !strings.Contains(result.Asset.StorageKey, "/hero.png") {
		t.Fatalf("expected storage key to include file name, got %q", result.Asset.StorageKey)
	}
	if got := result.Asset.Metadata.UploadStatus; got != "pending" {
		t.Fatalf("expected pending upload status, got %q", got)
	}
	if len(store.assets) != 1 {
		t.Fatalf("expected one stored asset, got %d", len(store.assets))
	}
}

func TestCompleteUploadPersistsUploadedMetadata(t *testing.T) {
	store := newFakeAssetStore()
	createdAt := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)
	store.assets["asset-1"] = storedAsset{
		Asset: Asset{
			ID:          "asset-1",
			WorkspaceID: "workspace-1",
			SiteID:      "site-1",
			Kind:        "image",
			StorageKey:  "workspaces/workspace-1/sites/site-1/assets/asset-1/hero.png",
			Metadata: AssetMetadata{
				FileName:           "hero.png",
				ContentType:        "image/png",
				RequestedSizeBytes: 2048,
				UploadStatus:       "pending",
			},
			CreatedAt: createdAt,
		},
	}
	storage := &fakeStorage{
		head:        ObjectHead{ContentType: "image/png", SizeBytes: 2048, ETag: "etag-1"},
		downloadURL: "http://download.test/object",
	}
	service := NewService(store, storage)
	altText := "Updated alt"

	width := 1440
	height := 960
	asset, err := service.CompleteUpload(context.Background(), "asset-1", CompleteUploadInput{
		AltText: &altText,
		Width:   &width,
		Height:  &height,
	})
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}

	if got := asset.Metadata.UploadStatus; got != "uploaded" {
		t.Fatalf("expected uploaded status, got %q", got)
	}
	if asset.DownloadURL != "http://download.test/object" {
		t.Fatalf("expected download url, got %q", asset.DownloadURL)
	}
	if store.assets["asset-1"].Metadata.ETag != "etag-1" {
		t.Fatalf("expected stored etag, got %#v", store.assets["asset-1"].Metadata)
	}
	if asset.AltText != "Updated alt" {
		t.Fatalf("expected alt text update, got %q", asset.AltText)
	}
	if asset.Metadata.Width != width || asset.Metadata.Height != height {
		t.Fatalf("expected dimensions %dx%d, got %dx%d", width, height, asset.Metadata.Width, asset.Metadata.Height)
	}
}

func TestListBySiteIncludesDownloadURLForUploadedAssets(t *testing.T) {
	store := newFakeAssetStore()
	store.assets["asset-1"] = storedAsset{
		Asset: Asset{
			ID:          "asset-1",
			WorkspaceID: "workspace-1",
			SiteID:      "site-1",
			Kind:        "image",
			StorageKey:  "key-1",
			Metadata: AssetMetadata{
				UploadStatus: "uploaded",
			},
			CreatedAt: time.Now().UTC(),
		},
	}
	storage := &fakeStorage{downloadURL: "http://download.test/object"}
	service := NewService(store, storage)

	assets, err := service.ListBySite(context.Background(), "site-1")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected one asset, got %d", len(assets))
	}
	if assets[0].DownloadURL != "http://download.test/object" {
		t.Fatalf("expected download url, got %q", assets[0].DownloadURL)
	}
}

func TestDeleteRemovesStorageObjectAndRow(t *testing.T) {
	store := newFakeAssetStore()
	store.assets["asset-1"] = storedAsset{
		Asset: Asset{
			ID:          "asset-1",
			WorkspaceID: "workspace-1",
			SiteID:      "site-1",
			Kind:        "image",
			StorageKey:  "key-1",
			CreatedAt:   time.Now().UTC(),
		},
	}
	storage := &fakeStorage{}
	service := NewService(store, storage)

	if err := service.Delete(context.Background(), "asset-1"); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if storage.deletedKey != "key-1" {
		t.Fatalf("expected storage delete for key-1, got %q", storage.deletedKey)
	}
	if len(store.assets) != 0 {
		t.Fatalf("expected asset row to be deleted, got %d rows", len(store.assets))
	}
}

func TestPublicDownloadURLBySiteSlugRequiresPublishedReference(t *testing.T) {
	store := newFakeAssetStore()
	store.siteSlugs["site-1"] = "loom-light"
	store.publishedSites["site-1"] = true
	store.publishedSnapshots["site-1"] = snapshotReferencingAsset("asset-1")
	store.assets["asset-1"] = storedAsset{
		Asset: Asset{
			ID:          "asset-1",
			WorkspaceID: "workspace-1",
			SiteID:      "site-1",
			Kind:        "image",
			StorageKey:  "key-1",
			Metadata: AssetMetadata{
				UploadStatus: "uploaded",
			},
			CreatedAt: time.Now().UTC(),
		},
	}
	service := NewService(store, &fakeStorage{downloadURL: "http://download.test/public"})

	downloadURL, err := service.PublicDownloadURLBySiteSlug(context.Background(), "loom-light", "asset-1")
	if err != nil {
		t.Fatalf("public download url: %v", err)
	}
	if downloadURL != "http://download.test/public" {
		t.Fatalf("expected public download url, got %q", downloadURL)
	}
}

func TestPublicDownloadURLBySiteSlugRejectsAssetNotInPublishedSnapshot(t *testing.T) {
	store := newFakeAssetStore()
	store.siteSlugs["site-1"] = "loom-light"
	store.publishedSites["site-1"] = true
	store.publishedSnapshots["site-1"] = snapshotReferencingAsset("asset-2")
	store.assets["asset-1"] = storedAsset{
		Asset: Asset{
			ID:          "asset-1",
			WorkspaceID: "workspace-1",
			SiteID:      "site-1",
			Kind:        "image",
			StorageKey:  "key-1",
			Metadata:    AssetMetadata{UploadStatus: "uploaded"},
			CreatedAt:   time.Now().UTC(),
		},
	}
	service := NewService(store, &fakeStorage{downloadURL: "http://download.test/public"})

	_, err := service.PublicDownloadURLBySiteSlug(context.Background(), "loom-light", "asset-1")
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("expected ErrAssetNotFound for unreferenced asset, got %v", err)
	}
}

func TestPublicDownloadURLBySiteSlugRejectsUnpublishedSite(t *testing.T) {
	store := newFakeAssetStore()
	store.siteSlugs["site-1"] = "loom-light"
	store.assets["asset-1"] = storedAsset{
		Asset: Asset{
			ID:          "asset-1",
			WorkspaceID: "workspace-1",
			SiteID:      "site-1",
			Kind:        "image",
			StorageKey:  "key-1",
			Metadata:    AssetMetadata{UploadStatus: "uploaded"},
			CreatedAt:   time.Now().UTC(),
		},
	}
	service := NewService(store, &fakeStorage{downloadURL: "http://download.test/public"})

	_, err := service.PublicDownloadURLBySiteSlug(context.Background(), "loom-light", "asset-1")
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("expected ErrAssetNotFound when site has no published version, got %v", err)
	}
}

func TestPublicDownloadURLByHostnameMatchesActiveDomain(t *testing.T) {
	store := newFakeAssetStore()
	store.siteHostnames["site-1"] = "loom-light.snaelda.app"
	store.publishedSites["site-1"] = true
	store.publishedSnapshots["site-1"] = snapshotReferencingAsset("asset-1")
	store.assets["asset-1"] = storedAsset{
		Asset: Asset{
			ID:          "asset-1",
			WorkspaceID: "workspace-1",
			SiteID:      "site-1",
			Kind:        "image",
			StorageKey:  "key-1",
			Metadata:    AssetMetadata{UploadStatus: "uploaded"},
			CreatedAt:   time.Now().UTC(),
		},
	}
	service := NewService(store, &fakeStorage{downloadURL: "http://download.test/public"})

	downloadURL, err := service.PublicDownloadURLByHostname(context.Background(), "loom-light.snaelda.app", "asset-1")
	if err != nil {
		t.Fatalf("public download url by hostname: %v", err)
	}
	if downloadURL != "http://download.test/public" {
		t.Fatalf("expected public download url, got %q", downloadURL)
	}
}

type fakeStorage struct {
	upload      PresignedUpload
	head        ObjectHead
	downloadURL string
	deletedKey  string
	headErr     error
	deleteErr   error
}

func (s *fakeStorage) CreateUpload(context.Context, string, string, time.Duration) (PresignedUpload, error) {
	return s.upload, nil
}

func (s *fakeStorage) CreateDownloadURL(context.Context, string, time.Duration) (string, error) {
	return s.downloadURL, nil
}

func (s *fakeStorage) HeadObject(context.Context, string) (ObjectHead, error) {
	if s.headErr != nil {
		return ObjectHead{}, s.headErr
	}
	return s.head, nil
}

func (s *fakeStorage) DeleteObject(_ context.Context, key string) error {
	s.deletedKey = key
	return s.deleteErr
}

type storedAsset struct {
	Asset
}

type fakeAssetStore struct {
	assets             map[string]storedAsset
	siteSlugs          map[string]string
	siteHostnames      map[string]string
	publishedSites     map[string]bool
	publishedSnapshots map[string]siteconfig.PublishedSnapshot
}

func newFakeAssetStore() *fakeAssetStore {
	return &fakeAssetStore{
		assets:             map[string]storedAsset{},
		siteSlugs:          map[string]string{},
		siteHostnames:      map[string]string{},
		publishedSites:     map[string]bool{},
		publishedSnapshots: map[string]siteconfig.PublishedSnapshot{},
	}
}

func snapshotReferencingAsset(assetIDs ...string) siteconfig.PublishedSnapshot {
	images := make([]any, 0, len(assetIDs))
	for index, id := range assetIDs {
		images = append(images, map[string]any{
			"title":   "Image " + id,
			"caption": "",
			"image": map[string]any{
				"assetId": id,
				"alt":     "alt " + id,
			},
		})
		_ = index
	}
	return siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site:          siteconfig.PublishedSite{ID: "site-1", Name: "Loom & Light", DefaultLocale: "en"},
		Theme:         siteconfig.ThemePreset(siteconfig.ThemePaletteMeanerDark),
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-home"}},
		},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-home",
			Title: "Home",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{{
				ID:      "block-gallery",
				Type:    "gallery",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading": "Featured work",
					"layout":  "grid",
					"images":  images,
				},
			}},
		}},
	}
}

func (s *fakeAssetStore) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if !strings.Contains(sql, "from assets") {
		return nil, errors.New("unexpected query")
	}
	siteID := args[0].(string)
	rows := []storedAsset{}
	for _, asset := range s.assets {
		if asset.SiteID == siteID {
			rows = append(rows, asset)
		}
	}
	return &fakeAssetRows{rows: rows}, nil
}

func (s *fakeAssetStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "insert into assets"):
		id := args[0].(string)
		siteID := args[2].(string)
		altText := args[5].(string)
		createdBy := args[7].(string)
		var metadata AssetMetadata
		if err := json.Unmarshal(args[6].([]byte), &metadata); err != nil {
			return fakeAssetRow{err: err}
		}
		asset := storedAsset{
			Asset: Asset{
				ID:          id,
				WorkspaceID: args[1].(string),
				SiteID:      siteID,
				Kind:        args[3].(string),
				StorageKey:  args[4].(string),
				AltText:     altText,
				Metadata:    metadata,
				CreatedBy:   createdBy,
				CreatedAt:   time.Now().UTC(),
			},
		}
		s.assets[id] = asset
		return fakeAssetRow{createdAt: asset.CreatedAt}
	case strings.Contains(sql, "join site_versions sv"):
		assetID := args[1].(string)
		asset, ok := s.assets[assetID]
		if !ok {
			return fakeAssetRow{err: pgx.ErrNoRows}
		}
		if !s.publishedSites[asset.SiteID] {
			return fakeAssetRow{err: pgx.ErrNoRows}
		}
		switch {
		case strings.Contains(sql, "where s.slug = $1"):
			siteSlug := args[0].(string)
			if s.siteSlugs[asset.SiteID] != siteSlug {
				return fakeAssetRow{err: pgx.ErrNoRows}
			}
		case strings.Contains(sql, "lower(d.hostname) = $1"):
			hostname := args[0].(string)
			if strings.ToLower(s.siteHostnames[asset.SiteID]) != hostname {
				return fakeAssetRow{err: pgx.ErrNoRows}
			}
		default:
			return fakeAssetRow{err: errors.New("unexpected public asset query")}
		}
		snapshot, ok := s.publishedSnapshots[asset.SiteID]
		var snapshotJSON []byte
		if ok {
			payload, err := json.Marshal(snapshot)
			if err != nil {
				return fakeAssetRow{err: err}
			}
			snapshotJSON = payload
		} else {
			snapshotJSON = []byte(`{}`)
		}
		return fakeAssetRow{asset: &asset.Asset, snapshotJSON: snapshotJSON}
	case strings.Contains(sql, "from assets"):
		id := args[0].(string)
		asset, ok := s.assets[id]
		if !ok {
			return fakeAssetRow{err: pgx.ErrNoRows}
		}
		return fakeAssetRow{asset: &asset.Asset}
	default:
		return fakeAssetRow{err: errors.New("unexpected query row")}
	}
}

func (s *fakeAssetStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "update assets"):
		id := args[0].(string)
		asset, ok := s.assets[id]
		if !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		if strings.Contains(sql, "metadata = $3") {
			var metadata AssetMetadata
			if err := json.Unmarshal(args[2].([]byte), &metadata); err != nil {
				return pgconn.CommandTag{}, err
			}
			asset.Metadata = metadata
		}
		asset.AltText = args[1].(string)
		s.assets[id] = asset
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "delete from assets"):
		id := args[0].(string)
		if _, ok := s.assets[id]; !ok {
			return pgconn.NewCommandTag("DELETE 0"), nil
		}
		delete(s.assets, id)
		return pgconn.NewCommandTag("DELETE 1"), nil
	default:
		return pgconn.CommandTag{}, errors.New("unexpected exec")
	}
}

type fakeAssetRow struct {
	asset        *Asset
	snapshotJSON []byte
	createdAt    time.Time
	err          error
}

func (r fakeAssetRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 1 {
		createdAt := dest[0].(*time.Time)
		*createdAt = r.createdAt
		return nil
	}
	metadataJSON, err := json.Marshal(r.asset.Metadata)
	if err != nil {
		return err
	}
	*dest[0].(*string) = r.asset.ID
	*dest[1].(*string) = r.asset.WorkspaceID
	*dest[2].(*string) = r.asset.SiteID
	*dest[3].(*string) = r.asset.Kind
	*dest[4].(*string) = r.asset.StorageKey
	*dest[5].(*string) = r.asset.PublicURL
	*dest[6].(*string) = r.asset.AltText
	*dest[7].(*[]byte) = metadataJSON
	*dest[8].(*string) = r.asset.CreatedBy
	*dest[9].(*time.Time) = r.asset.CreatedAt
	if len(dest) > 10 {
		*dest[10].(*[]byte) = r.snapshotJSON
	}
	return nil
}

type fakeAssetRows struct {
	rows  []storedAsset
	index int
}

func (r *fakeAssetRows) Close() {}

func (r *fakeAssetRows) Err() error { return nil }

func (r *fakeAssetRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeAssetRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeAssetRows) Next() bool {
	return r.index < len(r.rows)
}

func (r *fakeAssetRows) Scan(dest ...any) error {
	if r.index >= len(r.rows) {
		return errors.New("out of rows")
	}
	row := fakeAssetRow{asset: &r.rows[r.index].Asset}
	r.index++
	return row.Scan(dest...)
}

func (r *fakeAssetRows) Values() ([]any, error) { return nil, errors.New("not implemented") }

func (r *fakeAssetRows) RawValues() [][]byte { return nil }

func (r *fakeAssetRows) Conn() *pgx.Conn { return nil }

func (r *fakeAssetRows) NextResultSet() bool { return false }
