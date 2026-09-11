package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// newSvcWithBooking builds a Service wired with a fake Repository and a fake
// BookingReader, plus a booking (teacherOwner, student) and a teacher profile
// (owner -> teacher id) ready to attach resources to.
func newSvcWithBooking(t *testing.T) (svc *Service, repo *fakeRepo, br *fakeBookingReader, teacherOwner, student, bookingID uuid.UUID) {
	t.Helper()
	repo = newFakeRepo()
	teacherOwner, teacherID := uuid.New(), uuid.New()
	repo.teacherByOwner[teacherOwner] = teacherID

	student = uuid.New()
	bookingID = uuid.New()

	br = newFakeBookingReader()
	br.byBooking[bookingID] = [2]uuid.UUID{teacherOwner, student}

	svc = NewService(repo, discardLogger())
	svc.SetBookingReader(br)
	return svc, repo, br, teacherOwner, student, bookingID
}

func mustPublishedResource(t *testing.T, svc *Service, teacherOwner uuid.UUID, typ Type, content Content) Resource {
	t.Helper()
	r, err := svc.Create(context.Background(), teacherOwner, typ, "Title", "", content, true)
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}
	return r
}

// --- Attach ---

func TestAttach_WrongTeacherOwnerForbidden(t *testing.T) {
	svc, _, _, teacherOwner, _, bookingID := newSvcWithBooking(t)
	res := mustPublishedResource(t, svc, teacherOwner, TypeArticle, Content{Body: "hi"})

	_, err := svc.Attach(context.Background(), uuid.New() /* not the teacher-owner */, bookingID, res.ID, KindMaterial, nil)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestAttach_BookingNotFound(t *testing.T) {
	svc, _, _, teacherOwner, _, _ := newSvcWithBooking(t)
	res := mustPublishedResource(t, svc, teacherOwner, TypeArticle, Content{Body: "hi"})

	_, err := svc.Attach(context.Background(), teacherOwner, uuid.New() /* unknown booking */, res.ID, KindMaterial, nil)
	if !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("got %v, want ErrBookingNotFound", err)
	}
}

func TestAttach_ResourceFromAnotherTeacherForbidden(t *testing.T) {
	svc, repo, _, teacherOwner, _, bookingID := newSvcWithBooking(t)

	otherOwner, otherTeacher := uuid.New(), uuid.New()
	repo.teacherByOwner[otherOwner] = otherTeacher
	other := mustPublishedResource(t, svc, otherOwner, TypeArticle, Content{Body: "hi"})

	_, err := svc.Attach(context.Background(), teacherOwner, bookingID, other.ID, KindMaterial, nil)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestAttach_UnpublishedRejected(t *testing.T) {
	svc, _, _, teacherOwner, _, bookingID := newSvcWithBooking(t)
	draft, err := svc.Create(context.Background(), teacherOwner, TypeArticle, "Draft", "", Content{Body: "hi"}, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = svc.Attach(context.Background(), teacherOwner, bookingID, draft.ID, KindMaterial, nil)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %v, want ValidationError (unpublished)", err)
	}
}

func TestAttach_ArchivedRejected(t *testing.T) {
	svc, _, _, teacherOwner, _, bookingID := newSvcWithBooking(t)
	res := mustPublishedResource(t, svc, teacherOwner, TypeArticle, Content{Body: "hi"})
	if _, err := svc.SetArchived(context.Background(), teacherOwner, res.ID, true); err != nil {
		t.Fatalf("archive: %v", err)
	}

	_, err := svc.Attach(context.Background(), teacherOwner, bookingID, res.ID, KindMaterial, nil)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %v, want ValidationError (archived)", err)
	}
}

