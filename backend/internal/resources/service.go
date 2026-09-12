package resources

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repository is the persistence port. Postgres impl alongside; fake in tests.
type Repository interface {
	// TeacherIDByOwner resolves the teacher profile owned by an account.
	TeacherIDByOwner(ctx context.Context, userID uuid.UUID) (uuid.UUID, bool, error)

	Create(ctx context.Context, p CreateParams) (Resource, error)
	ByID(ctx context.Context, id uuid.UUID) (Resource, error)
	List(ctx context.Context, teacherID uuid.UUID, q ListQuery) ([]Resource, int, error)
	Update(ctx context.Context, id uuid.UUID, title, instructions string, content Content) (Resource, error)
	SetStatus(ctx context.Context, id uuid.UUID, status Status) (Resource, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (Resource, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// IsAssigned reports whether the resource is attached to any lesson / course.
	IsAssigned(ctx context.Context, id uuid.UUID) (bool, error)

	// --- phase A2: booking attachment ---

	// AttachResource attaches a resource to a booking. A duplicate
	// (bookingID, resourceID) maps to ErrAlreadyAttached (race-safe, never a
	// check-then-insert). position is assigned by the repository.
	AttachResource(ctx context.Context, bookingID, resourceID uuid.UUID, kind string, assignedBy uuid.UUID, dueAt *time.Time) (BookingResource, error)
	// DetachResource removes an attachment. ErrAttachmentNotFound when it
	// doesn't exist on this booking.
	DetachResource(ctx context.Context, bookingID, attachmentID uuid.UUID) error
	// ListBookingResources returns a booking's attachments, ordered by position.
	ListBookingResources(ctx context.Context, bookingID uuid.UUID) ([]BookingResource, error)
	// GetBookingResourceByPair looks up a booking's attachment of one specific
	// resource. ok is false when the resource is not attached to the booking.
	GetBookingResourceByPair(ctx context.Context, bookingID, resourceID uuid.UUID) (BookingResource, bool, error)

	// --- phase A3: submissions ---

	// StartSubmission idempotently gets-or-creates a student's submission for
	// a lesson homework.
	StartSubmission(ctx context.Context, resourceID, studentID, bookingID uuid.UUID) (Submission, error)
	// StartCourseSubmission idempotently gets-or-creates a student's
	// submission against a course-embedded resource (phase C2). Mirrors
	// StartSubmission's upsert idiom against submissions_course_uniq instead
	// of submissions_lesson_uniq.
	StartCourseSubmission(ctx context.Context, resourceID, studentID, enrollmentID uuid.UUID) (Submission, error)
	SaveSubmissionAnswers(ctx context.Context, id uuid.UUID, answers map[string][]string) (Submission, error)
	SubmitSubmission(ctx context.Context, id uuid.UUID, p SubmitParams) (Submission, error)
	GradeSubmission(ctx context.Context, id uuid.UUID, score *int, feedback string, gradedBy uuid.UUID, gradedAt time.Time) (Submission, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (Submission, error)
	// ListSubmissionsForBooking returns every submission filed against one
	// booking's homeworks (a booking has exactly one student).
	ListSubmissionsForBooking(ctx context.Context, bookingID uuid.UUID) ([]Submission, error)
	// InboxSubmissions is the teacher grading inbox: a teacher's own resources'
	// submissions, filtered by status ("" = no filter), newest-submitted-first.
	InboxSubmissions(ctx context.Context, teacherID uuid.UUID, status string, limit, offset int) ([]Submission, int, error)

	// FileAssetAccessible reports whether some resource carrying fileAssetID
	// (as its material file or listening audio) is attached to a booking whose
	// student is requesterID. Backs files.AssigneeChecker.
	FileAssetAccessible(ctx context.Context, fileAssetID, requesterID uuid.UUID) (bool, error)
	// ResourceIDsForFileAsset resolves which resource(s) reference fileAssetID
	// as their material file or listening audio — backs FileAssetAccessible's
	// course-based widening (phase C2): the service checks each returned id
	// against EnrollmentReader.StudentResourceAccess.
	ResourceIDsForFileAsset(ctx context.Context, fileAssetID uuid.UUID) ([]uuid.UUID, error)
	// UserContact is a plain contact lookup, used only for the grading-done email.
	UserContact(ctx context.Context, userID uuid.UUID) (email, displayName string, err error)
}

// CreateParams is the repository's insert payload.
type CreateParams struct {
	TeacherID    uuid.UUID
	Type         Type
	Title        string
	Instructions string
	Content      Content
	Status       Status
}

// Service holds the authoring rules. Handlers call it; it never sees a *gin.Context.
type Service struct {
	repo           Repository
	bookings       BookingReader    // nil until SetBookingReader; guarded, fails closed
	enrollments    EnrollmentReader // nil until SetEnrollmentReader; guarded, fails closed (phase C2)
	mailer         Mailer           // never nil (noopMailer default)
	courseProgress CourseProgress   // nil until SetCourseProgress; optional, best-effort (phase C2)
	now            func() time.Time
	logger         *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, mailer: noopMailer{}, now: time.Now, logger: logger}
}

// SetBookingReader wires the bookings module's read port in. Optional, but
// every attach / detach / submission call fails closed (booking not found)
// without it.
func (s *Service) SetBookingReader(b BookingReader) { s.bookings = b }

// SetMailer wires the submission-graded email adapter in. Optional: without
// it, grading a submission sends nothing.
func (s *Service) SetMailer(m Mailer) {
	if m != nil {
		s.mailer = m
	}
}

// SetEnrollmentReader wires the courses module's read port in (phase C2).
// Optional, but every course-context submission call fails closed
// (enrollment not found) without it.
func (s *Service) SetEnrollmentReader(e EnrollmentReader) { s.enrollments = e }

// SetCourseProgress wires the courses module's best-effort progress-write
// port in (phase C2). Optional: without it, submitting a course-embedded
// resource marks nothing complete on the course side.
func (s *Service) SetCourseProgress(p CourseProgress) { s.courseProgress = p }

// Library returns a page of the caller's resources.
func (s *Service) Library(ctx context.Context, ownerID uuid.UUID, q ListQuery) (Page, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Page{}, err
	}
	if q.Type != "" && !q.Type.valid() {
		return Page{}, invalid("Unknown resource type filter.")
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
		items = []Resource{}
	}
	return Page{Resources: items, Total: total}, nil
}

// Create authors a new resource (draft or published).
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, t Type, title, instructions string, content Content, publish bool) (Resource, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Resource{}, err
	}
	if !t.valid() {
		return Resource{}, invalid("Unknown resource type.")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Resource{}, invalid("Give the resource a title.")
	}
	cleaned, err := normalizeAndValidateContent(t, content)
	if err != nil {
		return Resource{}, err
	}
	status := StatusDraft
	if publish {
		status = StatusPublished
	}
	return s.repo.Create(ctx, CreateParams{
		TeacherID:    tid,
		Type:         t,
		Title:        title,
		Instructions: strings.TrimSpace(instructions),
		Content:      cleaned,
		Status:       status,
	})
}

