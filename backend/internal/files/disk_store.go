package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// diskStore keeps blobs as files under a base directory (FILES_DISK_PATH). It's
// the dev default. Object keys are "/"-separated relative paths; ".." is
// rejected so a key can never escape the base dir.
type diskStore struct{ base string }

// NewDiskStore builds a disk-backed Blob at base, creating the directory.
func NewDiskStore(base string) (Blob, error) {
	if strings.TrimSpace(base) == "" {
		return nil, errors.New("files: FILES_DISK_PATH is empty")
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("files: resolve disk path: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("files: create disk path: %w", err)
	}
	return &diskStore{base: abs}, nil
}

func (s *diskStore) Provider() string { return ProviderDisk }

func (s *diskStore) path(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("files: empty object key")
	}
	for _, seg := range strings.Split(strings.Trim(filepath.ToSlash(key), "/"), "/") {
		if seg == ".." || seg == "." || seg == "" {
			return "", fmt.Errorf("files: bad object key %q", key)
		}
	}
	full := filepath.Join(s.base, filepath.FromSlash(key))
	if full != s.base && !strings.HasPrefix(full, s.base+string(os.PathSeparator)) {
		return "", fmt.Errorf("files: key %q escapes the store", key)
	}
	return full, nil
}

func (s *diskStore) Put(_ context.Context, key, _ string, r io.Reader) error {
	full, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("files: mkdir: %w", err)
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("files: create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("files: write: %w", err)
	}
	return nil
}

func (s *diskStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := s.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("files: open: %w", err)
	}
	return f, nil
}

// PresignedGetURL is not supported on disk — the handler streams via Open.
func (s *diskStore) PresignedGetURL(context.Context, string, time.Duration) (string, bool, error) {
	return "", false, nil
}

func (s *diskStore) Delete(_ context.Context, key string) error {
	full, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("files: delete: %w", err)
	}
	return nil
}
