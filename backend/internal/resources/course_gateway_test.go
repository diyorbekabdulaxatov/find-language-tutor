package resources

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestCourseGateway_ResourceOwnedAndPublished exercises the adapter courses
// actually calls through (courses.ResourceReader), independent of the
// courses package's own fake-based tests.
func TestCourseGateway_ResourceOwnedAndPublished(t *testing.T) {
	svc, repo, owner, teacher := newSvc(t)
	ctx := context.Background()
	gw := NewCourseGateway(svc)

	published, err := svc.Create(ctx, owner, TypeArticle, "Article", "", Content{Body: "hi"}, true)
	if err != nil {
		t.Fatalf("create published: %v", err)
	}
	draft, err := svc.Create(ctx, owner, TypeArticle, "Draft", "", Content{Body: "hi"}, false)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	archived, err := svc.Create(ctx, owner, TypeArticle, "Archived", "", Content{Body: "hi"}, true)
	if err != nil {
		t.Fatalf("create archived: %v", err)
	}
	if _, err := svc.SetArchived(ctx, owner, archived.ID, true); err != nil {
		t.Fatalf("archive: %v", err)
	}
	otherTeacher := uuid.New()
	repo.teacherByOwner[uuid.New()] = otherTeacher

	cases := []struct {
		name       string
		resourceID uuid.UUID
		teacherID  uuid.UUID
		want       bool
	}{
		{"owned and published", published.ID, teacher, true},
		{"owned but draft", draft.ID, teacher, false},
		{"owned but archived", archived.ID, teacher, false},
		{"published but a different teacher", published.ID, otherTeacher, false},
		{"unknown resource id", uuid.New(), teacher, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := gw.ResourceOwnedAndPublished(ctx, tc.resourceID, tc.teacherID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
