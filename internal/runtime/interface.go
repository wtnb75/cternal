package runtime

import (
	"context"
	"io"
)

// Filter specifies criteria for container listing.
type Filter struct {
	Name   string
	Status string // "running", "exited", etc.
	Labels map[string]string
}

// Container holds basic container metadata.
type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
}

// ExecOptions configures a docker-exec style connection.
type ExecOptions struct {
	Shell []string // e.g. ["/bin/bash"] or ["sh"]
	Env   []string
	Cols  uint16
	Rows  uint16
}

// LogsOptions configures a logs-mode connection.
type LogsOptions struct {
	Since      string // RFC3339 timestamp or duration (e.g. "1h")
	Follow     bool
	Timestamps bool   // prefix each line with an RFC3339Nano timestamp (docker logs -t)
}

// Stream represents a bidirectional PTY-capable connection to a container.
type Stream interface {
	Read() ([]byte, error)
	Write(data []byte) error
	Resize(cols, rows uint16) error
	Close() error
}

// Runtime abstracts container runtime operations.
type Runtime interface {
	ListContainers(ctx context.Context, filter Filter) ([]Container, error)
	Exec(ctx context.Context, id string, opts ExecOptions) (Stream, error)
	Attach(ctx context.Context, id string) (Stream, error)
	Logs(ctx context.Context, id string, opts LogsOptions) (io.ReadCloser, error)
}
