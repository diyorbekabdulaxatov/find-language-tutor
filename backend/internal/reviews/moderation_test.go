package reviews

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
)

// --- minimal rbac fake: the guard only calls PermissionKeysForUser ---

type fakeRBACRepo struct{ perms map[uuid.UUID][]string }

func (f *fakeRBACRepo) grant(uid uuid.UUID, perms ...rbac.Permission) {
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
func (f *fakeRBACRepo) DeleteRole(context.Context, uuid.UUID) error              { return nil }
func (f *fakeRBACRepo) UserExists(context.Context, uuid.UUID) (bool, error)      { return true, nil }
func (f *fakeRBACRepo) AssignRole(context.Context, uuid.UUID, uuid.UUID) error   { return nil }
func (f *fakeRBACRepo) UnassignRole(context.Context, uuid.UUID, uuid.UUID) error { return nil }

var _ rbac.Repository = (*fakeRBACRepo)(nil)

// modHarness wires the review handler behind auth + a real rbac.Guard on
// /v1/admin, the same shape cmd/api builds.
type modHarness struct {
	router *gin.Engine
	tm     *auth.TokenManager
	rbacR  *fakeRBACRepo
	repo   *fakeRepo
}

func newModHarness(repo *fakeRepo) *modHarness {
	gin.SetMode(gin.TestMode)
	h := &modHarness{
		tm:    auth.NewTokenManager("reviews-mod-test", time.Minute),
		rbacR: &fakeRBACRepo{perms: map[uuid.UUID][]string{}},
		repo:  repo,
	}
	handler := NewHandler(NewService(repo, discardLogger()), discardLogger())
	r := gin.New()
	RegisterAdminRoutes(r.Group("/v1/admin", auth.RequireAuth(h.tm)), handler, rbac.NewGuard(rbac.NewService(h.rbacR)))
	h.router = r
	return h
}

func (h *modHarness) bearer(uid uuid.UUID, perms ...rbac.Permission) string {
	h.rbacR.grant(uid, perms...)
	tok, err := h.tm.IssueAccess(auth.User{ID: uid, Email: "u@x", DisplayName: "U"}, time.Now())
	if err != nil {
		panic(err)
	}
	return "Bearer " + tok
}

func (h *modHarness) do(method, path, bearer, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", bearer)
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// seedModReview stores a booking-tied review for teacher tid at the given rating.
func seedModReview(repo *fakeRepo, tid uuid.UUID, slug string, rating int) uuid.UUID {
	bid := uuid.New()
	id := uuid.New()
	repo.addExistingReview(AdminReview{
		ID:          id,
		TeacherSlug: slug,
		StudentName: "Aziz",
		BookingID:   &bid,
		Rating:      rating,
		Comment:     "review",
		CreatedAt:   fixedNow,
	}, tid)
	return id
}

func TestModeration_RequiresPermission(t *testing.T) {
	h := newModHarness(newFakeRepo())
	// a valid token but no reviews.moderate
	w := h.do(http.MethodGet, "/v1/admin/reviews", h.bearer(uuid.New()), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestModeration_ListAndFilter(t *testing.T) {
	repo := newFakeRepo()
	tid := uuid.New()
	seedModReview(repo, tid, "nodira-karimova", 5)
	lowID := seedModReview(repo, tid, "nodira-karimova", 2)
	seedModReview(repo, uuid.New(), "elena-kim", 4)

	h := newModHarness(repo)
	tok := h.bearer(uuid.New(), rbac.PermReviewsModerate)

	// all
	w := h.do(http.MethodGet, "/v1/admin/reviews", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", w.Code, w.Body.String())
	}
	var list adminReviewListDTO
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 3 || len(list.Reviews) != 3 {
		t.Fatalf("total=%d len=%d, want 3", list.Total, len(list.Reviews))
	}

	// teacher + max_rating filter narrows to the one 2-star for nodira
	w = h.do(http.MethodGet, "/v1/admin/reviews?teacher=nodira-karimova&max_rating=3", tok, "")
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 || list.Reviews[0].ID != lowID.String() {
		t.Fatalf("filtered wrong: %+v", list)
	}

	// bad visibility → 400
	if got := h.do(http.MethodGet, "/v1/admin/reviews?visibility=bogus", tok, "").Code; got != http.StatusBadRequest {
		t.Fatalf("bad visibility status = %d, want 400", got)
	}
}

func TestModeration_HideUnhide_MovesAggregate(t *testing.T) {
	repo := newFakeRepo()
	repo.rating, repo.reviewCount = 4.0, 2 // baseline: 2 historical reviews averaging 4.0
	tid := uuid.New()
	id := seedModReview(repo, tid, "nodira-karimova", 5)
	repo.recompute() // fold the one visible 5-star: (4*2 + 5)/3 = 4.333 -> 4.3, count 3

	if repo.rating != 4.3 || repo.reviewCount != 3 {
		t.Fatalf("setup: rating=%v count=%d, want 4.3 / 3", repo.rating, repo.reviewCount)
	}

	h := newModHarness(repo)
	tok := h.bearer(uuid.New(), rbac.PermReviewsModerate)

	// hide → back to the baseline
	w := h.do(http.MethodPost, "/v1/admin/reviews/"+id.String()+"/hide", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("hide status = %d body %s", w.Code, w.Body.String())
	}
	var dto adminReviewDTO
	_ = json.Unmarshal(w.Body.Bytes(), &dto)
	if !dto.Hidden {
		t.Errorf("response hidden = false, want true")
	}
	if repo.rating != 4.0 || repo.reviewCount != 2 {
		t.Errorf("after hide: rating=%v count=%d, want 4.0 / 2", repo.rating, repo.reviewCount)
	}

	// unhide → folded back in
	w = h.do(http.MethodPost, "/v1/admin/reviews/"+id.String()+"/unhide", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("unhide status = %d", w.Code)
	}
	if repo.rating != 4.3 || repo.reviewCount != 3 {
		t.Errorf("after unhide: rating=%v count=%d, want 4.3 / 3", repo.rating, repo.reviewCount)
	}
}

func TestModeration_Remove(t *testing.T) {
	repo := newFakeRepo()
	repo.rating, repo.reviewCount = 5.0, 1
	tid := uuid.New()
	id := seedModReview(repo, tid, "nodira-karimova", 1)
	repo.recompute() // (5*1 + 1)/2 = 3.0, count 2

	h := newModHarness(repo)
	tok := h.bearer(uuid.New(), rbac.PermReviewsModerate)

	w := h.do(http.MethodDelete, "/v1/admin/reviews/"+id.String(), tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body %s", w.Code, w.Body.String())
	}
	if repo.rating != 5.0 || repo.reviewCount != 1 {
		t.Errorf("after remove: rating=%v count=%d, want 5.0 / 1", repo.rating, repo.reviewCount)
	}
	// second delete → 404
	if got := h.do(http.MethodDelete, "/v1/admin/reviews/"+id.String(), tok, "").Code; got != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404", got)
	}
}
