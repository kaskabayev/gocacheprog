package protocol

import (
	"io"
	"time"
)

// Request represents an incoming cache operation request
type Request struct {
	ID       int64
	Command  string
	ActionID []byte `json:",omitempty"`
	OutputID []byte `json:",omitempty"`
	// For "put", Body is provided as io.Reader
	Body io.Reader `json:"-"`
	// For "put", BodySize (if > 0) indicates that a binary payload follows.
	BodySize int64 `json:",omitempty"`
}

// Response represents the result of a cache operation
type Response struct {
	ID            int64
	Err           string     `json:",omitempty"`
	KnownCommands []string   `json:",omitempty"`
	Miss          bool       `json:",omitempty"`
	OutputID      []byte     `json:",omitempty"`
	Size          int64      `json:",omitempty"`
	Time          *time.Time `json:",omitempty"`
	DiskPath      string     `json:",omitempty"`
}
