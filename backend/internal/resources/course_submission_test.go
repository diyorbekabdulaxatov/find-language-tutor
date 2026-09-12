package resources

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- fakeRepo: phase C2 additions ---

func (r *fakeRepo) StartCourseSubmission(_ context.Context, resourceID, studentID, enrollmentID uuid.UUID) (Submission, error) {
	key := resourceID.String() + "|" + studentID.String() + "|" + enrollmentID.String()
	if id, ok := r.courseSubmissionKeys[key]; ok {
		return r.submissions[id], nil
	}
	now := time.Now()
	sub := Submission{
		ID: uuid.New(), ResourceID: resourceID, StudentID: studentID, Context: "course", EnrollmentID: enrollmentID,
		Status: SubmissionInProgress, Answers: map[string][]string{}, CreatedAt: now, UpdatedAt: now,
	}
	r.submissions[sub.ID] = sub
	r.courseSubmissionKeys[key] = sub.ID
	return sub, nil
}

func (r *fakeRepo) ResourceIDsForFileAsset(_ context.Context, fileAssetID uuid.UUID) ([]uuid.UUID, error) {
	return r.resourceFileIndex[fileAssetID], nil
}

// --- fakeEnrollmentReader: an in-memory resources.EnrollmentReader ---

type fakeEnrollmentReader struct {
	byEnrollment  map[uuid.UUID][2]uuid.UUID // enrollmentID -> [teacherOwnerID, studentID]
	grants        map[string]bool            // "enrollmentID|resourceID" -> granted
	studentAccess map[string]bool            // "resourceID|studentID" -> granted
	err           error
}

func newFakeEnrollmentReader() *fakeEnrollmentReader {
	return &fakeEnrollmentReader{
		byEnrollment:  map[uuid.UUID][2]uuid.UUID{},
		grants:        map[string]bool{},
		studentAccess: map[string]bool{},
	}
}

func (f *fakeEnrollmentReader) Enrollment(_ context.Context, enrollmentID uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	if f.err != nil {
		return uuid.Nil, uuid.Nil, false, f.err
	}
	b, ok := f.byEnrollment[enrollmentID]
	if !ok {
		return uuid.Nil, uuid.Nil, false, nil
	}
	return b[0], b[1], true, nil
}

func (f *fakeEnrollmentReader) EnrollmentGrantsResource(_ context.Context, enrollmentID, resourceID uuid.UUID) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.grants[pairKey(enrollmentID, resourceID)], nil
}

func (f *fakeEnrollmentReader) StudentResourceAccess(_ context.Context, resourceID, studentID uuid.UUID) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.studentAccess[pairKey(resourceID, studentID)], nil
}

var _ EnrollmentReader = (*fakeEnrollmentReader)(nil)

// --- fakeCourseProgress: an in-memory resources.CourseProgress ---

type courseProgressCall struct{ enrollmentID, resourceID uuid.UUID }

type fakeCourseProgress struct {
	calls []courseProgressCall
	err   error
}

func (f *fakeCourseProgress) ItemCompleted(_ context.Context, enrollmentID, resourceID uuid.UUID) error {
	f.calls = append(f.calls, courseProgressCall{enrollmentID, resourceID})
	return f.err
}

var _ CourseProgress = (*fakeCourseProgress)(nil)

// newSvcWithEnrollment builds a Service wired with a fake Repository and a
// fake EnrollmentReader, plus a teacher profile and one enrollment
// (teacherOwner, student) ready to submit course-embedded resources against.
func newSvcWithEnrollment(t *testing.T) (svc *Service, repo *fakeRepo, er *fakeEnrollmentReader, teacherOwner, student, enrollmentID uuid.UUID) {
	t.Helper()
	repo = newFakeRepo()
	teacherOwner, teacherID := uuid.New(), uuid.New()
	repo.teacherByOwner[teacherOwner] = teacherID

	student = uuid.New()
	enrollmentID = uuid.New()

	er = newFakeEnrollmentReader()
	er.byEnrollment[enrollmentID] = [2]uuid.UUID{teacherOwner, student}

	svc = NewService(repo, discardLogger())
	svc.SetEnrollmentReader(er)
	return svc, repo, er, teacherOwner, student, enrollmentID
}

func mustQuizResource(t *testing.T, svc *Service, teacherOwner uuid.UUID) Resource {
	t.Helper()
	content := Content{Questions: []Question{
		{ID: "q1", Prompt: "2+2?", Kind: "single",
			Choices: []Choice{{ID: "c1", Text: "3"}, {ID: "c2", Text: "4"}}, Correct: []string{"c2"}},
	}}
	return mustPublishedResource(t, svc, teacherOwner, TypeQuiz, content)
}

// --- StartCourseSubmission ---

func TestStartCourseSubmission_HappyPath(t *testing.T) {
	svc, _, er, teacherOwner, student, enrollmentID := newSvcWithEnrollment(t)
	res := mustQuizResource(t, svc, teacherOwner)
	er.grants[pairKey(enrollmentID, res.ID)] = true

	sub, err := svc.StartCourseSubmission(context.Background(), student, res.ID, enrollmentID)
	if err != nil {
		t.Fatalf("start course submission: %v", err)
	}
	if sub.Context != "course" || sub.EnrollmentID != enrollmentID || sub.Status != SubmissionInProgress {
		t.Errorf("unexpected submission: %+v", sub)
	}

	// Idempotent: a second call returns the same row.
	sub2, err := svc.StartCourseSubmission(context.Background(), student, res.ID, enrollmentID)
	if err != nil || sub2.ID != sub.ID {
		t.Errorf("second start should return the same submission: %+v (err=%v)", sub2, err)
	}
}

