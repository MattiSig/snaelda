package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	defaultAssetKind = "image"
	maxAssetNameLen  = 200
	maxAltTextLen    = 180
	maxAssetSize     = 20 << 20
)

var (
	ErrSiteRequired            = errors.New("site id is required")
	ErrAssetKindInvalid        = errors.New("asset kind is not supported")
	ErrAssetNameRequired       = errors.New("asset file name is required")
	ErrAssetContentTypeInvalid = errors.New("asset content type is not supported")
	ErrAssetSizeInvalid        = errors.New("asset size must be between 1 byte and 20 MB")
	ErrAssetNotFound           = errors.New("asset was not found")
	ErrNoAssetChanges          = errors.New("asset update requires at least one change")
	ErrAssetUploadIncomplete   = errors.New("asset upload is not complete")
	ErrAssetUploadMismatch     = errors.New("uploaded asset does not match the requested file")
)

var allowedImageContentTypes = map[string]bool{
	"image/avif": true,
	"image/gif":  true,
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Service struct {
	db             DB
	storage        Storage
	uploadURLTTL   time.Duration
	downloadURLTTL time.Duration
}

type Asset struct {
	ID          string        `json:"id"`
	WorkspaceID string        `json:"workspaceId"`
	SiteID      string        `json:"siteId,omitempty"`
	Kind        string        `json:"kind"`
	StorageKey  string        `json:"storageKey"`
	PublicURL   string        `json:"publicUrl,omitempty"`
	DownloadURL string        `json:"downloadUrl,omitempty"`
	AltText     string        `json:"altText,omitempty"`
	Metadata    AssetMetadata `json:"metadata"`
	CreatedBy   string        `json:"createdBy,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
}

type AssetMetadata struct {
	FileName           string     `json:"fileName,omitempty"`
	ContentType        string     `json:"contentType,omitempty"`
	RequestedSizeBytes int64      `json:"requestedSizeBytes,omitempty"`
	SizeBytes          int64      `json:"sizeBytes,omitempty"`
	Width              int        `json:"width,omitempty"`
	Height             int        `json:"height,omitempty"`
	ETag               string     `json:"etag,omitempty"`
	UploadStatus       string     `json:"uploadStatus,omitempty"`
	UploadedAt         *time.Time `json:"uploadedAt,omitempty"`
}

type CreateUploadInput struct {
	WorkspaceID string
	SiteID      string
	UserID      string
	Kind        string
	FileName    string
	ContentType string
	SizeBytes   int64
	AltText     string
}

type CreateUploadResult struct {
	Asset  Asset           `json:"asset"`
	Upload PresignedUpload `json:"upload"`
}

type CompleteUploadInput struct {
	AltText *string
	Width   *int
	Height  *int
}

type UpdateAssetInput struct {
	AltText *string
}

func NewService(db DB, storage Storage) *Service {
	return &Service{
		db:             db,
		storage:        storage,
		uploadURLTTL:   defaultUploadURLTTL,
		downloadURLTTL: defaultDownloadURLTTL,
	}
}

func (s *Service) CreateUpload(ctx context.Context, input CreateUploadInput) (CreateUploadResult, error) {
	if strings.TrimSpace(input.SiteID) == "" {
		return CreateUploadResult{}, ErrSiteRequired
	}

	kind := strings.TrimSpace(input.Kind)
	if kind == "" {
		kind = defaultAssetKind
	}
	if kind != defaultAssetKind {
		return CreateUploadResult{}, ErrAssetKindInvalid
	}

	fileName := sanitizeFileName(input.FileName)
	if fileName == "" {
		return CreateUploadResult{}, ErrAssetNameRequired
	}

	contentType := strings.ToLower(strings.TrimSpace(input.ContentType))
	if !allowedImageContentTypes[contentType] {
		return CreateUploadResult{}, ErrAssetContentTypeInvalid
	}
	if input.SizeBytes <= 0 || input.SizeBytes > maxAssetSize {
		return CreateUploadResult{}, ErrAssetSizeInvalid
	}

	assetID, err := ids.New()
	if err != nil {
		return CreateUploadResult{}, fmt.Errorf("generate asset id: %w", err)
	}
	storageKey := buildStorageKey(input.WorkspaceID, input.SiteID, assetID, fileName)

	upload, err := s.storage.CreateUpload(ctx, storageKey, contentType, s.uploadURLTTL)
	if err != nil {
		return CreateUploadResult{}, err
	}

	metadata := AssetMetadata{
		FileName:           fileName,
		ContentType:        contentType,
		RequestedSizeBytes: input.SizeBytes,
		UploadStatus:       "pending",
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return CreateUploadResult{}, fmt.Errorf("encode asset metadata: %w", err)
	}

	asset := Asset{
		ID:          assetID,
		WorkspaceID: input.WorkspaceID,
		SiteID:      input.SiteID,
		Kind:        kind,
		StorageKey:  storageKey,
		AltText:     normalizeAltText(input.AltText),
		Metadata:    metadata,
		CreatedBy:   strings.TrimSpace(input.UserID),
	}

	if err := s.db.QueryRow(ctx, `
		insert into assets (id, workspace_id, site_id, kind, storage_key, alt_text, metadata, created_by)
		values ($1, $2, $3, $4, $5, nullif($6, ''), $7, nullif($8, '')::uuid)
		returning created_at
	`, asset.ID, asset.WorkspaceID, asset.SiteID, asset.Kind, asset.StorageKey, asset.AltText, metadataJSON, asset.CreatedBy).Scan(&asset.CreatedAt); err != nil {
		return CreateUploadResult{}, fmt.Errorf("create asset upload: %w", err)
	}

	return CreateUploadResult{Asset: asset, Upload: upload}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, assetID string, input CompleteUploadInput) (Asset, error) {
	asset, err := s.loadAsset(ctx, assetID)
	if err != nil {
		return Asset{}, err
	}

	head, err := s.storage.HeadObject(ctx, asset.StorageKey)
	if err != nil {
		return Asset{}, ErrAssetUploadIncomplete
	}

	expectedType := strings.ToLower(strings.TrimSpace(asset.Metadata.ContentType))
	if expectedType != "" && !strings.EqualFold(expectedType, head.ContentType) {
		return Asset{}, ErrAssetUploadMismatch
	}
	if asset.Metadata.RequestedSizeBytes > 0 && asset.Metadata.RequestedSizeBytes != head.SizeBytes {
		return Asset{}, ErrAssetUploadMismatch
	}

	asset.Metadata.SizeBytes = head.SizeBytes
	asset.Metadata.ETag = head.ETag
	asset.Metadata.UploadStatus = "uploaded"
	now := time.Now().UTC()
	asset.Metadata.UploadedAt = &now
	if input.Width != nil && *input.Width > 0 {
		asset.Metadata.Width = *input.Width
	}
	if input.Height != nil && *input.Height > 0 {
		asset.Metadata.Height = *input.Height
	}
	if input.AltText != nil {
		asset.AltText = normalizeAltText(*input.AltText)
	}

	metadataJSON, err := json.Marshal(asset.Metadata)
	if err != nil {
		return Asset{}, fmt.Errorf("encode asset metadata: %w", err)
	}

	if _, err := s.db.Exec(ctx, `
		update assets
		set alt_text = nullif($2, ''),
		    metadata = $3
		where id = $1
	`, asset.ID, asset.AltText, metadataJSON); err != nil {
		return Asset{}, fmt.Errorf("complete asset upload: %w", err)
	}

	asset.DownloadURL, err = s.downloadURLForAsset(ctx, asset)
	if err != nil {
		return Asset{}, err
	}

	return asset, nil
}

func (s *Service) ListBySite(ctx context.Context, siteID string) ([]Asset, error) {
	rows, err := s.db.Query(ctx, `
		select id::text,
		       workspace_id::text,
		       coalesce(site_id::text, ''),
		       kind,
		       storage_key,
		       coalesce(public_url, ''),
		       coalesce(alt_text, ''),
		       metadata,
		       coalesce(created_by::text, ''),
		       created_at
		from assets
		where site_id = $1
		order by created_at desc, id desc
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list site assets: %w", err)
	}
	defer rows.Close()

	assets := []Asset{}
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		if asset.Metadata.UploadStatus == "uploaded" {
			asset.DownloadURL, err = s.downloadURLForAsset(ctx, asset)
			if err != nil {
				return nil, err
			}
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate site assets: %w", err)
	}
	return assets, nil
}

