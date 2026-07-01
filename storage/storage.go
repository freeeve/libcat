// Package storage abstracts where lcat writes its artifacts, so the same build
// runs unchanged against a local directory, cloud object storage (S3/GCS/Azure),
// or a git working tree. The core depends only on the Sink interface; cloud
// adapters live in their own packages so their SDKs never reach the core -- this
// is what keeps lcat cloud-agnostic and its baseline dependency-free.
package storage

import (
	"io"
	"os"
	"path/filepath"
)

// Sink is a write-only, path-addressed blob store. Paths are relative and use
// forward slashes; each Sink maps them onto its backend (a directory, a bucket
// prefix, a git tree).
type Sink interface {
	Create(path string) (io.WriteCloser, error)
}

// Dir is a Sink backed by a local directory tree -- the default for the CLI and
// for a Fargate/container task. Cloud sinks (S3, GCS, git) implement the same
// interface in their own packages.
type Dir string

// Create opens path (relative to the directory, slash-separated) for writing,
// creating parent directories as needed.
func (d Dir) Create(path string) (io.WriteCloser, error) {
	full := filepath.Join(string(d), filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	return os.Create(full)
}
