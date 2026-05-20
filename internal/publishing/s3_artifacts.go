package publishing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3ArtifactStoreConfig configures the S3-backed artifact store.
type S3ArtifactStoreConfig struct {
	// Client is the AWS S3 client used for object operations. Required.
	Client *s3.Client
	// Bucket is the S3 bucket name where artifacts are persisted. Required.
	Bucket string
	// Prefix is an optional key prefix used to namespace artifacts (e.g.
	// "published-artifacts/") so they do not collide with other tenants of the
	// same bucket. Leading and trailing slashes are normalized.
	Prefix string
}

type s3ArtifactStore struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3ArtifactStore returns an ArtifactStore that persists published artifacts
// to an S3-compatible object store (e.g. SeaweedFS in local dev, AWS S3 in
// production). Saved bundles are written under a deterministic
// `<prefix>/<siteID>/<versionID>/<path>` key structure, which makes published
// versions immutable and cleanly addressable by the public render path.
func NewS3ArtifactStore(cfg S3ArtifactStoreConfig) (ArtifactStore, error) {
	if cfg.Client == nil {
		return nil, errors.New("s3 artifact store: client is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("s3 artifact store: bucket is required")
	}
	return &s3ArtifactStore{
		client: cfg.Client,
		bucket: strings.TrimSpace(cfg.Bucket),
		prefix: normalizeKeyPrefix(cfg.Prefix),
	}, nil
}

func (s *s3ArtifactStore) Save(ctx context.Context, siteID string, versionID string, bundle ArtifactBundle) error {
	if strings.TrimSpace(siteID) == "" {
		return fmt.Errorf("save artifacts: site id is required")
	}
	if strings.TrimSpace(versionID) == "" {
		return fmt.Errorf("save artifacts: version id is required")
	}
	if err := s.ensureBucket(ctx); err != nil {
		return err
	}

	for _, file := range bundle.Files {
		key, err := s.buildObjectKey(siteID, versionID, file.Path)
		if err != nil {
			return err
		}
		body := []byte(file.Body)
		input := &s3.PutObjectInput{
			Bucket:        aws.String(s.bucket),
			Key:           aws.String(key),
			Body:          bytes.NewReader(body),
			ContentLength: aws.Int64(int64(len(body))),
			ContentType:   aws.String(resolveArtifactContentType(file)),
			CacheControl:  aws.String("public, max-age=31536000, immutable"),
		}
		if _, err := s.client.PutObject(ctx, input); err != nil {
			return fmt.Errorf("write artifact %s: %w", file.Path, err)
		}
	}
	return nil
}

func (s *s3ArtifactStore) Load(ctx context.Context, siteID string, versionID string, artifactPath string) (ArtifactFile, error) {
	if strings.TrimSpace(siteID) == "" {
		return ArtifactFile{}, fmt.Errorf("load artifacts: site id is required")
	}
	if strings.TrimSpace(versionID) == "" {
		return ArtifactFile{}, fmt.Errorf("load artifacts: version id is required")
	}
	key, err := s.buildObjectKey(siteID, versionID, artifactPath)
	if err != nil {
		return ArtifactFile{}, err
	}

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return ArtifactFile{}, ErrArtifactNotFound
		}
		return ArtifactFile{}, fmt.Errorf("load artifact %s: %w", artifactPath, err)
	}
	defer output.Body.Close()

	body, err := io.ReadAll(output.Body)
	if err != nil {
		return ArtifactFile{}, fmt.Errorf("read artifact %s: %w", artifactPath, err)
	}
	contentType := strings.TrimSpace(aws.ToString(output.ContentType))
	if contentType == "" {
		contentType = contentTypeForArtifactPath(artifactPath)
	}
	return ArtifactFile{
		Path:        strings.TrimSpace(artifactPath),
		ContentType: contentType,
		Body:        string(body),
	}, nil
}

func (s *s3ArtifactStore) Delete(ctx context.Context, siteID string, versionID string) error {
	if strings.TrimSpace(siteID) == "" || strings.TrimSpace(versionID) == "" {
		return nil
	}
	prefix := s.buildVersionPrefix(siteID, versionID)

	for {
		list, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String(prefix),
		})
		if err != nil {
			return fmt.Errorf("list artifacts for cleanup: %w", err)
		}
		if len(list.Contents) == 0 {
			return nil
		}
		objects := make([]s3types.ObjectIdentifier, 0, len(list.Contents))
		for _, object := range list.Contents {
			objects = append(objects, s3types.ObjectIdentifier{Key: object.Key})
		}
		if _, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &s3types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		}); err != nil {
			return fmt.Errorf("delete artifacts: %w", err)
		}
		if aws.ToBool(list.IsTruncated) {
			continue
		}
		return nil
	}
}

func (s *s3ArtifactStore) ensureBucket(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	}); err == nil {
		return nil
	}
	if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	}); err != nil {
		if _, headErr := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String(s.bucket),
		}); headErr != nil {
			return fmt.Errorf("ensure artifact bucket %q: %w", s.bucket, err)
		}
	}
	return nil
}

func (s *s3ArtifactStore) buildVersionPrefix(siteID string, versionID string) string {
	parts := []string{}
	if s.prefix != "" {
		parts = append(parts, s.prefix)
	}
	parts = append(parts, strings.TrimSpace(siteID), strings.TrimSpace(versionID))
	return strings.TrimPrefix(path.Join(parts...), "/") + "/"
}

func (s *s3ArtifactStore) buildObjectKey(siteID string, versionID string, relativePath string) (string, error) {
	clean := strings.TrimSpace(relativePath)
	if clean == "" {
		return "", fmt.Errorf("artifact path is required")
	}
	clean = path.Clean(clean)
	if clean == "." || clean == "/" {
		return "", fmt.Errorf("artifact path is required")
	}
	if strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("artifact path %q must be relative", relativePath)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("artifact path %q escapes the artifact root", relativePath)
	}
	return s.buildVersionPrefix(siteID, versionID) + clean, nil
}

func normalizeKeyPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, "/")
	return trimmed
}

func isNotFound(err error) bool {
	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var noKey *s3types.NoSuchKey
	if errors.As(err, &noKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchBucket":
			return true
		}
	}
	return false
}

func resolveArtifactContentType(file ArtifactFile) string {
	if value := strings.TrimSpace(file.ContentType); value != "" {
		return value
	}
	return contentTypeForArtifactPath(file.Path)
}

func contentTypeForArtifactPath(artifactPath string) string {
	switch strings.ToLower(path.Ext(strings.TrimSpace(artifactPath))) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
