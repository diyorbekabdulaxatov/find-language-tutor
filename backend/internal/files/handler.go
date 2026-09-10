package files

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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

// RegisterRoutes mounts POST /v1/uploads and GET /v1/files/:id. Both require a
// valid access token.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, requireAuth gin.HandlerFunc) {
	rg.POST("/uploads", requireAuth, h.Upload)
	rg.GET("/files/:id", requireAuth, h.Download)
}

type uploadResponse struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	URL         string `json:"url"` // GET path for the bytes
}

// Upload handles POST /v1/uploads — multipart form, field "file".
func (h *Handler) Upload(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	// Cap what gin buffers/parses.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadBytes+(1<<20))

	fh, err := c.FormFile("file")
	if err != nil {
		web.BadRequest(c, `Send the file as multipart form field "file".`)
		return
	}
	f, err := fh.Open()
	if err != nil {
		h.logger.Error("open upload", slog.Any("error", err))
		web.Internal(c)
		return
	}
	defer f.Close()

	ct := fh.Header.Get("Content-Type")
	asset, err := h.svc.Upload(c.Request.Context(), uid, fh.Filename, ct, fh.Size, f)
	if h.rendered(c, err, "upload file") {
		return
	}

	c.JSON(http.StatusCreated, uploadResponse{
		ID:          asset.ID.String(),
		Filename:    asset.Filename,
		ContentType: asset.ContentType,
		Bytes:       asset.Bytes,
		URL:         "/v1/files/" + asset.ID.String(),
	})
}

// Download handles GET /v1/files/:id.
func (h *Handler) Download(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		web.BadRequest(c, "The file id must be a UUID.")
		return
	}

	redirectURL, body, asset, err := h.svc.Download(c.Request.Context(), id, uid)
	if h.rendered(c, err, "download file", slog.String("file_id", id.String())) {
		return
	}
	if redirectURL != "" {
		c.Redirect(http.StatusFound, redirectURL)
		return
	}
	defer body.Close()

	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", asset.Filename))
	c.DataFromReader(http.StatusOK, asset.Bytes, asset.ContentType, body, nil)
}

func (h *Handler) rendered(c *gin.Context, err error, op string, attrs ...slog.Attr) bool {
	if err == nil {
		return false
	}
	var ve ValidationError
	switch {
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
	case errors.Is(err, ErrUnsupportedType):
		web.WriteError(c, http.StatusUnsupportedMediaType, "unsupported_type",
			"That file type isn't allowed. Use a PDF, image, or audio file.")
	case errors.Is(err, ErrTooLarge):
		web.WriteError(c, http.StatusRequestEntityTooLarge, "file_too_large",
			fmt.Sprintf("Files must be under %d MB.", MaxUploadBytes>>20))
	case errors.Is(err, ErrEmptyUpload):
		web.BadRequest(c, "The file is empty.")
	case errors.Is(err, ErrAssetNotFound):
		web.NotFound(c, "No file with that id.")
	case errors.Is(err, ErrForbidden):
		web.Forbidden(c, "You don't have access to this file.")
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
