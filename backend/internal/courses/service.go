package courses

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Repository is the persistence port. Postgres impl alongside; fake in tests.
type Repository interface {
	// TeacherIDByOwner resolves the teacher profile owned by an account.
	TeacherIDByOwner(ctx context.Context, userID uuid.UUID) (uuid.UUID, bool, error)

	// Courses.
	Create(ctx context.Context, p CreateParams) (Course, error)
	ByID(ctx context.Context, id uuid.UUID) (Course, error)
	List(ctx context.Context, teacherID uuid.UUID, q ListQuery) ([]Course, int, error)
	Update(ctx context.Context, id uuid.UUID, p UpdateParams) (Course, error)
	SetStatus(ctx context.Context, id uuid.UUID, status Status) (Course, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (Course, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Sections.
	AddSection(ctx context.Context, courseID uuid.UUID, title string) (Section, error)
	SectionByID(ctx context.Context, id uuid.UUID) (Section, error)
	ListSections(ctx context.Context, courseID uuid.UUID) ([]Section, error)
	RenameSection(ctx context.Context, id uuid.UUID, title string) (Section, error)
	DeleteSection(ctx context.Context, id uuid.UUID) error
	// ReorderSections rewrites positions 0..n-1 from orderedIDs in one
	// statement. The caller (Service) has already validated orderedIDs is
	// exactly courseID's current section ids.
	ReorderSections(ctx context.Context, courseID uuid.UUID, orderedIDs []uuid.UUID) error

	// Items.
	AddItem(ctx context.Context, p AddItemParams) (Item, error)
	ItemByID(ctx context.Context, id uuid.UUID) (Item, error)
	ListItems(ctx context.Context, sectionID uuid.UUID) ([]Item, error)
	// ListItemsByCourse returns every item across courseID's sections, in
	// curriculum order, for assembling a CourseDetail in one extra query.
	ListItemsByCourse(ctx context.Context, courseID uuid.UUID) ([]Item, error)
	// UpdateItem edits an item's title and, for a video item, its phase-D1
	// preview flag / duration. A nil optional field leaves the stored value
	// alone, so the plain rename path passes both as nil.
	UpdateItem(ctx context.Context, id uuid.UUID, p UpdateItemParams) (Item, error)
	DeleteItem(ctx context.Context, id uuid.UUID) error
	// ReorderItems rewrites positions 0..n-1 from orderedIDs in one statement.
	// The caller (Service) has already validated orderedIDs is exactly
	// sectionID's current item ids.
	ReorderItems(ctx context.Context, sectionID uuid.UUID, orderedIDs []uuid.UUID) error

	// --- catalog (phase C2) ---

	// TeacherOwnerID resolves the account that owns a teacher profile (the
	// reverse of TeacherIDByOwner). ok is false for an unclaimed profile.
	TeacherOwnerID(ctx context.Context, teacherID uuid.UUID) (uuid.UUID, bool, error)
	// TeacherSummaryByID loads the light teacher identity embedded in catalog rows.
	TeacherSummaryByID(ctx context.Context, teacherID uuid.UUID) (TeacherSummary, error)
	// CatalogList returns a page of published, non-archived courses from
	// approved teachers, newest-first unless q.Sort says otherwise.
	CatalogList(ctx context.Context, q CatalogQuery) ([]CatalogEntry, int, error)
	// PreviewItem (phase D1) resolves a curriculum item together with the
	// storefront state of the course it actually belongs to, for the public
	// preview-stream decision. ErrItemNotFound when the item doesn't exist or
	// isn't part of courseID.
	PreviewItem(ctx context.Context, courseID, itemID uuid.UUID) (PreviewRef, error)

	// --- enrollment (phase C2) ---

	// EnsureEnrollment inserts a new enrollment, or — on the (course_id,
	// student_id) unique constraint's 23505 — loads and returns the existing
	// one. Insert-first idempotency, never check-then-insert.
	EnsureEnrollment(ctx context.Context, courseID, studentID uuid.UUID, source EnrollmentSource, amountPaidMinor int64, currency string) (Enrollment, error)
	EnrollmentByCourseAndStudent(ctx context.Context, courseID, studentID uuid.UUID) (Enrollment, bool, error)
	// EnrollmentParticipants resolves an enrollment's two participants
	// directly: the course's teacher's owning account, and the enrolled
	// student. found is false when the enrollment id is unknown (or its
	// teacher profile is unclaimed, with no owning account to authorize).
	EnrollmentParticipants(ctx context.Context, enrollmentID uuid.UUID) (teacherOwnerID, studentID uuid.UUID, found bool, err error)
	// ListEnrollmentsForStudent is the "my learning" list, newest first, each
	// row already carrying its course summary and progress counts.
	ListEnrollmentsForStudent(ctx context.Context, studentID uuid.UUID) ([]EnrollmentSummary, error)
	// EnrollmentGrantsResource reports whether the given enrollment's course
	// actually embeds resourceID as a curriculum item.
	EnrollmentGrantsResource(ctx context.Context, enrollmentID, resourceID uuid.UUID) (bool, error)
	// StudentResourceAccess reports whether studentID has some active
	// enrollment granting access to resourceID.
	StudentResourceAccess(ctx context.Context, resourceID, studentID uuid.UUID) (bool, error)
	// StudentHasVideoAccess reports whether studentID has some active
	// enrollment whose course embeds fileAssetID as a video item.
	StudentHasVideoAccess(ctx context.Context, fileAssetID, studentID uuid.UUID) (bool, error)
	// ItemForEnrollmentResource resolves the curriculum item a course-context
	// submission's resource corresponds to, scoped to the enrollment's own
	// course. found is false when resourceID isn't actually embedded there.
	ItemForEnrollmentResource(ctx context.Context, enrollmentID, resourceID uuid.UUID) (Item, bool, error)

	// --- progress (phase C2) ---

	// UpsertItemProgress merges the given optional fields into a video item's
	// progress row (creating it on first touch).
	UpsertItemProgress(ctx context.Context, enrollmentID, itemID uuid.UUID, positionSeconds *int, completed *bool) (ItemProgress, error)
	// CompleteItemProgress marks an item complete unconditionally, from the
	// course-context submission path (resources.CourseProgress).
	CompleteItemProgress(ctx context.Context, enrollmentID, itemID uuid.UUID, completedAt time.Time) (ItemProgress, error)
	ListItemProgressForEnrollment(ctx context.Context, enrollmentID uuid.UUID) ([]ItemProgress, error)

	// --- admin moderation (phase C3) ---

	// AdminList returns a page of the moderation queue: every course
	// regardless of status/teacher/suspension, newest first.
	AdminList(ctx context.Context, q AdminCourseQuery, limit, offset int) ([]AdminCourse, int, error)
	// SetSuspended sets or clears a course's operator takedown flag.
	// Idempotent — setting the same value twice is a no-op that still returns
	// the current row. ErrNotFound for an unknown id.
	SetSuspended(ctx context.Context, id uuid.UUID, suspended bool) (AdminCourse, error)
}

// CreateParams is the repository's course-insert payload.
type CreateParams struct {
	TeacherID        uuid.UUID
	Title            string
	Subtitle         string
	Description      string
	PriceAmountMinor int64
	PriceCurrency    string
}

// UpdateParams is the repository's course-update payload (a full replace of
// the editable fields).
type UpdateParams struct {
	Title            string
	Subtitle         string
	Description      string
	CoverAssetID     *uuid.UUID
	PriceAmountMinor int64
	PriceCurrency    string
}

// AddItemParams is the repository's item-insert payload.
type AddItemParams struct {
	SectionID       uuid.UUID
	Kind            ItemKind
	Title           string
	VideoAssetID    *uuid.UUID
	ResourceID      *uuid.UUID
	IsPreview       bool
	DurationSeconds int
}

// UpdateItemParams is the repository's item-edit payload. Title is a full
// replace ("" clears the override); the two pointers are merge-on-write, so
// nil means "leave as stored".
type UpdateItemParams struct {
	Title           string
	IsPreview       *bool
	DurationSeconds *int
}

// Service holds the authoring rules. Handlers call it; it never sees a *gin.Context.
type Service struct {
	repo      Repository
	resources ResourceReader // nil until SetResourceReader; guarded, fails closed
	files     FileReader     // nil until SetFileReader; guarded, fails closed
	payments  PaymentGateway // nil until SetPaymentGateway; guarded, fails closed (phase C2)
	reviews   ReviewRepository // nil until SetReviewRepository; guarded (phase D2)
	accounts  AccountReader  // nil until SetAccountReader; publish fails closed
	now       func() time.Time
	logger    *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, now: time.Now, logger: logger}
}

// SetResourceReader wires the resources module's read port in. Optional, but
// every `resource`-kind AddItem call fails closed (validation error) without it.
func (s *Service) SetResourceReader(r ResourceReader) { s.resources = r }

// SetFileReader wires the files module's read port in. Optional, but every
// `video`-kind AddItem call and every cover-image Update fails closed
// (validation error) without it.
func (s *Service) SetFileReader(f FileReader) { s.files = f }

// SetPaymentGateway wires the payments module's course-purchase adapter in
// (phase C2). Optional, but Purchase on a priced course fails closed
// (ErrPurchaseUnavailable) without it — a free course still enrolls fine.
func (s *Service) SetPaymentGateway(p PaymentGateway) { s.payments = p }

// SetAccountReader wires the auth module's email-verification lookup in.
// Publishing fails closed (ErrEmailNotVerified) without it.
func (s *Service) SetAccountReader(a AccountReader) { s.accounts = a }

// --- courses ---

// Library returns a page of the caller's courses (summary rows, no curriculum).
func (s *Service) Library(ctx context.Context, ownerID uuid.UUID, q ListQuery) (Page, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Page{}, err
	}
	if q.Status != "" && q.Status != StatusDraft && q.Status != StatusPublished {
		return Page{}, invalid("Status filter must be draft or published.")
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}

	items, total, err := s.repo.List(ctx, tid, q)
	if err != nil {
		return Page{}, err
	}
	if items == nil {
		items = []Course{}
	}
	return Page{Courses: items, Total: total}, nil
}