// Get returns one of the caller's own resources.
func (s *Service) Get(ctx context.Context, ownerID, id uuid.UUID) (Resource, error) {
	return s.owned(ctx, ownerID, id)
}

// Update replaces the editable fields. Type is immutable; content is
// re-validated against the existing type.
func (s *Service) Update(ctx context.Context, ownerID, id uuid.UUID, title, instructions string, content Content) (Resource, error) {
	r, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return Resource{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Resource{}, invalid("Give the resource a title.")
	}
	cleaned, err := normalizeAndValidateContent(r.Type, content)
	if err != nil {
		return Resource{}, err
	}
	return s.repo.Update(ctx, id, title, strings.TrimSpace(instructions), cleaned)
}

// SetPublished publishes / unpublishes a resource.
func (s *Service) SetPublished(ctx context.Context, ownerID, id uuid.UUID, published bool) (Resource, error) {
	if _, err := s.owned(ctx, ownerID, id); err != nil {
		return Resource{}, err
	}
	status := StatusDraft
	if published {
		status = StatusPublished
	}
	return s.repo.SetStatus(ctx, id, status)
}

// SetArchived archives / restores a resource.
func (s *Service) SetArchived(ctx context.Context, ownerID, id uuid.UUID, archived bool) (Resource, error) {
	if _, err := s.owned(ctx, ownerID, id); err != nil {
		return Resource{}, err
	}
	return s.repo.SetArchived(ctx, id, archived)
}

