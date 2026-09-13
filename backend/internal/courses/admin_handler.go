package courses

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// RegisterAdminRoutes mounts the moderation surface onto the given group
// (expected to be "/v1/admin", already behind auth.RequireAuth). Every route
// carries the `courses.moderate` permission check, mirroring
// reviews.RegisterAdminRoutes.
func RegisterAdminRoutes(rg *gin.RouterGroup, h *Handler, guard *rbac.Guard) {
	rg.GET("/courses", guard.Require(rbac.PermCoursesModerate), h.AdminList)
	rg.POST("/courses/:id/suspend", guard.Require(rbac.PermCoursesModerate), h.AdminSuspend)
	rg.POST("/courses/:id/unsuspend", guard.Require(rbac.PermCoursesModerate), h.AdminUnsuspend)
}

// AdminList handles GET /v1/admin/courses?status&suspended&teacher_slug&q&page&page_size.
func (h *Handler) AdminList(c *gin.Context) {
	page, err := optionalInt(c.Query("page"))
	if err != nil {
		web.BadRequest(c, "`page` must be an integer.")
		return
	}
	pageSize, err := optionalInt(c.Query("page_size"))
	if err != nil {
		web.BadRequest(c, "`page_size` must be an integer.")
		return
	}

	res, err := h.svc.AdminModerate(c.Request.Context(), AdminCourseQuery{
		Status:      c.Query("status"),
		Suspended:   c.Query("suspended"),
		TeacherSlug: c.Query("teacher_slug"),
		Q:           c.Query("q"),
		Page:        page,
		PageSize:    pageSize,
	})
	if h.rendered(c, err, "list course moderation queue") {
		return
	}
	c.JSON(http.StatusOK, toAdminCourseListDTO(res))
}

// AdminSuspend handles POST /v1/admin/courses/:id/suspend.
func (h *Handler) AdminSuspend(c *gin.Context) { h.setSuspended(c, true) }

// AdminUnsuspend handles POST /v1/admin/courses/:id/unsuspend.
func (h *Handler) AdminUnsuspend(c *gin.Context) { h.setSuspended(c, false) }

func (h *Handler) setSuspended(c *gin.Context, suspended bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The course id must be a UUID.")
		return
	}
	var a AdminCourse
	if suspended {
		a, err = h.svc.AdminSuspend(c.Request.Context(), id)
	} else {
		a, err = h.svc.AdminUnsuspend(c.Request.Context(), id)
	}
	if h.rendered(c, err, "moderate course", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toAdminCourseDTO(a))
}
