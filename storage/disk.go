package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	actionsDir = "actions"
	outputsDir = "outputs"
)

// CacheStorage defines the interface for cache storage implementations
type CacheStorage interface {
	Get(ctx context.Context, actionID string) (string, error)
	Put(ctx context.Context, actionID string, outputID string, r io.Reader) (string, error)
	Close() error
}

// DiskCache implements CacheStorage using the filesystem
type DiskCache struct {
	rootDir     string
	actionsPath string
	outputsPath string
}

// NewDiskCache creates a new disk-based cache at the specified root directory
func NewDiskCache(root string) (*DiskCache, error) {
	actionsPath := filepath.Join(root, actionsDir)
	outputsPath := filepath.Join(root, outputsDir)
	for _, dir := range []string{root, actionsPath, outputsPath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return &DiskCache{
		rootDir:     root,
		actionsPath: actionsPath,
		outputsPath: outputsPath,
	}, nil
}

// Put stores data in the cache and returns the path where it was stored
func (d *DiskCache) Put(ctx context.Context, actionID string, outputID string, r io.Reader) (string, error) {
	// output ID file
	outputPath := filepath.Join(d.outputsPath, outputID)
	if _, err := os.Stat(outputPath); err == nil {
		return outputPath, nil
	}

	// read content and write to output file
	var buf bytes.Buffer
	data, err := io.ReadAll(io.TeeReader(r, &buf))
	if err != nil {
		return "", fmt.Errorf("reading content: %w", err)
	}

	// create output file
	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("writing output file: %w", err)
	}

	// action ID mapping file, maps action ID to output ID
	mappingPath := filepath.Join(d.actionsPath, actionID)
	if err := os.WriteFile(mappingPath, []byte(outputID), 0644); err != nil {
		return "", fmt.Errorf("writing mapping file: %w", err)
	}
	return outputPath, nil
}

// Get retrieves data from the cache based on the actionID
func (d *DiskCache) Get(ctx context.Context, actionID string) (string, error) {
	// action ID mapping file
	actionPath := filepath.Join(d.actionsPath, actionID)
	outputID, err := os.ReadFile(actionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading mapping file: %w", err)
	}

	// output ID file from action mapping file
	outputPath := filepath.Join(d.outputsPath, strings.TrimSpace(string(outputID)))
	if _, err := os.Stat(outputPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stating output file: %w", err)
	}
	return outputPath, nil
}

// Close performs any necessary cleanup operations
func (d *DiskCache) Close() error {
	return nil
}
