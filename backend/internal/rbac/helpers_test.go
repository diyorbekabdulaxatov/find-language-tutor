package rbac

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
)

// fakeRepo is an in-memory Repository for service + guard + handler tests.
type fakeRepo struct {
	users       map[uuid.UUID]bool
	roles       map[uuid.UUID]*Role
	rolesByUser map[uuid.UUID][]uuid.UUID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:       map[uuid.UUID]bool{},
		roles:       map[uuid.UUID]*Role{},
		rolesByUser: map[uuid.UUID][]uuid.UUID{},
	}
}

func (f *fakeRepo) addRole(name string, isSystem bool, perms ...Permission) *Role {
	r := &Role{ID: uuid.New(), Name: name, IsSystem: isSystem, CreatedAt: time.Now(), Permissions: append([]Permission(nil), perms...)}
	f.roles[r.ID] = r
	return r
}

func (f *fakeRepo) grant(userID uuid.UUID, roleID uuid.UUID) {
	f.users[userID] = true
	f.rolesByUser[userID] = append(f.rolesByUser[userID], roleID)
	f.roles[roleID].UserCount++
}

func (f *fakeRepo) PermissionKeysForUser(_ context.Context, userID uuid.UUID) ([]string, error) {
	set := map[string]struct{}{}
	for _, rid := range f.rolesByUser[userID] {
		for _, p := range f.roles[rid].Permissions {
			set[string(p)] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakeRepo) RoleRefsForUser(_ context.Context, userID uuid.UUID) ([]RoleRef, error) {
	var out []RoleRef
	for _, rid := range f.rolesByUser[userID] {
		r := f.roles[rid]
		out = append(out, RoleRef{ID: r.ID, Name: r.Name})
	}
	return out, nil
}

func (f *fakeRepo) ListRoles(_ context.Context) ([]Role, error) {
	var out []Role
	for _, r := range f.roles {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *fakeRepo) GetRole(_ context.Context, id uuid.UUID) (Role, error) {
	r, ok := f.roles[id]
	if !ok {
		return Role{}, ErrRoleNotFound
	}
	return *r, nil
}

func (f *fakeRepo) CreateRole(_ context.Context, name, description string, isSystem bool, permissions []string) (Role, error) {
	for _, r := range f.roles {
		if r.Name == name {
			return Role{}, ErrRoleExists
		}
	}
	r := &Role{ID: uuid.New(), Name: name, Description: description, IsSystem: isSystem, CreatedAt: time.Now(), Permissions: toPermissions(permissions)}
	f.roles[r.ID] = r
	return *r, nil
}

func (f *fakeRepo) UpdateRole(_ context.Context, id uuid.UUID, description string, permissions *[]string) (Role, error) {
	r, ok := f.roles[id]
	if !ok {
		return Role{}, ErrRoleNotFound
	}
	r.Description = description
	if permissions != nil {
		r.Permissions = toPermissions(*permissions)
	}
	return *r, nil
}

func (f *fakeRepo) DeleteRole(_ context.Context, id uuid.UUID) error {
	if _, ok := f.roles[id]; !ok {
		return ErrRoleNotFound
	}
	delete(f.roles, id)
	return nil
}

func (f *fakeRepo) UserExists(_ context.Context, userID uuid.UUID) (bool, error) {
	return f.users[userID], nil
}

func (f *fakeRepo) AssignRole(_ context.Context, userID, roleID uuid.UUID) error {
	for _, rid := range f.rolesByUser[userID] {
		if rid == roleID {
			return nil // idempotent
		}
	}
	f.rolesByUser[userID] = append(f.rolesByUser[userID], roleID)
	if r := f.roles[roleID]; r != nil {
		r.UserCount++
	}
	return nil
}

func (f *fakeRepo) UnassignRole(_ context.Context, userID, roleID uuid.UUID) error {
	cur := f.rolesByUser[userID]
	out := cur[:0]
	for _, rid := range cur {
		if rid != roleID {
			out = append(out, rid)
		}
	}
	f.rolesByUser[userID] = out
	return nil
}

var _ Repository = (*fakeRepo)(nil)
