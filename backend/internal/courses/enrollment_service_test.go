package courses

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- fakeRepo: phase C2 additions ---

func (r *fakeRepo) TeacherOwnerID(_ context.Context, teacherID uuid.UUID) (uuid.UUID, bool, error) {
	for owner, tid := range r.teacherByOwner {
		if tid == teacherID {
			return owner, true, nil
		}
	}
	return uuid.Nil, false, nil
}

func (r *fakeRepo) TeacherSummaryByID(_ context.Context, teacherID uuid.UUID) (TeacherSummary, error) {
	return TeacherSummary{ID: teacherID, DisplayName: "Teacher", Slug: "teacher-" + teacherID.String()[:8]}, nil
}

func (r *fakeRepo) CatalogList(_ context.Context, q CatalogQuery) ([]CatalogEntry, int, error) {
	var out []CatalogEntry
	for _, c := range r.courses {
		if c.Status != StatusPublished || c.ArchivedAt != nil || c.SuspendedAt != nil {
			continue
		}
		out = append(out, CatalogEntry{Course: c, Teacher: TeacherSummary{ID: c.TeacherID}})
	}
	return out, len(out), nil
}

// --- phase C3: admin moderation ---

func (r *fakeRepo) AdminList(_ context.Context, q AdminCourseQuery, limit, offset int) ([]AdminCourse, int, error) {
	var out []AdminCourse
	for _, c := range r.courses {
		switch q.Status {
		case "":
		case "archived":
			if c.ArchivedAt == nil {
				continue
			}
		default:
			if c.ArchivedAt != nil || string(c.Status) != q.Status {
				continue
			}
		}
		switch q.Suspended {
		case "":
		case "true":
			if c.SuspendedAt == nil {
				continue
			}
		case "false":
			if c.SuspendedAt != nil {
				continue
			}
		}
		teacher := TeacherSummary{ID: c.TeacherID}
		if q.TeacherSlug != "" && teacher.Slug != q.TeacherSlug {
			continue
		}
		if q.Q != "" && !strings.Contains(strings.ToLower(c.Title), strings.ToLower(q.Q)) {
			continue
		}
		out = append(out, AdminCourse{Course: c, Teacher: teacher})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Course.CreatedAt.After(out[j].Course.CreatedAt) })
	total := len(out)
	start := offset
	if start > len(out) {
		start = len(out)
	}
	end := start + limit
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}

func (r *fakeRepo) SetSuspended(_ context.Context, id uuid.UUID, suspended bool) (AdminCourse, error) {
	c, ok := r.courses[id]
	if !ok {
		return AdminCourse{}, ErrNotFound
	}
	if suspended {
		if c.SuspendedAt == nil {
			now := time.Now()
			c.SuspendedAt = &now
		}
	} else {
		c.SuspendedAt = nil
	}
	c.UpdatedAt = time.Now()
	r.courses[id] = c
	return AdminCourse{Course: c, Teacher: TeacherSummary{ID: c.TeacherID}}, nil
}

func (r *fakeRepo) EnsureEnrollment(_ context.Context, courseID, studentID uuid.UUID, source EnrollmentSource, amountPaidMinor int64, currency string) (Enrollment, error) {
	for _, e := range r.enrollments {
		if e.CourseID == courseID && e.StudentID == studentID {
			return e, nil
		}
	}
	e := Enrollment{
		ID: uuid.New(), CourseID: courseID, StudentID: studentID, Source: source,
		AmountPaidMinor: amountPaidMinor, Currency: currency, CreatedAt: time.Now(),
	}
	r.enrollments[e.ID] = e
	return e, nil
}

func (r *fakeRepo) EnrollmentByCourseAndStudent(_ context.Context, courseID, studentID uuid.UUID) (Enrollment, bool, error) {
	for _, e := range r.enrollments {
		if e.CourseID == courseID && e.StudentID == studentID {
			return e, true, nil
		}
	}
	return Enrollment{}, false, nil
}

