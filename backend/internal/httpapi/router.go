package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/availability"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// Deps is everything the HTTP layer needs, wired up in cmd/api.
type Deps struct {
	Config              *config.Config
	Logger              *slog.Logger
	Pool                *pgxpool.Pool
	Redis               *redis.Client
	AuthHandler         *auth.Handler
	AuthMiddleware      gin.HandlerFunc // auth.RequireAuth(tokenManager)
	TeacherHandler      *teachers.Handler
	AvailabilityHandler *availability.Handler
	BookingHandler      *bookings.Handler
}

// NewRouter builds the gin engine with middleware, the health check, and every
// module's routes mounted under /v1.
func NewRouter(d Deps) *gin.Engine {
	if d.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		RequestID(),
		StructuredLogger(d.Logger),
		Recovery(d.Logger),
		CORS(d.Config.AllowedOrigins),
	)

	r.GET("/healthz", healthHandler(d.Pool, d.Redis))

	v1 := r.Group("/v1")

	auth.RegisterRoutes(v1.Group("/auth"), d.AuthHandler)

	teacherRoutes := v1.Group("/teachers")
	teachers.RegisterRoutes(teacherRoutes, d.TeacherHandler, d.AuthMiddleware)
	availability.RegisterRoutes(teacherRoutes, d.AvailabilityHandler, d.AuthMiddleware)
	bookings.RegisterTeacherSlotRoute(teacherRoutes, d.BookingHandler)

	bookings.RegisterRoutes(v1.Group("/bookings"), d.BookingHandler, d.AuthMiddleware)

	return r
}
