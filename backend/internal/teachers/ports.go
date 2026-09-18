package teachers

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// AccountReader answers "has this account confirmed its email?". Defined
// here (the consuming module) and implemented by auth.Service; cmd/api wires
// it with SetAccountReader. Nil fails closed — a profile can't be created
// until the port is wired, which is the safe default.
type AccountReader interface {
	EmailVerified(ctx context.Context, userID uuid.UUID) (bool, error)
}

// Notifier hears about profiles entering the moderation queue — on first
// submission and on every resubmission after a rejection. Implemented by
// internal/moderationmail (an email to the ops inbox); nil is a no-op.
type Notifier interface {
	TeacherSubmitted(ctx context.Context, slug, displayName string, resubmitted bool)
}

// FileReader is the files module seen from here: ownership + type checks for
// the media a profile references, and public serving of those bytes.
// Implemented by files.TeacherGateway, wired by cmd/api with SetFileReader.
// Nil fails closed — a write naming an asset id is refused, and the media
// route 404s — since there is nothing sensible to do without it.
type FileReader interface {
	// FileOwnedBy reports whether fileAssetID belongs to callerID and, when it
	// does, its stored content type. A missing or foreign asset is ok=false
	// with no error.
	FileOwnedBy(ctx context.Context, fileAssetID, callerID uuid.UUID) (ok bool, contentType string, err error)
	// PublicAsset serves fileAssetID's bytes with NO access check: either a
	// redirect URL (R2) or an open reader the caller must Close (disk).
	PublicAsset(ctx context.Context, fileAssetID uuid.UUID) (redirectURL string, body io.ReadCloser, contentType string, err error)
}
