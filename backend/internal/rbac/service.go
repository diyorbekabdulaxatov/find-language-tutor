package rbac

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Repository is the persistence port. The Postgres implementation lives
// alongside; tests use a fake.
type Repository interface {
	// PermissionKeysForUser returns the union of permission keys across the
	// user's roles, sorted. An unknown user yields an empty slice.
	PermissionKeysForUser(ctx context.Context, userID uuid.UUID) ([]string, error)
	// RoleRefsForUser returns the user's roles ({id, name}).
	RoleRefsForUser(ctx context.Context, userID uuid.UUID) ([]RoleRef, error)

	ListRoles(ctx context.Context) ([]Role, error)
	// GetRole returns the role with its permissions + user count, or ErrRoleNotFound.
	GetRole(ctx context.Context, id uuid.UUID) (Role, error)
	// CreateRole inserts a role and its permissions. ErrRoleExists on a dup name.
	CreateRole(ctx context.Context, name, description string, isSystem bool, permissions []string) (Role, error)
	// UpdateRole writes the description (always) and, when permissions != nil,
	// replaces the permission set.
	UpdateRole(ctx context.Context, id uuid.UUID, description string, permissions *[]string) (Role, error)
	DeleteRole(ctx context.Context, id uuid.UUID) error

	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	AssignRole(ctx context.Context, userID, roleID uuid.UUID) error
	UnassignRole(ctx context.Context, userID, roleID uuid.UUID) error
}

// Service holds the RBAC rules. Handlers + the guard call it; it never sees an
// *gin.Context.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// PermissionsFor resolves a user's effective permissions from the database. A
// holder of the system `superadmin` role always gets the full catalog, even if
// the stored role_permissions rows lag behind a catalog change.
func (s *Service) PermissionsFor(ctx context.Context, userID uuid.UUID) ([]Permission, error) {
	roles, err := s.repo.RoleRefsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r.Name == SuperadminRoleName {
			return append([]Permission(nil), AllPermissions...), nil
		}
	}
	keys, err := s.repo.PermissionKeysForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toPermissions(keys), nil
}

// PermissionKeysFor is PermissionsFor as a sorted []string, for the auth adapter.
func (s *Service) PermissionKeysFor(ctx context.Context, userID uuid.UUID) ([]string, error) {
	perms, err := s.PermissionsFor(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	sort.Strings(out)
	return out, nil
}

// RolesForUser returns the user's role refs.
func (s *Service) RolesForUser(ctx context.Context, userID uuid.UUID) ([]RoleRef, error) {
	return s.repo.RoleRefsForUser(ctx, userID)
}

// Catalog is the permission catalog for GET /v1/admin/permissions.
func (s *Service) Catalog() []PermissionInfo { return Catalog }

// ListRoles returns every role with its permissions + assignment count.
func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	return s.repo.ListRoles(ctx)
}

// GetRole returns one role, or ErrRoleNotFound.
func (s *Service) GetRole(ctx context.Context, id uuid.UUID) (Role, error) {
	return s.repo.GetRole(ctx, id)
}

// CreateRole creates a non-system role. Every permission must be in the catalog
// (ValidationError unknown_permission otherwise); the name must be unique
// (ErrRoleExists).
func (s *Service) CreateRole(ctx context.Context, name, description string, permissions []string) (Role, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Role{}, invalid("invalid_name", "a role name is required.")
	}
	perms, err := validatePermissions(permissions)
	if err != nil {
		return Role{}, err
	}
	return s.repo.CreateRole(ctx, name, strings.TrimSpace(description), false, perms)
}

// UpdateRole edits a role. description is always applied. permissions, when
// non-nil, replaces the set — but an is_system role's permissions are locked
// (ErrRoleLocked).
func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, description *string, permissions *[]string) (Role, error) {
	role, err := s.repo.GetRole(ctx, id)
	if err != nil {
		return Role{}, err
	}
	if permissions != nil && role.IsSystem {
		return Role{}, ErrRoleLocked
	}

	desc := role.Description
	if description != nil {
		desc = strings.TrimSpace(*description)
	}

	var validated *[]string
	if permissions != nil {
		v, err := validatePermissions(*permissions)
		if err != nil {
			return Role{}, err
		}
		validated = &v
	}
	return s.repo.UpdateRole(ctx, id, desc, validated)
}

// DeleteRole removes a non-system role that no user holds. ErrRoleLocked for a
// system role; ErrRoleInUse when user_roles still references it.
func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) error {
	role, err := s.repo.GetRole(ctx, id)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return ErrRoleLocked
	}
	if role.UserCount > 0 {
		return ErrRoleInUse
	}
	return s.repo.DeleteRole(ctx, id)
}

// AssignRole grants a role to a user (idempotent). 404 for an unknown user or
// role.
func (s *Service) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	if ok, err := s.repo.UserExists(ctx, userID); err != nil {
		return err
	} else if !ok {
		return ErrUserNotFound
	}
	if _, err := s.repo.GetRole(ctx, roleID); err != nil {
		return err
	}
	return s.repo.AssignRole(ctx, userID, roleID)
}

// UnassignRole removes a role from a user (idempotent). 404 for an unknown user
// or role.
func (s *Service) UnassignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	if ok, err := s.repo.UserExists(ctx, userID); err != nil {
		return err
	} else if !ok {
		return ErrUserNotFound
	}
	if _, err := s.repo.GetRole(ctx, roleID); err != nil {
		return err
	}
	return s.repo.UnassignRole(ctx, userID, roleID)
}
