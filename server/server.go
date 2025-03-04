package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/kaskabayev/gocacheprog/protocol"
	"github.com/kaskabayev/gocacheprog/storage"
)

// Server handles the cache protocol communication
type Server struct {
	cache   storage.CacheStorage
	reader  *bufio.Reader
	writer  *bufio.Writer
	dec     *json.Decoder
	enc     *json.Encoder
	writeMu sync.Mutex
}

// NewServer creates a new cache server with the given storage and I/O
func NewServer(cache storage.CacheStorage, reader *bufio.Reader, writer *bufio.Writer) *Server {
	return &Server{
		cache:  cache,
		reader: reader,
		writer: writer,
		dec:    json.NewDecoder(reader),
		enc:    json.NewEncoder(writer),
	}
}

// writeResponse sends a response to the client with proper synchronization
func (s *Server) writeResponse(resp protocol.Response) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.enc.Encode(resp); err != nil {
		return fmt.Errorf("failed to encode response: %v", err)
	}
	if err := s.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush response: %v", err)
	}
	return nil
}

// SendHandshake sends the initial handshake response with supported commands
func (s *Server) SendHandshake() error {
	supportedCommands := []string{"get", "put", "close"}
	handshakeResp := protocol.Response{KnownCommands: supportedCommands}

	return s.writeResponse(handshakeResp)
}

// ProcessRequests handles incoming requests until EOF
func (s *Server) ProcessRequests(ctx context.Context) error {
	for {
		var req protocol.Request
		if err := s.dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("error decoding request: %w", err)
		}

		// For "put" requests, if BodySize > 0, decode extra binary payload.
		if req.Command == "put" {
			req.Body = bytes.NewReader(nil)
			if req.BodySize > 0 {
				var bodyData []byte
				if err := s.dec.Decode(&bodyData); err != nil {
					return fmt.Errorf("error decoding put body: %w", err)
				}
				if int64(len(bodyData)) != req.BodySize {
					return fmt.Errorf("incorrect body length: got %d, expected %d", len(bodyData), req.BodySize)
				}
				req.Body = bytes.NewReader(bodyData)
			}
		}

		go func(req protocol.Request) {
			s.handleRequest(ctx, req)
		}(req)
	}
}

// handleRequest processes a single cache request
func (s *Server) handleRequest(ctx context.Context, req protocol.Request) {
	resp := protocol.Response{ID: req.ID}

	var err error
	switch req.Command {
	case "get":
		actionHex := hex.EncodeToString(req.ActionID)
		if resp.DiskPath, err = s.cache.Get(ctx, actionHex); err != nil {
			resp.Err = err.Error()
		}
		if resp.DiskPath == "" {
			resp.Miss = true
		}
	case "put":
		actionHex := hex.EncodeToString(req.ActionID)
		outputHex := hex.EncodeToString(req.OutputID)
		r := req.Body
		if resp.DiskPath, err = s.cache.Put(ctx, actionHex, outputHex, r); err != nil {
			resp.Err = err.Error()
		}
	case "close":
	}

	if req.Command != "close" && resp.DiskPath != "" {
		if fi, err := os.Stat(resp.DiskPath); err == nil {
			resp.Size = fi.Size()
			modTime := fi.ModTime()
			resp.Time = &modTime

			resp.OutputID, err = hex.DecodeString(filepath.Base(resp.DiskPath))
			if err != nil {
				resp.Err = err.Error()
			}
		} else {
			resp.Err = err.Error()
		}
	}

	s.writeResponse(resp)
}
