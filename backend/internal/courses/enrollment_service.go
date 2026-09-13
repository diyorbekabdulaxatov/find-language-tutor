package courses

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// --- catalog ---

// Catalog returns a page of the public course catalog: published,
// non-archived courses from approved teachers only. Unauthenticated.
func (s *Service) Catalog(ctx context.Context, q CatalogQuery) (CatalogPage, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}
	switch q.Sort {
	case "":
		q.Sort = "newest"
	case "newest", "price_asc", "price_desc":
	default:
		return CatalogPage{}, invalid("`sort` must be one of newest, price_asc, price_desc.")
	}
	if q.MaxPriceMinor != nil && *q.MaxPriceMinor < 0 {
		return CatalogPage{}, invalid("`max_price_minor` can't be negative.")
	}

	entries, total, err := s.repo.CatalogList(ctx, q)
	if err != nil {
		return CatalogPage{}, err
	}
	if entries == nil {
		entries = []CatalogEntry{}
	}
	return CatalogPage{Entries: entries, Total: total}, nil
}

// CatalogDetail returns the public course landing page. 404 unless the
// course is published and not archived — draft/archived courses never leak
// to a non-owner viewer here. callerID is uuid.Nil for an unauthenticated
// viewer (is_enrolled / is_owner both read false).
func (s *Service) CatalogDetail(ctx context.Context, callerID, courseID uuid.UUID) (CatalogDetail, error) {
	c, err := s.repo.ByID(ctx, courseID)
	if err != nil {
		return CatalogDetail{}, err
	}
	if c.Status != StatusPublished || c.ArchivedAt != nil || c.SuspendedAt != nil {
		return CatalogDetail{}, ErrNotFound
	}
	teacher, err := s.repo.TeacherSummaryByID(ctx, c.TeacherID)
	if err != nil {
		return CatalogDetail{}, err
	}
	sections, err := s.repo.ListSections(ctx, courseID)
	if err != nil {
		return CatalogDetail{}, err
	}
	items, err := s.repo.ListItemsByCourse(ctx, courseID)
	if err != nil {
		return CatalogDetail{}, err
	}
	bySection := make(map[uuid.UUID][]ItemOutline, len(sections))
	for _, it := range items {
		bySection[it.SectionID] = append(bySection[it.SectionID], ItemOutline{
			ID: it.ID, Kind: it.Kind, Title: it.Title, Position: it.Position,
		})
	}
	outline := make([]SectionOutline, len(sections))
	for i, sec := range sections {
		outline[i] = SectionOutline{ID: sec.ID, Title: sec.Title, Position: sec.Position, Items: bySection[sec.ID]}
	}

	var isOwner, isEnrolled bool
	if callerID != uuid.Nil {
		isOwner = s.isCourseOwner(ctx, callerID, c.TeacherID)
		if !isOwner {
			_, ok, err := s.repo.EnrollmentByCourseAndStudent(ctx, courseID, callerID)
			if err != nil {
				return CatalogDetail{}, err
			}
			isEnrolled = ok
		}
	}

	return CatalogDetail{Course: c, Teacher: teacher, Outline: outline, IsEnrolled: isEnrolled, IsOwner: isOwner}, nil
}

// CoverImage streams a published course's cover image with no auth — 404
// when the course isn't published, is archived, or has no cover set, so a
// draft's cover is never leaked. A nil FileReader also fails closed to
// ErrNotFound (there is nothing sensible to serve without it).
func (s *Service) CoverImage(ctx context.Context, courseID uuid.UUID) (redirectURL string, body io.ReadCloser, contentType string, err error) {
	c, err := s.repo.ByID(ctx, courseID)
	if err != nil {
		return "", nil, "", err
	}
	if c.Status != StatusPublished || c.ArchivedAt != nil || c.SuspendedAt != nil || c.CoverAssetID == nil {
		return "", nil, "", ErrNotFound
	}
	if s.files == nil {
		return "", nil, "", ErrNotFound
	}
	return s.files.PublicAsset(ctx, *c.CoverAssetID)
}

