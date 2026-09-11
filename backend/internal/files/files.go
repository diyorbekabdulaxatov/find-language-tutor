package files

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Asset is a stored file the app can hand back to an authorised caller.
type Asset struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Provider    string
	ObjectKey   string
	Filename    string
	ContentType string
	Bytes       int64
	CreatedAt   time.Time
}

// Limits.
const (
	// MaxUploadBytes caps a single non-video upload. Materials are small PDFs /
	// images; listening-task audio is a few minutes. 25 MiB is comfortable
	// headroom.
	MaxUploadBytes = 25 << 20

	// MaxVideoUploadBytes caps a single video upload (phase C1: course video
	// items). Proxied through this same multipart endpoint like every other
	// upload for now — R2 is not implemented yet (NewBlob in storage.go still
	// errors on FILES_STORE=r2), so there is no presigned direct-to-R2 PUT to
	// offload large files onto; that's future work once R2 itself exists, not
	// something built here. 500 MiB is generous headroom for a lesson-length
	// screen recording without inviting unbounded uploads through a single
	// in-process multipart handler.
	MaxVideoUploadBytes = 500 << 20
)

// allowedContentTypes is the upload whitelist. Anything else is a 415.
var allowedContentTypes = map[string]string{
	"application/pdf": ".pdf",
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"audio/mpeg":      ".mp3",
	"audio/mp4":       ".m4a",
	"audio/aac":       ".aac",
	"audio/wav":       ".wav",
	"audio/x-wav":     ".wav",
	"audio/ogg":       ".ogg",
	"audio/webm":      ".weba",
	"video/mp4":       ".mp4",
	"video/webm":      ".webm",
	"video/quicktime": ".mov",
}

// isVideoContentType reports whether ct (already normalized — lowercased,
// parameters stripped) is one of the video types above, so Upload can apply
// the wider MaxVideoUploadBytes cap instead of MaxUploadBytes.
func isVideoContentType(ct string) bool {
	switch ct {
	case "video/mp4", "video/webm", "video/quicktime":
		return true
	}
	return false
}

// maxUploadBytesFor returns the size cap for a negotiated, already-normalized
// content type: the wider video cap for video/*, MaxUploadBytes otherwise.
func maxUploadBytesFor(contentType string) int64 {
	if isVideoContentType(contentType) {
		return MaxVideoUploadBytes
	}
	return MaxUploadBytes
}

// Errors. The handler maps each to a status.
var (
	ErrAssetNotFound     = errors.New("files: file not found")
	ErrForbidden         = errors.New("files: not allowed to access this file")
	ErrUnsupportedType   = errors.New("files: unsupported file type")
	ErrTooLarge          = errors.New("files: file is too large")
	ErrEmptyUpload       = errors.New("files: upload is empty")
)

// ValidationError is a client-fixable upload problem the handler renders as 400.
type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

func invalid(format string, a ...any) error { return ValidationError{fmt.Sprintf(format, a...)} }
