package courses

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// --- phase C3: admin moderation ---

func TestService_AdminModerate_FiltersByStatusAndSuspended(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()

	published, _ := e.publishWithOneVideoItem(t, owner, 1000)
	draft := mustCreate(t, e, owner, "A Draft")

	if _, err := e.svc.AdminSuspend(ctx, published.Course.ID); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	// status=published, any suspension -> the one published course.
	page, err := e.svc.AdminModerate(ctx, AdminCourseQuery{Status: "published"})
	if err != nil {
		t.Fatalf("moderate: %v", err)
	}
	if page.Total != 1 || page.Courses[0].Course.ID != published.Course.ID {
		t.Errorf("status=published: got total=%d courses=%+v", page.Total, page.Courses)
	}

	// status=draft -> the draft course only.
	page, err = e.svc.AdminModerate(ctx, AdminCourseQuery{Status: "draft"})
	if err != nil {
		t.Fatalf("moderate: %v", err)
	}
	if page.Total != 1 || page.Courses[0].Course.ID != draft.Course.ID {
		t.Errorf("status=draft: got total=%d courses=%+v", page.Total, page.Courses)
	}

	// suspended=true -> the published-but-suspended course.
	page, err = e.svc.AdminModerate(ctx, AdminCourseQuery{Suspended: "true"})
	if err != nil {
		t.Fatalf("moderate: %v", err)
	}
	if page.Total != 1 || page.Courses[0].Course.ID != published.Course.ID {
		t.Errorf("suspended=true: got total=%d courses=%+v", page.Total, page.Courses)
	}

	// suspended=false -> the draft course only.
	page, err = e.svc.AdminModerate(ctx, AdminCourseQuery{Suspended: "false"})
	if err != nil {
		t.Fatalf("moderate: %v", err)
	}
	if page.Total != 1 || page.Courses[0].Course.ID != draft.Course.ID {
		t.Errorf("suspended=false: got total=%d courses=%+v", page.Total, page.Courses)
	}

	// No filter -> both.
	page, err = e.svc.AdminModerate(ctx, AdminCourseQuery{})
	if err != nil {
		t.Fatalf("moderate: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("no filter: want 2 courses, got %d", page.Total)
	}
}

func TestService_AdminModerate_FiltersByQ(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	mustCreate(t, e, owner, "Uzbek for Beginners")
	mustCreate(t, e, owner, "Advanced Russian Grammar")

	page, err := e.svc.AdminModerate(ctx, AdminCourseQuery{Q: "uzbek"})
	if err != nil {
		t.Fatalf("moderate: %v", err)
	}
	if page.Total != 1 || page.Courses[0].Course.Title != "Uzbek for Beginners" {
		t.Errorf("q=uzbek: got total=%d courses=%+v", page.Total, page.Courses)
	}
}

func TestService_AdminModerate_RejectsBadFilters(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()

	if _, err := e.svc.AdminModerate(ctx, AdminCourseQuery{Status: "bogus"}); err == nil {
		t.Error("want a validation error for an unrecognised status filter")
	}
	if _, err := e.svc.AdminModerate(ctx, AdminCourseQuery{Suspended: "bogus"}); err == nil {
		t.Error("want a validation error for an unrecognised suspended filter")
	}
}

func TestService_AdminSuspendUnsuspend_Idempotent(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d, _ := e.publishWithOneVideoItem(t, owner, 1000)

	a, err := e.svc.AdminSuspend(ctx, d.Course.ID)
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if a.Course.SuspendedAt == nil {
		t.Fatal("expected suspended_at to be set")
	}
	firstSuspendedAt := *a.Course.SuspendedAt

	// Suspending an already-suspended course is a no-op 200, not an error —
	// and must not slide suspended_at forward.
	a, err = e.svc.AdminSuspend(ctx, d.Course.ID)
	if err != nil {
		t.Fatalf("suspend again: %v", err)
	}
	if a.Course.SuspendedAt == nil || !a.Course.SuspendedAt.Equal(firstSuspendedAt) {
		t.Errorf("re-suspending should be a no-op, suspended_at changed: got %v, want %v", a.Course.SuspendedAt, firstSuspendedAt)
	}

	a, err = e.svc.AdminUnsuspend(ctx, d.Course.ID)
	if err != nil {
		t.Fatalf("unsuspend: %v", err)
	}
	if a.Course.SuspendedAt != nil {
		t.Errorf("expected suspended_at to be cleared, got %v", a.Course.SuspendedAt)
	}

	// Unsuspending an already-active course is also a no-op 200.
	a, err = e.svc.AdminUnsuspend(ctx, d.Course.ID)
	if err != nil {
		t.Fatalf("unsuspend again: %v", err)
	}
	if a.Course.SuspendedAt != nil {
		t.Errorf("expected suspended_at to remain cleared, got %v", a.Course.SuspendedAt)
	}
}

func TestService_AdminSuspend_UnknownCourse(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()

	if _, err := e.svc.AdminSuspend(ctx, uuid.New()); err == nil {
		t.Error("want an error for an unknown course id")
	}
}
