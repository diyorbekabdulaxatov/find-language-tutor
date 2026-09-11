package courses

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

// RegisterRoutes mounts the authoring library onto "/v1/courses". Every route
// needs a valid access token; the caller must own a teacher profile (checked
// in the service).
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

	g.POST("/:id/sections", h.AddSection)
	g.PATCH("/:id/sections/:sectionId", h.RenameSection)
	g.DELETE("/:id/sections/:sectionId", h.DeleteSection)
	g.PUT("/:id/sections/reorder", h.ReorderSections)

	g.POST("/:id/sections/:sectionId/items", h.AddItem)
	g.PATCH("/:id/sections/:sectionId/items/:itemId", h.RenameItem)
	g.DELETE("/:id/sections/:sectionId/items/:itemId", h.DeleteItem)
	g.PUT("/:id/sections/:sectionId/items/reorder", h.ReorderItems)
}

func (h *Handler) Create(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	var req createCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON.")
		return
	}
	d, err := h.svc.Create(c.Request.Context(), uid, req.Title, req.Subtitle, req.Description, req.PriceAmountMinor, req.PriceCurrency)
	if h.rendered(c, err, "create course") {
		return
	}
	c.JSON(http.StatusCreated, toCourseDetailDTO(d))
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
		Status:          Status(c.Query("status")),
		IncludeArchived: c.Query("archived") == "true",
		Page:            page,
		PageSize:        pageSize,
	})
	if h.rendered(c, err, "list courses") {
		return
	}
	c.JSON(http.StatusOK, toCourseListDTO(res))
}

func (h *Handler) Get(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	d, err := h.svc.Get(c.Request.Context(), uid, id)
	if h.rendered(c, err, "get course", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

func (h *Handler) Update(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req updateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, "Request body is not valid JSON.")
		return
	}
	coverAssetID, err := parseOptionalUUID(req.CoverAssetID)
	if err != nil {
		web.BadRequest(c, "`cover_asset_id` must be a UUID.")
		return
	}
	d, err := h.svc.Update(c.Request.Context(), uid, id, req.Title, req.Subtitle, req.Description, coverAssetID, req.PriceAmountMinor, req.PriceCurrency)
	if h.rendered(c, err, "update course", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

func (h *Handler) publish(published bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, id, ok := h.callerAndID(c)
		if !ok {
			return
		}
		d, err := h.svc.SetPublished(c.Request.Context(), uid, id, published)
		if h.rendered(c, err, "set course published", slog.String("course_id", id.String())) {
			return
		}
		c.JSON(http.StatusOK, toCourseDetailDTO(d))
	}
}

func (h *Handler) archive(archived bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, id, ok := h.callerAndID(c)
		if !ok {
			return
		}
		d, err := h.svc.SetArchived(c.Request.Context(), uid, id, archived)
		if h.rendered(c, err, "set course archived", slog.String("course_id", id.String())) {
			return
		}
		c.JSON(http.StatusOK, toCourseDetailDTO(d))
	}
}

func (h *Handler) Delete(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	if h.rendered(c, h.svc.Delete(c.Request.Context(), uid, id), "delete course", slog.String("course_id", id.String())) {
		return
	}
	c.Status(http.StatusNoContent)
}

// --- sections ---

