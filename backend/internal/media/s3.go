package media

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/mooc-platform/backend/internal/config"
)

type StorageService struct {
	client      *minio.Client
	mediaBucket string
	badgeBucket string
}

func NewStorageService(cfg *config.Config) (*StorageService, error) {
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOSecure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init MinIO client: %w", err)
	}

	return &StorageService{
		client:      client,
		mediaBucket: "mooc-media",
		badgeBucket: "mooc-badges",
	}, nil
}

type PresignedUploadOutput struct {
	UploadURL string            `json:"upload_url"`
	ObjectKey string            `json:"object_key"`
	ExpiresAt time.Time         `json:"expires_at"`
	Headers   map[string]string `json:"headers,omitempty"`
}

func (s *StorageService) GeneratePresignedUploadURL(ctx context.Context, objectKey string, contentType string) (*PresignedUploadOutput, error) {
	expiry := 24 * time.Hour

	reqParams := make(url.Values)
	if contentType != "" {
		reqParams.Set("response-content-type", contentType)
	}

	presignedURL, err := s.client.PresignedPutObject(ctx, s.mediaBucket, objectKey, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned upload url: %w", err)
	}

	return &PresignedUploadOutput{
		UploadURL: presignedURL.String(),
		ObjectKey: objectKey,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

func (s *StorageService) GeneratePresignedDownloadURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = 1 * time.Hour
	}

	presignedURL, err := s.client.PresignedGetObject(ctx, s.mediaBucket, objectKey, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download url: %w", err)
	}

	return presignedURL.String(), nil
}

func (s *StorageService) StatObject(ctx context.Context, objectKey string) (minio.ObjectInfo, error) {
	return s.client.StatObject(ctx, s.mediaBucket, objectKey, minio.StatObjectOptions{})
}

func (s *StorageService) GetClient() *minio.Client {
	return s.client
}

func (s *StorageService) GetMediaBucket() string {
	return s.mediaBucket
}