func (s *Service) Update(ctx context.Context, assetID string, input UpdateAssetInput) (Asset, error) {
	if input.AltText == nil {
		return Asset{}, ErrNoAssetChanges
	}

	asset, err := s.loadAsset(ctx, assetID)
	if err != nil {
		return Asset{}, err
	}
	asset.AltText = normalizeAltText(*input.AltText)

	tag, err := s.db.Exec(ctx, `
		update assets
		set alt_text = nullif($2, '')
		where id = $1
	`, asset.ID, asset.AltText)
	if err != nil {
		return Asset{}, fmt.Errorf("update asset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Asset{}, ErrAssetNotFound
	}

	if asset.Metadata.UploadStatus == "uploaded" {
		asset.DownloadURL, err = s.downloadURLForAsset(ctx, asset)
		if err != nil {
			return Asset{}, err
		}
	}

	return asset, nil
}

func (s *Service) DownloadURL(ctx context.Context, assetID string) (string, error) {
	asset, err := s.loadAsset(ctx, assetID)
	if err != nil {
		return "", err
	}
	return s.downloadURLForAsset(ctx, asset)
}

func (s *Service) PublicDownloadURLBySiteSlug(ctx context.Context, siteSlug string, assetID string) (string, error) {
	asset, err := scanAssetRow(s.db.QueryRow(ctx, `
		select a.id::text,
		       a.workspace_id::text,
		       coalesce(a.site_id::text, ''),
		       a.kind,
		       a.storage_key,
		       coalesce(a.public_url, ''),
		       coalesce(a.alt_text, ''),
		       a.metadata,
		       coalesce(a.created_by::text, ''),
		       a.created_at
		from assets a
		join sites s on s.id = a.site_id
		where s.slug = $1
		  and a.id = $2
		  and s.published_version_id is not null
	`, strings.TrimSpace(siteSlug), strings.TrimSpace(assetID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAssetNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load public asset: %w", err)
	}
	return s.downloadURLForAsset(ctx, asset)
}

func (s *Service) Delete(ctx context.Context, assetID string) error {
	asset, err := s.loadAsset(ctx, assetID)
	if err != nil {
		return err
	}

	if err := s.storage.DeleteObject(ctx, asset.StorageKey); err != nil {
		return err
	}

	tag, err := s.db.Exec(ctx, `delete from assets where id = $1`, asset.ID)
	if err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (s *Service) loadAsset(ctx context.Context, assetID string) (Asset, error) {
	asset, err := scanAssetRow(s.db.QueryRow(ctx, `
		select id::text,
		       workspace_id::text,
		       coalesce(site_id::text, ''),
		       kind,
		       storage_key,
		       coalesce(public_url, ''),
		       coalesce(alt_text, ''),
		       metadata,
		       coalesce(created_by::text, ''),
		       created_at
		from assets
		where id = $1
	`, assetID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, ErrAssetNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("load asset: %w", err)
	}
	return asset, nil
}

type assetScanner interface {
	Scan(dest ...any) error
}

func scanAssetRow(row assetScanner) (Asset, error) {
	asset, err := scanAsset(row)
	if err != nil {
		return Asset{}, err
	}
	return asset, nil
}

func scanAsset(row assetScanner) (Asset, error) {
	var asset Asset
	var metadataJSON []byte
	if err := row.Scan(
		&asset.ID,
		&asset.WorkspaceID,
		&asset.SiteID,
		&asset.Kind,
		&asset.StorageKey,
		&asset.PublicURL,
		&asset.AltText,
		&metadataJSON,
		&asset.CreatedBy,
		&asset.CreatedAt,
	); err != nil {
		return Asset{}, err
	}
	if len(metadataJSON) == 0 {
		metadataJSON = []byte(`{}`)
	}
	if err := json.Unmarshal(metadataJSON, &asset.Metadata); err != nil {
		return Asset{}, fmt.Errorf("decode asset metadata: %w", err)
	}
	return asset, nil
}

func sanitizeFileName(value string) string {
	name := strings.TrimSpace(filepath.Base(value))
	if name == "." || name == "/" {
		return ""
	}
	if len(name) > maxAssetNameLen {
		name = name[:maxAssetNameLen]
	}
	return name
}

func buildStorageKey(workspaceID string, siteID string, assetID string, fileName string) string {
	return fmt.Sprintf("workspaces/%s/sites/%s/assets/%s/%s", workspaceID, siteID, assetID, fileName)
}

func normalizeAltText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxAltTextLen {
		return value[:maxAltTextLen]
	}
	return value
}

func (s *Service) downloadURLForAsset(ctx context.Context, asset Asset) (string, error) {
	if asset.Metadata.UploadStatus != "uploaded" {
		return "", ErrAssetUploadIncomplete
	}
	if strings.TrimSpace(asset.PublicURL) != "" {
		return asset.PublicURL, nil
	}
	return s.storage.CreateDownloadURL(ctx, asset.StorageKey, s.downloadURLTTL)
}