// --- purchase / enrollment ---

// Purchase enrolls callerID in courseID: 404 unless the course is published,
// not archived, and not suspended (phase C3 — an operator takedown reads the
// same as an unpublished course to a would-be new buyer; see courses.go's
// SuspendedAt doc comment); 403 if callerID's own teacher profile owns the
// course; idempotent if already enrolled (returns the existing enrollment
// unchanged, no error); a 0-priced course enrolls directly with no payment
// step; otherwise the PaymentGateway is charged before the enrollment is
// created.
//
// justPurchased is true only for a brand-new PAID purchase (the handler's 201
// case) — false for an idempotent already-enrolled return or a fresh free
// enrollment (both the handler's 200 case).
func (s *Service) Purchase(ctx context.Context, callerID, courseID uuid.UUID, methodToken string) (summary EnrollmentSummary, justPurchased bool, err error) {
	c, err := s.repo.ByID(ctx, courseID)
	if err != nil {
		return EnrollmentSummary{}, false, err
	}
	if c.Status != StatusPublished || c.ArchivedAt != nil || c.SuspendedAt != nil {
		return EnrollmentSummary{}, false, ErrNotFound
	}
	if s.isCourseOwner(ctx, callerID, c.TeacherID) {
		return EnrollmentSummary{}, false, ErrCannotBuyOwnCourse
	}

	if existing, ok, eErr := s.repo.EnrollmentByCourseAndStudent(ctx, courseID, callerID); eErr != nil {
		return EnrollmentSummary{}, false, eErr
	} else if ok {
		result, sErr := s.enrollmentSummary(ctx, existing, c)
		return result, false, sErr
	}

	if c.PriceAmountMinor == 0 {
		enrolled, eErr := s.repo.EnsureEnrollment(ctx, courseID, callerID, EnrollmentFree, 0, c.PriceCurrency)
		if eErr != nil {
			return EnrollmentSummary{}, false, eErr
		}
		result, sErr := s.enrollmentSummary(ctx, enrolled, c)
		return result, false, sErr
	}

	if s.payments == nil {
		return EnrollmentSummary{}, false, ErrPurchaseUnavailable
	}
	snap, pErr := s.payments.Purchase(ctx, courseID, callerID, c.PriceAmountMinor, c.PriceCurrency, strings.TrimSpace(methodToken))
	if pErr != nil {
		return EnrollmentSummary{}, false, pErr
	}
	enrolled, eErr := s.repo.EnsureEnrollment(ctx, courseID, callerID, EnrollmentPurchase, snap.AmountMinor, snap.Currency)
	if eErr != nil {
		return EnrollmentSummary{}, false, eErr
	}

	// Phase C3: credit the teacher's revenue-share into the shared payout
	// ledger, now that the enrollment (and so a course_enrollment_id to attach
	// the row to) exists — unlike a booking, which exists before its payment,
	// a course_enrollment is created only after Purchase above already
	// captured the charge, so this can't happen inside the payments module's
	// own capture-webhook handling (see payments.CourseService.CreditCourseSale's
	// doc comment for the full reasoning). Best-effort: a ledger-write failure
	// must never turn an otherwise-successful purchase into an error response,
	// so it is logged and swallowed, same "caller-side, non-fatal" idiom as
	// ItemCompleted below. EnsureEnrollment's own idempotency plus this call's
	// ON CONFLICT (course_enrollment_id) DO NOTHING keep a retried Purchase
	// (or a concurrent double-submit) from ever double-crediting the teacher.
	if cErr := s.payments.CreditCourseSale(ctx, enrolled.ID, snap.AmountMinor, snap.Currency); cErr != nil {
		s.logger.Error("credit course sale to teacher payout ledger",
			slog.String("enrollment_id", enrolled.ID.String()),
			slog.String("course_id", courseID.String()),
			slog.Any("error", cErr))
	}

	result, sErr := s.enrollmentSummary(ctx, enrolled, c)
	return result, sErr == nil, sErr
}