func (r *fakeRepo) EnrollmentParticipants(_ context.Context, enrollmentID uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	e, ok := r.enrollments[enrollmentID]
	if !ok {
		return uuid.Nil, uuid.Nil, false, nil
	}
	c, ok := r.courses[e.CourseID]
	if !ok {
		return uuid.Nil, uuid.Nil, false, nil
	}
	owner, ok, err := r.TeacherOwnerID(context.Background(), c.TeacherID)
	if err != nil || !ok {
		return uuid.Nil, uuid.Nil, false, err
	}
	return owner, e.StudentID, true, nil
}

func (r *fakeRepo) ListEnrollmentsForStudent(_ context.Context, studentID uuid.UUID) ([]EnrollmentSummary, error) {
	var out []EnrollmentSummary
	for _, e := range r.enrollments {
		if e.StudentID != studentID {
			continue
		}
		c := r.courses[e.CourseID]
		total, completed := r.itemCounts(e)
		out = append(out, EnrollmentSummary{Enrollment: e, Course: c, TotalItems: total, CompletedItems: completed})
	}
	return out, nil
}

func (r *fakeRepo) itemCounts(e Enrollment) (total, completed int) {
	for _, it := range r.items {
		sec, ok := r.sections[it.SectionID]
		if !ok || sec.CourseID != e.CourseID {
			continue
		}
		total++
	}
	for _, p := range r.progress {
		if p.EnrollmentID == e.ID && p.Status == ItemCompleted {
			completed++
		}
	}
	return
}