// Create authors a new, empty draft course.
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, title, subtitle, description string, priceAmountMinor int64, priceCurrency string) (CourseDetail, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return CourseDetail{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return CourseDetail{}, invalid("Give the course a title.")
	}
	if err := validateCourseText(title, subtitle, description); err != nil {
		return CourseDetail{}, err
	}
	if priceAmountMinor < 0 {
		return CourseDetail{}, invalid("Price can't be negative.")
	}
	currency, err := normalizeCurrency(priceCurrency)
	if err != nil {
		return CourseDetail{}, err
	}
	c, err := s.repo.Create(ctx, CreateParams{
		TeacherID:        tid,
		Title:            title,
		Subtitle:         strings.TrimSpace(subtitle),
		Description:      strings.TrimSpace(description),
		PriceAmountMinor: priceAmountMinor,
		PriceCurrency:    currency,
	})
	if err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// Get returns one of the caller's own courses with its full curriculum tree.
func (s *Service) Get(ctx context.Context, ownerID, id uuid.UUID) (CourseDetail, error) {
	c, _, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// Update replaces the editable course fields. A non-nil coverAssetID is
// validated through FileReader: must belong to the caller and be an image.
func (s *Service) Update(ctx context.Context, ownerID, id uuid.UUID, title, subtitle, description string, coverAssetID *uuid.UUID, priceAmountMinor int64, priceCurrency string) (CourseDetail, error) {
	_, _, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return CourseDetail{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return CourseDetail{}, invalid("Give the course a title.")
	}
	if err := validateCourseText(title, subtitle, description); err != nil {
		return CourseDetail{}, err
	}
	if priceAmountMinor < 0 {
		return CourseDetail{}, invalid("Price can't be negative.")
	}
	currency, err := normalizeCurrency(priceCurrency)
	if err != nil {
		return CourseDetail{}, err
	}
	if coverAssetID != nil {
		if err := s.checkFileOwned(ctx, *coverAssetID, ownerID, "image/", "The cover must be an image you've uploaded."); err != nil {
			return CourseDetail{}, err
		}
	}
	updated, err := s.repo.Update(ctx, id, UpdateParams{
		Title: title, Subtitle: strings.TrimSpace(subtitle), Description: strings.TrimSpace(description),
		CoverAssetID: coverAssetID, PriceAmountMinor: priceAmountMinor, PriceCurrency: currency,
	})
	if err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, updated)
}

// SetPublished publishes / unpublishes a course. Publishing requires at least
// one section, each with at least one item; price may be 0 (a free course is
// valid). Unpublishing has no curriculum requirement.
func (s *Service) SetPublished(ctx context.Context, ownerID, id uuid.UUID, publish bool) (CourseDetail, error) {
	_, tid, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return CourseDetail{}, err
	}
	if publish {
		// Moderation gate: a pending / rejected / suspended teacher can draft,
		// but nothing of theirs goes on the storefront. The catalog and
		// purchase paths re-check, so a later suspension takes effect too.
		if err := s.requireApprovedTeacher(ctx, tid); err != nil {
			return CourseDetail{}, err
		}
		// Same idea for the account: nothing goes on sale from an address
		// nobody has confirmed. Checked at publish only — a teacher who was
		// approved has already been through it.
		if err := s.requireVerifiedEmail(ctx, ownerID); err != nil {
			return CourseDetail{}, err
		}
		sections, err := s.repo.ListSections(ctx, id)
		if err != nil {
			return CourseDetail{}, err
		}
		if len(sections) == 0 {
			return CourseDetail{}, invalid("Add at least one section.")
		}
		items, err := s.repo.ListItemsByCourse(ctx, id)
		if err != nil {
			return CourseDetail{}, err
		}
		counts := make(map[uuid.UUID]int, len(sections))
		for _, it := range items {
			counts[it.SectionID]++
		}
		for _, sec := range sections {
			if counts[sec.ID] == 0 {
				return CourseDetail{}, invalid("Section %q has no items yet.", sec.Title)
			}
		}
	}
	status := StatusDraft
	if publish {
		status = StatusPublished
	}
	updated, err := s.repo.SetStatus(ctx, id, status)
	if err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, updated)
}