// enrollmentSummary loads the total/completed item counts for one enrollment
// against its already-loaded course.
func (s *Service) enrollmentSummary(ctx context.Context, e Enrollment, c Course) (EnrollmentSummary, error) {
	items, err := s.repo.ListItemsByCourse(ctx, c.ID)
	if err != nil {
		return EnrollmentSummary{}, err
	}
	progress, err := s.repo.ListItemProgressForEnrollment(ctx, e.ID)
	if err != nil {
		return EnrollmentSummary{}, err
	}
	completed := 0
	for _, p := range progress {
		if p.Status == ItemCompleted {
			completed++
		}
	}
	return EnrollmentSummary{Enrollment: e, Course: c, TotalItems: len(items), CompletedItems: completed}, nil
}

// MyEnrollments returns the caller's "my learning" list, newest first.
func (s *Service) MyEnrollments(ctx context.Context, studentID uuid.UUID) ([]EnrollmentSummary, error) {
	rows, err := s.repo.ListEnrollmentsForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []EnrollmentSummary{}
	}
	return rows, nil
}

// --- the player ---

// Learn returns the full curriculum tree for the enrolled-student (or
// owning-teacher preview) player. 404 if the course doesn't exist; 403 if
// the caller is neither enrolled nor the owner.
func (s *Service) Learn(ctx context.Context, callerID, courseID uuid.UUID) (LearnDetail, error) {
	c, err := s.repo.ByID(ctx, courseID)
	if err != nil {
		return LearnDetail{}, err
	}

	isOwner := s.isCourseOwner(ctx, callerID, c.TeacherID)
	var enrollment *Enrollment
	if !isOwner {
		e, ok, err := s.repo.EnrollmentByCourseAndStudent(ctx, courseID, callerID)
		if err != nil {
			return LearnDetail{}, err
		}
		if !ok {
			return LearnDetail{}, ErrForbidden
		}
		enrollment = &e
	}

	sections, err := s.repo.ListSections(ctx, courseID)
	if err != nil {
		return LearnDetail{}, err
	}
	items, err := s.repo.ListItemsByCourse(ctx, courseID)
	if err != nil {
		return LearnDetail{}, err
	}

	var progressByItem map[uuid.UUID]ItemProgress
	if enrollment != nil {
		rows, err := s.repo.ListItemProgressForEnrollment(ctx, enrollment.ID)
		if err != nil {
			return LearnDetail{}, err
		}
		progressByItem = make(map[uuid.UUID]ItemProgress, len(rows))
		for _, p := range rows {
			progressByItem[p.ItemID] = p
		}
	}

	bySection := make(map[uuid.UUID][]LearnItem, len(sections))
	for _, it := range items {
		li := LearnItem{Item: it, Progress: ItemProgress{ItemID: it.ID, Status: ItemInProgress}}
		if p, ok := progressByItem[it.ID]; ok {
			li.Progress = p
		}
		if it.Kind == ItemKindResource && it.ResourceID != nil && s.resources != nil {
			view, err := s.resources.PublicResource(ctx, *it.ResourceID)
			if err != nil {
				return LearnDetail{}, err
			}
			li.Resource = &view
		}
		bySection[it.SectionID] = append(bySection[it.SectionID], li)
	}
	out := make([]LearnSection, len(sections))
	for i, sec := range sections {
		out[i] = LearnSection{Section: sec, Items: bySection[sec.ID]}
	}
	var enrollmentID *uuid.UUID
	if enrollment != nil {
		enrollmentID = &enrollment.ID
	}
	return LearnDetail{Course: c, EnrollmentID: enrollmentID, Sections: out}, nil
}