func (r *fakeRepo) EnrollmentGrantsResource(_ context.Context, enrollmentID, resourceID uuid.UUID) (bool, error) {
	e, ok := r.enrollments[enrollmentID]
	if !ok {
		return false, nil
	}
	for _, it := range r.items {
		sec, ok := r.sections[it.SectionID]
		if !ok || sec.CourseID != e.CourseID {
			continue
		}
		if it.ResourceID != nil && *it.ResourceID == resourceID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) StudentResourceAccess(_ context.Context, resourceID, studentID uuid.UUID) (bool, error) {
	for _, e := range r.enrollments {
		if e.StudentID != studentID {
			continue
		}
		if ok, _ := r.EnrollmentGrantsResource(context.Background(), e.ID, resourceID); ok {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) StudentHasVideoAccess(_ context.Context, fileAssetID, studentID uuid.UUID) (bool, error) {
	for _, e := range r.enrollments {
		if e.StudentID != studentID {
			continue
		}
		for _, it := range r.items {
			sec, ok := r.sections[it.SectionID]
			if !ok || sec.CourseID != e.CourseID {
				continue
			}
			if it.VideoAssetID != nil && *it.VideoAssetID == fileAssetID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *fakeRepo) ItemForEnrollmentResource(_ context.Context, enrollmentID, resourceID uuid.UUID) (Item, bool, error) {
	e, ok := r.enrollments[enrollmentID]
	if !ok {
		return Item{}, false, nil
	}
	for _, it := range r.items {
		sec, ok := r.sections[it.SectionID]
		if !ok || sec.CourseID != e.CourseID {
			continue
		}
		if it.ResourceID != nil && *it.ResourceID == resourceID {
			return it, true, nil
		}
	}
	return Item{}, false, nil
}

func (r *fakeRepo) UpsertItemProgress(_ context.Context, enrollmentID, itemID uuid.UUID, positionSeconds *int, completed *bool) (ItemProgress, error) {
	var existing *ItemProgress
	for id, p := range r.progress {
		if p.EnrollmentID == enrollmentID && p.ItemID == itemID {
			pp := r.progress[id]
			existing = &pp
			break
		}
	}
	p := ItemProgress{ID: uuid.New(), EnrollmentID: enrollmentID, ItemID: itemID, Status: ItemInProgress, UpdatedAt: time.Now()}
	if existing != nil {
		p = *existing
	}
	if positionSeconds != nil {
		p.VideoPositionSeconds = *positionSeconds
	}
	if completed != nil {
		if *completed {
			p.Status = ItemCompleted
			if p.CompletedAt == nil {
				now := time.Now()
				p.CompletedAt = &now
			}
		} else {
			p.Status = ItemInProgress
			p.CompletedAt = nil
		}
	}
	p.UpdatedAt = time.Now()
	r.progress[p.ID] = p
	return p, nil
}

func (r *fakeRepo) CompleteItemProgress(_ context.Context, enrollmentID, itemID uuid.UUID, completedAt time.Time) (ItemProgress, error) {
	for id, p := range r.progress {
		if p.EnrollmentID == enrollmentID && p.ItemID == itemID {
			p.Status = ItemCompleted
			if p.CompletedAt == nil {
				p.CompletedAt = &completedAt
			}
			p.UpdatedAt = time.Now()
			r.progress[id] = p
			return p, nil
		}
	}
	p := ItemProgress{ID: uuid.New(), EnrollmentID: enrollmentID, ItemID: itemID, Status: ItemCompleted, CompletedAt: &completedAt, UpdatedAt: time.Now()}
	r.progress[p.ID] = p
	return p, nil
}

func (r *fakeRepo) ListItemProgressForEnrollment(_ context.Context, enrollmentID uuid.UUID) ([]ItemProgress, error) {
	var out []ItemProgress
	for _, p := range r.progress {
		if p.EnrollmentID == enrollmentID {
			out = append(out, p)
		}
	}
	return out, nil
}

// --- fakePaymentGateway: an in-memory courses.PaymentGateway ---

type fakePaymentGateway struct {
	decline     bool // next call declines
	captureFail bool // next call fails capture (retryable)
	calls       int
	lastAmount  int64
	lastCurr    string

	// phase C3
	creditCalls  int
	creditErr    error
	lastCreditID uuid.UUID
	lastCredited int64
}

func (f *fakePaymentGateway) Purchase(_ context.Context, _, _ uuid.UUID, amountMinor int64, currency, _ string) (PurchaseSnapshot, error) {
	f.calls++
	f.lastAmount, f.lastCurr = amountMinor, currency
	if f.decline {
		f.decline = false
		return PurchaseSnapshot{}, PaymentFailedError{Reason: "card declined"}
	}
	if f.captureFail {
		f.captureFail = false
		return PurchaseSnapshot{}, ErrCaptureFailed
	}
	return PurchaseSnapshot{Status: "captured", AmountMinor: amountMinor, Currency: currency}, nil
}

func (f *fakePaymentGateway) CreditCourseSale(_ context.Context, enrollmentID uuid.UUID, priceAmountMinor int64, _ string) error {
	f.creditCalls++
	f.lastCreditID = enrollmentID
	f.lastCredited = priceAmountMinor
	return f.creditErr
}

var _ PaymentGateway = (*fakePaymentGateway)(nil)

// --- helpers ---

// publishWithOneVideoItem builds a minimal publishable course: one section,
// one video item pointing at a file the teacher owns.
func (e *testEnv) publishWithOneVideoItem(t *testing.T, ownerID uuid.UUID, priceMinor int64) (CourseDetail, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	d, err := e.svc.Create(ctx, ownerID, "Course", "", "", priceMinor, "UZS")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	d, err = e.svc.AddSection(ctx, ownerID, d.Course.ID, "Section 1")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	secID := d.Sections[0].Section.ID
	videoID := uuid.New()
	e.file.put(videoID, ownerID, "video/mp4")
	d, err = e.svc.AddItem(ctx, ownerID, d.Course.ID, secID, ItemKindVideo, "", &videoID, nil)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	d, err = e.svc.SetPublished(ctx, ownerID, d.Course.ID, true)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	return d, d.Sections[0].Items[0].ID
}

// --- Purchase ---

func TestService_Purchase_FreeCourse(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 0)

	student := uuid.New()
	summary, created, err := e.svc.Purchase(ctx, student, d.Course.ID, "")
	if err != nil {
		t.Fatalf("purchase free course: %v", err)
	}
	if created {
		t.Error("a free enrollment should report justPurchased=false (handler renders 200)")
	}
	if summary.Enrollment.Source != EnrollmentFree || summary.Enrollment.AmountPaidMinor != 0 {
		t.Errorf("unexpected enrollment: %+v", summary.Enrollment)
	}
}

func TestService_Purchase_Paid_HappyPath(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 50000)
	pay := &fakePaymentGateway{}
	e.svc.SetPaymentGateway(pay)

	student := uuid.New()
	summary, created, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_ok")
	if err != nil {
		t.Fatalf("purchase: %v", err)
	}
	if !created {
		t.Error("a brand-new paid purchase should report justPurchased=true (handler renders 201)")
	}
	if summary.Enrollment.Source != EnrollmentPurchase || summary.Enrollment.AmountPaidMinor != 50000 {
		t.Errorf("unexpected enrollment: %+v", summary.Enrollment)
	}
	if pay.calls != 1 {
		t.Errorf("expected exactly one gateway call, got %d", pay.calls)
	}
	if pay.creditCalls != 1 {
		t.Errorf("expected exactly one CreditCourseSale call once the enrollment exists, got %d", pay.creditCalls)
	}
	if pay.lastCreditID != summary.Enrollment.ID {
		t.Errorf("CreditCourseSale should be called with the new enrollment's id: got %v, want %v", pay.lastCreditID, summary.Enrollment.ID)
	}
	if pay.lastCredited != 50000 {
		t.Errorf("CreditCourseSale should be called with the full price paid (share math lives in payments): got %d, want 50000", pay.lastCredited)
	}
}

func TestService_Purchase_FreeCourse_NeverCreditsLedger(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 0)
	pay := &fakePaymentGateway{}
	e.svc.SetPaymentGateway(pay) // wired, but a free course never reaches it

	student := uuid.New()
	if _, _, err := e.svc.Purchase(ctx, student, d.Course.ID, ""); err != nil {
		t.Fatalf("purchase free course: %v", err)
	}
	if pay.calls != 0 || pay.creditCalls != 0 {
		t.Errorf("a free course must never call the payment gateway or credit a ledger: calls=%d creditCalls=%d", pay.calls, pay.creditCalls)
	}
}

func TestService_Purchase_Declined(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 50000)
	pay := &fakePaymentGateway{decline: true}
	e.svc.SetPaymentGateway(pay)

	student := uuid.New()
	_, _, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_decline")
	var pf PaymentFailedError
	if !errors.As(err, &pf) {
		t.Fatalf("want PaymentFailedError, got %v (%T)", err, err)
	}
	if _, ok, _ := e.repo.EnrollmentByCourseAndStudent(ctx, d.Course.ID, student); ok {
		t.Error("a declined purchase should not create an enrollment")
	}
}

func TestService_Purchase_CaptureFailThenRetry(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 50000)
	pay := &fakePaymentGateway{captureFail: true}
	e.svc.SetPaymentGateway(pay)

	student := uuid.New()
	if _, _, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_capture_fail"); !errors.Is(err, ErrCaptureFailed) {
		t.Fatalf("want ErrCaptureFailed, got %v", err)
	}
	if _, ok, _ := e.repo.EnrollmentByCourseAndStudent(ctx, d.Course.ID, student); ok {
		t.Error("a capture failure should not create an enrollment")
	}

	summary, created, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_capture_fail")
	if err != nil {
		t.Fatalf("retry purchase: %v", err)
	}
	if !created || summary.Enrollment.Source != EnrollmentPurchase {
		t.Errorf("retry should succeed and create the enrollment: %+v created=%v", summary.Enrollment, created)
	}
}