func TestAttach_HomeworkOnNonSubmittableTypeRejected(t *testing.T) {
	svc, _, _, teacherOwner, _, bookingID := newSvcWithBooking(t)

	material := mustPublishedResource(t, svc, teacherOwner, TypeMaterial, Content{URL: "https://example.com/x.pdf"})
	if _, err := svc.Attach(context.Background(), teacherOwner, bookingID, material.ID, KindHomework, nil); !isValidation(err) {
		t.Errorf("material as homework: got %v, want ValidationError", err)
	}

	article := mustPublishedResource(t, svc, teacherOwner, TypeArticle, Content{Body: "hi"})
	if _, err := svc.Attach(context.Background(), teacherOwner, bookingID, article.ID, KindHomework, nil); !isValidation(err) {
		t.Errorf("article as homework: got %v, want ValidationError", err)
	}

	// Material as material (not homework) succeeds.
	if _, err := svc.Attach(context.Background(), teacherOwner, bookingID, material.ID, KindMaterial, nil); err != nil {
		t.Errorf("material as material should succeed: %v", err)
	}
}

func TestAttach_DuplicateIsConflict(t *testing.T) {
	svc, _, _, teacherOwner, _, bookingID := newSvcWithBooking(t)
	res := mustPublishedResource(t, svc, teacherOwner, TypeArticle, Content{Body: "hi"})

	if _, err := svc.Attach(context.Background(), teacherOwner, bookingID, res.ID, KindMaterial, nil); err != nil {
		t.Fatalf("first attach: %v", err)
	}
	_, err := svc.Attach(context.Background(), teacherOwner, bookingID, res.ID, KindMaterial, nil)
	if !errors.Is(err, ErrAlreadyAttached) {
		t.Fatalf("got %v, want ErrAlreadyAttached", err)
	}
}

func isValidation(err error) bool {
	var ve ValidationError
	return errors.As(err, &ve)
}

// --- Detach ---

func TestDetach_TeacherOwnerOnly(t *testing.T) {
	svc, _, _, teacherOwner, _, bookingID := newSvcWithBooking(t)
	res := mustPublishedResource(t, svc, teacherOwner, TypeArticle, Content{Body: "hi"})
	att, err := svc.Attach(context.Background(), teacherOwner, bookingID, res.ID, KindMaterial, nil)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if err := svc.Detach(context.Background(), uuid.New(), bookingID, att.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-owner detach: got %v, want ErrForbidden", err)
	}
	if err := svc.Detach(context.Background(), teacherOwner, bookingID, att.ID); err != nil {
		t.Fatalf("owner detach: %v", err)
	}
	// Detaching an unknown attachment id 404s.
	if err := svc.Detach(context.Background(), teacherOwner, bookingID, uuid.New()); !errors.Is(err, ErrAttachmentNotFound) {
		t.Errorf("re-detach: got %v, want ErrAttachmentNotFound", err)
	}
}

// --- ForBooking: content stripping ---

func TestForBooking_StudentSeesStrippedContent_TeacherSeesFull(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := mustPublishedResource(t, svc, teacherOwner, TypeQuiz, quizContent())
	if _, err := svc.Attach(context.Background(), teacherOwner, bookingID, res.ID, KindHomework, nil); err != nil {
		t.Fatalf("attach: %v", err)
	}

	studentView, err := svc.ForBooking(context.Background(), student, bookingID)
	if err != nil {
		t.Fatalf("student ForBooking: %v", err)
	}
	if len(studentView) != 1 {
		t.Fatalf("got %d attachments", len(studentView))
	}
	for _, q := range studentView[0].Content.Questions {
		if q.Correct != nil {
			t.Errorf("student view leaked Correct: %+v", q)
		}
	}

	teacherView, err := svc.ForBooking(context.Background(), teacherOwner, bookingID)
	if err != nil {
		t.Fatalf("teacher ForBooking: %v", err)
	}
	if len(teacherView[0].Content.Questions) == 0 || teacherView[0].Content.Questions[0].Correct == nil {
		t.Errorf("teacher view should keep Correct: %+v", teacherView[0].Content)
	}

	// A non-participant is forbidden.
	if _, err := svc.ForBooking(context.Background(), uuid.New(), bookingID); !errors.Is(err, ErrForbidden) {
		t.Errorf("stranger: got %v, want ErrForbidden", err)
	}
}

// --- submission lifecycle ---

