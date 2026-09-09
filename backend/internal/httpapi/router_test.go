package httpapi

import (
	"log/slog"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
)

// TestNewRouter_MountsEveryRouteWithoutConflict assembles the whole route tree
// the way cmd/api does. gin panics at registration time on a wildcard conflict
// (two different param names in the same position, e.g. /admin/bookings/:id vs
// /admin/bookings/:slug), so this is the cheap guard that adding a module's
// routes has not broken the tree. The handlers are nil: nothing is served here,
// only registered (the auth handler is real because its RegisterRoutes reads the
// token manager off it).
func TestNewRouter_MountsEveryRouteWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tm := auth.NewTokenManager("router-test", time.Minute)
	r := NewRouter(Deps{
		Config:         &config.Config{Env: "test", AllowedOrigins: []string{"http://localhost:3000"}},
		Logger:         slog.Default(),
		AuthHandler:    auth.NewHandler(nil, tm, auth.CookieConfig{Name: "ftr_session", Path: "/v1/auth"}, slog.Default()),
		AuthMiddleware: auth.RequireAuth(tm),
	})

	want := []string{
		"GET /v1/bookings/:id/disputes",
		"POST /v1/bookings/:id/disputes",
		"GET /v1/admin/bookings",
		"GET /v1/admin/bookings/:id",
		"POST /v1/admin/bookings/:id/force-cancel",
		"GET /v1/admin/disputes",
		"POST /v1/admin/disputes/:id/resolve",
	}
	got := map[string]bool{}
	for _, ri := range r.Routes() {
		got[ri.Method+" "+ri.Path] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("route not mounted: %s", w)
		}
	}
}
