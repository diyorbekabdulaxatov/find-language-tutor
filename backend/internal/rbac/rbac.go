// Package rbac is the role-based access-control module: a permission catalog
// (permissions.go), named roles that bundle permissions, and per-user role
// assignments. A caller's effective permissions are resolved from the database
// on every request — never carried in the JWT — so revoking a role takes effect
// immediately.
//
// Layout mirrors the other modules: domain types + errors here, a Service with
// the rules, a Repository port (Postgres impl alongside, fake in tests), a gin
// guard + admin handlers, and RegisterAdminRoutes. The Service never sees an
// *gin.Context.
//
// Boundary with auth: auth defines auth.PermissionsPort and this package
// implements it (AuthPermissions in auth_adapter.go) so GET /v1/auth/me can
// carry the caller's permissions. auth never imports this package.
package rbac

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SuperadminRoleName is the one system role that always holds every permission
// in the catalog — enforced both by the seed (recomputed from AllPermissions)
// and by a guard in Service.PermissionsFor.
const SuperadminRoleName = "superadmin"

// Role is a named bundle of permissions.
type Role struct {
	ID          uuid.UUID
	Name        string
	Description string
	IsSystem    bool
	CreatedAt   time.Time
	Permissions []Permission
	UserCount   int64
}

// RoleRef is the light {id, name} shape embedded in user views.
type RoleRef struct {
	ID   uuid.UUID
	Name string
}

// Domain errors. The handler maps each to an HTTP status; anything else is 500.
var (
	ErrRoleNotFound = errors.New("rbac: role not found")
	// ErrRoleExists — CreateRole with a name already in use. 409 role_exists.
	ErrRoleExists = errors.New("rbac: role name already in use")
	// ErrRoleLocked — an attempt to re-permission or delete an is_system role.
	// 403 role_locked.
	ErrRoleLocked = errors.New("rbac: system role cannot be modified")
	// ErrRoleInUse — DeleteRole for a role still assigned to users. 409 role_in_use.
	ErrRoleInUse = errors.New("rbac: role is still assigned to users")
	// ErrUserNotFound — assign/unassign for an unknown user id. 404.
	ErrUserNotFound = errors.New("rbac: user not found")
)

// ValidationError is a client-fixable problem (an unknown permission key, an
// empty role name). The handler renders it as 400.
type ValidationError struct {
	Code string
	msg  string
}

func (e ValidationError) Error() string { return e.msg }

func invalid(code, format string, args ...any) error {
	return ValidationError{Code: code, msg: fmt.Sprintf(format, args...)}
}

// validatePermissions returns a ValidationError (code unknown_permission) if any
// key is not in the catalog, and the de-duplicated keys otherwise.
func validatePermissions(keys []string) ([]string, error) {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !ValidPermission(k) {
			return nil, invalid("unknown_permission", "unknown permission %q.", k)
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out, nil
}

func toPermissions(keys []string) []Permission {
	out := make([]Permission, len(keys))
	for i, k := range keys {
		out[i] = Permission(k)
	}
	return out
}

func containsPermission(set []Permission, p Permission) bool {
	for _, x := range set {
		if x == p {
			return true
		}
	}
	return false
}
