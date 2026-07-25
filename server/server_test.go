package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaskabayev/gocacheprog/protocol"
	"github.com/kaskabayev/gocacheprog/storage"
)

var _ storage.CacheStorage = (*mockCache)(nil)

type mockCache struct {
	getPath string
	getErr  error
	putPath string
	putErr  error
}

func (m *mockCache) Get(_ context.Context, _ string) (string, error) {
	return m.getPath, m.getErr
}

func (m *mockCache) Put(_ context.Context, _, _ string, _ io.Reader) (string, error) {
	return m.putPath, m.putErr
}

func (m *mockCache) Close() error { return nil }

func handleAndGetResp(t *testing.T, srv *Server, buf *bytes.Buffer, req protocol.Request) protocol.Response {
	t.Helper()
	srv.handleRequest(context.Background(), req)
	srv.writer.Flush()

	var resp protocol.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func TestHandleRequestGetMissWhenFileVanishes(t *testing.T) {
	dir := t.TempDir()
	fakePath := filepath.Join(dir, "nonexistent")

	cache := &mockCache{getPath: fakePath}
	var buf bytes.Buffer
	srv := NewServer(cache, bufio.NewReader(nil), bufio.NewWriter(&buf))

	resp := handleAndGetResp(t, srv, &buf, protocol.Request{
		ID:       1,
		Command:  "get",
		ActionID: []byte("action1"),
	})

	if !resp.Miss {
		t.Fatalf("expected miss, got hit with DiskPath=%q", resp.DiskPath)
	}
	if resp.DiskPath != "" {
		t.Fatalf("expected empty DiskPath, got %q", resp.DiskPath)
	}
	if resp.Err != "" {
		t.Fatalf("expected no error, got %q", resp.Err)
	}
}

func TestHandleRequestPutErrorWhenFileVanishes(t *testing.T) {
	dir := t.TempDir()
	fakePath := filepath.Join(dir, "nonexistent")

	cache := &mockCache{putPath: fakePath}
	var buf bytes.Buffer
	srv := NewServer(cache, bufio.NewReader(nil), bufio.NewWriter(&buf))

	resp := handleAndGetResp(t, srv, &buf, protocol.Request{
		ID:       1,
		Command:  "put",
		ActionID: []byte("action1"),
		OutputID: []byte("output1"),
		Body:     strings.NewReader("content"),
	})

	if resp.Err == "" {
		t.Fatalf("expected error when file vanishes after put, got none")
	}
	if resp.DiskPath != "" {
		t.Fatalf("expected empty DiskPath on error, got %q", resp.DiskPath)
	}
}

func TestHandleRequestGetHit(t *testing.T) {
	dir := t.TempDir()
	outputID := "abcdef1234567890"
	outputPath := filepath.Join(dir, outputID)

	content := []byte("test content")
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		t.Fatalf("failed to create output file: %v", err)
	}

	cache := &mockCache{getPath: outputPath}
	var buf bytes.Buffer
	srv := NewServer(cache, bufio.NewReader(nil), bufio.NewWriter(&buf))

	resp := handleAndGetResp(t, srv, &buf, protocol.Request{
		ID:       1,
		Command:  "get",
		ActionID: []byte("action1"),
	})

	if resp.Miss {
		t.Fatalf("expected hit, got miss")
	}
	if resp.DiskPath != outputPath {
		t.Fatalf("expected DiskPath=%q, got %q", outputPath, resp.DiskPath)
	}
	if resp.Size != int64(len(content)) {
		t.Fatalf("expected Size=%d, got %d", len(content), resp.Size)
	}
	if resp.Time == nil {
		t.Fatalf("expected non-nil Time")
	}
	if resp.Err != "" {
		t.Fatalf("expected no error, got %q", resp.Err)
	}

	expectedOutputID, _ := hex.DecodeString(outputID)
	if !bytes.Equal(resp.OutputID, expectedOutputID) {
		t.Fatalf("expected OutputID=%x, got %x", expectedOutputID, resp.OutputID)
	}
}
