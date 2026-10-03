package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"testing"
)

// fakeStore is an in-memory ObjectStore with fault injection.
type fakeStore struct {
	mu       sync.Mutex
	objects  map[string][]byte
	getCalls int
	getErr   error
	putErr   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}}
}

func (f *fakeStore) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeStore) PutObject(_ context.Context, key string, r io.Reader, _ int64) error {
	if f.putErr != nil {
		return f.putErr
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = data
	return nil
}

func (f *fakeStore) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func TestS3CachePutUploadsOutputAndMapping(t *testing.T) {
	store := newFakeStore()
	cache := NewS3Cache(newTestDiskCache(t), store, "pre")

	mustPut(t, cache, context.Background(), "aa11", "bb21", "payload")
	cache.Close()

	if got := string(store.objects["pre/outputs/bb21"]); got != "payload" {
		t.Fatalf("remote output = %q, want %q", got, "payload")
	}
	if got := string(store.objects["pre/actions/aa11"]); got != "bb21" {
		t.Fatalf("remote mapping = %q, want %q", got, "bb21")
	}
}

func TestS3CacheGetLocalHitSkipsStore(t *testing.T) {
	store := newFakeStore()
	cache := NewS3Cache(newTestDiskCache(t), store, "pre")

	// Warm the local disk directly so no background upload is queued.
	mustPut(t, cache.DiskCache, context.Background(), "aa11", "bb21", "payload")

	path, err := cache.Get(context.Background(), "aa11")
	if err != nil || path == "" {
		t.Fatalf("Get = %q, %v; want local hit", path, err)
	}
	store.mu.Lock()
	calls := store.getCalls
	store.mu.Unlock()
	if calls != 0 {
		t.Fatalf("store was consulted %d times on a local hit", calls)
	}
	cache.Close()
}

func TestS3CacheGetFallsBackToStore(t *testing.T) {
	store := newFakeStore()
	store.objects["pre/actions/aa11"] = []byte("bb21")
	store.objects["pre/outputs/bb21"] = []byte("remote payload")
	cache := NewS3Cache(newTestDiskCache(t), store, "pre")

	path := mustGet(t, cache, context.Background(), "aa11")
	assertFileContents(t, path, "remote payload")

	// The downloaded entry is now local: it survives store failures.
	store.getErr = ErrNotFound
	mustGet(t, cache, context.Background(), "aa11")
	cache.Close()
}

func TestS3CacheGetCorruptRemoteMappingIsMiss(t *testing.T) {
	store := newFakeStore()
	store.objects["pre/actions/aa11"] = []byte("../../evil")
	cache := NewS3Cache(newTestDiskCache(t), store, "pre")

	assertMiss(t, cache, context.Background(), "aa11")
	cache.Close()
}

func TestS3CacheStoreFailuresNeverFailRequests(t *testing.T) {
	store := newFakeStore()
	store.getErr = ErrNotFound
	store.putErr = ErrNotFound
	cache := NewS3Cache(newTestDiskCache(t), store, "pre")

	assertMiss(t, cache, context.Background(), "aa11")
	path := mustPut(t, cache, context.Background(), "aa11", "bb21", "payload")
	assertFileContents(t, path, "payload")
	cache.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("local entry missing after failed upload: %v", err)
	}
}
