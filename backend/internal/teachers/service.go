package teachers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

// ErrNotFound is returned by GetBySlug when no teacher has the given slug.
var ErrNotFound = errors.New("teacher not found")

// ErrProfileExists is returned by Create when the caller already owns a profile
// (one teacher profile per account).
var ErrProfileExists = errors.New("account already has a teacher profile")

// ErrNotOwner is returned by Update when the acting user does not own the
// teacher profile being edited (including when the profile is unclaimed).
var ErrNotOwner = errors.New("not the owner of this teacher profile")

// ErrEmailNotVerified — the account must confirm its address before it can
// put a teacher profile in front of students.
var ErrEmailNotVerified = errors.New("email address not verified")

// maxSlugDedupeAttempts bounds the numeric-suffix search for a free slug.
const maxSlugDedupeAttempts = 1000

// Repository is the persistence port for this module. The concrete
// implementation (repositoryPostgres) lives alongside; tests use a fake.
type Repository interface {
	// List returns one page of teachers matching params (already normalized)
	// together with the total count of matches, ignoring pagination.
	List(ctx context.Context, params ListParams) (page []Teacher, total int, err error)

	// GetBySlug returns the full aggregate for one teacher, or ErrNotFound.
	GetBySlug(ctx context.Context, slug string) (*Teacher, error)

	// LanguageFacets returns the taught-language counts across the whole catalog.
	LanguageFacets(ctx context.Context) ([]LanguageFacet, error)

	// Slugs returns every teacher slug, for the frontend's static generation.
	Slugs(ctx context.Context) ([]string, error)

	// RefBySlug resolves a slug to its id and owning account, or ErrNotFound.
	RefBySlug(ctx context.Context, slug string) (Ref, error)
	// Resubmit moves a rejected profile back to pending (no-op otherwise).
	Resubmit(ctx context.Context, id uuid.UUID) error

	// RefByOwner returns the profile owned by the given account, or ErrNotFound.
	RefByOwner(ctx context.Context, ownerID uuid.UUID) (Ref, error)

	// SlugExists reports whether a teacher already has the given slug.
	SlugExists(ctx context.Context, slug string) (bool, error)

	// Create inserts a new profile (row + child collections) owned by ownerID
	// under the pre-computed slug, and returns its id.
	Create(ctx context.Context, ownerID uuid.UUID, slug string, in ProfileInput) (uuid.UUID, error)

	// Update writes the merged scalar fields for teacherID and replaces any
	// child collection flagged in the ProfileUpdate.
	Update(ctx context.Context, teacherID uuid.UUID, upd ProfileUpdate) error
}

// Service holds the teacher-profiles business logic. Handlers call it; it never
// sees an *gin.Context.
type Service struct {
	repo     Repository
	accounts AccountReader // nil fails closed (see SetAccountReader)
	notifier Notifier      // nil is a no-op
	files    FileReader    // nil fails closed (see FileReader)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SetAccountReader wires the auth module's verification lookup in.
func (s *Service) SetAccountReader(a AccountReader) { s.accounts = a }

// SetNotifier wires the moderation-queue notifier in.
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// SetFileReader wires the files module's ownership check + public serving in.
func (s *Service) SetFileReader(f FileReader) { s.files = f }

// MediaKind names one of the profile's uploaded media slots; it is also the
// last path segment of the public media route.
type MediaKind string

const (
	MediaAvatar     MediaKind = "avatar"
	MediaIntroVideo MediaKind = "intro-video"
)

// mediaPath is the public route that serves a profile's uploaded media. It is
// written into the matching *_url column when an asset is set, so the many
// readers of avatar_url (cards, bookings, admin) need no change; the frontend
// prefixes the API origin. Slugs are immutable, so the path never goes stale.
func mediaPath(slug string, kind MediaKind) string {
	return "/v1/teachers/" + slug + "/media/" + string(kind)
}

// applyMedia checks the referenced uploads belong to ownerID and are of the
// right type, then derives the URL columns from them: an asset id wins over
// whatever avatar_url / intro_video_url the request carried, and a cleared
// asset (nil while cur had one) blanks the URL rather than leaving the dead
// media path behind.
func (s *Service) applyMedia(ctx context.Context, ownerID uuid.UUID, slug string, cur *Teacher, in *ProfileInput) error {
	if in.AvatarAssetID != nil {
		if err := s.checkFileOwned(ctx, *in.AvatarAssetID, ownerID, "image/"); errors.Is(err, errMediaTypeMismatch) {
			return invalid("The photo must be an image you've uploaded.")
		} else if err != nil {
			return err
		}
		in.AvatarURL = mediaPath(slug, MediaAvatar)
	} else if cur != nil && cur.AvatarAssetID != nil {
		in.AvatarURL = ""
	}
	if in.IntroVideoAssetID != nil {
		if err := s.checkFileOwned(ctx, *in.IntroVideoAssetID, ownerID, "video/"); errors.Is(err, errMediaTypeMismatch) {
			return invalid("The intro video must be a video you've uploaded.")
		} else if err != nil {
			return err
		}
		in.IntroVideoURL = mediaPath(slug, MediaIntroVideo)
	} else if cur != nil && cur.IntroVideoAssetID != nil {
		in.IntroVideoURL = ""
	}
	return nil
}

// errMediaTypeMismatch: the asset is the caller's but not of the wanted kind;
// the call site turns it into the slot-specific message (kept as literals
// there so the i18n guard sees them).
var errMediaTypeMismatch = errors.New("media asset has the wrong content type")

func (s *Service) checkFileOwned(ctx context.Context, fileAssetID, callerID uuid.UUID, wantPrefix string) error {
	if s.files == nil {
		return invalid("File validation isn't available right now.")
	}
	ok, contentType, err := s.files.FileOwnedBy(ctx, fileAssetID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return invalid("That file doesn't belong to you.")
	}
	if !strings.HasPrefix(contentType, wantPrefix) {
		return errMediaTypeMismatch
	}
	return nil
}

// Media serves one of an APPROVED profile's uploaded media slots with no
// auth — a public page's <img> / <video> can't attach a bearer token. Any
// other status, an unknown slug, an unknown kind, or an empty slot reads as
// ErrNotFound so a pending profile's photo is never leaked. The caller must
// Close body when it is non-nil.
func (s *Service) Media(ctx context.Context, slug string, kind MediaKind) (redirectURL string, body io.ReadCloser, contentType string, err error) {
	t, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return "", nil, "", err
	}
	if t.Status != StatusApproved || s.files == nil {
		return "", nil, "", ErrNotFound
	}
	var id *uuid.UUID
	switch kind {
	case MediaAvatar:
		id = t.AvatarAssetID
	case MediaIntroVideo:
		id = t.IntroVideoAssetID
	}
	if id == nil {
		return "", nil, "", ErrNotFound
	}
	return s.files.PublicAsset(ctx, *id)
}

