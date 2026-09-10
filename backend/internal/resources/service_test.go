package resources

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeRepo is an in-memory Repository.
type fakeRepo struct {
	teacherByOwner map[uuid.UUID]uuid.UUID
	byID           map[uuid.UUID]Resource
	assigned       map[uuid.UUID]bool
	seq            int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		teacherByOwner: map[uuid.UUID]uuid.UUID{},
		byID:           map[uuid.UUID]Resource{},
		assigned:       map[uuid.UUID]bool{},
	}
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
