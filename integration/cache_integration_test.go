package integration

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/kaskabayev/gocacheprog/storage"
)

func TestCacheIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocacheprog-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := storage.NewDiskCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	testCases := []struct {
		actionID  string
		outputID  string
		content   string
		wantError bool
	}{
		{"action1", "output1", "test output 1", false},
		{"action2", "output2", "test output 2", false},
		{"action1", "output1", "test output 1 again", false},
	}

	for _, tc := range testCases {
		path, err := cache.Put(ctx, tc.actionID, tc.outputID, strings.NewReader(tc.content))
		if tc.wantError && err == nil {
			t.Errorf("Expected error for duplicate put of %s/%s", tc.actionID, tc.outputID)
		}
		if !tc.wantError && err != nil {
			t.Errorf("Unexpected error for put of %s/%s: %v", tc.actionID, tc.outputID, err)
		}
		if !tc.wantError && path == "" {
			t.Errorf("Expected non-empty path for put of %s/%s", tc.actionID, tc.outputID)
		}

		gotPath, err := cache.Get(ctx, tc.actionID)
		if err != nil {
			t.Errorf("Get error for %s: %v", tc.actionID, err)
		}
		if !tc.wantError && gotPath == "" {
			t.Errorf("Expected non-empty path from get for %s", tc.actionID)
		}
	}
}

func TestBinaryIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocacheprog-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	cache, err := storage.NewDiskCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	path, err := cache.Put(ctx, "test-action", "test-output", strings.NewReader("hello"))
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}
	if path == "" {
		t.Error("Expected non-empty path")
	}
}