// Delete removes a resource that has never been assigned. ErrInUse otherwise
// (archive it instead).
func (s *Service) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	if _, err := s.owned(ctx, ownerID, id); err != nil {
		return err
	}
	used, err := s.repo.IsAssigned(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return ErrInUse
	}
	return s.repo.Delete(ctx, id)
}

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

func (s *Service) owned(ctx context.Context, ownerID, id uuid.UUID) (Resource, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Resource{}, err
	}
	r, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Resource{}, err
	}
	if r.TeacherID != tid {
		return Resource{}, ErrForbidden
	}
	return r, nil
}

func (s *Service) log() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// booking resolves a booking's participants through BookingReader. A nil
// reader (or an unknown booking id) reports found=false, so every caller below
// fails closed to ErrBookingNotFound rather than panicking.
func (s *Service) booking(ctx context.Context, bookingID uuid.UUID) (teacherOwnerID, studentID uuid.UUID, found bool, err error) {
	if s.bookings == nil {
		return uuid.Nil, uuid.Nil, false, nil
	}
	return s.bookings.Booking(ctx, bookingID)
}

// enrollment resolves an enrollment's participants through EnrollmentReader,
// the course-context sibling of booking above. A nil reader (or an unknown
// enrollment id) reports found=false, so every caller fails closed to
// ErrEnrollmentNotFound rather than panicking.
func (s *Service) enrollment(ctx context.Context, enrollmentID uuid.UUID) (teacherOwnerID, studentID uuid.UUID, found bool, err error) {
	if s.enrollments == nil {
		return uuid.Nil, uuid.Nil, false, nil
	}
	return s.enrollments.Enrollment(ctx, enrollmentID)
}

// --- phase A2: attaching a resource to a booking ---

// Attach attaches one of the caller's own published resources to a booking as
// material or homework. Caller must be the booking's teacher-owner; the
// resource must belong to the same teacher, be published, and not archived.
// `homework` is rejected for material/article (no submission flow — attach it
// as material instead).
func (s *Service) Attach(ctx context.Context, callerID, bookingID, resourceID uuid.UUID, kind string, dueAt *time.Time) (BookingResource, error) {
	if !validKind(kind) {
		return BookingResource{}, invalid("`kind` must be `material` or `homework`.")
	}
	teacherOwnerID, _, found, err := s.booking(ctx, bookingID)
	if err != nil {
		return BookingResource{}, err
	}
	if !found {
		return BookingResource{}, ErrBookingNotFound
	}
	if teacherOwnerID == uuid.Nil || teacherOwnerID != callerID {
		return BookingResource{}, ErrForbidden
	}

	tid, err := s.teacher(ctx, callerID)
	if err != nil {
		return BookingResource{}, err
	}
	res, err := s.repo.ByID(ctx, resourceID)
	if err != nil {
		return BookingResource{}, err
	}
	if res.TeacherID != tid {
		return BookingResource{}, ErrForbidden
	}
	if res.Status != StatusPublished {
		return BookingResource{}, invalid("Only a published resource can be attached to a lesson.")
	}
	if res.ArchivedAt != nil {
		return BookingResource{}, invalid("An archived resource can't be attached to a lesson.")
	}
	if kind == KindHomework && !res.Type.submittable() {
		return BookingResource{}, invalid("A %s resource has no submission flow — attach it as material instead.", res.Type)
	}

	attached, err := s.repo.AttachResource(ctx, bookingID, resourceID, kind, callerID, dueAt)
	if err != nil {
		return BookingResource{}, err
	}
	return attached, nil
}

