package auth

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// PermissionsPort is the slice of the RBAC module the auth responses need: the
// flat set of permission keys a user holds across all their roles. internal/rbac
// provides an adapter that satisfies it; auth never imports rbac, so the
// dependency runs one way. cmd/api injects it with Service.SetPermissionsPort.
//
// Permissions are resolved per request and are NEVER in the JWT — a demoted
// admin loses access immediately, not at the next token refresh.
type PermissionsPort interface {
	// PermissionsFor returns the user's effective permission keys, sorted. An
	// unknown user is not an error — it returns an empty slice.
	PermissionsFor(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// permissionsFor is a nil-safe, error-swallowing wrapper: no port, an unknown
// user, or a lookup failure all yield an empty (non-nil) slice so a permissions
// hiccup never blocks a login or a /me read.
func (s *Service) permissionsFor(ctx context.Context, userID uuid.UUID) []string {
	if s.perms == nil {
		return []string{}
	}
	perms, err := s.perms.PermissionsFor(ctx, userID)
	if err != nil {
		slog.Default().Error("resolve permissions", slog.String("user_id", userID.String()), slog.Any("error", err))
		return []string{}
	}
	if perms == nil {
		return []string{}
	}
	return perms
}
