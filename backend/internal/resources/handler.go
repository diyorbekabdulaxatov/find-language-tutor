package resources

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Handler adapts HTTP to the Service. The *gin.Context never leaves this file.
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

// RegisterRoutes mounts the library onto "/v1/resources". Every route needs a
// valid access token; the caller must own a teacher profile (checked in the
// service).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	g := rg.Group("", requireAuth)
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Update)
	g.POST("/:id/publish", h.publish(true))
	g.POST("/:id/unpublish", h.publish(false))
	g.POST("/:id/archive", h.archive(true))
	g.POST("/:id/unarchive", h.archive(false))
	g.DELETE("/:id", h.Delete)
}

func (h *Handler) Create(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON.")
		return
	}
	r, err := h.svc.Create(c.Request.Context(), uid, Type(req.Type), req.Title, req.Instructions, req.Content, req.Publish)
	if h.rendered(c, err, "create resource") {
		return
	}
	c.JSON(http.StatusCreated, toResourceDTO(r))
}

func (h *Handler) List(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
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

	res, err := h.svc.Library(c.Request.Context(), uid, ListQuery{
		Type:            Type(c.Query("type")),
		Status:          Status(c.Query("status")),
		IncludeArchived: c.Query("archived") == "true",
		Page:            page,
		PageSize:        pageSize,
	})
	if h.rendered(c, err, "list resources") {
		return
	}
	c.JSON(http.StatusOK, toResourceListDTO(res))
}

func (h *Handler) Get(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	r, err := h.svc.Get(c.Request.Context(), uid, id)
	if h.rendered(c, err, "get resource", slog.String("resource_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toResourceDTO(r))
}

func (h *Handler) Update(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON.")
		return
	}
	r, err := h.svc.Update(c.Request.Context(), uid, id, req.Title, req.Instructions, req.Content)
	if h.rendered(c, err, "update resource", slog.String("resource_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toResourceDTO(r))
}

func (h *Handler) publish(published bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, id, ok := h.callerAndID(c)
		if !ok {
			return
		}
		r, err := h.svc.SetPublished(c.Request.Context(), uid, id, published)
		if h.rendered(c, err, "set resource published", slog.String("resource_id", id.String())) {
			return
		}
		c.JSON(http.StatusOK, toResourceDTO(r))
	}
}

func (h *Handler) archive(archived bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, id, ok := h.callerAndID(c)
		if !ok {
			return
		}
		r, err := h.svc.SetArchived(c.Request.Context(), uid, id, archived)
		if h.rendered(c, err, "set resource archived", slog.String("resource_id", id.String())) {
			return
		}
		c.JSON(http.StatusOK, toResourceDTO(r))
	}
}

func (h *Handler) Delete(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	if h.rendered(c, h.svc.Delete(c.Request.Context(), uid, id), "delete resource", slog.String("resource_id", id.String())) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) callerAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The resource id must be a UUID.")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, id, true
}

func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}
	var ve ValidationError
	switch {
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
	case errors.Is(err, ErrNoTeacher):
		web.WriteError(c, http.StatusForbidden, "no_teacher_profile",
			"Create a teacher profile before building resources.")
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "No resource with that id.")
	case errors.Is(err, ErrForbidden):
		web.Forbidden(c, "This resource belongs to another teacher.")
	case errors.Is(err, ErrInUse):
		web.WriteError(c, http.StatusConflict, "resource_in_use",
			"This resource is attached to a lesson or course. Archive it instead.")
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

func optionalInt(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	return strconv.Atoi(v)
}
