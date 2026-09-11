package files

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestService_FileOwnedBy(t *testing.T) {
	svc := NewService(&fakeRepo{}, &memBlob{}, discardLogger())
	ctx := context.Background()
	owner := uuid.New()
	a, err := svc.Upload(ctx, owner, "clip.mp4", "video/mp4", 10, strings.NewReader("0123456789"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	ok, ct, err := svc.FileOwnedBy(ctx, a.ID, owner)
	if err != nil || !ok || ct != "video/mp4" {
		t.Errorf("owner check: ok=%v ct=%q err=%v", ok, ct, err)
	}

	ok, ct, err = svc.FileOwnedBy(ctx, a.ID, uuid.New())
	if err != nil || ok || ct != "" {
		t.Errorf("non-owner check: ok=%v ct=%q err=%v, want ok=false", ok, ct, err)
	}

	ok, ct, err = svc.FileOwnedBy(ctx, uuid.New(), owner)
	if err != nil || ok || ct != "" {
		t.Errorf("unknown asset check: ok=%v ct=%q err=%v, want ok=false", ok, ct, err)
	}
}

func TestCourseGateway_FileOwnedBy(t *testing.T) {
	svc := NewService(&fakeRepo{}, &memBlob{}, discardLogger())
	gw := NewCourseGateway(svc)
	ctx := context.Background()
	owner := uuid.New()
	a, err := svc.Upload(ctx, owner, "cover.png", "image/png", 5, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	ok, ct, err := gw.FileOwnedBy(ctx, a.ID, owner)
	if err != nil || !ok || ct != "image/png" {
		t.Errorf("gateway owner check: ok=%v ct=%q err=%v", ok, ct, err)
	}
}

// TestService_Upload_VideoAllowsWiderSizeCap: a payload larger than
// MaxUploadBytes but within MaxVideoUploadBytes succeeds for a video content
// type and fails for a non-video one, and the reverse cap (>MaxVideoUploadBytes)
// is rejected even for video.
func TestService_Upload_VideoAllowsWiderSizeCap(t *testing.T) {
	svc := NewService(&fakeRepo{}, &memBlob{}, discardLogger())
	ctx := context.Background()
	owner := uuid.New()
	overNonVideoCap := int64(MaxUploadBytes) + 1

	if _, err := svc.Upload(ctx, owner, "big.pdf", "application/pdf", overNonVideoCap, strings.NewReader("x")); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized pdf: %v, want ErrTooLarge", err)
	}
	if _, err := svc.Upload(ctx, owner, "big.mp4", "video/mp4", overNonVideoCap, strings.NewReader("x")); err != nil {
		t.Errorf("video over the non-video cap but under the video cap should succeed: %v", err)
	}
	overVideoCap := int64(MaxVideoUploadBytes) + 1
	if _, err := svc.Upload(ctx, owner, "huge.mp4", "video/mp4", overVideoCap, strings.NewReader("x")); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized video: %v, want ErrTooLarge", err)
	}
}

func TestService_Upload_AcceptsVideoContentTypes(t *testing.T) {
	svc := NewService(&fakeRepo{}, &memBlob{}, discardLogger())
	ctx := context.Background()
	owner := uuid.New()
	for _, ct := range []string{"video/mp4", "video/webm", "video/quicktime"} {
		if _, err := svc.Upload(ctx, owner, "clip", ct, 5, strings.NewReader("hello")); err != nil {
			t.Errorf("%s should be accepted: %v", ct, err)
		}
	}
}
