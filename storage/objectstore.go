package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound is returned by ObjectStore when the object does not exist.
var ErrNotFound = errors.New("object not found")

// ObjectStore is a flat key/object store such as S3.
type ObjectStore interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key string, r io.Reader, size int64) error
}
