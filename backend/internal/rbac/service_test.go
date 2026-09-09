package rbac

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestPermissionsFor_UnionAndDedupe(t *testing.T) {
	repo := newFakeRepo()
	r1 := repo.addRole("a", false, PermMetricsView, PermUsersView)
	r2 := repo.addRole("b", false, PermUsersView, PermTeachersView)
	user := uuid.New()
	repo.grant(user, r1.ID)
	repo.grant(user, r2.ID)

	perms, err := NewService(repo).PermissionsFor(context.Background(), user)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	got := map[Permission]int{}
	for _, p := range perms {
		got[p]++
	}
	if len(perms) != 3 || got[PermUsersView] != 1 {
		t.Fatalf("want 3 deduped perms, got %v", perms)
	}
}

func TestPermissionsFor_SuperadminAlwaysFullCatalog(t *testing.T) {
	repo := newFakeRepo()
	// superadmin role deliberately seeded with only ONE stored permission —
	// the guard must still hand back the whole catalog.
	sa := repo.addRole(SuperadminRoleName, true, PermMetricsView)
	user := uuid.New()
	repo.grant(user, sa.ID)

	perms, err := NewService(repo).PermissionsFor(context.Background(), user)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(perms) != len(AllPermissions) {
		t.Fatalf("superadmin got %d perms, want full catalog of %d", len(perms), len(AllPermissions))
	}
}

func TestCreateRole_UnknownPermission(t *testing.T) {
	_, err := NewService(newFakeRepo()).CreateRole(context.Background(), "x", "", []string{"metrics.view", "nope.nope"})
	var ve ValidationError
	if !errors.As(err, &ve) || ve.Code != "unknown_permission" {
		t.Fatalf("err = %v, want ValidationError{unknown_permission}", err)
	}
}

func TestCreateRole_DuplicateName(t *testing.T) {
	repo := newFakeRepo()
	repo.addRole("support", false)
	_, err := NewService(repo).CreateRole(context.Background(), "support", "", nil)
	if !errors.Is(err, ErrRoleExists) {
		t.Fatalf("err = %v, want ErrRoleExists", err)
	}
}

func TestUpdateRole_SystemRolePermissionsLocked(t *testing.T) {
	repo := newFakeRepo()
	sa := repo.addRole(SuperadminRoleName, true, PermMetricsView)
	svc := NewService(repo)

	perms := []string{"metrics.view"}
	if _, err := svc.UpdateRole(context.Background(), sa.ID, nil, &perms); !errors.Is(err, ErrRoleLocked) {
		t.Fatalf("editing system perms: err = %v, want ErrRoleLocked", err)
	}
	// description-only edit is allowed
	desc := "new description"
	if _, err := svc.UpdateRole(context.Background(), sa.ID, &desc, nil); err != nil {
		t.Fatalf("description edit on system role: %v", err)
	}
}

func TestDeleteRole_SystemAndInUse(t *testing.T) {
	repo := newFakeRepo()
	sa := repo.addRole(SuperadminRoleName, true)
	support := repo.addRole("support", false)
	repo.grant(uuid.New(), support.ID)
	svc := NewService(repo)

	if err := svc.DeleteRole(context.Background(), sa.ID); !errors.Is(err, ErrRoleLocked) {
		t.Fatalf("delete system: err = %v, want ErrRoleLocked", err)
	}
	if err := svc.DeleteRole(context.Background(), support.ID); !errors.Is(err, ErrRoleInUse) {
		t.Fatalf("delete in-use: err = %v, want ErrRoleInUse", err)
	}
}

func TestAssignUnassign_IdempotentAndUnknown(t *testing.T) {
	repo := newFakeRepo()
	role := repo.addRole("support", false)
	user := uuid.New()
	repo.users[user] = true
	svc := NewService(repo)
	ctx := context.Background()

	if err := svc.AssignRole(ctx, user, role.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := svc.AssignRole(ctx, user, role.ID); err != nil {
		t.Fatalf("assign again (idempotent): %v", err)
	}
	if err := svc.AssignRole(ctx, uuid.New(), role.ID); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("assign unknown user: err = %v, want ErrUserNotFound", err)
	}
	if err := svc.AssignRole(ctx, user, uuid.New()); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("assign unknown role: err = %v, want ErrRoleNotFound", err)
	}
	if err := svc.UnassignRole(ctx, user, role.ID); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	if err := svc.UnassignRole(ctx, user, role.ID); err != nil {
		t.Fatalf("unassign again (idempotent): %v", err)
	}
}
