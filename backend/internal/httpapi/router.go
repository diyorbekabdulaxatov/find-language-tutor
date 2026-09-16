package httpapi

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/admin"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/availability"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/bookings"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/courses"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/disputes"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/files"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/payments"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/payouts"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/ratelimit"
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
	OptionalAuth        gin.HandlerFunc // auth.OptionalAuth(tokenManager)
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
	CourseHandler       *courses.Handler
	// RateLimiter backs the per-route throttles; nil falls back to an
	// in-process limiter (tests, Redis-less dev).
	RateLimiter ratelimit.Limiter
}

// maxJSONBodyBytes caps every non-upload request body. The largest legitimate
// JSON body is a quiz resource's content (a few dozen questions) — well under
// 1 MiB.
const maxJSONBodyBytes = 1 << 20

// NewRouter builds the gin engine with middleware, the health check, and every
// module's routes mounted under /v1.
func NewRouter(d Deps) *gin.Engine {
	if d.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Trust no proxy unless told to: gin's default believes any
	// X-Forwarded-For, which would let a client pick its own IP for the
	// per-IP limits below.
	_ = r.SetTrustedProxies(d.Config.TrustedProxies)

	r.Use(
		RequestID(),
		StructuredLogger(d.Logger),
		Recovery(d.Logger),
		SecurityHeaders(d.Config.CookieSecure),
		CORS(d.Config.AllowedOrigins),
		Locale(),
		MaxBodyBytes(maxJSONBodyBytes, "/v1/uploads"),
	)

	r.GET("/healthz", healthHandler(d.Pool, d.Redis))

	limiter := d.RateLimiter
	if limiter == nil {
		limiter = ratelimit.NewMemory()
	}
	limit := func(rules ...Rule) gin.HandlerFunc { return RateLimit(limiter, d.Logger, rules...) }

	v1 := r.Group("/v1")

	authGroup := v1.Group("/auth", NoStore())
	auth.RegisterRoutes(authGroup, d.AuthHandler, auth.RouteLimits{
		// Login: a per-IP ceiling against spraying, plus a per-account one
		// against stuffing from many IPs. The account limit is deliberately
		// loose enough that locking someone out on purpose costs an attacker
		// a sustained effort, while 10 failures in 10 minutes is far more than
		// any real user needs.
		Login: limit(
			Rule{Name: "login_ip", Limit: 30, Window: time.Minute, Key: PerIP},
			Rule{Name: "login_email", Limit: 10, Window: 10 * time.Minute, Key: PerBodyField("email")},
		),
		Register: limit(Rule{Name: "register_ip", Limit: 20, Window: time.Hour, Key: PerIP}),
		// Refresh is called on every page load and shared across concurrent
		// 401s by the frontend client, so this is only a runaway guard.
		Refresh: limit(Rule{Name: "refresh_ip", Limit: 120, Window: time.Minute, Key: PerIP}),
		ForgotPassword: limit(
			Rule{Name: "forgot_ip", Limit: 10, Window: 15 * time.Minute, Key: PerIP},
			Rule{Name: "forgot_email", Limit: 3, Window: time.Hour, Key: PerBodyField("email")},
		),
		// Tokens are 256-bit so guessing is hopeless anyway; this just keeps
		// the argon2/DB cost of a flood bounded.
		ResetPassword:      limit(Rule{Name: "reset_ip", Limit: 10, Window: 15 * time.Minute, Key: PerIP}),
		VerifyEmail:        limit(Rule{Name: "verify_ip", Limit: 20, Window: 15 * time.Minute, Key: PerIP}),
		ResendVerification: limit(Rule{Name: "resend_user", Limit: 5, Window: time.Hour, Key: PerUser}),
	})

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

	files.RegisterRoutes(v1, d.FileHandler, d.AuthMiddleware,
		limit(Rule{Name: "upload_user", Limit: 60, Window: time.Hour, Key: PerUser}))
	resources.RegisterRoutes(v1.Group("/resources"), d.ResourceHandler, d.AuthMiddleware)
	resources.RegisterSubmissionRoutes(v1.Group("/submissions"), d.ResourceHandler, d.AuthMiddleware)

	courseRoutes := v1.Group("/courses")
	courses.RegisterRoutes(courseRoutes, d.CourseHandler, d.AuthMiddleware)
	// Phase C2: public catalog + cover image (no auth, or optional auth for
	// personalised catalog detail), and the purchase/player/progress routes
	// (auth required, enrollment/ownership checked in the service).
	courses.RegisterCatalogRoutes(courseRoutes, d.CourseHandler, d.OptionalAuth)
	courses.RegisterLearnerRoutes(courseRoutes, d.CourseHandler, d.AuthMiddleware)
	courses.RegisterEnrollmentRoutes(v1.Group("/enrollments"), d.CourseHandler, d.AuthMiddleware)

	// /v1/admin is behind a valid access token; each route then enforces its
	// own RBAC permission via the guard.
	adminGroup := v1.Group("/admin", d.AuthMiddleware)
	admin.RegisterRoutes(adminGroup, d.AdminHandler, d.RBACGuard)
	rbac.RegisterAdminRoutes(adminGroup, d.RBACHandler, d.RBACGuard)
	disputes.RegisterAdminRoutes(adminGroup, d.DisputeHandler, d.RBACGuard)
	payouts.RegisterAdminRoutes(adminGroup, d.PayoutHandler, d.RBACGuard)
	reviews.RegisterAdminRoutes(adminGroup, d.ReviewHandler, d.RBACGuard)
	courses.RegisterAdminRoutes(adminGroup, d.CourseHandler, d.RBACGuard)

	return r
}
