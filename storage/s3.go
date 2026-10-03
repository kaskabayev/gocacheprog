package storage

import (
	"context"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	uploadWorkers = 4
	uploadQueue   = 1024
	uploadTimeout = 30 * time.Second
	drainTimeout  = 30 * time.Second
	// Upper bound on the size of a mapping object read from the store.
	maxIDSize = 4096
)

// uploadJob uploads one cached entry: its output payload, then its action
// mapping, so a visible mapping always has its payload available.
type uploadJob struct {
	outputKey  string
	outputPath string
	actionKey  string
	actionID   string
}

// S3Cache is a two-tier cache: local disk (L1) backed by an object store
// (L2). Get falls through to the store on a local miss; Put stores locally
// and uploads in the background. Store failures never fail a request: the
// remote layer is best-effort and builds keep working offline.
type S3Cache struct {
	*DiskCache
	store  ObjectStore
	prefix string

	jobs    chan uploadJob
	wg      sync.WaitGroup
	dropped atomic.Int64 // uploads dropped because the queue was full
	failed  atomic.Int64 // uploads that errored against the store
}

// NewS3Cache layers the object store under the disk cache and starts the
// background upload workers.
func NewS3Cache(disk *DiskCache, store ObjectStore, prefix string) *S3Cache {
	c := &S3Cache{
		DiskCache: disk,
		store:     store,
		prefix:    prefix,
		jobs:      make(chan uploadJob, uploadQueue),
	}
	c.wg.Add(uploadWorkers)
	for range uploadWorkers {
		go c.worker()
	}
	return c
}

func (c *S3Cache) actionKey(id string) string { return path.Join(c.prefix, "actions", id) }
func (c *S3Cache) outputKey(id string) string { return path.Join(c.prefix, "outputs", id) }

// Get returns the entry from local disk, falling back to the object store on
// a local miss. Any store problem degrades to a cache miss.
func (c *S3Cache) Get(ctx context.Context, actionID string) (string, error) {
	if p, err := c.DiskCache.Get(ctx, actionID); err != nil || p != "" {
		return p, err
	}

	rc, err := c.store.GetObject(ctx, c.actionKey(actionID))
	if err != nil {
		return "", nil
	}
	defer rc.Close()
	outputID, err := io.ReadAll(io.LimitReader(rc, maxIDSize))
	if err != nil {
		return "", nil
	}
	id := strings.TrimSpace(string(outputID))
	if _, err := hex.DecodeString(id); err != nil {
		return "", nil
	}

	body, err := c.store.GetObject(ctx, c.outputKey(id))
	if err != nil {
		return "", nil
	}
	defer body.Close()
	if _, err := c.DiskCache.Put(ctx, actionID, id, body); err != nil {
		return "", nil
	}
	return c.DiskCache.Get(ctx, actionID)
}

// Put stores the entry locally and queues a background upload. It never
// fails because of the store.
func (c *S3Cache) Put(ctx context.Context, actionID string, outputID string, r io.Reader) (string, error) {
	p, err := c.DiskCache.Put(ctx, actionID, outputID, r)
	if err != nil {
		return "", err
	}
	select {
	case c.jobs <- uploadJob{
		outputKey:  c.outputKey(outputID),
		outputPath: p,
		actionKey:  c.actionKey(actionID),
		actionID:   outputID,
	}:
	default:
		c.dropped.Add(1)
	}
	return p, nil
}

// Close stops accepting uploads, drains pending ones (bounded by
// drainTimeout), reports upload statistics, and closes the local cache.
func (c *S3Cache) Close() error {
	close(c.jobs)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(drainTimeout):
	}
	if d := c.dropped.Load(); d > 0 {
		log.Printf("dropped %d cache uploads (queue full)", d)
	}
	if f := c.failed.Load(); f > 0 {
		log.Printf("%d cache uploads failed", f)
	}
	return c.DiskCache.Close()
}

func (c *S3Cache) worker() {
	defer c.wg.Done()
	for job := range c.jobs {
		c.upload(job)
	}
}

func (c *S3Cache) upload(job uploadJob) {
	f, err := os.Open(job.outputPath)
	if err != nil {
		c.failed.Add(1)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		c.failed.Add(1)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()
	if err := c.store.PutObject(ctx, job.outputKey, f, fi.Size()); err != nil {
		c.failed.Add(1)
		return
	}
	if err := c.store.PutObject(ctx, job.actionKey, strings.NewReader(job.actionID), int64(len(job.actionID))); err != nil {
		c.failed.Add(1)
	}
}