func (h *Handler) AddSection(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req createSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"title"}.`)
		return
	}
	d, err := h.svc.AddSection(c.Request.Context(), uid, id, req.Title)
	if h.rendered(c, err, "add course section", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusCreated, toCourseDetailDTO(d))
}

func (h *Handler) RenameSection(c *gin.Context) {
	uid, id, sectionID, ok := h.callerAndSectionID(c)
	if !ok {
		return
	}
	var req updateSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"title"}.`)
		return
	}
	d, err := h.svc.RenameSection(c.Request.Context(), uid, id, sectionID, req.Title)
	if h.rendered(c, err, "rename course section", slog.String("section_id", sectionID.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

func (h *Handler) DeleteSection(c *gin.Context) {
	uid, id, sectionID, ok := h.callerAndSectionID(c)
	if !ok {
		return
	}
	d, err := h.svc.DeleteSection(c.Request.Context(), uid, id, sectionID)
	if h.rendered(c, err, "delete course section", slog.String("section_id", sectionID.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

func (h *Handler) ReorderSections(c *gin.Context) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return
	}
	var req reorderSectionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"section_ids": [uuid, ...]}.`)
		return
	}
	ids, err := parseUUIDs(req.SectionIDs)
	if err != nil {
		web.BadRequest(c, "`section_ids` must all be UUIDs.")
		return
	}
	d, err := h.svc.ReorderSections(c.Request.Context(), uid, id, ids)
	if h.rendered(c, err, "reorder course sections", slog.String("course_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

// --- items ---

func (h *Handler) AddItem(c *gin.Context) {
	uid, id, sectionID, ok := h.callerAndSectionID(c)
	if !ok {
		return
	}
	var req createItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"kind": "video"|"resource", "title"?, "video_asset_id"?, "resource_id"?}.`)
		return
	}
	videoAssetID, err := parseOptionalUUID(req.VideoAssetID)
	if err != nil {
		web.BadRequest(c, "`video_asset_id` must be a UUID.")
		return
	}
	resourceID, err := parseOptionalUUID(req.ResourceID)
	if err != nil {
		web.BadRequest(c, "`resource_id` must be a UUID.")
		return
	}
	d, err := h.svc.AddItem(c.Request.Context(), uid, id, sectionID, ItemKind(req.Kind), req.Title, videoAssetID, resourceID)
	if h.rendered(c, err, "add course item", slog.String("section_id", sectionID.String())) {
		return
	}
	c.JSON(http.StatusCreated, toCourseDetailDTO(d))
}

func (h *Handler) RenameItem(c *gin.Context) {
	uid, id, sectionID, itemID, ok := h.callerAndItemID(c)
	if !ok {
		return
	}
	var req updateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"title"}.`)
		return
	}
	d, err := h.svc.RenameItem(c.Request.Context(), uid, id, sectionID, itemID, req.Title)
	if h.rendered(c, err, "rename course item", slog.String("item_id", itemID.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

func (h *Handler) DeleteItem(c *gin.Context) {
	uid, id, sectionID, itemID, ok := h.callerAndItemID(c)
	if !ok {
		return
	}
	d, err := h.svc.DeleteItem(c.Request.Context(), uid, id, sectionID, itemID)
	if h.rendered(c, err, "delete course item", slog.String("item_id", itemID.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

func (h *Handler) ReorderItems(c *gin.Context) {
	uid, id, sectionID, ok := h.callerAndSectionID(c)
	if !ok {
		return
	}
	var req reorderItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"item_ids": [uuid, ...]}.`)
		return
	}
	ids, err := parseUUIDs(req.ItemIDs)
	if err != nil {
		web.BadRequest(c, "`item_ids` must all be UUIDs.")
		return
	}
	d, err := h.svc.ReorderItems(c.Request.Context(), uid, id, sectionID, ids)
	if h.rendered(c, err, "reorder course items", slog.String("section_id", sectionID.String())) {
		return
	}
	c.JSON(http.StatusOK, toCourseDetailDTO(d))
}

// --- param helpers ---

func (h *Handler) callerAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The course id must be a UUID.")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, id, true
}

func (h *Handler) callerAndSectionID(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	uid, id, ok := h.callerAndID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	sectionID, err := uuid.Parse(c.Param("sectionId"))
	if err != nil {
		web.BadRequest(c, "The section id must be a UUID.")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return uid, id, sectionID, true
}

func (h *Handler) callerAndItemID(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	uid, id, sectionID, ok := h.callerAndSectionID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		web.BadRequest(c, "The item id must be a UUID.")
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return uid, id, sectionID, itemID, true
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
			"Create a teacher profile before building courses.")
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "No course with that id.")
	case errors.Is(err, ErrForbidden):
		web.Forbidden(c, "This course belongs to another teacher.")
	case errors.Is(err, ErrInUse):
		web.WriteError(c, http.StatusConflict, "course_in_use",
			"This course has been published and can't be deleted. Archive it instead.")
	case errors.Is(err, ErrSectionNotFound):
		web.NotFound(c, "No section with that id on this course.")
	case errors.Is(err, ErrItemNotFound):
		web.NotFound(c, "No item with that id on this section.")
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
