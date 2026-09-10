package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
}

func errCode(w *httptest.ResponseRecorder) string {
	var b struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return b.Error.Code
}

// --- phase A: dashboard ---

func TestMetrics_ShapeAndPermission(t *testing.T) {
	repo := &fakeRepo{metrics: Metrics{
		Currency:   "UZS",
		UsersTotal: 12, UsersThisWeek: 3, TeachersTotal: 11, TeachersPending: 1,
		TeachersApproved: 9, TeachersVerified: 4, ActiveStudents: 7,
		BookingsTotal: 4, BookingsThisWeek: 2, BookingsUpcoming: 1,
		BookingsCompleted: 2, BookingsCancelled: 1,
		GMVMinor: 18_000_000, CapturedMinor: 9_000_000, RefundedMinor: 1_000_000,
		PayoutsOwedMinor: 5_000_000, PayoutsPaidMinor: 3_000_000,
		ReviewsVisible: 45, AverageRating: 4.8, DisputesOpen: 2,
	}}
	r, h, full := newRouter(repo, fakeProfiles{})

	w := req(r, http.MethodGet, "/v1/admin/metrics", full, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var m metricsDTO
	decode(t, w, &m)
	if m.UsersTotal != 12 || m.TeachersPending != 1 || m.GMVMinor != 18_000_000 || m.Currency != "UZS" {
		t.Fatalf("metrics mapped wrong: %+v", m)
	}
	if m.CapturedMinor != 9_000_000 || m.PayoutsOwedMinor != 5_000_000 ||
		m.AverageRating != 4.8 || m.DisputesOpen != 2 || m.BookingsUpcoming != 1 {
		t.Fatalf("expanded metrics mapped wrong: %+v", m)
	}

	// a token without metrics.view is 403
	w = req(r, http.MethodGet, "/v1/admin/metrics", h.bearerWith(rbac.PermUsersView), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("scoped token status = %d, want 403", w.Code)
	}
	// no token is 401
	w = req(r, http.MethodGet, "/v1/admin/metrics", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token status = %d, want 401", w.Code)
	}
}

func TestListUsers_PaginationAndQ(t *testing.T) {
	repo := &fakeRepo{users: []UserRow{
		{ID: uuid.New(), Email: "ada@example.com", DisplayName: "Ada"},
		{ID: uuid.New(), Email: "bob@example.com", DisplayName: "Bob", IsTeacher: true, BookingCount: 3},
		{ID: uuid.New(), Email: "carol@example.com", DisplayName: "Carol Ada"},
	}}
	r, _, full := newRouter(repo, fakeProfiles{})

	var page usersPageDTO
	decode(t, req(r, http.MethodGet, "/v1/admin/users?q=ADA", full, ""), &page)
	if page.Total != 2 || len(page.Users) != 2 {
		t.Fatalf("q filter: total=%d len=%d", page.Total, len(page.Users))
	}

	decode(t, req(r, http.MethodGet, "/v1/admin/users?page=2&page_size=2", full, ""), &page)
	if page.Total != 3 || len(page.Users) != 1 {
		t.Fatalf("page 2: total=%d len=%d", page.Total, len(page.Users))
	}
	if repo.lastLimit != 2 || repo.lastOffset != 2 {
		t.Fatalf("limit/offset: %d/%d", repo.lastLimit, repo.lastOffset)
	}
}

func TestGetUser_ShapeAndNotFound(t *testing.T) {
	id := uuid.New()
	start := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{userDetail: map[uuid.UUID]UserDetail{
		id: {
			User:           UserCore{ID: id, Email: "ada@example.com", DisplayName: "Ada"},
			Roles:          []RoleRef{{ID: uuid.New(), Name: "support"}},
			TeacherProfile: &UserTeacherProfile{Slug: "ada", Status: "approved", Verified: true},
			Bookings: []UserBookingRow{
				{ID: uuid.New(), Status: "confirmed", StartAt: start, RoleInBooking: BookingRoleStudent, OtherPartyName: "Nodira", PriceMinor: 9_000_000, Currency: "UZS"},
			},
			Payments: PaymentsSummary{AuthorizedMinor: 9_000_000, Currency: "UZS"},
		},
	}}
	r, _, full := newRouter(repo, fakeProfiles{})

	w := req(r, http.MethodGet, "/v1/admin/users/"+id.String(), full, "")
	var d userDetailDTO
	decode(t, w, &d)
	if d.User.Email != "ada@example.com" || len(d.Roles) != 1 || d.Roles[0].Name != "support" {
		t.Fatalf("detail mapped wrong: %+v", d)
	}
	if d.TeacherProfile == nil || len(d.Bookings) != 1 || d.Bookings[0].RoleInBooking != "student" {
		t.Fatalf("detail children wrong: %+v", d)
	}

	w = req(r, http.MethodGet, "/v1/admin/users/"+uuid.New().String(), full, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown user = %d, want 404", w.Code)
	}
}

// --- phase B: moderation ---

func modFixtures() (*fakeRepo, fakeProfiles) {
	repo := &fakeRepo{
		moderation: map[string]Moderation{
			"pending-t":  {Status: "pending", Owner: Owner{ID: uuid.New(), Email: "p@x", DisplayName: "P"}},
			"approved-t": {Status: "approved", Owner: Owner{ID: uuid.New(), Email: "a@x", DisplayName: "A"}},
		},
		teachers: []TeacherRow{
			{Slug: "pending-t", DisplayName: "Pending T", Status: "pending"},
			{Slug: "approved-t", DisplayName: "Approved T", Status: "approved"},
		},
	}
	profiles := fakeProfiles{bySlug: map[string]*teachers.Teacher{
		"pending-t":  {ID: uuid.New(), Slug: "pending-t", DisplayName: "Pending T", Kind: teachers.KindProfessional, Status: teachers.StatusPending},
		"approved-t": {ID: uuid.New(), Slug: "approved-t", DisplayName: "Approved T", Kind: teachers.KindProfessional, Status: teachers.StatusApproved},
	}}
	return repo, profiles
}

func TestListTeachers_StatusFilter(t *testing.T) {
	repo, profiles := modFixtures()
	r, _, full := newRouter(repo, profiles)
	var page teachersPageDTO
	decode(t, req(r, http.MethodGet, "/v1/admin/teachers?status=pending", full, ""), &page)
	if page.Total != 1 || page.Teachers[0].Slug != "pending-t" {
		t.Fatalf("status filter: %+v", page)
	}
	if w := req(r, http.MethodGet, "/v1/admin/teachers?status=bogus", full, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, want 400", w.Code)
	}
}

func TestApprove_FromPending_AndAlreadyApproved(t *testing.T) {
	repo, profiles := modFixtures()
	r, _, full := newRouter(repo, profiles)

	w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/approve", full, "")
	if w.Code != http.StatusOK {
		t.Fatalf("approve = %d (%s)", w.Code, w.Body.String())
	}
	var d teacherDetailDTO
	decode(t, w, &d)
	if d.Status != "approved" || d.Owner.Email != "p@x" {
		t.Fatalf("approve result: status=%q owner=%+v", d.Status, d.Owner)
	}
	if len(repo.statusCalls) != 1 || repo.statusCalls[0].note != "" {
		t.Fatalf("status write: %+v", repo.statusCalls)
	}

	w = req(r, http.MethodPost, "/v1/admin/teachers/approved-t/approve", full, "")
	if w.Code != http.StatusConflict || errCode(w) != "invalid_transition" {
		t.Fatalf("already approved: %d / %s", w.Code, w.Body.String())
	}
}

func TestReject_NoteRequiredAndState(t *testing.T) {
	repo, profiles := modFixtures()
	r, _, full := newRouter(repo, profiles)

	if w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/reject", full, `{"note":""}`); w.Code != http.StatusBadRequest {
		t.Fatalf("empty note = %d, want 400", w.Code)
	}
	w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/reject", full, `{"note":"blurry photo"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reject = %d (%s)", w.Code, w.Body.String())
	}
	var d teacherDetailDTO
	decode(t, w, &d)
	if d.Status != "rejected" || d.ModerationNote != "blurry photo" {
		t.Fatalf("reject result: %+v", d)
	}
	if w := req(r, http.MethodPost, "/v1/admin/teachers/approved-t/reject", full, `{"note":"x"}`); w.Code != http.StatusConflict {
		t.Fatalf("reject approved = %d, want 409", w.Code)
	}
}