// RecordProgress upserts a video item's playback position and/or completion.
// Caller must be enrolled (403 otherwise); the item must be kind `video`
// (400 for `resource` — those complete through their submission instead).
func (s *Service) RecordProgress(ctx context.Context, callerID, courseID, itemID uuid.UUID, positionSeconds *int, completed *bool) (ItemProgress, error) {
	if _, err := s.repo.ByID(ctx, courseID); err != nil {
		return ItemProgress{}, err
	}
	e, ok, err := s.repo.EnrollmentByCourseAndStudent(ctx, courseID, callerID)
	if err != nil {
		return ItemProgress{}, err
	}
	if !ok {
		return ItemProgress{}, ErrForbidden
	}
	it, err := s.repo.ItemByID(ctx, itemID)
	if err != nil {
		return ItemProgress{}, err
	}
	sec, err := s.repo.SectionByID(ctx, it.SectionID)
	if err != nil {
		return ItemProgress{}, err
	}
	if sec.CourseID != courseID {
		return ItemProgress{}, ErrItemNotFound
	}
	if it.Kind != ItemKindVideo {
		return ItemProgress{}, invalid("Only video items track position/completion here — a resource item completes through its submission.")
	}
	if positionSeconds != nil && *positionSeconds < 0 {
		return ItemProgress{}, invalid("`position_seconds` can't be negative.")
	}
	return s.repo.UpsertItemProgress(ctx, e.ID, itemID, positionSeconds, completed)
}

// ItemCompleted satisfies resources.CourseProgress: a course-embedded
// resource's submission reaching submitted/graded marks the corresponding
// curriculum item complete. Best-effort from the caller's side (resources
// logs-and-swallows any error) — a resourceID that doesn't actually resolve
// to a curriculum item of this enrollment's course (shouldn't happen, since
// StartCourseSubmission already checked EnrollmentGrantsResource) is a no-op,
// not an error.
func (s *Service) ItemCompleted(ctx context.Context, enrollmentID, resourceID uuid.UUID) error {
	it, ok, err := s.repo.ItemForEnrollmentResource(ctx, enrollmentID, resourceID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	_, err = s.repo.CompleteItemProgress(ctx, enrollmentID, it.ID, s.now().UTC())
	return err
}

// --- structural-typing ports (no import of internal/resources or internal/files) ---

// Enrollment satisfies resources.EnrollmentReader: resolves an enrollment's
// two participants for authorization.
func (s *Service) Enrollment(ctx context.Context, enrollmentID uuid.UUID) (courseTeacherOwnerID, studentID uuid.UUID, found bool, err error) {
	return s.repo.EnrollmentParticipants(ctx, enrollmentID)
}

// EnrollmentGrantsResource satisfies resources.EnrollmentReader.
func (s *Service) EnrollmentGrantsResource(ctx context.Context, enrollmentID, resourceID uuid.UUID) (bool, error) {
	return s.repo.EnrollmentGrantsResource(ctx, enrollmentID, resourceID)
}

// StudentResourceAccess satisfies resources.EnrollmentReader.
func (s *Service) StudentResourceAccess(ctx context.Context, resourceID, studentID uuid.UUID) (bool, error) {
	return s.repo.StudentResourceAccess(ctx, resourceID, studentID)
}

// CanAccess satisfies files.AssigneeChecker: true iff fileAssetID is a video
// item's asset in some course requesterID is actively enrolled in.
func (s *Service) CanAccess(ctx context.Context, fileAssetID, requesterID uuid.UUID) (bool, error) {
	return s.repo.StudentHasVideoAccess(ctx, fileAssetID, requesterID)
}

// --- helpers ---

// isCourseOwner reports whether callerID's own teacher profile is
// courseTeacherID — swallowing "caller has no teacher profile" as simply
// false, since most callers here (students) never own one.
func (s *Service) isCourseOwner(ctx context.Context, callerID, courseTeacherID uuid.UUID) bool {
	tid, ok, err := s.repo.TeacherIDByOwner(ctx, callerID)
	return err == nil && ok && tid == courseTeacherID
}