func TestService_Purchase_AlreadyEnrolled_Idempotent(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 50000)
	pay := &fakePaymentGateway{}
	e.svc.SetPaymentGateway(pay)

	student := uuid.New()
	first, created1, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_ok")
	if err != nil || !created1 {
		t.Fatalf("first purchase: %+v err=%v", first, err)
	}
	second, created2, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_ok")
	if err != nil {
		t.Fatalf("second purchase: %v", err)
	}
	if created2 {
		t.Error("an already-enrolled purchase should report justPurchased=false")
	}
	if second.Enrollment.ID != first.Enrollment.ID {
		t.Error("second purchase should return the same enrollment, not a new one")
	}
	if pay.calls != 1 {
		t.Errorf("the gateway should not be charged again once enrolled, got %d calls", pay.calls)
	}
	if pay.creditCalls != 2 {
		t.Errorf("retrying an already-enrolled purchase should retry the ledger credit too (idempotent on the payments side), got %d calls", pay.creditCalls)
	}
}

// A purchase that captures payment, creates the enrollment, but fails to
// credit the teacher's payout ledger (e.g. a transient DB error) must not
// lose that revenue permanently: retrying the same purchase call — the
// client's only recourse, since the charge already succeeded — has to retry
// the credit too, not just short-circuit on "already enrolled".
func TestService_Purchase_RetryHealsAFailedLedgerCredit(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 50000)
	pay := &fakePaymentGateway{creditErr: errors.New("transient ledger write failure")}
	e.svc.SetPaymentGateway(pay)

	student := uuid.New()
	first, created1, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_ok")
	if err != nil || !created1 {
		t.Fatalf("first purchase: %+v err=%v", first, err)
	}
	if pay.creditCalls != 1 {
		t.Fatalf("expected the first (failing) credit attempt, got %d calls", pay.creditCalls)
	}

	pay.creditErr = nil // the transient failure clears before the retry
	second, created2, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_ok")
	if err != nil {
		t.Fatalf("retry purchase: %v", err)
	}
	if created2 {
		t.Error("the retry should still report justPurchased=false — it's the same enrollment")
	}
	if pay.calls != 1 {
		t.Errorf("the retry must not charge the student again, got %d gateway calls", pay.calls)
	}
	if pay.creditCalls != 2 {
		t.Errorf("the retry should re-attempt the ledger credit, got %d calls", pay.creditCalls)
	}
	if second.Enrollment.ID != first.Enrollment.ID {
		t.Error("retry should return the same enrollment")
	}
}

