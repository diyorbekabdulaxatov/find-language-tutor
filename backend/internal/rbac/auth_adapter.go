package rbac

import (
	"context"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

// AuthPermissions adapts *Service to auth.PermissionsPort so the auth responses
// (login / register / refresh / GET /v1/auth/me) can carry the caller's
// permission keys. auth never imports this package; cmd/api injects the adapter
// with auth.Service.SetPermissionsPort.
type AuthPermissions struct{ svc *Service }

// NewAuthPermissions wraps the RBAC service as an auth.PermissionsPort.
func NewAuthPermissions(svc *Service) *AuthPermissions { return &AuthPermissions{svc: svc} }

var _ auth.PermissionsPort = (*AuthPermissions)(nil)

func (a *AuthPermissions) PermissionsFor(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return a.svc.PermissionKeysFor(ctx, userID)
}
