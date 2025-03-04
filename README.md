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

### Project Structure

The project is organized into several packages:

- `main.go`: Entry point of the application, handles initialization and configuration
- `server/`: Contains the server implementation for handling client requests
- `storage/`: Implements the disk-based caching mechanism
- `protocol/`: Defines the communication protocol between client and server

## Testing

Run the simple math tests using, to see the cache generation:

```bash
go test ./tests/...
```