func TestSuspend_NoteRequiredAndState(t *testing.T) {
	repo, profiles := modFixtures()
	r, _, full := newRouter(repo, profiles)

	if w := req(r, http.MethodPost, "/v1/admin/teachers/approved-t/suspend", full, `{"note":""}`); w.Code != http.StatusBadRequest {
		t.Fatalf("empty note = %d, want 400", w.Code)
	}
	if w := req(r, http.MethodPost, "/v1/admin/teachers/approved-t/suspend", full, `{"note":"chargebacks"}`); w.Code != http.StatusOK {
		t.Fatalf("suspend = %d (%s)", w.Code, w.Body.String())
	}
	if w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/suspend", full, `{"note":"x"}`); w.Code != http.StatusConflict {
		t.Fatalf("suspend pending = %d, want 409", w.Code)
	}
}

func TestVerify_IndependentOfStatus(t *testing.T) {
	repo, profiles := modFixtures()
	r, _, full := newRouter(repo, profiles)

	w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/verify", full, `{"verified":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("verify = %d (%s)", w.Code, w.Body.String())
	}
	if len(repo.verifyCalls) != 1 || !repo.verifyCalls[0].verified {
		t.Fatalf("verify write: %+v", repo.verifyCalls)
	}
	if repo.moderation["pending-t"].Status != "pending" {
		t.Fatalf("verify changed status")
	}
	if w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/verify", full, `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing verified = %d, want 400", w.Code)
	}
}

