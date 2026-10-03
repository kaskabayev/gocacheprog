package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/kaskabayev/gocacheprog/server"
	"github.com/kaskabayev/gocacheprog/storage"
)

func main() {
	cacheDir := flag.String("cache-dir", "", "Path to the cache directory")
	s3Bucket := flag.String("s3-bucket", "", "S3 bucket for the remote cache layer (enables remote caching)")
	s3Prefix := flag.String("s3-prefix", "", "Key prefix inside the S3 bucket")
	s3Endpoint := flag.String("s3-endpoint", "", "S3-compatible endpoint, e.g. https://minio.example:9000 (default: AWS S3)")
	s3AccessKey := flag.String("s3-access-key", "", "S3 access key (default: environment or IAM role)")
	s3SecretKey := flag.String("s3-secret-key", "", "S3 secret key (default: environment or IAM role)")
	flag.Parse()
	if *cacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		*cacheDir = filepath.Join(userCache, ".gocacheprog")
	}
	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache directory %s: %v", *cacheDir, err)
	}

	// Initialize the disk cache
	diskCache, err := storage.NewDiskCache(*cacheDir)
	if err != nil {
		log.Fatalf("Failed to initialize disk cache: %v", err)
	}

	// Optionally layer a remote object store behind the local disk
	var cache storage.CacheStorage = diskCache
	if *s3Bucket != "" {
		store, err := storage.NewS3ObjectStore(*s3Endpoint, *s3Bucket, *s3AccessKey, *s3SecretKey)
		if err != nil {
			log.Fatalf("Failed to configure S3 cache: %v", err)
		}
		cache = storage.NewS3Cache(diskCache, store, *s3Prefix)
	}

	// Set up I/O
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	// Create and start the server
	cacheServer := server.NewServer(cache, reader, writer)
	if err := cacheServer.SendHandshake(); err != nil {
		log.Fatalf("Failed to send handshake: %v", err)
	}

	// Process requests until "close" or EOF
	if err := cacheServer.ProcessRequests(context.Background()); err != nil {
		log.Fatalf("Error processing requests: %v", err)
	}
	if err := cache.Close(); err != nil {
		log.Fatalf("Error closing cache: %v", err)
	}
}
