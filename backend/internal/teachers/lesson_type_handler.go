package teachers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// Wire shapes for a teacher's 1-on-1 offerings.

type lessonPriceDTO struct {
	DurationMinutes int      `json:"duration_minutes"`
	Price           moneyDTO `json:"price"`
}

type lessonTypeDTO struct {
	ID          uuid.UUID        `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	IsTrial     bool             `json:"is_trial"`
	Archived    bool             `json:"archived"`
	Position    int              `json:"position"`
	From        moneyDTO         `json:"from"`
	Prices      []lessonPriceDTO `json:"prices"`
}

func toLessonTypeDTO(lt LessonType) lessonTypeDTO {
	prices := make([]lessonPriceDTO, len(lt.Prices))
	for i, p := range lt.Prices {
		prices[i] = lessonPriceDTO{DurationMinutes: p.DurationMinutes, Price: money(p.Price)}
	}
	return lessonTypeDTO{
		ID:          lt.ID,
		Title:       lt.Title,
		Description: lt.Description,
		IsTrial:     lt.IsTrial,
		Archived:    lt.Archived,
		Position:    lt.Position,
		From:        money(lt.From()),
		Prices:      prices,
	}
}

func toLessonTypeListDTO(types []LessonType) []lessonTypeDTO {
	out := make([]lessonTypeDTO, len(types))
	for i, lt := range types {
		out[i] = toLessonTypeDTO(lt)
	}
	return out
}

type lessonPriceBody struct {
	DurationMinutes int   `json:"duration_minutes"`
	PriceMinor      int64 `json:"price_minor"`
}

type lessonTypeBody struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	IsTrial     bool              `json:"is_trial"`
	Position    int               `json:"position"`
	Prices      []lessonPriceBody `json:"prices"`
}

func (b lessonTypeBody) toInput() LessonTypeInput {
	prices := make([]LessonPrice, len(b.Prices))
	for i, p := range b.Prices {
		prices[i] = LessonPrice{
			DurationMinutes: p.DurationMinutes,
			Price:           Money{AmountMinor: p.PriceMinor, Currency: CurrencyUZS},
		}
	}
	return LessonTypeInput{
		Title:       b.Title,
		Description: b.Description,
		IsTrial:     b.IsTrial,
		Position:    b.Position,
		Prices:      prices,
	}
}

// RegisterLessonTypeRoutes mounts the offering endpoints onto the teachers
// group. The public list hangs off a slug; every write is scoped to the
// caller's own profile, so no id of someone else's is ever addressable.
func RegisterLessonTypeRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.GET("/:slug/lesson-types", h.ListLessonTypes)
	rg.GET("/me/lesson-types", requireAuth, h.ListOwnLessonTypes)
	rg.POST("/me/lesson-types", requireAuth, h.CreateLessonType)
	rg.PATCH("/me/lesson-types/:id", requireAuth, h.UpdateLessonType)
	rg.POST("/me/lesson-types/:id/archive", requireAuth, h.ArchiveLessonType)
	rg.POST("/me/lesson-types/:id/restore", requireAuth, h.RestoreLessonType)
}

// ListLessonTypes handles GET /v1/teachers/{slug}/lesson-types.
func (h *Handler) ListLessonTypes(c *gin.Context) {
	types, err := h.svc.LessonTypes(c.Request.Context(), c.Param("slug"))
	switch {
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "Teacher not found.")
		return
	case err != nil:
		h.logger.Error("list lesson types", slog.Any("error", err))
		web.Internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"lesson_types": toLessonTypeListDTO(types)})
}

// ListOwnLessonTypes handles GET /v1/teachers/me/lesson-types.
func (h *Handler) ListOwnLessonTypes(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	types, err := h.svc.OwnLessonTypes(c.Request.Context(), uid)
	if h.renderLessonTypeErr(c, err, "list own lesson types") {
		return
	}
	c.JSON(http.StatusOK, gin.H{"lesson_types": toLessonTypeListDTO(types)})
}

// CreateLessonType handles POST /v1/teachers/me/lesson-types.
func (h *Handler) CreateLessonType(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	var body lessonTypeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		web.BadRequest(c, "Invalid request body.")
		return
	}
	lt, err := h.svc.CreateLessonType(c.Request.Context(), uid, body.toInput())
	if h.renderLessonTypeErr(c, err, "create lesson type") {
		return
	}
	c.JSON(http.StatusCreated, toLessonTypeDTO(lt))
}

// UpdateLessonType handles PATCH /v1/teachers/me/lesson-types/{id}.
func (h *Handler) UpdateLessonType(c *gin.Context) {
	uid, id, ok := h.lessonTypeTarget(c)
	if !ok {
		return
	}
	var body lessonTypeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		web.BadRequest(c, "Invalid request body.")
		return
	}
	lt, err := h.svc.UpdateLessonType(c.Request.Context(), uid, id, body.toInput())
	if h.renderLessonTypeErr(c, err, "update lesson type") {
		return
	}
	c.JSON(http.StatusOK, toLessonTypeDTO(lt))
}

// ArchiveLessonType handles POST /v1/teachers/me/lesson-types/{id}/archive.
func (h *Handler) ArchiveLessonType(c *gin.Context) { h.setArchived(c, true) }

// RestoreLessonType handles POST /v1/teachers/me/lesson-types/{id}/restore.
func (h *Handler) RestoreLessonType(c *gin.Context) { h.setArchived(c, false) }

func (h *Handler) setArchived(c *gin.Context, archived bool) {
	uid, id, ok := h.lessonTypeTarget(c)
	if !ok {
		return
	}
	lt, err := h.svc.SetLessonTypeArchived(c.Request.Context(), uid, id, archived)
	if h.renderLessonTypeErr(c, err, "archive lesson type") {
		return
	}
	c.JSON(http.StatusOK, toLessonTypeDTO(lt))
}

func (h *Handler) lessonTypeTarget(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.NotFound(c, "Lesson not found.")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, id, true
}

// renderLessonTypeErr maps the service's errors onto statuses. It returns true
// when it wrote a response.
func (h *Handler) renderLessonTypeErr(c *gin.Context, err error, op string) bool {
	var ve ValidationError
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound):
		web.NotFound(c, "You have not created a teacher profile yet.")
	case errors.Is(err, ErrLessonTypeNotFound):
		web.NotFound(c, "Lesson not found.")
	case errors.Is(err, ErrTrialExists):
		web.WriteError(c, http.StatusConflict, "trial_exists", "You already offer a trial lesson.")
	case errors.Is(err, ErrTooManyLessonTypes):
		web.BadRequest(c, "You can list at most 8 lessons.")
	case errors.As(err, &ve):
		web.BadRequestErr(c, ve)
	default:
		h.logger.Error(op, slog.Any("error", err))
		web.Internal(c)
	}
	return true
}