func TestModeration_UnknownSlugAndPermission(t *testing.T) {
	repo, profiles := modFixtures()
	r, h, full := newRouter(repo, profiles)

	if w := req(r, http.MethodPost, "/v1/admin/teachers/ghost/approve", full, ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown slug = %d, want 404", w.Code)
	}
	// teachers.view alone cannot moderate
	viewer := h.bearerWith(rbac.PermTeachersView)
	if w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/approve", viewer, ""); w.Code != http.StatusForbidden {
		t.Fatalf("viewer approve = %d, want 403", w.Code)
	}
	if w := req(r, http.MethodGet, "/v1/admin/teachers/pending-t", viewer, ""); w.Code != http.StatusOK {
		t.Fatalf("viewer GET = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	// verify needs teachers.verify, not teachers.moderate
	moderator := h.bearerWith(rbac.PermTeachersModerate)
	if w := req(r, http.MethodPost, "/v1/admin/teachers/pending-t/verify", moderator, `{"verified":true}`); w.Code != http.StatusForbidden {
		t.Fatalf("moderator verify = %d, want 403", w.Code)
	}
}

func TestListTeachers_NonApprovedHiddenFromPublic_isDBLevel(t *testing.T) {
	// Sanity: the admin list surfaces every status; hiding from PUBLIC reads is
	// enforced in SQL / the teachers service (covered in internal/teachers).
	repo, profiles := modFixtures()
	r, _, full := newRouter(repo, profiles)
	var page teachersPageDTO
	decode(t, req(r, http.MethodGet, "/v1/admin/teachers", full, ""), &page)
	if page.Total != 2 {
		t.Fatalf("admin list should show all statuses, got %d", page.Total)
	}
}
