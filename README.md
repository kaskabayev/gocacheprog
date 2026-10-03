# GoCacheProg

GoCache is an example of caching system for GOCACHEPROG Go 1.24 environmental variable, implemented in Go that provides a disk-based caching solution.

# Sources

* https://tip.golang.org/src/cmd/go/internal/cache/
* https://tip.golang.org/src/cmd/go/internal/cache/cache.go
* https://tip.golang.org/src/cmd/go/internal/cache/prog.go

## Features

- Disk-based persistent caching
- Client-server architecture with handshake protocol
- Configurable cache directory location
- Buffered I/O for efficient data handling
- Simple and clean API

## Requirements

- Go 1.24 or higher

## Installation

### Clone a repository

```bash
git clone git@github.com:kaskabayev/gocacheprog.git
```

### Build a binary

```bash
go build -o gocache .
```

## Usage

### Setting the GOCACHEPROG environmental variable

```bash
export GOCACHEPROG="./gocache --cache-dir PATH/TO/TEMP_FOLDER"
```

If no cache directory is specified, it defaults to `USER_CACHE_DIR/.gocacheprog`, e.g. for MacOS it is `~/Library/Caches/...`

### Remote caching (S3)

Pass `--s3-bucket` to layer an S3-compatible object store behind the local disk: builds store results locally and upload them in the background; cache misses fall back to the bucket and download on demand. Remote failures never fail a build — the cache degrades to local-only.

```bash
# AWS S3 (credentials from the standard environment or IAM role)
export GOCACHEPROG="/tmp/gocache --cache-dir /tmp/my-cache --s3-bucket my-builds"

# Any S3-compatible endpoint (MinIO, Cloudflare R2, ...)
export GOCACHEPROG="/tmp/gocache --cache-dir /tmp/my-cache \
  --s3-bucket my-builds --s3-endpoint https://minio.example:9000"
```

Flags: `--s3-prefix` namespaces keys inside the bucket; `--s3-access-key`/`--s3-secret-key` override credentials (default: MinIO/AWS environment variables, then IAM roles).

Object layout: `PREFIX/actions/<hex>` maps an action ID to its output ID; `PREFIX/outputs/<hex>` holds the content-addressed payload.

## Testing

### Unit Tests

Run all unit tests (storage + server + example math tests):

```bash
go test ./...
```

Run with verbose output:

```bash
go test -v ./...
```

Run with race detector:

```bash
go test -race ./...
```

### End-to-End Testing

Test the cache against real Go builds using the `GOCACHEPROG` environment variable.

#### Quick Smoke Test

```bash
# Build the binary to an absolute path (so it's reachable from any directory)
go build -o /tmp/gocache .

# Create a test project
mkdir /tmp/mytest && cd /tmp/mytest
go mod init mytest
cat > main.go << 'EOF'
package main
import "fmt"
func main() { fmt.Println("hello") }
EOF

# Run with GOCACHEPROG
export GOCACHEPROG="/tmp/gocache --cache-dir /tmp/my-cache"
go build -v .    # cold build — compiles packages
go build -v .    # warm build — should print nothing (all cached)
```

If the second `go build -v` prints no package names, the cache is working.

#### Verify Cache Is Active

Compare build times with and without GOCACHEPROG:

```bash
export GOCACHEPROG="/tmp/gocache --cache-dir /tmp/my-cache"
go clean -cache                         # nuke default Go cache
time go build -v . 2>&1 | wc -l        # should print 0 (all from gocacheprog)

unset GOCACHEPROG
go clean -cache
time go build -v . 2>&1 | wc -l        # should print ~60 (full recompile)
```

#### Inspect Cache Contents

```bash
ls /tmp/my-cache/actions/    # actionID -> outputID mappings
ls /tmp/my-cache/outputs/    # compiled output files

# Check a specific action
cat /tmp/my-cache/actions/<actionID>    # prints the outputID it maps to

# Verify every action points to an existing output
for f in /tmp/my-cache/actions/*; do
  outputID=$(cat "$f")
  [ -f "/tmp/my-cache/outputs/$outputID" ] || echo "BROKEN: $(basename $f)"
done
```

#### Stress Test: Concurrent Builds

```bash
export GOCACHEPROG="/tmp/gocache --cache-dir /tmp/my-cache"
for i in $(seq 1 5); do go build -o /tmp/bin_$i . & done; wait
# All binaries should work
for i in $(seq 1 5); do /tmp/bin_$i; rm /tmp/bin_$i; done
```

#### Test Cache Survives `go clean`

```bash
export GOCACHEPROG="/tmp/gocache --cache-dir /tmp/my-cache"
go build .
go clean -cache                  # clears default cache, NOT gocacheprog
go build -v . 2>&1 | wc -l      # should print 0 (served from gocacheprog)
```

#### Test File Vanishing (Regression)

```bash
export GOCACHEPROG="/tmp/gocache --cache-dir /tmp/my-cache"
go build .
rm /tmp/my-cache/outputs/*       # delete all cached outputs
go build .                       # should recompile gracefully, no crash
```

### Project Structure

The project is organized into several packages:

- `main.go`: Entry point of the application, handles initialization and configuration
- `server/`: Contains the server implementation for handling client requests
- `storage/`: Implements the disk-based caching mechanism
- `protocol/`: Defines the communication protocol between client and server
