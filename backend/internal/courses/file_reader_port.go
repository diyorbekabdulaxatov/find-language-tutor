package courses

import (
	"context"

	"github.com/google/uuid"
)

// FileReader is the slice of the files module courses needs to validate a
// `video`-kind item and a course's cover image: does fileAssetID belong to
// the calling account, and what is its stored content type?
// internal/files provides the adapter (files.NewCourseGateway); this package
// never imports internal/files, so the dependency graph stays one-way
// (files -> courses), same shape as resources -> courses.
//
// Deliberate naming note: file ownership (files.Asset.OwnerID) is per
// *uploading account*, not per teacher profile — the same id
// files.Service.Upload is called with (auth.UserID(c), not a teachers.id).
// courses.Service has both the caller's account id and their teacher id on
// hand; it passes the account id here, so the parameter is named callerID
// rather than teacherID to say so plainly.
//
// A nil reader (SetFileReader never called) makes AddItem / cover-image
// validation fail closed for every video / cover reference.
type FileReader interface {
	// FileOwnedBy reports whether fileAssetID belongs to callerID, and its
	// stored content type. ok is false when the asset doesn't exist or
	// belongs to someone else.
	FileOwnedBy(ctx context.Context, fileAssetID, callerID uuid.UUID) (ok bool, contentType string, err error)
}