func attachHomework(t *testing.T, svc *Service, teacherOwner, bookingID uuid.UUID, typ Type, content Content) Resource {
	t.Helper()
	res := mustPublishedResource(t, svc, teacherOwner, typ, content)
	if _, err := svc.Attach(context.Background(), teacherOwner, bookingID, res.ID, KindHomework, nil); err != nil {
		t.Fatalf("attach homework: %v", err)
	}
	return res
}

func TestStartSubmission_Idempotent(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeQuiz, quizContent())

	s1, err := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	if err != nil {
		t.Fatalf("start 1: %v", err)
	}
	s2, err := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	if err != nil {
		t.Fatalf("start 2: %v", err)
	}
	if s1.ID != s2.ID {
		t.Errorf("start is not idempotent: %s != %s", s1.ID, s2.ID)
	}

	// Not the booking's student.
	if _, err := svc.StartSubmission(context.Background(), uuid.New(), res.ID, bookingID); !errors.Is(err, ErrForbidden) {
		t.Errorf("wrong student: got %v, want ErrForbidden", err)
	}
}

func TestStartSubmission_RequiresHomeworkAttachment(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	// Attached as MATERIAL, not homework.
	res := mustPublishedResource(t, svc, teacherOwner, TypeQuiz, quizContent())
	if _, err := svc.Attach(context.Background(), teacherOwner, bookingID, res.ID, KindMaterial, nil); err != nil {
		t.Fatalf("attach: %v", err)
	}

	if _, err := svc.StartSubmission(context.Background(), student, res.ID, bookingID); !errors.Is(err, ErrHomeworkNotAssigned) {
		t.Errorf("got %v, want ErrHomeworkNotAssigned", err)
	}
}

func TestSaveAnswers_OnlyWhileInProgress(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeQuiz, quizContent())
	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)

	if _, err := svc.SaveAnswers(context.Background(), uuid.New(), sub.ID, map[string][]string{}); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-owner save: got %v, want ErrForbidden", err)
	}

	saved, err := svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{"q": {"b"}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Status != SubmissionInProgress {
		t.Fatalf("unexpected status: %s", saved.Status)
	}

	if _, err := svc.Submit(context.Background(), student, sub.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{}); !errors.Is(err, ErrInvalidSubmissionState) {
		t.Errorf("save after submit: got %v, want ErrInvalidSubmissionState", err)
	}
}

// quizContentMulti / quizContentText build quiz-like content with one
// multi-select and one text question, for scoring tests.
func quizContentMulti() Content {
	return Content{Questions: []Question{{
		ID: "q1", Prompt: "Pick the vowels", Kind: "multi",
		Choices: []Choice{{ID: "a", Text: "a"}, {ID: "b", Text: "b"}, {ID: "e", Text: "e"}},
		Correct: []string{"a", "e"}, Points: 2,
	}}}
}

func quizContentTextQ() Content {
	return Content{Questions: []Question{{
		ID: "q1", Prompt: "Capital of France?", Kind: "text",
		Correct: []string{"Paris"}, Points: 3,
	}}}
}

func TestSubmit_AutoGrade_SingleChoice_AllOrNothing(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeQuiz, quizContent()) // single-choice, correct = "b", points 1

	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	// quizContent's question has no explicit ID (normalizeQuestion assigns a
	// uuid), so fetch the persisted resource to answer by its real question id.
	got, _ := svc.Get(context.Background(), teacherOwner, res.ID)
	qid := got.Content.Questions[0].ID
	svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{qid: {"b"}})

	graded, err := svc.Submit(context.Background(), student, sub.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if graded.Status != SubmissionGraded {
		t.Fatalf("status = %s, want graded", graded.Status)
	}
	if graded.AutoScore == nil || graded.AutoMax == nil || *graded.AutoScore != 1 || *graded.AutoMax != 1 {
		t.Fatalf("score = %v / %v, want 1/1", graded.AutoScore, graded.AutoMax)
	}
}

