package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestDiskStore_PutOpenDelete(t *testing.T) {
	s, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()
	if err := s.Put(ctx, "a/b/c.txt", "text/plain", strings.NewReader("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}
	rc, err := s.Open(ctx, "a/b/c.txt")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "hello" {
		t.Errorf("got %q", got)
	}
	if _, ok, err := s.PresignedGetURL(ctx, "a/b/c.txt", time.Minute); err != nil || ok {
		t.Errorf("disk presign: url=%v ok=%v err=%v (want ok=false)", ok, ok, err)
	}
	if err := s.Delete(ctx, "a/b/c.txt"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := s.Open(ctx, "a/b/c.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("open after delete: %v", err)
	}
}

func TestDiskStore_RejectsTraversal(t *testing.T) {
	s, _ := NewDiskStore(t.TempDir())
	if err := s.Put(context.Background(), "../escape.txt", "text/plain", strings.NewReader("x")); err == nil {
		t.Fatal("path traversal was allowed")
	}
}

// --- service tests ---

type fakeRepo struct {
	byID map[uuid.UUID]Asset
	err  error
}

func (r *fakeRepo) Create(_ context.Context, a Asset) (Asset, error) {
	if r.err != nil {
		return Asset{}, r.err
	}
	a.ID = uuid.New()
	if r.byID == nil {
		r.byID = map[uuid.UUID]Asset{}
	}
	r.byID[a.ID] = a
	return a, nil
}

func (r *fakeRepo) ByID(_ context.Context, id uuid.UUID) (Asset, error) {
	a, ok := r.byID[id]
	if !ok {
		return Asset{}, ErrAssetNotFound
	}
	return a, nil
}

type memBlob struct{ puts, deletes int }

func (b *memBlob) Provider() string { return ProviderDisk }
func (b *memBlob) Put(context.Context, string, string, io.Reader) error { b.puts++; return nil }
func (b *memBlob) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte("bytes"))), nil
}
func (b *memBlob) PresignedGetURL(context.Context, string, time.Duration) (string, bool, error) {
	return "", false, nil
}
func (b *memBlob) Delete(context.Context, string) error { b.deletes++; return nil }

var _ Blob = (*memBlob)(nil)

func TestService_Upload_Validation(t *testing.T) {
	blob := &memBlob{}
	svc := NewService(&fakeRepo{}, blob, discardLogger())
	ctx := context.Background()
	owner := uuid.New()

	if _, err := svc.Upload(ctx, owner, "x.exe", "application/x-msdownload", 10, strings.NewReader("x")); !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("bad type: %v", err)
	}
	if _, err := svc.Upload(ctx, owner, "x.pdf", "application/pdf", 0, strings.NewReader("")); !errors.Is(err, ErrEmptyUpload) {
		t.Errorf("empty: %v", err)
	}
	if _, err := svc.Upload(ctx, owner, "x.pdf", "application/pdf", MaxUploadBytes+1, strings.NewReader("x")); !errors.Is(err, ErrTooLarge) {
		t.Errorf("too large: %v", err)
	}

	a, err := svc.Upload(ctx, owner, "notes.pdf", "application/pdf; charset=binary", 5, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("valid upload: %v", err)
	}
	if a.ContentType != "application/pdf" || a.Filename != "notes.pdf" || a.Provider != ProviderDisk {
		t.Errorf("unexpected asset: %+v", a)
	}
	if blob.puts != 1 {
		t.Errorf("blob.Put called %d times", blob.puts)
	}
}

func TestService_Upload_CleansOrphanOnDBError(t *testing.T) {
	blob := &memBlob{}
	svc := NewService(&fakeRepo{err: errors.New("db down")}, blob, discardLogger())
	if _, err := svc.Upload(context.Background(), uuid.New(), "a.pdf", "application/pdf", 5, strings.NewReader("hi")); err == nil {
		t.Fatal("want db error")
	}
	if blob.deletes != 1 {
		t.Errorf("orphan blob not deleted (deletes=%d)", blob.deletes)
	}
}

func TestService_Download_OwnerOnly(t *testing.T) {
	blob := &memBlob{}
	svc := NewService(&fakeRepo{}, blob, discardLogger())
	ctx := context.Background()
	owner := uuid.New()

	a, _ := svc.Upload(ctx, owner, "notes.pdf", "application/pdf", 5, strings.NewReader("hello"))

	if _, _, _, err := svc.Download(ctx, a.ID, uuid.New()); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-owner download: %v", err)
	}
	_, body, got, err := svc.Download(ctx, a.ID, owner)
	if err != nil {
		t.Fatalf("owner download: %v", err)
	}
	body.Close()
	if got.ID != a.ID {
		t.Errorf("wrong asset")
	}
}