// Detach removes an attachment. Teacher-owner only. Existing submissions
// against the resource are left untouched — history survives detaching.
func (s *Service) Detach(ctx context.Context, callerID, bookingID, attachmentID uuid.UUID) error {
	teacherOwnerID, _, found, err := s.booking(ctx, bookingID)
	if err != nil {
		return err
	}
	if !found {
		return ErrBookingNotFound
	}
	if teacherOwnerID == uuid.Nil || teacherOwnerID != callerID {
		return ErrForbidden
	}
	return s.repo.DetachResource(ctx, bookingID, attachmentID)
}

// ForBooking returns a booking's attachments with full display content.
// Caller must be a participant (the student or the teacher-owner). The
// student sees every question's Correct field stripped and, per attachment,
// their own submission summary; the teacher sees the full content and no
// submission (the grading inbox is the teacher's view, built separately).
func (s *Service) ForBooking(ctx context.Context, callerID, bookingID uuid.UUID) ([]AttachedResource, error) {
	teacherOwnerID, studentID, found, err := s.booking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrBookingNotFound
	}
	isStudent := studentID != uuid.Nil && studentID == callerID
	isTeacher := teacherOwnerID != uuid.Nil && teacherOwnerID == callerID
	if !isStudent && !isTeacher {
		return nil, ErrForbidden
	}

	rows, err := s.repo.ListBookingResources(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	var byResource map[uuid.UUID]Submission
	if isStudent {
		subs, err := s.repo.ListSubmissionsForBooking(ctx, bookingID)
		if err != nil {
			return nil, err
		}
		byResource = make(map[uuid.UUID]Submission, len(subs))
		for _, sub := range subs {
			byResource[sub.ResourceID] = sub
		}
	}

	out := make([]AttachedResource, len(rows))
	for i, r := range rows {
		var sub *SubmissionSummary
		if isStudent {
			r.Content = r.Content.Public()
			if s, ok := byResource[r.ResourceID]; ok {
				r.SubmissionStatus = s.Status
				sub = &SubmissionSummary{
					ID: s.ID, Status: s.Status, AutoScore: s.AutoScore, AutoMax: s.AutoMax,
					TeacherScore: s.TeacherScore, SubmittedAt: s.SubmittedAt, GradedAt: s.GradedAt,
				}
			}
		}
		out[i] = AttachedResource{BookingResource: r, Submission: sub}
	}
	return out, nil
}

