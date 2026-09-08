package rbac

import (
	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

const ctxPermissionsKey = "rbac.permissions"

// Guard turns the RBAC service into gin middleware. It expects auth.RequireAuth
// to have already run on the group (so auth.UserID is populated); it then
// resolves the caller's permissions from the database and enforces the required
// one.
type Guard struct {
	svc *Service
}

// NewGuard builds a Guard over the RBAC service.
func NewGuard(svc *Service) *Guard { return &Guard{svc: svc} }

// Require returns middleware that 403s unless the caller holds perm. The
// resolved permission set is stashed for Can().
func (g *Guard) Require(perm Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := auth.UserID(c)
		if !ok {
			web.Unauthorized(c, "A valid access token is required.")
			return
		}
		perms, err := g.svc.PermissionsFor(c.Request.Context(), uid)
		if err != nil {
			web.Internal(c)
			return
		}
		c.Set(ctxPermissionsKey, perms)
		if !containsPermission(perms, perm) {
			web.Forbidden(c, "You do not have permission to perform this action.")
			return
		}
		c.Next()
	}
}

// Can reports whether the current request holds perm, using the set resolved by
// Require. Returns false when no Require middleware ran on this route.
func Can(c *gin.Context, perm Permission) bool {
	v, ok := c.Get(ctxPermissionsKey)
	if !ok {
		return false
	}
	perms, ok := v.([]Permission)
	return ok && containsPermission(perms, perm)
}
