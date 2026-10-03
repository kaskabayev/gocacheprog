package storage

import (
	"context"
	"io"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3ObjectStore is an ObjectStore backed by any S3-compatible service.
type S3ObjectStore struct {
	client *minio.Client
	bucket string
}

// NewS3ObjectStore connects to an S3-compatible endpoint. endpoint may be a
// bare host ("s3.amazonaws.com") or a URL ("https://minio.example:9000").
// Credentials come from s3AccessKey/s3SecretKey when either is set, else
// from the standard MinIO and AWS environment variables, else from instance
// IAM roles. No network calls are made here.
func NewS3ObjectStore(endpoint, bucket, accessKey, secretKey string) (*S3ObjectStore, error) {
	secure := true
	if u, err := url.Parse(endpoint); err == nil && u.Scheme != "" {
		secure = u.Scheme == "https"
		endpoint = u.Host
	}
	creds := credentials.NewChainCredentials([]credentials.Provider{
		&credentials.EnvMinio{},
		&credentials.EnvAWS{},
		&credentials.IAM{},
	})
	if accessKey != "" || secretKey != "" {
		creds = credentials.NewStaticV4(accessKey, secretKey, "")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  creds,
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}
	return &S3ObjectStore{client: client, bucket: bucket}, nil
}

// GetObject returns a reader for the object, or ErrNotFound if it does not
// exist.
func (s *S3ObjectStore) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// minio clients read lazily; force the request to surface errors now.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		if e := minio.ToErrorResponse(err); e.Code == "NoSuchKey" || e.StatusCode == 404 {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return obj, nil
}

// PutObject stores the object atomically.
func (s *S3ObjectStore) PutObject(ctx context.Context, key string, r io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{})
	return err
}
