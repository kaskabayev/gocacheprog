package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskCache(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocacheprog-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewDiskCache(tempDir)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	ctx := context.Background()
	actionID := "action-123"
	outputID := "output-456"
	content := "test binary payload"

	// Test 1: Put - should successfully stream content and write atomically
	path, err := cache.Put(ctx, actionID, outputID, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	expectedPath := filepath.Join(tempDir, outputsDir, outputID)
	if path != expectedPath {
		t.Errorf("unexpected output path: %s", path)
	}

	// Verify file content directly
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}

	// Test 2: Get (Cache Hit) - should resolve actionID to outputID path
	gotPath, err := cache.Get(ctx, actionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if gotPath != path {
		t.Errorf("Get returned %q, expected %q", gotPath, path)
	}

	// Test 3: Get (Cache Miss - Unknown Action)
	missPath, err := cache.Get(ctx, "unknown-action")
	if err != nil {
		t.Fatalf("Get on missing action returned error: %v", err)
	}
	if missPath != "" {
		t.Errorf("expected empty path for miss, got %q", missPath)
	}

	// Test 4: Put (Idempotent) - same output ID should bypass writing
	path2, err := cache.Put(ctx, "action-789", outputID, strings.NewReader("different content"))
	if err != nil {
		t.Fatalf("Put failed on existing output: %v", err)
	}
	if path2 != path {
		t.Errorf("expected same path on duplicate output ID, got %q", path2)
	}
	
	// Ensure it didn't overwrite the existing output file since the outputID already existed
	data2, _ := os.ReadFile(path2)
	if string(data2) != content {
		t.Errorf("Put overwrote existing output ID payload! expected %q, got %q", content, string(data2))
	}
}
