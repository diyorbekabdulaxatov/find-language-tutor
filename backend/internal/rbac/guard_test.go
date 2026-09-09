package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func guardRouter(repo Repository) (*gin.Engine, *auth.TokenManager) {
	gin.SetMode(gin.TestMode)
	tm := auth.NewTokenManager("guard-test", time.Minute)
	guard := NewGuard(NewService(repo))
	r := gin.New()
	g := r.Group("/v1/admin", auth.RequireAuth(tm))
	g.GET("/metrics", guard.Require(PermMetricsView), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"can_moderate": Can(c, PermTeachersModerate)})
	})
	return r, tm
}

func bearer(tm *auth.TokenManager, id uuid.UUID) string {
	tok, _ := tm.IssueAccess(auth.User{ID: id, Email: "x@x", DisplayName: "X"}, time.Now())
	return "Bearer " + tok
}

func TestGuard_NoToken_401(t *testing.T) {
	r, _ := guardRouter(newFakeRepo())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/admin/metrics", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestGuard_MissingPermission_403(t *testing.T) {
	repo := newFakeRepo()
	role := repo.addRole("nothing", false) // no permissions
	user := uuid.New()
	repo.grant(user, role.ID)

	r, tm := guardRouter(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/metrics", nil)
	req.Header.Set("Authorization", bearer(tm, user))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", w.Code, w.Body.String())
	}
}

func TestGuard_HasPermission_PassesThrough_AndCan(t *testing.T) {
	repo := newFakeRepo()
	role := repo.addRole("viewer", false, PermMetricsView, PermTeachersModerate)
	user := uuid.New()
	repo.grant(user, role.ID)

	r, tm := guardRouter(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/metrics", nil)
	req.Header.Set("Authorization", bearer(tm, user))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if want := `"can_moderate":true`; !contains(w.Body.String(), want) {
		t.Fatalf("Can() not reflected: %s", w.Body.String())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