// SummaryForBooking returns each attachment's display fields plus the
// viewer's own submission status (when they are the booking's student),
// without full content. Backs bookings.ResourceReader, embedding the summary
// in a BookingDTO — the caller (bookings.Service) has already authorized
// viewerID as a participant, so no auth check happens here.
func (s *Service) SummaryForBooking(ctx context.Context, bookingID, viewerID uuid.UUID) ([]BookingResource, error) {
	rows, err := s.repo.ListBookingResources(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	subs, err := s.repo.ListSubmissionsForBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	byResource := make(map[uuid.UUID]Submission, len(subs))
	for _, sub := range subs {
		if sub.StudentID == viewerID {
			byResource[sub.ResourceID] = sub
		}
	}
	for i := range rows {
		if sub, ok := byResource[rows[i].ResourceID]; ok {
			rows[i].SubmissionStatus = sub.Status
		}
	}
	return rows, nil
}

// --- phase A3: submissions ---

// StartSubmission idempotently begins (or resumes) a student's homework: the
// resource must be attached to the booking as homework, the caller must be
// the booking's student, and the resource type must carry a submission flow.
func (s *Service) StartSubmission(ctx context.Context, studentID, resourceID, bookingID uuid.UUID) (Submission, error) {
	_, bookingStudentID, found, err := s.booking(ctx, bookingID)
	if err != nil {
		return Submission{}, err
	}
	if !found {
		return Submission{}, ErrBookingNotFound
	}
	if bookingStudentID == uuid.Nil || bookingStudentID != studentID {
		return Submission{}, ErrForbidden
	}

	res, err := s.repo.ByID(ctx, resourceID)
	if err != nil {
		return Submission{}, err
	}
	if !res.Type.submittable() {
		return Submission{}, invalid("A %s resource can't be submitted.", res.Type)
	}

	attachment, ok, err := s.repo.GetBookingResourceByPair(ctx, bookingID, resourceID)
	if err != nil {
		return Submission{}, err
	}
	if !ok || attachment.Kind != KindHomework {
		return Submission{}, ErrHomeworkNotAssigned
	}

	return s.repo.StartSubmission(ctx, resourceID, studentID, bookingID)
}

// StartCourseSubmission idempotently begins (or resumes) a student's work
// against a course-embedded resource (phase C2). Mirrors StartSubmission,
// but resolves participants via EnrollmentReader instead of BookingReader,
// and checks EnrollmentGrantsResource instead of GetBookingResourceByPair —
// there is no separate "attach" step for courses: the resource IS the
// curriculum item.
func (s *Service) StartCourseSubmission(ctx context.Context, studentID, resourceID, enrollmentID uuid.UUID) (Submission, error) {
	_, enrolledStudentID, found, err := s.enrollment(ctx, enrollmentID)
	if err != nil {
		return Submission{}, err
	}
	if !found {
		return Submission{}, ErrEnrollmentNotFound
	}
	if enrolledStudentID == uuid.Nil || enrolledStudentID != studentID {
		return Submission{}, ErrForbidden
	}

	res, err := s.repo.ByID(ctx, resourceID)
	if err != nil {
		return Submission{}, err
	}
	if !res.Type.submittable() {
		return Submission{}, invalid("A %s resource can't be submitted.", res.Type)
	}

	if s.enrollments == nil {
		return Submission{}, ErrEnrollmentNotFound
	}
	granted, err := s.enrollments.EnrollmentGrantsResource(ctx, enrollmentID, resourceID)
	if err != nil {
		return Submission{}, err
	}
	if !granted {
		return Submission{}, ErrResourceNotInCourse
	}

	return s.repo.StartCourseSubmission(ctx, resourceID, studentID, enrollmentID)
}

// SaveAnswers persists a draft of the student's answers. Owner-only, and only
// while the submission is still in_progress.
func (s *Service) SaveAnswers(ctx context.Context, callerID, submissionID uuid.UUID, answers map[string][]string) (Submission, error) {
	sub, err := s.repo.GetSubmission(ctx, submissionID)
	if err != nil {
		return Submission{}, err
	}
	if sub.StudentID != callerID {
		return Submission{}, ErrForbidden
	}
	if sub.Status != SubmissionInProgress {
		return Submission{}, ErrInvalidSubmissionState
	}
	return s.repo.SaveSubmissionAnswers(ctx, submissionID, answers)
}

// Submit finalizes a submission. Owner-only, only from in_progress.
// quiz/listening/reading auto-grade against the resource's questions and land
// directly on `graded` — nothing left for a teacher to do. `writing` lands on
// `submitted` and awaits a teacher's grade.
func (s *Service) Submit(ctx context.Context, callerID, submissionID uuid.UUID) (Submission, error) {
	sub, err := s.repo.GetSubmission(ctx, submissionID)
	if err != nil {
		return Submission{}, err
	}
	if sub.StudentID != callerID {
		return Submission{}, ErrForbidden
	}
	if sub.Status != SubmissionInProgress {
		return Submission{}, ErrInvalidSubmissionState
	}

	res, err := s.repo.ByID(ctx, sub.ResourceID)
	if err != nil {
		return Submission{}, err
	}

	now := s.now().UTC()
	var result Submission
	switch res.Type {
	case TypeWriting:
		result, err = s.repo.SubmitSubmission(ctx, submissionID, SubmitParams{Status: SubmissionSubmitted, SubmittedAt: now})
	case TypeQuiz, TypeListening, TypeReading:
		score, max := autoScore(res.Content, sub.Answers)
		result, err = s.repo.SubmitSubmission(ctx, submissionID, SubmitParams{
			Status: SubmissionGraded, AutoScore: &score, AutoMax: &max, SubmittedAt: now,
		})
	default:
		return Submission{}, invalid("A %s resource can't be submitted.", res.Type)
	}
	if err != nil {
		return Submission{}, err
	}
	// A course-context submission reaching submitted (writing, pending
	// grading) or graded (auto-graded) marks its curriculum item complete —
	// both outcomes above land here, so one call site after the switch
	// covers it. Best-effort: logged and swallowed, never fails Submit.
	if result.EnrollmentID != uuid.Nil {
		s.notifyCourseProgress(ctx, result)
	}
	return result, nil
}

// notifyCourseProgress is the guarded, best-effort CourseProgress call. Runs
// synchronously (unlike notifyGraded's detached goroutine) — it's a local DB
// upsert, not a network email send, so there's no slow I/O to shield the
// response from, and staying synchronous keeps it deterministic in tests.
func (s *Service) notifyCourseProgress(ctx context.Context, sub Submission) {
	if s.courseProgress == nil {
		return
	}
	if err := s.courseProgress.ItemCompleted(ctx, sub.EnrollmentID, sub.ResourceID); err != nil {
		s.log().Error("notify course progress",
			slog.String("submission_id", sub.ID.String()), slog.Any("error", err))
	}
}

// Grade records a teacher's grade on a `writing` submission: only from
// `submitted`, only by the booking's teacher-owner. score may be nil
// (feedback-only grading). Best-effort emails the student; a mail failure is
// logged and never fails the request.
func (s *Service) Grade(ctx context.Context, teacherCallerID, submissionID uuid.UUID, score *int, feedback string) (Submission, error) {
	sub, err := s.repo.GetSubmission(ctx, submissionID)
	if err != nil {
		return Submission{}, err
	}

	// Authorize before any state-revealing validation below — otherwise a
	// caller who isn't even a participant on the booking/enrollment could
	// distinguish "not writing type" (400) from "not submitted yet" (409) via
	// a valid but not-theirs submission id, learning submission state before
	// being told they're forbidden.
	var teacherOwnerID uuid.UUID
	var found bool
	switch {
	case sub.BookingID != uuid.Nil:
		teacherOwnerID, _, found, err = s.booking(ctx, sub.BookingID)
	case sub.EnrollmentID != uuid.Nil:
		teacherOwnerID, _, found, err = s.enrollment(ctx, sub.EnrollmentID)
	}
	if err != nil {
		return Submission{}, err
	}
	if !found || teacherOwnerID == uuid.Nil || teacherOwnerID != teacherCallerID {
		return Submission{}, ErrForbidden
	}

	res, err := s.repo.ByID(ctx, sub.ResourceID)
	if err != nil {
		return Submission{}, err
	}
	if res.Type != TypeWriting {
		return Submission{}, invalid("Only writing submissions are graded.")
	}
	if sub.Status != SubmissionSubmitted {
		return Submission{}, ErrInvalidSubmissionState
	}

	graded, err := s.repo.GradeSubmission(ctx, submissionID, score, strings.TrimSpace(feedback), teacherCallerID, s.now().UTC())
	if err != nil {
		return Submission{}, err
	}

	s.notifyGraded(ctx, graded, res.Title)
	return graded, nil
}

// notifyGraded is the guarded, best-effort Mailer call: a lookup or send
// failure is logged and swallowed, never fails the grading request.
func (s *Service) notifyGraded(ctx context.Context, sub Submission, resourceTitle string) {
	email, name, err := s.repo.UserContact(ctx, sub.StudentID)
	if err != nil {
		s.log().Error("load student contact for grading email",
			slog.String("submission_id", sub.ID.String()), slog.Any("error", err))
		return
	}
	// Detached so the send can complete after this request returns, without
	// risking a stuck goroutine outliving the process (mail sends have their
	// own client timeout).
	go func(ctx context.Context) {
		if err := s.mailer.SubmissionGraded(ctx, email, name, resourceTitle, sub.TeacherScore, nil, sub.TeacherFeedback); err != nil {
			s.log().Error("send submission graded email",
				slog.String("submission_id", sub.ID.String()), slog.Any("error", err))
		}
	}(context.WithoutCancel(ctx))
}

// GetSubmission returns one submission. The owning student or the
// booking's/enrollment's teacher-owner only.
func (s *Service) GetSubmission(ctx context.Context, callerID, id uuid.UUID) (Submission, error) {
	sub, err := s.repo.GetSubmission(ctx, id)
	if err != nil {
		return Submission{}, err
	}
	if sub.StudentID == callerID {
		return sub, nil
	}
	var teacherOwnerID uuid.UUID
	var found bool
	switch {
	case sub.BookingID != uuid.Nil:
		teacherOwnerID, _, found, err = s.booking(ctx, sub.BookingID)
	case sub.EnrollmentID != uuid.Nil:
		teacherOwnerID, _, found, err = s.enrollment(ctx, sub.EnrollmentID)
	}
	if err != nil {
		return Submission{}, err
	}
	if found && teacherOwnerID != uuid.Nil && teacherOwnerID == callerID {
		return sub, nil
	}
	return Submission{}, ErrForbidden
}

// Inbox returns a page of the caller's grading inbox: their own resources'
// submissions, filtered by status ("" defaults to `submitted`; "all" lifts
// the filter), newest-submitted-first.
func (s *Service) Inbox(ctx context.Context, teacherCallerID uuid.UUID, q SubmissionQuery) (SubmissionPage, error) {
	tid, err := s.teacher(ctx, teacherCallerID)
	if err != nil {
		return SubmissionPage{}, err
	}

	filter := q.Status
	switch {
	case filter == "":
		filter = SubmissionSubmitted
	case filter == "all":
		filter = ""
	case filter != SubmissionInProgress && filter != SubmissionSubmitted && filter != SubmissionGraded:
		return SubmissionPage{}, invalid("`status` must be one of in_progress, submitted, graded, all.")
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

	items, total, err := s.repo.InboxSubmissions(ctx, tid, filter, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return SubmissionPage{}, err
	}
	if items == nil {
		items = []Submission{}
	}
	return SubmissionPage{Submissions: items, Total: total}, nil
}

// CanAccessFile backs files.AssigneeChecker: true iff some resource carrying
// fileAssetID is attached to a booking whose student is requesterID, OR
// (phase C2) carried by a resource embedded in some course requesterID is
// enrolled in.
func (s *Service) CanAccessFile(ctx context.Context, fileAssetID, requesterID uuid.UUID) (bool, error) {
	ok, err := s.repo.FileAssetAccessible(ctx, fileAssetID, requesterID)
	if err != nil || ok {
		return ok, err
	}
	if s.enrollments == nil {
		return false, nil
	}
	resourceIDs, err := s.repo.ResourceIDsForFileAsset(ctx, fileAssetID)
	if err != nil {
		return false, err
	}
	for _, rid := range resourceIDs {
		granted, err := s.enrollments.StudentResourceAccess(ctx, rid, requesterID)
		if err != nil {
			return false, err
		}
		if granted {
			return true, nil
		}
	}
	return false, nil
}
