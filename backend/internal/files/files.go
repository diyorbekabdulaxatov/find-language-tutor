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
	// MaxUploadBytes caps a single upload. Materials are small PDFs / images;
	// listening-task audio is a few minutes. 25 MiB is comfortable headroom.
	MaxUploadBytes = 25 << 20
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
