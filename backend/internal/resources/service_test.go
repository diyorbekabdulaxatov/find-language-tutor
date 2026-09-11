package resources

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeRepo is an in-memory Repository.
type fakeRepo struct {
	teacherByOwner map[uuid.UUID]uuid.UUID
	byID           map[uuid.UUID]Resource
	assigned       map[uuid.UUID]bool
	seq            int

	// phase A2/A3
	attachments    map[uuid.UUID]BookingResource // by attachment id
	attachPairs    map[string]uuid.UUID          // "bookingID|resourceID" -> attachment id
	submissions    map[uuid.UUID]Submission
	submissionKeys map[string]uuid.UUID // "resourceID|studentID|bookingID" -> submission id
	userContacts   map[uuid.UUID][2]string
	fileAccessible map[string]bool // "fileAssetID|requesterID" -> ok
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		teacherByOwner: map[uuid.UUID]uuid.UUID{},
		byID:           map[uuid.UUID]Resource{},
		assigned:       map[uuid.UUID]bool{},

		attachments:    map[uuid.UUID]BookingResource{},
		attachPairs:    map[string]uuid.UUID{},
		submissions:    map[uuid.UUID]Submission{},
		submissionKeys: map[string]uuid.UUID{},
		userContacts:   map[uuid.UUID][2]string{},
		fileAccessible: map[string]bool{},
	}
}

// --- phase A2: booking attachment ---

func pairKey(a, b uuid.UUID) string { return a.String() + "|" + b.String() }

func (r *fakeRepo) AttachResource(_ context.Context, bookingID, resourceID uuid.UUID, kind string, assignedBy uuid.UUID, dueAt *time.Time) (BookingResource, error) {
	key := pairKey(bookingID, resourceID)
	if _, exists := r.attachPairs[key]; exists {
		return BookingResource{}, ErrAlreadyAttached
	}
	res, ok := r.byID[resourceID]
	if !ok {
		return BookingResource{}, ErrNotFound
	}
	position := 0
	for _, a := range r.attachments {
		if a.BookingID == bookingID {
			position++
		}
	}
	br := BookingResource{
		ID: uuid.New(), BookingID: bookingID, ResourceID: resourceID, Kind: kind, Position: position,
		AssignedBy: assignedBy, DueAt: dueAt, CreatedAt: time.Now(),
		Type: res.Type, Title: res.Title, Instructions: res.Instructions, ResourceStatus: res.Status, Content: res.Content,
	}
	r.attachments[br.ID] = br
	r.attachPairs[key] = br.ID
	return br, nil
}

func (r *fakeRepo) DetachResource(_ context.Context, bookingID, attachmentID uuid.UUID) error {
	a, ok := r.attachments[attachmentID]
	if !ok || a.BookingID != bookingID {
		return ErrAttachmentNotFound
	}
	delete(r.attachments, attachmentID)
	delete(r.attachPairs, pairKey(bookingID, a.ResourceID))
	return nil
}

