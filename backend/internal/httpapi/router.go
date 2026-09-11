package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/admin"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/availability"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/disputes"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/files"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/payments"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/payouts"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/resources"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/reviews"
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
	AdminHandler        *admin.Handler
	RBACHandler         *rbac.Handler
	RBACGuard           *rbac.Guard
	TeacherHandler      *teachers.Handler
	AvailabilityHandler *availability.Handler
	BookingHandler      *bookings.Handler
	PaymentHandler      *payments.Handler
	ReviewHandler       *reviews.Handler
	DisputeHandler      *disputes.Handler
	PayoutHandler       *payouts.Handler
	FileHandler         *files.Handler
	ResourceHandler     *resources.Handler
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
	reviews.RegisterTeacherRoutes(teacherRoutes, d.ReviewHandler)

	bookingRoutes := v1.Group("/bookings")
	bookings.RegisterRoutes(bookingRoutes, d.BookingHandler, d.AuthMiddleware)
	reviews.RegisterBookingRoutes(bookingRoutes, d.ReviewHandler, d.AuthMiddleware)
	disputes.RegisterBookingRoutes(bookingRoutes, d.DisputeHandler, d.AuthMiddleware)
	resources.RegisterBookingRoutes(bookingRoutes, d.ResourceHandler, d.AuthMiddleware)

	payments.RegisterRoutes(v1.Group("/payments"), d.PaymentHandler, d.AuthMiddleware)

	files.RegisterRoutes(v1, d.FileHandler, d.AuthMiddleware)
	resources.RegisterRoutes(v1.Group("/resources"), d.ResourceHandler, d.AuthMiddleware)
	resources.RegisterSubmissionRoutes(v1.Group("/submissions"), d.ResourceHandler, d.AuthMiddleware)

	// /v1/admin is behind a valid access token; each route then enforces its
	// own RBAC permission via the guard.
	adminGroup := v1.Group("/admin", d.AuthMiddleware)
	admin.RegisterRoutes(adminGroup, d.AdminHandler, d.RBACGuard)
	rbac.RegisterAdminRoutes(adminGroup, d.RBACHandler, d.RBACGuard)
	disputes.RegisterAdminRoutes(adminGroup, d.DisputeHandler, d.RBACGuard)
	payouts.RegisterAdminRoutes(adminGroup, d.PayoutHandler, d.RBACGuard)
	reviews.RegisterAdminRoutes(adminGroup, d.ReviewHandler, d.RBACGuard)

	return r
}
