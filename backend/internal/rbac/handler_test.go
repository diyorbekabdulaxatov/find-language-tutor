package rbac

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func adminRouter(repo Repository) (*gin.Engine, *auth.TokenManager) {
	gin.SetMode(gin.TestMode)
	tm := auth.NewTokenManager("rbac-handler-test", time.Minute)
	guard := NewGuard(NewService(repo))
	h := NewHandler(NewService(repo), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := gin.New()
	RegisterAdminRoutes(r.Group("/v1/admin", auth.RequireAuth(tm)), h, guard)
	return r, tm
}

// grantManager returns a bearer for a user who holds roles.manage +
// users.manage_roles.
func grantManager(repo *fakeRepo, tm *auth.TokenManager) string {
	role := repo.addRole("manager", false, PermRolesManage, PermUsersManageRoles)
	uid := uuid.New()
	repo.grant(uid, role.ID)
	tok, _ := tm.IssueAccess(auth.User{ID: uid, Email: "m@x", DisplayName: "M"}, time.Now())
	return "Bearer " + tok
}

func send(r http.Handler, method, path, bearer, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	rq := httptest.NewRequest(method, path, rdr)
	if bearer != "" {
		rq.Header.Set("Authorization", bearer)
	}
	if body != "" {
		rq.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, rq)
	return w
}

func TestHandler_RoleCRUD(t *testing.T) {
	repo := newFakeRepo()
	r, tm := adminRouter(repo)
	mgr := grantManager(repo, tm)

	// create
	w := send(r, http.MethodPost, "/v1/admin/roles", mgr, `{"name":"editors","description":"d","permissions":["teachers.view","teachers.moderate"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d (%s)", w.Code, w.Body.String())
	}
	var role roleDTO
	_ = json.Unmarshal(w.Body.Bytes(), &role)
	if role.Name != "editors" || len(role.Permissions) != 2 {
		t.Fatalf("created role wrong: %+v", role)
	}

	// unknown permission -> 400 unknown_permission
	w = send(r, http.MethodPost, "/v1/admin/roles", mgr, `{"name":"x","permissions":["ghost"]}`)
	if w.Code != http.StatusBadRequest || errBody(w) != "unknown_permission" {
		t.Fatalf("unknown perm: %d / %s", w.Code, w.Body.String())
	}

	// patch permissions
	w = send(r, http.MethodPatch, "/v1/admin/roles/"+role.ID, mgr, `{"permissions":["metrics.view"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d (%s)", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &role)
	if len(role.Permissions) != 1 || role.Permissions[0] != "metrics.view" {
		t.Fatalf("patched perms wrong: %+v", role.Permissions)
	}

	// delete
	w = send(r, http.MethodDelete, "/v1/admin/roles/"+role.ID, mgr, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", w.Code)
	}
}

func TestHandler_AssignUnassign(t *testing.T) {
	repo := newFakeRepo()
	r, tm := adminRouter(repo)
	mgr := grantManager(repo, tm)

	target := repo.addRole("support", false, PermMetricsView)
	user := uuid.New()
	repo.users[user] = true

	w := send(r, http.MethodPost, "/v1/admin/users/"+user.String()+"/roles", mgr, `{"role_id":"`+target.ID.String()+`"}`)
	if w.Code != http.StatusNoContent {
		t.Fatalf("assign status = %d (%s)", w.Code, w.Body.String())
	}
	// unknown user -> 404
	w = send(r, http.MethodPost, "/v1/admin/users/"+uuid.New().String()+"/roles", mgr, `{"role_id":"`+target.ID.String()+`"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("assign unknown user status = %d", w.Code)
	}
	w = send(r, http.MethodDelete, "/v1/admin/users/"+user.String()+"/roles/"+target.ID.String(), mgr, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("unassign status = %d", w.Code)
	}
}

func TestHandler_RolesEndpoint_RequiresRolesManage(t *testing.T) {
	repo := newFakeRepo()
	r, tm := adminRouter(repo)

	// user with only metrics.view
	role := repo.addRole("m", false, PermMetricsView)
	uid := uuid.New()
	repo.grant(uid, role.ID)
	tok, _ := tm.IssueAccess(auth.User{ID: uid, Email: "m@x", DisplayName: "M"}, time.Now())

	w := send(r, http.MethodGet, "/v1/admin/roles", "Bearer "+tok, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func errBody(w *httptest.ResponseRecorder) string {
	var b struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return b.Error.Code
}