func TestService_Purchase_CannotBuyOwnCourse(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 0)

	if _, _, err := e.svc.Purchase(ctx, owner, d.Course.ID, ""); !errors.Is(err, ErrCannotBuyOwnCourse) {
		t.Errorf("want ErrCannotBuyOwnCourse, got %v", err)
	}
}

func TestService_Purchase_DraftCourseNotFound(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d := mustCreate(t, e, owner, "Draft course") // never published

	if _, _, err := e.svc.Purchase(ctx, uuid.New(), d.Course.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for a draft course, got %v", err)
	}
}

// --- RecordProgress ---

func TestService_RecordProgress_Upsert(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, itemID := e.publishWithOneVideoItem(t, owner, 0)
	student := uuid.New()
	if _, _, err := e.svc.Purchase(ctx, student, d.Course.ID, ""); err != nil {
		t.Fatalf("purchase: %v", err)
	}

	pos := 42
	p, err := e.svc.RecordProgress(ctx, student, d.Course.ID, itemID, &pos, nil)
	if err != nil {
		t.Fatalf("record progress: %v", err)
	}
	if p.VideoPositionSeconds != 42 || p.Status != ItemInProgress {
		t.Errorf("unexpected progress: %+v", p)
	}

	done := true
	p, err = e.svc.RecordProgress(ctx, student, d.Course.ID, itemID, nil, &done)
	if err != nil {
		t.Fatalf("mark complete: %v", err)
	}
	if p.Status != ItemCompleted || p.CompletedAt == nil {
		t.Errorf("expected completed: %+v", p)
	}
	if p.VideoPositionSeconds != 42 {
		t.Errorf("marking complete without a position shouldn't reset it: %+v", p)
	}
}

func TestService_RecordProgress_RejectsNonEnrolled(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, itemID := e.publishWithOneVideoItem(t, owner, 0)

	if _, err := e.svc.RecordProgress(ctx, uuid.New(), d.Course.ID, itemID, nil, nil); !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for a non-enrolled caller, got %v", err)
	}
}

func TestService_RecordProgress_RejectsResourceItem(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, teacherID := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Section 1")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	secID := d.Sections[0].Section.ID
	resID := uuid.New()
	e.res.allow(resID, teacherID)
	d, err = e.svc.AddItem(ctx, owner, d.Course.ID, secID, ItemKindResource, "", nil, &resID)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	itemID := d.Sections[0].Items[0].ID
	d, err = e.svc.SetPublished(ctx, owner, d.Course.ID, true)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	student := uuid.New()
	if _, _, err := e.svc.Purchase(ctx, student, d.Course.ID, ""); err != nil {
		t.Fatalf("purchase: %v", err)
	}

	if _, err := e.svc.RecordProgress(ctx, student, d.Course.ID, itemID, nil, nil); err == nil {
		t.Error("a resource item should reject RecordProgress")
	} else {
		asValidationError(t, err)
	}
}

