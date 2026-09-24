package media

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
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
	endpointHost := cfg.MinIOEndpoint
	opts := minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOSecure,
		Region: cfg.S3Region,
	}

	if cfg.ExternalMinIOEndpoint != "" {
		u, err := url.Parse(cfg.ExternalMinIOEndpoint)
		if err == nil && u.Host != "" {
			endpointHost = u.Host
			dialTarget := cfg.MinIOEndpoint
			opts.Transport = &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if strings.HasPrefix(addr, endpointHost) {
						addr = dialTarget
					}
					var dialer net.Dialer
					return dialer.DialContext(ctx, network, addr)
				},
			}
		}
	}

	client, err := minio.New(endpointHost, &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to init storage client: %w", err)
	}

	mediaBucket := cfg.MediaBucket
	if mediaBucket == "" {
		mediaBucket = "mooc-media"
	}
	badgeBucket := cfg.BadgeBucket
	if badgeBucket == "" {
		badgeBucket = "mooc-badges"
	}

	return &StorageService{
		client:      client,
		mediaBucket: mediaBucket,
		badgeBucket: badgeBucket,
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

func (s *StorageService) GeneratePresignedUpload(ctx context.Context, objectKey string, contentType string) (string, error) {
	out, err := s.GeneratePresignedUploadURL(ctx, objectKey, contentType)
	if err != nil {
		return "", err
	}
	return out.UploadURL, nil
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
