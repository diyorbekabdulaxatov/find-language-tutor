package admin

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeRepo is an in-memory admin.Repository for service + handler tests.
type fakeRepo struct {
	metrics    Metrics
	users      []UserRow
	userDetail map[uuid.UUID]UserDetail
	teachers   []TeacherRow
	moderation map[string]Moderation

	statusCalls []statusCall
	verifyCalls []verifyCall
	lastLimit   int
	lastOffset  int
}

type statusCall struct{ slug, status, note string }
type verifyCall struct {
	slug     string
	verified bool
}

func (f *fakeRepo) Metrics(context.Context) (Metrics, error) { return f.metrics, nil }

func (f *fakeRepo) ListUsers(_ context.Context, q string, limit, offset int) ([]UserRow, int, error) {
	f.lastLimit, f.lastOffset = limit, offset
	var m []UserRow
	for _, u := range f.users {
		if q == "" || containsFold(u.Email, q) || containsFold(u.DisplayName, q) {
			m = append(m, u)
		}
	}
	return paginate(m, offset, limit), len(m), nil
}

func (f *fakeRepo) GetUserDetail(_ context.Context, id uuid.UUID) (UserDetail, error) {
	d, ok := f.userDetail[id]
	if !ok {
		return UserDetail{}, ErrUserNotFound
	}
	return d, nil
}

func (f *fakeRepo) ListTeachers(_ context.Context, status, q string, limit, offset int) ([]TeacherRow, int, error) {
	var m []TeacherRow
	for _, tr := range f.teachers {
		if status != "" && tr.Status != status {
			continue
		}
		if q == "" || containsFold(tr.DisplayName, q) || containsFold(tr.Slug, q) || containsFold(tr.Owner.Email, q) {
			m = append(m, tr)
		}
	}
	return paginate(m, offset, limit), len(m), nil
}

func (f *fakeRepo) GetModeration(_ context.Context, slug string) (Moderation, error) {
	m, ok := f.moderation[slug]
	if !ok {
		return Moderation{}, ErrTeacherNotFound
	}
	return m, nil
}

func (f *fakeRepo) SetTeacherStatus(_ context.Context, slug, status, note string) error {
	f.statusCalls = append(f.statusCalls, statusCall{slug, status, note})
	m := f.moderation[slug]
	m.Status, m.ModerationNote = status, note
	f.moderation[slug] = m
	return nil
}

func (f *fakeRepo) SetTeacherVerified(_ context.Context, slug string, v bool) error {
	f.verifyCalls = append(f.verifyCalls, verifyCall{slug, v})
	m := f.moderation[slug]
	m.Verified = v
	f.moderation[slug] = m
	return nil
}

func paginate[T any](s []T, offset, limit int) []T {
	if offset > len(s) {
		offset = len(s)
	}
	end := offset + limit
	if end > len(s) {
		end = len(s)
	}
	return s[offset:end]
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// fakeProfiles is a fake TeacherProfiles port.
type fakeProfiles struct{ bySlug map[string]*teachers.Teacher }

func (f fakeProfiles) GetForAdmin(_ context.Context, slug string) (*teachers.Teacher, error) {
	t, ok := f.bySlug[slug]
	if !ok {
		return nil, teachers.ErrNotFound
	}
	return t, nil
}

// newRouter wires the admin handler behind auth.RequireAuth + a real rbac.Guard.
// The returned bearer holds every admin permission; use scopedBearer for a
// narrower token.
func newRouter(repo Repository, profiles TeacherProfiles) (*gin.Engine, *rbacHarness, string) {
	gin.SetMode(gin.TestMode)
	h := &rbacHarness{
		tm:    auth.NewTokenManager("admin-test", time.Minute),
		rbacR: newFakeRBACRepo(),
	}
	guard := rbac.NewGuard(rbac.NewService(h.rbacR))
	handler := NewHandler(NewService(repo, profiles), discardLogger())
	r := gin.New()
	RegisterRoutes(r.Group("/v1/admin", auth.RequireAuth(h.tm)), handler, guard)

	full := h.bearerWith(rbac.AllPermissions...)
	return r, h, full
}

type rbacHarness struct {
	tm    *auth.TokenManager
	rbacR *fakeRBACRepo
}

func (h *rbacHarness) bearerWith(perms ...rbac.Permission) string {
	uid := uuid.New()
	h.rbacR.grant(uid, perms)
	tok, _ := h.tm.IssueAccess(auth.User{ID: uid, Email: "a@x", DisplayName: "A"}, time.Now())
	return "Bearer " + tok
}

// fakeRBACRepo is the minimal rbac.Repository the guard needs (permission
// resolution only).
type fakeRBACRepo struct {
	perms map[uuid.UUID][]string
}

func newFakeRBACRepo() *fakeRBACRepo { return &fakeRBACRepo{perms: map[uuid.UUID][]string{}} }

func (f *fakeRBACRepo) grant(uid uuid.UUID, perms []rbac.Permission) {
	for _, p := range perms {
		f.perms[uid] = append(f.perms[uid], string(p))
	}
}

func (f *fakeRBACRepo) PermissionKeysForUser(_ context.Context, uid uuid.UUID) ([]string, error) {
	return f.perms[uid], nil
}
func (f *fakeRBACRepo) RoleRefsForUser(context.Context, uuid.UUID) ([]rbac.RoleRef, error) {
	return nil, nil
}
func (f *fakeRBACRepo) ListRoles(context.Context) ([]rbac.Role, error) { return nil, nil }
func (f *fakeRBACRepo) GetRole(context.Context, uuid.UUID) (rbac.Role, error) {
	return rbac.Role{}, rbac.ErrRoleNotFound
}
func (f *fakeRBACRepo) CreateRole(context.Context, string, string, bool, []string) (rbac.Role, error) {
	return rbac.Role{}, nil
}
func (f *fakeRBACRepo) UpdateRole(context.Context, uuid.UUID, string, *[]string) (rbac.Role, error) {
	return rbac.Role{}, nil
}
func (f *fakeRBACRepo) DeleteRole(context.Context, uuid.UUID) error            { return nil }
func (f *fakeRBACRepo) UserExists(context.Context, uuid.UUID) (bool, error)    { return true, nil }
func (f *fakeRBACRepo) AssignRole(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeRBACRepo) UnassignRole(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

var _ rbac.Repository = (*fakeRBACRepo)(nil)

func req(r http.Handler, method, path, bearer, body string) *httptest.ResponseRecorder {
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