// --- Learn ---

func TestService_Learn_EnrolledOrOwner(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, itemID := e.publishWithOneVideoItem(t, owner, 0)

	// owner preview: no enrollment needed.
	learn, err := e.svc.Learn(ctx, owner, d.Course.ID)
	if err != nil {
		t.Fatalf("owner learn: %v", err)
	}
	if len(learn.Sections) != 1 || len(learn.Sections[0].Items) != 1 {
		t.Fatalf("unexpected learn tree: %+v", learn)
	}
	if learn.Sections[0].Items[0].Progress.Status != ItemInProgress {
		t.Errorf("owner preview should show not-started progress: %+v", learn.Sections[0].Items[0].Progress)
	}
	if learn.EnrollmentID != nil {
		t.Errorf("owner preview has no enrollment, want nil EnrollmentID, got %v", *learn.EnrollmentID)
	}

	// a random caller: forbidden.
	if _, err := e.svc.Learn(ctx, uuid.New(), d.Course.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for a non-enrolled non-owner, got %v", err)
	}

	// an enrolled student: allowed, with their own progress.
	student := uuid.New()
	if _, _, err := e.svc.Purchase(ctx, student, d.Course.ID, ""); err != nil {
		t.Fatalf("purchase: %v", err)
	}
	pos := 10
	if _, err := e.svc.RecordProgress(ctx, student, d.Course.ID, itemID, &pos, nil); err != nil {
		t.Fatalf("record progress: %v", err)
	}
	learn, err = e.svc.Learn(ctx, student, d.Course.ID)
	if err != nil {
		t.Fatalf("student learn: %v", err)
	}
	if learn.Sections[0].Items[0].Progress.VideoPositionSeconds != 10 {
		t.Errorf("student's own progress should show up: %+v", learn.Sections[0].Items[0].Progress)
	}
	if learn.EnrollmentID == nil {
		t.Fatal("an enrolled student's Learn() should carry their EnrollmentID")
	}
	enrollment, ok, err := e.repo.EnrollmentByCourseAndStudent(ctx, d.Course.ID, student)
	if err != nil || !ok {
		t.Fatalf("load enrollment: ok=%v err=%v", ok, err)
	}
	if *learn.EnrollmentID != enrollment.ID {
		t.Errorf("EnrollmentID = %v, want %v", *learn.EnrollmentID, enrollment.ID)
	}
}

// --- ItemCompleted (resources.CourseProgress) ---

func TestService_ItemCompleted(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, teacherID := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Section 1")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	secID := d.Sections[0].Section.ID
	resID := uuid.New()
	e.res.allow(resID, teacherID)
	d, err = e.svc.AddItem(ctx, owner, d.Course.ID, secID, ItemKindResource, "", nil, &resID)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	itemID := d.Sections[0].Items[0].ID
	if _, err := e.svc.SetPublished(ctx, owner, d.Course.ID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}
	student := uuid.New()
	summary, _, err := e.svc.Purchase(ctx, student, d.Course.ID, "")
	if err != nil {
		t.Fatalf("purchase: %v", err)
	}

	if err := e.svc.ItemCompleted(ctx, summary.Enrollment.ID, resID); err != nil {
		t.Fatalf("item completed: %v", err)
	}
	rows, err := e.repo.ListItemProgressForEnrollment(ctx, summary.Enrollment.ID)
	if err != nil || len(rows) != 1 || rows[0].ItemID != itemID || rows[0].Status != ItemCompleted {
		t.Errorf("expected the resource item marked complete: rows=%+v err=%v", rows, err)
	}

	// An unrelated resource id should be a silent no-op, not an error.
	if err := e.svc.ItemCompleted(ctx, summary.Enrollment.ID, uuid.New()); err != nil {
		t.Errorf("unrelated resource id should no-op: %v", err)
	}
}

// --- catalog / cover ---

func TestService_CoverImage_404WhenDraft(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course") // draft, never published

	if _, _, _, err := e.svc.CoverImage(ctx, d.Course.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for a draft course's cover, got %v", err)
	}
}

