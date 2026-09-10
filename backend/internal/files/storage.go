// Package files owns file_assets and the blob store behind them. The bytes of
// an upload live in a Blob store — local disk in dev, Cloudflare R2 in prod —
// selected by FILES_STORE. The rest of the app only ever holds a file_asset id.
//
// Boundary: this is a leaf-ish module. internal/resources references
// file_asset ids from its JSONB content; it does not import this package's
// storage internals. The download handler here is the single gate on the bytes.
package files

import (
	"context"
	"errors"
	"io"
	"time"
)

// Provider names, matching the file_assets CHECK constraint.
const (
	ProviderDisk = "disk"
	ProviderR2   = "r2"
)

// Blob is the object store port. Put writes bytes at a key; Open reads them
// back; PresignedGetURL returns a time-limited direct URL when the backend
// supports one (R2) — disk returns ok=false and the handler streams via Open.
type Blob interface {
	// Provider is the value stored on file_assets.provider.
	Provider() string
	Put(ctx context.Context, key, contentType string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (url string, ok bool, err error)
	Delete(ctx context.Context, key string) error
}

// ErrNotFound is returned by Open/Delete for an absent key.
var ErrNotFound = errors.New("files: object not found")

// NewBlob builds the Blob for the configured store. "disk" is the dev default;
// "r2" is not implemented yet (the port is ready for it).
func NewBlob(store, diskPath string) (Blob, error) {
	switch store {
	case "", ProviderDisk:
		return NewDiskStore(diskPath)
	case ProviderR2:
		return nil, errors.New("files: FILES_STORE=r2 is not implemented yet — use disk")
	default:
		return nil, errors.New("files: FILES_STORE must be 'disk' or 'r2'")
	}
}
