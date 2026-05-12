package assets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultUploadURLTTL   = 15 * time.Minute
	defaultDownloadURLTTL = 15 * time.Minute
)

type StorageConfig struct {
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

type PresignedUpload struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type ObjectHead struct {
	ContentType string
	SizeBytes   int64
	ETag        string
}

type Storage interface {
	CreateUpload(ctx context.Context, key string, contentType string, expires time.Duration) (PresignedUpload, error)
	CreateDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error)
	HeadObject(ctx context.Context, key string) (ObjectHead, error)
	DeleteObject(ctx context.Context, key string) error
}

type S3Storage struct {
	bucket  string
	client  *s3.Client
	presign *s3.PresignClient
}

func NewS3Storage(cfg StorageConfig) (*S3Storage, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("asset storage bucket is required")
	}
	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = "us-east-1"
	}

	awsConfig, err := awscfg.LoadDefaultConfig(
		context.Background(),
		awscfg.WithRegion(cfg.Region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = cfg.ForcePathStyle
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &S3Storage{
		bucket:  cfg.Bucket,
		client:  client,
		presign: s3.NewPresignClient(client),
	}, nil
}

func (s *S3Storage) CreateUpload(ctx context.Context, key string, contentType string, expires time.Duration) (PresignedUpload, error) {
	if err := s.ensureBucket(ctx); err != nil {
		return PresignedUpload{}, err
	}
	if expires <= 0 {
		expires = defaultUploadURLTTL
	}

	request, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return PresignedUpload{}, fmt.Errorf("presign upload url: %w", err)
	}

	return PresignedUpload{
		URL:       request.URL,
		Method:    "PUT",
		Headers:   signedHeadersMap(request.SignedHeader),
		ExpiresAt: time.Now().UTC().Add(expires),
	}, nil
}

func (s *S3Storage) CreateDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if expires <= 0 {
		expires = defaultDownloadURLTTL
	}

	request, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("presign download url: %w", err)
	}
	return request.URL, nil
}

func (s *S3Storage) HeadObject(ctx context.Context, key string) (ObjectHead, error) {
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectHead{}, fmt.Errorf("head object: %w", err)
	}

	return ObjectHead{
		ContentType: aws.ToString(result.ContentType),
		SizeBytes:   aws.ToInt64(result.ContentLength),
		ETag:        strings.Trim(aws.ToString(result.ETag), `"`),
	}, nil
}

func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (s *S3Storage) ensureBucket(ctx context.Context) error {
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
			return fmt.Errorf("ensure bucket %q: %w", s.bucket, err)
		}
	}

	return nil
}

func signedHeadersMap(values map[string][]string) map[string]string {
	headers := map[string]string{}
	for key, value := range values {
		if strings.EqualFold(key, "host") || len(value) == 0 {
			continue
		}
		headers[key] = strings.Join(value, ", ")
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}