func (r *fakeRepo) ListBookingResources(_ context.Context, bookingID uuid.UUID) ([]BookingResource, error) {
	var out []BookingResource
	for _, a := range r.attachments {
		if a.BookingID == bookingID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

func (r *fakeRepo) GetBookingResourceByPair(_ context.Context, bookingID, resourceID uuid.UUID) (BookingResource, bool, error) {
	id, ok := r.attachPairs[pairKey(bookingID, resourceID)]
	if !ok {
		return BookingResource{}, false, nil
	}
	return r.attachments[id], true, nil
}

// --- phase A3: submissions ---

func (r *fakeRepo) StartSubmission(_ context.Context, resourceID, studentID, bookingID uuid.UUID) (Submission, error) {
	key := resourceID.String() + "|" + studentID.String() + "|" + bookingID.String()
	if id, ok := r.submissionKeys[key]; ok {
		return r.submissions[id], nil
	}
	now := time.Now()
	sub := Submission{
		ID: uuid.New(), ResourceID: resourceID, StudentID: studentID, Context: "lesson", BookingID: bookingID,
		Status: SubmissionInProgress, Answers: map[string][]string{}, CreatedAt: now, UpdatedAt: now,
	}
	r.submissions[sub.ID] = sub
	r.submissionKeys[key] = sub.ID
	return sub, nil
}

func (r *fakeRepo) SaveSubmissionAnswers(_ context.Context, id uuid.UUID, answers map[string][]string) (Submission, error) {
	sub, ok := r.submissions[id]
	if !ok {
		return Submission{}, ErrSubmissionNotFound
	}
	sub.Answers = answers
	r.submissions[id] = sub
	return sub, nil
}

func (r *fakeRepo) SubmitSubmission(_ context.Context, id uuid.UUID, p SubmitParams) (Submission, error) {
	sub, ok := r.submissions[id]
	if !ok {
		return Submission{}, ErrSubmissionNotFound
	}
	sub.Status = p.Status
	sub.AutoScore = p.AutoScore
	sub.AutoMax = p.AutoMax
	t := p.SubmittedAt
	sub.SubmittedAt = &t
	r.submissions[id] = sub
	return sub, nil
}

func (r *fakeRepo) GradeSubmission(_ context.Context, id uuid.UUID, score *int, feedback string, gradedBy uuid.UUID, gradedAt time.Time) (Submission, error) {
	sub, ok := r.submissions[id]
	if !ok {
		return Submission{}, ErrSubmissionNotFound
	}
	sub.TeacherScore = score
	sub.TeacherFeedback = feedback
	sub.GradedBy = gradedBy
	t := gradedAt
	sub.GradedAt = &t
	sub.Status = SubmissionGraded
	r.submissions[id] = sub
	return sub, nil
}

func (r *fakeRepo) GetSubmission(_ context.Context, id uuid.UUID) (Submission, error) {
	sub, ok := r.submissions[id]
	if !ok {
		return Submission{}, ErrSubmissionNotFound
	}
	return sub, nil
}

func (r *fakeRepo) ListSubmissionsForBooking(_ context.Context, bookingID uuid.UUID) ([]Submission, error) {
	var out []Submission
	for _, s := range r.submissions {
		if s.BookingID == bookingID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *fakeRepo) InboxSubmissions(_ context.Context, teacherID uuid.UUID, status string, limit, offset int) ([]Submission, int, error) {
	var out []Submission
	for _, s := range r.submissions {
		res, ok := r.byID[s.ResourceID]
		if !ok || res.TeacherID != teacherID {
			continue
		}
		if status != "" && s.Status != status {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	total := len(out)
	if offset > len(out) {
		offset = len(out)
	}
	end := len(out)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return out[offset:end], total, nil
}

func (r *fakeRepo) FileAssetAccessible(_ context.Context, fileAssetID, requesterID uuid.UUID) (bool, error) {
	return r.fileAccessible[pairKey(fileAssetID, requesterID)], nil
}

func (r *fakeRepo) UserContact(_ context.Context, userID uuid.UUID) (string, string, error) {
	c, ok := r.userContacts[userID]
	if !ok {
		return "", "", nil
	}
	return c[0], c[1], nil
}

// fakeBookingReader is an in-memory resources.BookingReader.
type fakeBookingReader struct {
	byBooking map[uuid.UUID][2]uuid.UUID // bookingID -> [teacherOwnerID, studentID]
	err       error
}

func newFakeBookingReader() *fakeBookingReader {
	return &fakeBookingReader{byBooking: map[uuid.UUID][2]uuid.UUID{}}
}

func (f *fakeBookingReader) Booking(_ context.Context, bookingID uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	if f.err != nil {
		return uuid.Nil, uuid.Nil, false, f.err
	}
	b, ok := f.byBooking[bookingID]
	if !ok {
		return uuid.Nil, uuid.Nil, false, nil
	}
	return b[0], b[1], true, nil
}

func (r *fakeRepo) TeacherIDByOwner(_ context.Context, u uuid.UUID) (uuid.UUID, bool, error) {
	tid, ok := r.teacherByOwner[u]
	return tid, ok, nil
}

func (r *fakeRepo) Create(_ context.Context, p CreateParams) (Resource, error) {
	r.seq++
	res := Resource{
		ID: uuid.New(), TeacherID: p.TeacherID, Type: p.Type, Title: p.Title,
		Instructions: p.Instructions, Content: p.Content, Status: p.Status,
	}
	r.byID[res.ID] = res
	return res, nil
}

func (r *fakeRepo) ByID(_ context.Context, id uuid.UUID) (Resource, error) {
	res, ok := r.byID[id]
	if !ok {
		return Resource{}, ErrNotFound
	}
	return res, nil
}

func (r *fakeRepo) List(_ context.Context, tid uuid.UUID, q ListQuery) ([]Resource, int, error) {
	var out []Resource
	for _, res := range r.byID {
		if res.TeacherID != tid {
			continue
		}
		if q.Type != "" && res.Type != q.Type {
			continue
		}
		if q.Status != "" && res.Status != q.Status {
			continue
		}
		if !q.IncludeArchived && res.ArchivedAt != nil {
			continue
		}
		out = append(out, res)
	}
	return out, len(out), nil
}

func (r *fakeRepo) Update(_ context.Context, id uuid.UUID, title, instr string, c Content) (Resource, error) {
	res, ok := r.byID[id]
	if !ok {
		return Resource{}, ErrNotFound
	}
	res.Title, res.Instructions, res.Content = title, instr, c
	r.byID[id] = res
	return res, nil
}

func (r *fakeRepo) SetStatus(_ context.Context, id uuid.UUID, s Status) (Resource, error) {
	res := r.byID[id]
	res.Status = s
	r.byID[id] = res
	return res, nil
}

func (r *fakeRepo) SetArchived(_ context.Context, id uuid.UUID, a bool) (Resource, error) {
	res := r.byID[id]
	if a {
		now := res.CreatedAt
		res.ArchivedAt = &now
	} else {
		res.ArchivedAt = nil
	}
	r.byID[id] = res
	return res, nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.byID, id)
	return nil
}

func (r *fakeRepo) IsAssigned(_ context.Context, id uuid.UUID) (bool, error) {
	return r.assigned[id], nil
}

func newSvc(t *testing.T) (*Service, *fakeRepo, uuid.UUID, uuid.UUID) {
	t.Helper()
	repo := newFakeRepo()
	owner, teacher := uuid.New(), uuid.New()
	repo.teacherByOwner[owner] = teacher
	return NewService(repo, discardLogger()), repo, owner, teacher
}

func quizContent() Content {
	return Content{Questions: []Question{{
		Prompt: "2+2?", Kind: "single",
		Choices: []Choice{{ID: "a", Text: "3"}, {ID: "b", Text: "4"}},
		Correct: []string{"b"},
	}}}
}

func TestCreate_RequiresTeacherProfile(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Create(context.Background(), uuid.New() /* not a teacher */, TypeQuiz, "Q", "", quizContent(), false)
	if !errors.Is(err, ErrNoTeacher) {
		t.Fatalf("got %v, want ErrNoTeacher", err)
	}
}

func TestCreate_And_Publish(t *testing.T) {
	svc, _, owner, _ := newSvc(t)
	ctx := context.Background()

	r, err := svc.Create(ctx, owner, TypeQuiz, "  Numbers quiz  ", "answer all", quizContent(), false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if r.Title != "Numbers quiz" || r.Status != StatusDraft {
		t.Fatalf("unexpected: %+v", r)
	}

	pub, err := svc.SetPublished(ctx, owner, r.ID, true)
	if err != nil || pub.Status != StatusPublished {
		t.Fatalf("publish: %v status=%s", err, pub.Status)
	}
}

func TestGet_OwnershipEnforced(t *testing.T) {
	svc, repo, owner, _ := newSvc(t)
	r, _ := svc.Create(context.Background(), owner, TypeArticle, "A", "", Content{Body: "hi"}, false)

	otherOwner, otherTeacher := uuid.New(), uuid.New()
	repo.teacherByOwner[otherOwner] = otherTeacher
	if _, err := svc.Get(context.Background(), otherOwner, r.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestUpdate_TypeIsImmutable_ContentRevalidated(t *testing.T) {
	svc, _, owner, _ := newSvc(t)
	r, _ := svc.Create(context.Background(), owner, TypeWriting, "W", "", Content{Prompt: "Write"}, false)

	// updating with content that's invalid for a writing task fails
	if _, err := svc.Update(context.Background(), owner, r.ID, "W2", "", Content{Prompt: ""}); err == nil {
		t.Error("blank prompt on update should fail")
	}
	got, err := svc.Update(context.Background(), owner, r.ID, "W2", "", Content{Prompt: "New prompt"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Content.Prompt != "New prompt" || got.Type != TypeWriting {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestDelete_BlockedWhenAssigned(t *testing.T) {
	svc, repo, owner, _ := newSvc(t)
	r, _ := svc.Create(context.Background(), owner, TypeArticle, "A", "", Content{Body: "hi"}, false)
	repo.assigned[r.ID] = true
	if err := svc.Delete(context.Background(), owner, r.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("got %v, want ErrInUse", err)
	}
	repo.assigned[r.ID] = false
	if err := svc.Delete(context.Background(), owner, r.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestLibrary_FilterAndPaging(t *testing.T) {
	svc, _, owner, _ := newSvc(t)
	ctx := context.Background()
	_, _ = svc.Create(ctx, owner, TypeQuiz, "Q1", "", quizContent(), true)
	_, _ = svc.Create(ctx, owner, TypeArticle, "A1", "", Content{Body: "x"}, false)

	page, err := svc.Library(ctx, owner, ListQuery{Type: TypeQuiz})
	if err != nil {
		t.Fatalf("library: %v", err)
	}
	if page.Total != 1 || page.Resources[0].Type != TypeQuiz {
		t.Fatalf("type filter wrong: %+v", page)
	}

	if _, err := svc.Library(ctx, owner, ListQuery{Type: "bogus"}); err == nil {
		t.Error("bad type filter should 400")
	}
}
