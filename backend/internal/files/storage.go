package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/yelnurq/startify/backend/internal/platform/config"
)

// ErrObjectNotFound reports a missing object.
var ErrObjectNotFound = errors.New("files: object not found")

// Storage keeps file contents.
type Storage interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// NewStorage connects to S3-compatible storage, or keeps objects in memory
// when no endpoint is configured (allowed only in tests by config validation).
func NewStorage(ctx context.Context, cfg config.S3) (Storage, error) {
	if cfg.Endpoint == "" {
		return NewMemoryStorage(), nil
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("files: s3 client: %w", err)
	}
	s := &S3Storage{client: client, bucket: cfg.Bucket}
	// The storage may still be starting next to the API (docker compose up).
	for attempt := 1; ; attempt++ {
		err = s.ensureBucket(ctx, cfg.Region)
		if err == nil || attempt == 15 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// S3Storage stores objects in an S3-compatible bucket (MinIO).
type S3Storage struct {
	client *minio.Client
	bucket string
}

func (s *S3Storage) ensureBucket(ctx context.Context, region string) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("files: bucket check: %w", err)
	}
	if exists {
		return nil
	}
	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: region})
	if err != nil && minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
		return fmt.Errorf("files: create bucket: %w", err)
	}
	return nil
}

// Put implements Storage.
func (s *S3Storage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("files: put %s: %w", key, err)
	}
	return nil
}

// Get implements Storage.
func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("files: get %s: %w", key, err)
	}
	// GetObject is lazy: Stat surfaces a missing object before streaming starts.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("files: stat %s: %w", key, err)
	}
	return obj, nil
}

// Delete implements Storage.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("files: delete %s: %w", key, err)
	}
	return nil
}

// MemoryStorage keeps objects in memory (tests).
type MemoryStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

// NewMemoryStorage returns an empty in-memory store.
func NewMemoryStorage() *MemoryStorage { return &MemoryStorage{objects: map[string][]byte{}} }

// Put implements Storage.
func (m *MemoryStorage) Put(_ context.Context, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = bytes.Clone(data)
	return nil
}

// Get implements Storage.
func (m *MemoryStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// Delete implements Storage.
func (m *MemoryStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// Len reports the number of stored objects.
func (m *MemoryStorage) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}