func TestService_CoverImage_404WhenPublishedWithNoCover(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 0) // published, but no cover_asset_id set

	if _, _, _, err := e.svc.CoverImage(ctx, d.Course.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for a published course with no cover, got %v", err)
	}
}

func TestService_CoverImage_Published(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 0)

	coverID := uuid.New()
	e.file.put(coverID, owner, "image/png")
	if _, err := e.svc.Update(ctx, owner, d.Course.ID, d.Course.Title, d.Course.Subtitle, d.Course.Description,
		&coverID, d.Course.PriceAmountMinor, d.Course.PriceCurrency); err != nil {
		t.Fatalf("set cover: %v", err)
	}

	redirectURL, body, contentType, err := e.svc.CoverImage(ctx, d.Course.ID)
	if err != nil {
		t.Fatalf("cover image: %v", err)
	}
	if redirectURL != "" {
		t.Errorf("fake file reader never redirects, got %q", redirectURL)
	}
	if contentType != "image/png" {
		t.Errorf("content type: got %q", contentType)
	}
	if body == nil {
		t.Fatal("expected a non-nil body")
	}
	body.Close()
}

func TestService_Catalog_PublishedOnly(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	_, _ = e.publishWithOneVideoItem(t, owner, 1000) // published
	mustCreate(t, e, owner, "Still a draft")         // draft

	page, err := e.svc.Catalog(ctx, CatalogQuery{})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected only the published course to be listed, got total=%d entries=%+v", page.Total, page.Entries)
	}
}

// --- phase C3: suspension's effect on the storefront ---

func TestService_Catalog_ExcludesSuspended(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 1000)

	if _, err := e.svc.AdminSuspend(ctx, d.Course.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	page, err := e.svc.Catalog(ctx, CatalogQuery{})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if page.Total != 0 {
		t.Errorf("a suspended course must not appear in the catalog: total=%d entries=%+v", page.Total, page.Entries)
	}
}

func TestService_CatalogDetail_404WhenSuspended(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 1000)

	if _, err := e.svc.AdminSuspend(ctx, d.Course.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if _, err := e.svc.CatalogDetail(ctx, uuid.Nil, d.Course.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for a suspended course's catalog detail, got %v", err)
	}
}

func TestService_CoverImage_404WhenSuspended(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 0)
	coverID := uuid.New()
	e.file.put(coverID, owner, "image/png")
	if _, err := e.svc.Update(ctx, owner, d.Course.ID, d.Course.Title, d.Course.Subtitle, d.Course.Description,
		&coverID, d.Course.PriceAmountMinor, d.Course.PriceCurrency); err != nil {
		t.Fatalf("set cover: %v", err)
	}

	if _, err := e.svc.AdminSuspend(ctx, d.Course.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if _, _, _, err := e.svc.CoverImage(ctx, d.Course.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for a suspended course's cover, got %v", err)
	}
}

func TestService_Purchase_RejectsNewPurchaseOnSuspendedCourse(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 50000)
	pay := &fakePaymentGateway{}
	e.svc.SetPaymentGateway(pay)

	if _, err := e.svc.AdminSuspend(ctx, d.Course.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	student := uuid.New()
	if _, _, err := e.svc.Purchase(ctx, student, d.Course.ID, "pm_ok"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for a new purchase on a suspended course, got %v", err)
	}
	if pay.calls != 0 {
		t.Errorf("the gateway must not be charged for a rejected purchase, got %d calls", pay.calls)
	}
}

// TestService_Learn_StillWorksAfterSuspension is the flip side of the above:
// suspension is a storefront takedown, not a revocation — a student enrolled
// before the suspension keeps their access via GET /v1/courses/{id}/learn.
func TestService_Learn_StillWorksAfterSuspension(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 0)

	student := uuid.New()
	if _, _, err := e.svc.Purchase(ctx, student, d.Course.ID, ""); err != nil {
		t.Fatalf("purchase: %v", err)
	}

	if _, err := e.svc.AdminSuspend(ctx, d.Course.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	learn, err := e.svc.Learn(ctx, student, d.Course.ID)
	if err != nil {
		t.Fatalf("an already-enrolled student's Learn() must keep working after suspension: %v", err)
	}
	if learn.EnrollmentID == nil {
		t.Error("expected the student's enrollment id to still be present")
	}
}
