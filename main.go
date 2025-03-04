package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/kaskabayev/gocache/server"
	"github.com/kaskabayev/gocache/storage"
)

func main() {
	cacheDir := flag.String("cache-dir", "", "Path to the cache directory")
	flag.Parse()
	if *cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		*cacheDir = filepath.Join(home, ".gocacheprog")
	}
	if err := os.MkdirAll(*cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache directory %s: %v", *cacheDir, err)
	}
	log.Printf("Cache directory is set to: %s", *cacheDir)

	// Initialize the disk cache
	diskCache, err := storage.NewDiskCache(*cacheDir)
	if err != nil {
		log.Fatalf("Failed to initialize disk cache: %v", err)
	}

	// Set up I/O
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	// Create and start the server
	cacheServer := server.NewServer(diskCache, reader, writer)
	if err := cacheServer.SendHandshake(); err != nil {
		log.Fatalf("Failed to send handshake: %v", err)
	}

	// Process requests until EOF
	if err := cacheServer.ProcessRequests(nil); err != nil {
		log.Fatalf("Error processing requests: %v", err)
	}
}