func TestStartCourseSubmission_ResourceNotInCourse(t *testing.T) {
	svc, _, _, teacherOwner, student, enrollmentID := newSvcWithEnrollment(t)
	res := mustQuizResource(t, svc, teacherOwner)
	// er.grants left empty: the resource is not actually part of the course.

	_, err := svc.StartCourseSubmission(context.Background(), student, res.ID, enrollmentID)
	if !errors.Is(err, ErrResourceNotInCourse) {
		t.Fatalf("got %v, want ErrResourceNotInCourse", err)
	}
}

func TestStartCourseSubmission_WrongStudentForbidden(t *testing.T) {
	svc, _, er, teacherOwner, _, enrollmentID := newSvcWithEnrollment(t)
	res := mustQuizResource(t, svc, teacherOwner)
	er.grants[pairKey(enrollmentID, res.ID)] = true

	_, err := svc.StartCourseSubmission(context.Background(), uuid.New() /* not the enrolled student */, res.ID, enrollmentID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestStartCourseSubmission_UnknownEnrollment(t *testing.T) {
	svc, _, _, teacherOwner, student, _ := newSvcWithEnrollment(t)
	res := mustQuizResource(t, svc, teacherOwner)

	_, err := svc.StartCourseSubmission(context.Background(), student, res.ID, uuid.New())
	if !errors.Is(err, ErrEnrollmentNotFound) {
		t.Fatalf("got %v, want ErrEnrollmentNotFound", err)
	}
}

// --- Submit notifies CourseProgress ---

func TestSubmit_NotifiesCourseProgress(t *testing.T) {
	svc, _, er, teacherOwner, student, enrollmentID := newSvcWithEnrollment(t)
	progress := &fakeCourseProgress{}
	svc.SetCourseProgress(progress)
	res := mustQuizResource(t, svc, teacherOwner)
	er.grants[pairKey(enrollmentID, res.ID)] = true

	sub, err := svc.StartCourseSubmission(context.Background(), student, res.ID, enrollmentID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := svc.Submit(context.Background(), student, sub.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if len(progress.calls) != 1 || progress.calls[0].enrollmentID != enrollmentID || progress.calls[0].resourceID != res.ID {
		t.Errorf("expected one ItemCompleted call for (%s, %s), got %+v", enrollmentID, res.ID, progress.calls)
	}
}

func TestSubmit_LessonContextDoesNotNotifyCourseProgress(t *testing.T) {
	svc, repo, br, teacherOwner, student, bookingID := newSvcWithBooking(t)
	progress := &fakeCourseProgress{}
	svc.SetCourseProgress(progress)
	_ = br
	res := mustQuizResource(t, svc, teacherOwner)
	if _, err := repo.AttachResource(context.Background(), bookingID, res.ID, KindHomework, teacherOwner, nil); err != nil {
		t.Fatalf("attach: %v", err)
	}
	sub, err := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := svc.Submit(context.Background(), student, sub.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if len(progress.calls) != 0 {
		t.Errorf("a lesson-context submission should never notify CourseProgress: %+v", progress.calls)
	}
}

// --- Grade via enrollment ---

func TestGrade_ViaEnrollment(t *testing.T) {
	svc, _, er, teacherOwner, student, enrollmentID := newSvcWithEnrollment(t)
	res := mustPublishedResource(t, svc, teacherOwner, TypeWriting, Content{Prompt: "Write an essay."})
	er.grants[pairKey(enrollmentID, res.ID)] = true

	sub, err := svc.StartCourseSubmission(context.Background(), student, res.ID, enrollmentID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{"essay": {"hello"}}); err != nil {
		t.Fatalf("save answers: %v", err)
	}
	if _, err := svc.Submit(context.Background(), student, sub.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}

	score := 90
	if _, err := svc.Grade(context.Background(), uuid.New() /* not the teacher-owner */, sub.ID, &score, "good"); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-owner grade: got %v, want ErrForbidden", err)
	}

	graded, err := svc.Grade(context.Background(), teacherOwner, sub.ID, &score, "good")
	if err != nil {
		t.Fatalf("teacher-owner grade: %v", err)
	}
	if graded.Status != SubmissionGraded || graded.TeacherScore == nil || *graded.TeacherScore != 90 {
		t.Errorf("unexpected graded submission: %+v", graded)
	}
}

// --- FileAssetAccessible course widening ---

func TestFileAssetAccessible_CourseWidening(t *testing.T) {
	svc, repo, er, teacherOwner, student, _ := newSvcWithEnrollment(t)
	fileID := uuid.New()
	res := mustPublishedResource(t, svc, teacherOwner, TypeMaterial, Content{FileAssetID: &fileID})
	repo.resourceFileIndex[fileID] = []uuid.UUID{res.ID}

	// Not accessible yet: no booking attachment, no course enrollment access.
	ok, err := svc.CanAccessFile(context.Background(), fileID, student)
	if err != nil || ok {
		t.Fatalf("expected not accessible yet, got ok=%v err=%v", ok, err)
	}

	er.studentAccess[pairKey(res.ID, student)] = true
	ok, err = svc.CanAccessFile(context.Background(), fileID, student)
	if err != nil || !ok {
		t.Fatalf("expected course-widened access, got ok=%v err=%v", ok, err)
	}
}

func TestFileAssetAccessible_NilEnrollmentReaderFailsClosed(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, discardLogger())
	fileID := uuid.New()
	ok, err := svc.CanAccessFile(context.Background(), fileID, uuid.New())
	if err != nil || ok {
		t.Fatalf("expected no access with a nil EnrollmentReader, got ok=%v err=%v", ok, err)
	}
}