func TestSubmit_AutoGrade_Multi_PartialAnswerScoresZero(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeQuiz, quizContentMulti())

	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	// Only one of the two correct choices: no partial credit, this question is
	// all-or-nothing.
	svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{"q1": {"a"}})

	graded, err := svc.Submit(context.Background(), student, sub.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if *graded.AutoScore != 0 || *graded.AutoMax != 2 {
		t.Fatalf("score = %d / %d, want 0/2", *graded.AutoScore, *graded.AutoMax)
	}
}

func TestSubmit_AutoGrade_Multi_FullAnswerScoresFull(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeQuiz, quizContentMulti())

	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{"q1": {"a", "e"}})

	graded, err := svc.Submit(context.Background(), student, sub.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if *graded.AutoScore != 2 || *graded.AutoMax != 2 {
		t.Fatalf("score = %d / %d, want 2/2", *graded.AutoScore, *graded.AutoMax)
	}
}

func TestSubmit_AutoGrade_Text_CaseInsensitiveTrimmedMatch(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeQuiz, quizContentTextQ())

	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{"q1": {"  paris  "}})

	graded, err := svc.Submit(context.Background(), student, sub.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if *graded.AutoScore != 3 || *graded.AutoMax != 3 {
		t.Fatalf("score = %d / %d, want 3/3", *graded.AutoScore, *graded.AutoMax)
	}
}

func TestSubmit_Writing_GoesToSubmittedNotGraded(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeWriting, Content{Prompt: "Write about your day."})

	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{"essay": {"It was good."}})

	submitted, err := svc.Submit(context.Background(), student, sub.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != SubmissionSubmitted {
		t.Fatalf("status = %s, want submitted", submitted.Status)
	}
	if submitted.AutoScore != nil || submitted.AutoMax != nil {
		t.Errorf("writing should not carry auto scores: %+v", submitted)
	}
	if submitted.SubmittedAt == nil {
		t.Error("submitted_at should be set")
	}
}

func TestGrade_TransitionsSubmittedToGraded_UnauthorizedRejected(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeWriting, Content{Prompt: "Write about your day."})
	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{"essay": {"It was good."}})
	submitted, _ := svc.Submit(context.Background(), student, sub.ID)

	// Wrong teacher.
	if _, err := svc.Grade(context.Background(), uuid.New(), submitted.ID, nil, "nice"); !errors.Is(err, ErrForbidden) {
		t.Errorf("wrong grader: got %v, want ErrForbidden", err)
	}

	score := 8
	graded, err := svc.Grade(context.Background(), teacherOwner, submitted.ID, &score, "Good effort!")
	if err != nil {
		t.Fatalf("grade: %v", err)
	}
	if graded.Status != SubmissionGraded {
		t.Fatalf("status = %s, want graded", graded.Status)
	}
	if graded.TeacherScore == nil || *graded.TeacherScore != 8 || graded.TeacherFeedback != "Good effort!" {
		t.Fatalf("unexpected grade: %+v", graded)
	}

	// Grading an already-graded submission fails.
	if _, err := svc.Grade(context.Background(), teacherOwner, submitted.ID, &score, "again"); !errors.Is(err, ErrInvalidSubmissionState) {
		t.Errorf("re-grade: got %v, want ErrInvalidSubmissionState", err)
	}
}

func TestGrade_RejectsAutoGradableTypes(t *testing.T) {
	svc, _, _, teacherOwner, student, bookingID := newSvcWithBooking(t)
	res := attachHomework(t, svc, teacherOwner, bookingID, TypeQuiz, quizContent())
	sub, _ := svc.StartSubmission(context.Background(), student, res.ID, bookingID)
	got, _ := svc.Get(context.Background(), teacherOwner, res.ID)
	svc.SaveAnswers(context.Background(), student, sub.ID, map[string][]string{got.Content.Questions[0].ID: {"b"}})
	graded, _ := svc.Submit(context.Background(), student, sub.ID)

	if _, err := svc.Grade(context.Background(), teacherOwner, graded.ID, nil, "x"); !isValidation(err) {
		t.Errorf("grading a quiz submission: got %v, want ValidationError", err)
	}
}
