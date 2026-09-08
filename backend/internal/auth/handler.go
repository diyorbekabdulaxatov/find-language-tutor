package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/web"
)

// CookieConfig controls the refresh-token cookie. The cookie is HttpOnly,
// SameSite=Lax, and scoped to Path so it is only ever sent to the auth
// endpoints.
type CookieConfig struct {
	Name   string        // e.g. "ftr_session"
	Path   string        // e.g. "/v1/auth"
	Domain string        // empty in dev
	Secure bool          // true outside development
	MaxAge time.Duration // = refresh-token TTL
}

// Handler adapts HTTP to the Service. It binds and validates input, calls the
// service, sets/clears the refresh cookie, and renders OpenAPI-shaped JSON —
// no business logic here, and the *gin.Context never leaves this file.
type Handler struct {
	svc    *Service
	tokens *TokenManager
	cookie CookieConfig
	logger *slog.Logger
}

func NewHandler(svc *Service, tokens *TokenManager, cookie CookieConfig, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, tokens: tokens, cookie: cookie, logger: logger}
}

// RegisterRoutes mounts the auth endpoints onto the given group (expected to be
// "/v1/auth"). /me is guarded by RequireAuth; the rest are public (refresh and
// logout authenticate via the cookie).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
	rg.POST("/refresh", h.Refresh)
	rg.POST("/logout", h.Logout)
	rg.GET("/me", RequireAuth(h.tokens), h.Me)
	rg.PATCH("/me", RequireAuth(h.tokens), h.UpdateMe)
}

// Register handles POST /v1/auth/register.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"email", "password", "display_name"}.`)
		return
	}

	res, err := h.svc.Register(c.Request.Context(), req.Email, req.Password, req.DisplayName, c.Request.UserAgent())
	if h.renderAuthError(c, err, "register") {
		return
	}

	h.setRefreshCookie(c, res)
	c.JSON(http.StatusCreated, toAuthResponse(res))
}

// Login handles POST /v1/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"email", "password"}.`)
		return
	}

	res, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, c.Request.UserAgent())
	if h.renderAuthError(c, err, "login") {
		return
	}

	h.setRefreshCookie(c, res)
	c.JSON(http.StatusOK, toAuthResponse(res))
}

// Refresh handles POST /v1/auth/refresh — rotate the refresh token.
func (h *Handler) Refresh(c *gin.Context) {
	raw, _ := c.Cookie(h.cookie.Name)

	res, err := h.svc.Refresh(c.Request.Context(), raw, c.Request.UserAgent())
	if err != nil {
		// Any refresh failure clears the (now useless) cookie.
		h.clearRefreshCookie(c)
		h.renderAuthError(c, err, "refresh")
		return
	}

	h.setRefreshCookie(c, res)
	c.JSON(http.StatusOK, toAuthResponse(res))
}

// Logout handles POST /v1/auth/logout — revoke the current session. 204.
func (h *Handler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(h.cookie.Name)

	if err := h.svc.Logout(c.Request.Context(), raw); err != nil {
		h.logger.Error("logout", slog.Any("error", err))
		web.Internal(c)
		return
	}

	h.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

// Me handles GET /v1/auth/me — the authenticated user. RequireAuth has run.
func (h *Handler) Me(c *gin.Context) {
	uid, ok := UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	user, err := h.svc.CurrentUser(c.Request.Context(), uid)
	switch {
	case errors.Is(err, ErrUserNotFound):
		web.Unauthorized(c, "Account no longer exists.")
		return
	case err != nil:
		h.logger.Error("current user", slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toUserDTO(user))
}

// UpdateMe handles PATCH /v1/auth/me — edit the authenticated account.
// RequireAuth has run.
func (h *Handler) UpdateMe(c *gin.Context) {
	uid, ok := UserID(c)
	if !ok {
		web.Unauthorized(c, "A valid access token is required.")
		return
	}

	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		web.BadRequest(c, `Request body must be {"display_name": "..."}.`)
		return
	}

	user, err := h.svc.UpdateCurrentUser(c.Request.Context(), uid, req.DisplayName)
	var ve ValidationError
	switch {
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
		return
	case errors.Is(err, ErrUserNotFound):
		web.Unauthorized(c, "Account no longer exists.")
		return
	case err != nil:
		h.logger.Error("update current user", slog.Any("error", err))
		web.Internal(c)
		return
	}

	c.JSON(http.StatusOK, toUserDTO(user))
}

// renderAuthError maps a service error to its HTTP response. It returns true if
// it wrote a response (the caller should stop).
func (h *Handler) renderAuthError(c *gin.Context, err error, op string) bool {
	var ve ValidationError
	switch {
	case err == nil:
		return false
	case errors.As(err, &ve):
		web.BadRequest(c, ve.Error())
	case errors.Is(err, ErrEmailTaken):
		web.WriteError(c, http.StatusConflict, "email_taken", "That email is already registered.")
	case errors.Is(err, ErrInvalidCredentials):
		web.Unauthorized(c, "Invalid email or password.")
	case errors.Is(err, ErrInvalidRefreshToken):
		web.Unauthorized(c, "Your session has expired. Please sign in again.")
	default:
		h.logger.Error(op, slog.Any("error", err))
		web.Internal(c)
	}
	return true
}

func (h *Handler) setRefreshCookie(c *gin.Context, res AuthResult) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		h.cookie.Name,
		res.RefreshToken,
		int(h.cookie.MaxAge.Seconds()),
		h.cookie.Path,
		h.cookie.Domain,
		h.cookie.Secure,
		true, // HttpOnly
	)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookie.Name, "", -1, h.cookie.Path, h.cookie.Domain, h.cookie.Secure, true)
}