// requireVerified is the gate in front of anything that makes a teacher
// visible to students. Fails closed when no AccountReader is wired.
func (s *Service) requireVerified(ctx context.Context, userID uuid.UUID) error {
	if s.accounts == nil {
		return ErrEmailNotVerified
	}
	ok, err := s.accounts.EmailVerified(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrEmailNotVerified
	}
	return nil
}

// List runs a teacher search and attaches the filter facets.
func (s *Service) List(ctx context.Context, params ListParams) (ListResult, error) {
	params = params.normalized()

	page, total, err := s.repo.List(ctx, params)
	if err != nil {
		return ListResult{}, err
	}

	facets, err := s.repo.LanguageFacets(ctx)
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Teachers: page,
		Total:    total,
		Facets:   Facets{Languages: facets},
	}, nil
}

// GetBySlug returns one full PUBLIC profile. A profile that is not `approved`
// is not public: it comes back as ErrNotFound, exactly like an unknown slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Teacher, error) {
	t, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if t.Status != StatusApproved {
		return nil, ErrNotFound
	}
	return t, nil
}

// GetForAdmin returns one full profile regardless of moderation status. Used by
// the admin endpoints; never mounted on a public route.
func (s *Service) GetForAdmin(ctx context.Context, slug string) (*Teacher, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// Slugs lists every teacher slug.
func (s *Service) Slugs(ctx context.Context) ([]string, error) {
	return s.repo.Slugs(ctx)
}

// GetOwnProfile returns the profile owned by ownerID, or ErrNotFound when the
// account has not claimed one.
func (s *Service) GetOwnProfile(ctx context.Context, ownerID uuid.UUID) (*Teacher, error) {
	ref, err := s.repo.RefByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetBySlug(ctx, ref.Slug)
}

// Create claims a teacher profile for ownerID. It fails with ErrProfileExists if
// the account already owns one, and with a ValidationError if the input is
// client-fixable. The slug is derived from the display name and de-duplicated
// with a numeric suffix.
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in ProfileInput) (*Teacher, error) {
	if _, err := s.repo.RefByOwner(ctx, ownerID); err == nil {
		return nil, ErrProfileExists
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err := s.requireVerified(ctx, ownerID); err != nil {
		return nil, err
	}

	if err := validateProfile(in); err != nil {
		return nil, err
	}

	slug, err := s.freeSlug(ctx, in.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := s.applyMedia(ctx, ownerID, slug, nil, &in); err != nil {
		return nil, err
	}

	if _, err := s.repo.Create(ctx, ownerID, slug, in); err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.TeacherSubmitted(ctx, slug, in.DisplayName, false)
	}
	return s.repo.GetBySlug(ctx, slug)
}

// Update edits the profile at slug. The caller must own it: ErrNotOwner
// otherwise (also when the profile is unclaimed), ErrNotFound if the slug is
// unknown, ValidationError if the merged profile is invalid. The slug itself is
// immutable.
func (s *Service) Update(ctx context.Context, slug string, actingUserID uuid.UUID, patch ProfilePatch) (*Teacher, error) {
	ref, err := s.repo.RefBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if ref.OwnerID == uuid.Nil || ref.OwnerID != actingUserID {
		return nil, ErrNotOwner
	}

	current, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	merged := mergePatch(current, patch)
	if err := validateProfile(merged); err != nil {
		return nil, err
	}
	if err := s.applyMedia(ctx, actingUserID, slug, current, &merged); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, ref.ID, ProfileUpdate{
		Fields:            merged,
		ReplaceLanguages:  patch.Languages != nil,
		ReplaceFocus:      patch.Focus != nil,
		ReplaceExperience: patch.Experience != nil,
	}); err != nil {
		return nil, err
	}
	// Editing a rejected profile is how a teacher answers the moderator's
	// note, so it goes back into the queue — no separate "resubmit" step.
	if current.Status == StatusRejected {
		if err := s.repo.Resubmit(ctx, ref.ID); err != nil {
			return nil, err
		}
		if s.notifier != nil {
			s.notifier.TeacherSubmitted(ctx, slug, merged.DisplayName, true)
		}
	}
	return s.repo.GetBySlug(ctx, slug)
}

// freeSlug returns slugify(displayName), appending -2, -3, … until it finds one
// no teacher is using.
func (s *Service) freeSlug(ctx context.Context, displayName string) (string, error) {
	base := slugify(displayName)
	for i := 0; i < maxSlugDedupeAttempts; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		exists, err := s.repo.SlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free slug for %q after %d attempts", displayName, maxSlugDedupeAttempts)
}
