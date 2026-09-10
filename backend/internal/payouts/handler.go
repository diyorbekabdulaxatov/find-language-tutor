package payouts

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
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

// RegisterAdminRoutes mounts the payout surface onto the given group (expected
// to be "/v1/admin", already behind auth.RequireAuth). Reads carry
// `payouts.view`; the run carries `payouts.run`.
func RegisterAdminRoutes(rg *gin.RouterGroup, h *Handler, guard *rbac.Guard) {
	rg.GET("/payouts", guard.Require(rbac.PermPayoutsView), h.Dashboard)
	rg.GET("/payouts/batches/:id", guard.Require(rbac.PermPayoutsView), h.Batch)
	rg.POST("/payouts/run", guard.Require(rbac.PermPayoutsRun), h.Run)
}

// Dashboard handles GET /v1/admin/payouts?page&page_size.
func (h *Handler) Dashboard(c *gin.Context) {
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

	d, err := h.svc.Dashboard(c.Request.Context(), page, pageSize)
	if h.rendered(c, err, "payout dashboard") {
		return
	}
	c.JSON(http.StatusOK, toDashboardDTO(d))
}

// Batch handles GET /v1/admin/payouts/batches/:id.
func (h *Handler) Batch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The batch id must be a UUID.")
		return
	}

	d, err := h.svc.Batch(c.Request.Context(), id)
	if h.rendered(c, err, "get payout batch", slog.String("batch_id", id.String())) {
		return
	}
	c.JSON(http.StatusOK, toBatchDetailDTO(d))
}

// Run handles POST /v1/admin/payouts/run. No request body.
func (h *Handler) Run(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	d, err := h.svc.Run(c.Request.Context(), uid)
	if h.rendered(c, err, "run payout batch") {
		return
	}
	c.JSON(http.StatusCreated, toBatchDetailDTO(d))
}

// rendered maps a service error to an HTTP response. Returns true when it wrote
// one (the caller should stop).
func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}

	switch {
	case errors.Is(err, ErrBatchNotFound):
		web.WriteError(c, http.StatusNotFound, "batch_not_found", "No payout batch with that id.")
	case errors.Is(err, ErrNothingToPay):
		web.WriteError(c, http.StatusConflict, "nothing_to_pay",
			"No earnings have cleared their holding period, so there is nothing to pay out.")
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
