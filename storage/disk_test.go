package storage

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestDiskCacheBasicFlow(t *testing.T) {
	cache := newTestDiskCache(t)
	ctx := context.Background()
	paths := map[string]string{}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "stores a new output",
			run: func(t *testing.T) {
				path := mustPut(t, cache, ctx, "action-1", "output-1", "binary payload 1")
				paths["action-1"] = path
				assertFileContents(t, path, "binary payload 1")
			},
		},
		{
			name: "gets an existing action",
			run: func(t *testing.T) {
				path := mustGet(t, cache, ctx, "action-1")
				if path != paths["action-1"] {
					t.Fatalf("Get(action-1) = %q, want %q", path, paths["action-1"])
				}
			},
		},
		{
			name: "misses an unknown action",
			run: func(t *testing.T) {
				assertMiss(t, cache, ctx, "unknown-action")
			},
		},
		{
			name: "reuses an existing output and records its mapping",
			run: func(t *testing.T) {
				path := mustPut(t, cache, ctx, "action-2", "output-1", "different content")
				paths["action-2"] = path
				if path != paths["action-1"] {
					t.Fatalf("Put(action-2) path = %q, want %q", path, paths["action-1"])
				}
				assertFileContents(t, path, "binary payload 1")

				mappedPath := mustGet(t, cache, ctx, "action-2")
				if mappedPath != paths["action-1"] {
					t.Fatalf("Get(action-2) = %q, want %q", mappedPath, paths["action-1"])
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, tt.run)
	}
}

func TestDiskCacheConcurrentPuts(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, cache *DiskCache, ctx context.Context)
	}{
		{
			name: "same action and output keep their bytes intact",
			run: func(t *testing.T, cache *DiskCache, ctx context.Context) {
				const workers = 50
				const actionID = "race-action-shared"
				const outputID = "race-output-shared"
				const content = "highly concurrent payload"

				var wg sync.WaitGroup
				errCh := make(chan error, workers)
				wg.Add(workers)

				for i := 0; i < workers; i++ {
					go func() {
						defer wg.Done()
						_, err := cache.Put(ctx, actionID, outputID, strings.NewReader(content))
						errCh <- err
					}()
				}

				wg.Wait()
				close(errCh)

				for err := range errCh {
					if err != nil {
						t.Fatalf("concurrent Put failed: %v", err)
					}
				}

				path := mustGet(t, cache, ctx, actionID)
				assertFileContents(t, path, content)
			},
		},
		{
			name: "first publish wins for a new output",
			run: func(t *testing.T, cache *DiskCache, ctx context.Context) {
				const actionSlow = "race-action-slow"
				const actionFast = "race-action-fast"
				const outputID = "race-output-first-publish"
				const slowContent = "slow payload"
				const fastContent = "fast payload"

				slowStarted := make(chan struct{})
				slowRelease := make(chan struct{})
				errCh := make(chan error, 1)

				go func() {
					_, err := cache.Put(ctx, actionSlow, outputID, &gatedReader{
						payload: []byte(slowContent),
						started: slowStarted,
						release: slowRelease,
					})
					errCh <- err
				}()

				<-slowStarted

				fastPath := mustPut(t, cache, ctx, actionFast, outputID, fastContent)

				close(slowRelease)
				if err := <-errCh; err != nil {
					t.Fatalf("slow Put failed: %v", err)
				}

				slowPath := mustGet(t, cache, ctx, actionSlow)
				if slowPath != fastPath {
					t.Fatalf("Get(%q) = %q, want %q", actionSlow, slowPath, fastPath)
				}

				assertFileContents(t, fastPath, fastContent)
			},
		},
		{
			name: "distinct actions can point at one existing output",
			run: func(t *testing.T, cache *DiskCache, ctx context.Context) {
				const workers = 50
				const seedAction = "race-action-seed"
				const outputID = "race-output-existing"
				const seedContent = "seed payload"

				sharedPath := mustPut(t, cache, ctx, seedAction, outputID, seedContent)

				var wg sync.WaitGroup
				errCh := make(chan error, workers)
				actionIDs := make([]string, workers)
				wg.Add(workers)

				for i := 0; i < workers; i++ {
					actionID := "race-action-distinct-" + strconv.Itoa(i)
					actionIDs[i] = actionID

					go func(actionID string) {
						defer wg.Done()
						_, err := cache.Put(ctx, actionID, outputID, strings.NewReader("different payload "+actionID))
						errCh <- err
					}(actionID)
				}

				wg.Wait()
				close(errCh)

				for err := range errCh {
					if err != nil {
						t.Fatalf("concurrent Put failed: %v", err)
					}
				}

				for _, actionID := range actionIDs {
					path := mustGet(t, cache, ctx, actionID)
					if path != sharedPath {
						t.Fatalf("Get(%q) = %q, want %q", actionID, path, sharedPath)
					}
				}

				assertFileContents(t, sharedPath, seedContent)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cache := newTestDiskCache(t)
			tt.run(t, cache, context.Background())
		})
	}
}

type gatedReader struct {
	payload []byte
	started chan struct{}
	release <-chan struct{}
	once    sync.Once
	blocked bool
}

func (r *gatedReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
	})

	if !r.blocked {
		r.blocked = true
		n := copy(p, r.payload)
		r.payload = r.payload[n:]
		return n, nil
	}

	<-r.release
	if len(r.payload) == 0 {
		return 0, io.EOF
	}

	n := copy(p, r.payload)
	r.payload = r.payload[n:]
	if len(r.payload) == 0 {
		return n, nil
	}

	return n, nil
}

func newTestDiskCache(t *testing.T) *DiskCache {
	t.Helper()

	cache, err := NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	return cache
}

func mustPut(t *testing.T, cache *DiskCache, ctx context.Context, actionID string, outputID string, content string) string {
	t.Helper()

	path, err := cache.Put(ctx, actionID, outputID, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Put(%q, %q) failed: %v", actionID, outputID, err)
	}

	return path
}

func mustGet(t *testing.T, cache *DiskCache, ctx context.Context, actionID string) string {
	t.Helper()

	path, err := cache.Get(ctx, actionID)
	if err != nil {
		t.Fatalf("Get(%q) failed: %v", actionID, err)
	}
	if path == "" {
		t.Fatalf("Get(%q) returned a miss", actionID)
	}

	return path
}

func assertMiss(t *testing.T, cache *DiskCache, ctx context.Context, actionID string) {
	t.Helper()

	path, err := cache.Get(ctx, actionID)
	if err != nil {
		t.Fatalf("Get(%q) failed: %v", actionID, err)
	}
	if path != "" {
		t.Fatalf("Get(%q) = %q, want miss", actionID, path)
	}
}

func assertFileContents(t *testing.T, path string, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %q: %v", path, err)
	}
	if got := string(data); got != want {
		t.Fatalf("contents of %q = %q, want %q", path, got, want)
	}
}
