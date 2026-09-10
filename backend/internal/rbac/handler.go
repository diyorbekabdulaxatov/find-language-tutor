package rbac

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Handler adapts HTTP to the Service: bind + validate, call the service, render
// the OpenAPI-shaped JSON. No business logic; the *gin.Context never leaves
// this file.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{svc: svc, logger: logger}
}

// RegisterAdminRoutes mounts the role-management endpoints onto the given group
// (expected to be "/v1/admin", already behind auth.RequireAuth). Each route adds
// its own permission check via the guard.
func RegisterAdminRoutes(rg *gin.RouterGroup, h *Handler, guard *Guard) {
	rg.GET("/permissions", guard.Require(PermRolesManage), h.Catalog)
	rg.GET("/roles", guard.Require(PermRolesManage), h.ListRoles)
	rg.POST("/roles", guard.Require(PermRolesManage), h.CreateRole)
	rg.PATCH("/roles/:id", guard.Require(PermRolesManage), h.UpdateRole)
	rg.DELETE("/roles/:id", guard.Require(PermRolesManage), h.DeleteRole)
	rg.POST("/users/:id/roles", guard.Require(PermUsersManageRoles), h.AssignRole)
	rg.DELETE("/users/:id/roles/:role_id", guard.Require(PermUsersManageRoles), h.UnassignRole)
}

// Catalog handles GET /v1/admin/permissions.
func (h *Handler) Catalog(c *gin.Context) {
	c.JSON(http.StatusOK, toCatalogDTO(h.svc.Catalog()))
}

// ListRoles handles GET /v1/admin/roles.
func (h *Handler) ListRoles(c *gin.Context) {
	roles, err := h.svc.ListRoles(c.Request.Context())
	if h.rendered(c, err, "list roles") {
		return
	}
	c.JSON(http.StatusOK, toRoleListDTO(roles))
}

// CreateRole handles POST /v1/admin/roles.
func (h *Handler) CreateRole(c *gin.Context) {
	var req createRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"name", "description"?, "permissions": []}.`)
		return
	}
	role, err := h.svc.CreateRole(c.Request.Context(), req.Name, req.Description, req.Permissions)
	if h.rendered(c, err, "create role") {
		return
	}
	c.JSON(http.StatusCreated, toRoleDTO(role))
}

// UpdateRole handles PATCH /v1/admin/roles/{id}.
func (h *Handler) UpdateRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The role id must be a UUID.")
		return
	}
	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"description"?, "permissions"?: []}.`)
		return
	}
	role, err := h.svc.UpdateRole(c.Request.Context(), id, req.Description, req.Permissions)
	if h.rendered(c, err, "update role", slog.String("role_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toRoleDTO(role))
}

// DeleteRole handles DELETE /v1/admin/roles/{id}.
func (h *Handler) DeleteRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The role id must be a UUID.")
		return
	}
	if h.rendered(c, h.svc.DeleteRole(c.Request.Context(), id), "delete role", slog.String("role_id", id.String())) {
		return
	}
	c.Status(http.StatusNoContent)
}

// AssignRole handles POST /v1/admin/users/{id}/roles — body {"role_id"}.
func (h *Handler) AssignRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The user id must be a UUID.")
		return
	}
	var req assignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"role_id": "<uuid>"}.`)
		return
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		web.BadRequest(c, "`role_id` must be a UUID.")
		return
	}
	if h.rendered(c, h.svc.AssignRole(c.Request.Context(), userID, roleID), "assign role") {
		return
	}
	c.Status(http.StatusNoContent)
}

// UnassignRole handles DELETE /v1/admin/users/{id}/roles/{role_id}.
func (h *Handler) UnassignRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The user id must be a UUID.")
		return
	}
	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		web.BadRequest(c, "The role id must be a UUID.")
		return
	}
	if h.rendered(c, h.svc.UnassignRole(c.Request.Context(), userID, roleID), "unassign role") {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}
	var ve ValidationError
	switch {
	case errors.Is(err, ErrRoleNotFound):
		web.NotFound(c, "No role with that id.")
	case errors.Is(err, ErrUserNotFound):
		web.NotFound(c, "No user with that id.")
	case errors.Is(err, ErrRoleExists):
		web.WriteError(c, http.StatusConflict, "role_exists", "A role with that name already exists.")
	case errors.Is(err, ErrRoleLocked):
		web.WriteError(c, http.StatusForbidden, "role_locked", "This is a system role and cannot be modified or deleted.")
	case errors.Is(err, ErrRoleInUse):
		web.WriteError(c, http.StatusConflict, "role_in_use", "This role is still assigned to one or more users. Unassign it first.")
	case errors.As(err, &ve):
		web.WriteError(c, http.StatusBadRequest, ve.Code, ve.Error())
	default:
		anys := make([]any, 0, len(attrs)+1)
		for _, a := range attrs {
			anys = append(anys, a)
		}
		anys = append(anys, slog.Any("error", err))
		h.logger.Error(op, anys...)
		web.Internal(c)
	}
	return true
}
