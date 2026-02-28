package storage

import (
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

	// 1. Write output payload atomically via temp file
	tempOutput, err := os.CreateTemp(d.outputsPath, outputID+".tmp.*")
	if err != nil {
		return "", fmt.Errorf("creating temp output file: %w", err)
	}
	tempOutputPath := tempOutput.Name()

	// Stream directly to temp file (avoids memory spikes)
	if _, err := io.Copy(tempOutput, r); err != nil {
		tempOutput.Close()
		os.Remove(tempOutputPath)
		return "", fmt.Errorf("writing to temp output file: %w", err)
	}
	tempOutput.Close()

	if err := os.Rename(tempOutputPath, outputPath); err != nil {
		os.Remove(tempOutputPath)
		return "", fmt.Errorf("renaming output file: %w", err)
	}

	// 2. Write action mapping atomically via temp file
	mappingPath := filepath.Join(d.actionsPath, actionID)
	tempMapping, err := os.CreateTemp(d.actionsPath, actionID+".tmp.*")
	if err != nil {
		return "", fmt.Errorf("creating temp mapping file: %w", err)
	}
	tempMappingPath := tempMapping.Name()

	if _, err := tempMapping.Write([]byte(outputID)); err != nil {
		tempMapping.Close()
		os.Remove(tempMappingPath)
		return "", fmt.Errorf("writing temp mapping file: %w", err)
	}
	tempMapping.Close()

	if err := os.Rename(tempMappingPath, mappingPath); err != nil {
		os.Remove(tempMappingPath)
		return "", fmt.Errorf("renaming mapping file: %w", err)
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