// SetArchived archives / restores a course.
func (s *Service) SetArchived(ctx context.Context, ownerID, id uuid.UUID, archived bool) (CourseDetail, error) {
	if _, _, err := s.owned(ctx, ownerID, id); err != nil {
		return CourseDetail{}, err
	}
	updated, err := s.repo.SetArchived(ctx, id, archived)
	if err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, updated)
}

// Delete removes a course that has never been published. ErrInUse otherwise
// (archive it instead).
func (s *Service) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	c, _, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if c.EverPublished {
		return ErrInUse
	}
	return s.repo.Delete(ctx, id)
}

// --- sections ---

// AddSection appends a new section to the course.
func (s *Service) AddSection(ctx context.Context, ownerID, courseID uuid.UUID, title string) (CourseDetail, error) {
	c, _, err := s.owned(ctx, ownerID, courseID)
	if err != nil {
		return CourseDetail{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return CourseDetail{}, invalid("Give the section a title.")
	}
	if utf8.RuneCountInString(title) > maxCourseTitle {
		return CourseDetail{}, invalid("Title must be at most %d characters.", maxCourseTitle)
	}
	if _, err := s.repo.AddSection(ctx, courseID, title); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// RenameSection edits a section's title.
func (s *Service) RenameSection(ctx context.Context, ownerID, courseID, sectionID uuid.UUID, title string) (CourseDetail, error) {
	c, _, _, err := s.ownedSection(ctx, ownerID, courseID, sectionID)
	if err != nil {
		return CourseDetail{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return CourseDetail{}, invalid("Give the section a title.")
	}
	if utf8.RuneCountInString(title) > maxCourseTitle {
		return CourseDetail{}, invalid("Title must be at most %d characters.", maxCourseTitle)
	}
	if _, err := s.repo.RenameSection(ctx, sectionID, title); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// DeleteSection removes a section and its items (DB cascade).
func (s *Service) DeleteSection(ctx context.Context, ownerID, courseID, sectionID uuid.UUID) (CourseDetail, error) {
	c, _, _, err := s.ownedSection(ctx, ownerID, courseID, sectionID)
	if err != nil {
		return CourseDetail{}, err
	}
	if err := s.repo.DeleteSection(ctx, sectionID); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// ReorderSections rewrites a course's section order. orderedIDs must contain
// exactly the course's current section ids, each once, else a ValidationError.
func (s *Service) ReorderSections(ctx context.Context, ownerID, courseID uuid.UUID, orderedIDs []uuid.UUID) (CourseDetail, error) {
	c, _, err := s.owned(ctx, ownerID, courseID)
	if err != nil {
		return CourseDetail{}, err
	}
	current, err := s.repo.ListSections(ctx, courseID)
	if err != nil {
		return CourseDetail{}, err
	}
	currentIDs := make([]uuid.UUID, len(current))
	for i, sec := range current {
		currentIDs[i] = sec.ID
	}
	if err := validateIDSet(currentIDs, orderedIDs, "sections"); err != nil {
		return CourseDetail{}, err
	}
	if err := s.repo.ReorderSections(ctx, courseID, orderedIDs); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// --- items ---

// AddItem appends a new item to a section: either a video (validated via
// FileReader — must be the caller's own file and a video content type) or a
// resource (validated via ResourceReader — must be the caller's own resource
// and published, not archived).
func (s *Service) AddItem(ctx context.Context, ownerID, courseID, sectionID uuid.UUID, kind ItemKind, title string, videoAssetID, resourceID *uuid.UUID, isPreview bool, durationSeconds int) (CourseDetail, error) {
	c, tid, _, err := s.ownedSection(ctx, ownerID, courseID, sectionID)
	if err != nil {
		return CourseDetail{}, err
	}
	if !kind.valid() {
		return CourseDetail{}, invalid("`kind` must be `video` or `resource`.")
	}
	title = strings.TrimSpace(title)

	switch kind {
	case ItemKindVideo:
		if videoAssetID == nil {
			return CourseDetail{}, invalid("A video item needs `video_asset_id`.")
		}
		if resourceID != nil {
			return CourseDetail{}, invalid("A video item can't also reference a resource.")
		}
		if err := s.checkFileOwned(ctx, *videoAssetID, ownerID, "video/", "That file isn't a video you've uploaded."); err != nil {
			return CourseDetail{}, err
		}
		if err := validDuration(durationSeconds); err != nil {
			return CourseDetail{}, err
		}
	case ItemKindResource:
		if resourceID == nil {
			return CourseDetail{}, invalid("A resource item needs `resource_id`.")
		}
		if videoAssetID != nil {
			return CourseDetail{}, invalid("A resource item can't also reference a video file.")
		}
		if s.resources == nil {
			return CourseDetail{}, invalid("Resource items aren't available right now.")
		}
		ok, err := s.resources.ResourceOwnedAndPublished(ctx, *resourceID, tid)
		if err != nil {
			return CourseDetail{}, err
		}
		if !ok {
			return CourseDetail{}, invalid("That resource isn't one of your published resources.")
		}
		// Phase D1: preview is a video-only affordance. Refusing here (rather
		// than silently dropping the flag) means a client that asks for
		// something the DB CHECK would reject gets told why.
		if isPreview {
			return CourseDetail{}, invalid("Only a video lesson can be a free preview.")
		}
		if durationSeconds != 0 {
			return CourseDetail{}, invalid("Only a video lesson has a duration.")
		}
	}

	if _, err := s.repo.AddItem(ctx, AddItemParams{
		SectionID: sectionID, Kind: kind, Title: title, VideoAssetID: videoAssetID, ResourceID: resourceID,
		IsPreview: isPreview, DurationSeconds: durationSeconds,
	}); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// UpdateItem edits an item's display-title override ("" clears it, falling
// back to the video filename / resource title in the UI) and, for a video
// item, its phase-D1 preview flag and duration. A nil isPreview /
// durationSeconds leaves the stored value alone, so a plain rename passes
// both as nil.
func (s *Service) UpdateItem(ctx context.Context, ownerID, courseID, sectionID, itemID uuid.UUID, title string, isPreview *bool, durationSeconds *int) (CourseDetail, error) {
	c, _, it, err := s.ownedItem(ctx, ownerID, courseID, sectionID, itemID)
	if err != nil {
		return CourseDetail{}, err
	}
	// Same video-only rule as AddItem, applied to whichever field is being
	// set. Clearing a flag that is already false is not an error — only
	// asking a resource item to *become* a preview is.
	if it.Kind != ItemKindVideo {
		if isPreview != nil && *isPreview {
			return CourseDetail{}, invalid("Only a video lesson can be a free preview.")
		}
		if durationSeconds != nil && *durationSeconds != 0 {
			return CourseDetail{}, invalid("Only a video lesson has a duration.")
		}
	}
	if durationSeconds != nil {
		if err := validDuration(*durationSeconds); err != nil {
			return CourseDetail{}, err
		}
	}
	if _, err := s.repo.UpdateItem(ctx, itemID, UpdateItemParams{
		Title: strings.TrimSpace(title), IsPreview: isPreview, DurationSeconds: durationSeconds,
	}); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// validDuration bounds a client-reported video length. 0 is legal and means
// "unknown" (a container the browser couldn't read a duration from).
func validDuration(d int) error {
	if d < 0 {
		return invalid("`duration_seconds` can't be negative.")
	}
	if d > MaxItemDurationSeconds {
		return invalid("`duration_seconds` can't be more than %d.", MaxItemDurationSeconds)
	}
	return nil
}

// DeleteItem removes an item from its section.
func (s *Service) DeleteItem(ctx context.Context, ownerID, courseID, sectionID, itemID uuid.UUID) (CourseDetail, error) {
	c, _, _, err := s.ownedItem(ctx, ownerID, courseID, sectionID, itemID)
	if err != nil {
		return CourseDetail{}, err
	}
	if err := s.repo.DeleteItem(ctx, itemID); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// ReorderItems rewrites a section's item order. orderedIDs must contain
// exactly the section's current item ids, each once, else a ValidationError.
func (s *Service) ReorderItems(ctx context.Context, ownerID, courseID, sectionID uuid.UUID, orderedIDs []uuid.UUID) (CourseDetail, error) {
	c, _, _, err := s.ownedSection(ctx, ownerID, courseID, sectionID)
	if err != nil {
		return CourseDetail{}, err
	}
	current, err := s.repo.ListItems(ctx, sectionID)
	if err != nil {
		return CourseDetail{}, err
	}
	currentIDs := make([]uuid.UUID, len(current))
	for i, it := range current {
		currentIDs[i] = it.ID
	}
	if err := validateIDSet(currentIDs, orderedIDs, "items"); err != nil {
		return CourseDetail{}, err
	}
	if err := s.repo.ReorderItems(ctx, sectionID, orderedIDs); err != nil {
		return CourseDetail{}, err
	}
	return s.detail(ctx, c)
}

// --- helpers ---

func (s *Service) teacher(ctx context.Context, ownerID uuid.UUID) (uuid.UUID, error) {
	tid, ok, err := s.repo.TeacherIDByOwner(ctx, ownerID)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, ErrNoTeacher
	}
	return tid, nil
}

// owned resolves a course the caller owns, returning it plus their teacher id.
func (s *Service) owned(ctx context.Context, ownerID, id uuid.UUID) (Course, uuid.UUID, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Course{}, uuid.Nil, err
	}
	c, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Course{}, uuid.Nil, err
	}
	if c.TeacherID != tid {
		return Course{}, uuid.Nil, ErrForbidden
	}
	return c, tid, nil
}

// ownedSection resolves a section the caller owns via its course, returning
// the course, the caller's teacher id, and the section.
func (s *Service) ownedSection(ctx context.Context, ownerID, courseID, sectionID uuid.UUID) (Course, uuid.UUID, Section, error) {
	c, tid, err := s.owned(ctx, ownerID, courseID)
	if err != nil {
		return Course{}, uuid.Nil, Section{}, err
	}
	sec, err := s.repo.SectionByID(ctx, sectionID)
	if err != nil {
		return Course{}, uuid.Nil, Section{}, err
	}
	if sec.CourseID != courseID {
		return Course{}, uuid.Nil, Section{}, ErrSectionNotFound
	}
	return c, tid, sec, nil
}

// ownedItem resolves an item the caller owns via its section and course.
func (s *Service) ownedItem(ctx context.Context, ownerID, courseID, sectionID, itemID uuid.UUID) (Course, uuid.UUID, Item, error) {
	c, tid, _, err := s.ownedSection(ctx, ownerID, courseID, sectionID)
	if err != nil {
		return Course{}, uuid.Nil, Item{}, err
	}
	it, err := s.repo.ItemByID(ctx, itemID)
	if err != nil {
		return Course{}, uuid.Nil, Item{}, err
	}
	if it.SectionID != sectionID {
		return Course{}, uuid.Nil, Item{}, ErrItemNotFound
	}
	return c, tid, it, nil
}

// detail assembles a CourseDetail: the course plus its curriculum tree, in
// two queries regardless of section count (ListSections + ListItemsByCourse).
func (s *Service) detail(ctx context.Context, c Course) (CourseDetail, error) {
	sections, err := s.repo.ListSections(ctx, c.ID)
	if err != nil {
		return CourseDetail{}, err
	}
	items, err := s.repo.ListItemsByCourse(ctx, c.ID)
	if err != nil {
		return CourseDetail{}, err
	}
	bySection := make(map[uuid.UUID][]Item, len(sections))
	for _, it := range items {
		bySection[it.SectionID] = append(bySection[it.SectionID], it)
	}
	out := make([]SectionDetail, len(sections))
	for i, sec := range sections {
		out[i] = SectionDetail{Section: sec, Items: bySection[sec.ID]}
	}
	return CourseDetail{Course: c, Sections: out}, nil
}

// checkFileOwned validates fileAssetID through FileReader: must belong to
// callerID and have a content type starting with wantPrefix ("image/" or
// "video/"). A nil FileReader fails closed.
func (s *Service) checkFileOwned(ctx context.Context, fileAssetID, callerID uuid.UUID, wantPrefix, mismatchMsg string) error {
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
		return invalid("%s", mismatchMsg)
	}
	return nil
}

// normalizeCurrency defaults "" to UZS and validates against the
// currency_code enum (UZS, USD).
// Text caps for the course fields the catalog renders.
const (
	maxCourseTitle       = 120
	maxCourseSubtitle    = 200
	maxCourseDescription = 8000
)

func validateCourseText(title, subtitle, description string) error {
	switch {
	case utf8.RuneCountInString(title) > maxCourseTitle:
		return invalid("Title must be at most %d characters.", maxCourseTitle)
	case utf8.RuneCountInString(subtitle) > maxCourseSubtitle:
		return invalid("Subtitle must be at most %d characters.", maxCourseSubtitle)
	case utf8.RuneCountInString(description) > maxCourseDescription:
		return invalid("Description must be at most %d characters.", maxCourseDescription)
	}
	return nil
}

// normalizeCurrency: UZS only, like teacher pricing — the payout ledger sums
// without a currency dimension, so a second currency would silently mix.
func normalizeCurrency(c string) (string, error) {
	switch c {
	case "", "UZS":
		return "UZS", nil
	default:
		return "", invalid("`price_currency` must be UZS.")
	}
}

// requireVerifiedEmail is the publish-side email gate. Fails closed when no
// AccountReader is wired.
func (s *Service) requireVerifiedEmail(ctx context.Context, userID uuid.UUID) error {
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

// requireApprovedTeacher is the moderation gate shared by publish, the
// storefront and purchase.
func (s *Service) requireApprovedTeacher(ctx context.Context, teacherID uuid.UUID) error {
	t, err := s.repo.TeacherSummaryByID(ctx, teacherID)
	if err != nil {
		return err
	}
	if !t.Approved {
		return ErrTeacherNotApproved
	}
	return nil
}

// validateIDSet reports whether proposed contains exactly the ids in current,
// each exactly once (order-independent membership check), returning a
// ValidationError describing noun (e.g. "sections", "items") otherwise.
func validateIDSet(current, proposed []uuid.UUID, noun string) error {
	if len(current) != len(proposed) {
		return invalid("Reordering must include exactly the course's current %s, each once.", noun)
	}
	set := make(map[uuid.UUID]bool, len(current))
	for _, id := range current {
		set[id] = true
	}
	seen := make(map[uuid.UUID]bool, len(proposed))
	for _, id := range proposed {
		if !set[id] || seen[id] {
			return invalid("Reordering must include exactly the course's current %s, each once.", noun)
		}
		seen[id] = true
	}
	return nil
}
