package disputes

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fixedNow is the reference instant for the deterministic tests.
var fixedNow = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

// fakeRepo is an in-memory Repository. Create enforces the same
// one-open-dispute-per-booking rule the partial unique index does, and Resolve
// the same status guard as the UPDATE.
type fakeRepo struct {
	bookings map[uuid.UUID]BookingRef
	// context carried alongside a booking for the operator queue rows.
	bookingCtx map[uuid.UUID]BookingContext

	disputes map[uuid.UUID]Dispute
	seq      int

	bookingErr error
	createErr  error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		bookings:   map[uuid.UUID]BookingRef{},
		bookingCtx: map[uuid.UUID]BookingContext{},
		disputes:   map[uuid.UUID]Dispute{},
	}
}

// addBooking registers a booking with its participants and queue context.
func (f *fakeRepo) addBooking(status string, studentID, teacherOwnerID uuid.UUID) uuid.UUID {
	id := uuid.New()
	f.bookings[id] = BookingRef{ID: id, Status: status, StudentID: studentID, TeacherOwnerID: teacherOwnerID}
	f.bookingCtx[id] = BookingContext{
		ID:      id,
		Status:  status,
		StartAt: fixedNow,
		Price:   Money{AmountMinor: 9_000_000, Currency: "UZS"},
		Teacher: TeacherRef{Slug: "nodira-karimova", DisplayName: "Nodira Karimova"},
		Student: StudentRef{ID: studentID, Email: "sardor@example.com", DisplayName: "Sardor"},
	}
	return id
}

// addDispute injects a dispute directly (bypassing the open-per-booking rule),
// for fixtures that need a closed thread.
func (f *fakeRepo) addDispute(d Dispute) uuid.UUID {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.CreatedAt.IsZero() {
		f.seq++
		d.CreatedAt = fixedNow.Add(time.Duration(f.seq) * time.Minute)
	}
	f.disputes[d.ID] = d
	return d.ID
}

func (f *fakeRepo) BookingForDispute(_ context.Context, bookingID uuid.UUID) (BookingRef, error) {
	if f.bookingErr != nil {
		return BookingRef{}, f.bookingErr
	}
	b, ok := f.bookings[bookingID]
	if !ok {
		return BookingRef{}, ErrBookingNotFound
	}
	return b, nil
}

func (f *fakeRepo) Create(_ context.Context, bookingID, raisedBy uuid.UUID, reason string) (Dispute, error) {
	if f.createErr != nil {
		return Dispute{}, f.createErr
	}
	for _, d := range f.disputes {
		if d.BookingID == bookingID && d.Status == StatusOpen {
			return Dispute{}, ErrDisputeExists // what SQLSTATE 23505 maps to
		}
	}
	f.seq++
	d := Dispute{
		ID:        uuid.New(),
		BookingID: bookingID,
		Status:    StatusOpen,
		Reason:    reason,
		RaisedBy:  UserRef{ID: raisedBy, DisplayName: "Raiser"},
		CreatedAt: fixedNow.Add(time.Duration(f.seq) * time.Minute),
	}
	f.disputes[d.ID] = d
	return d, nil
}

func (f *fakeRepo) ListForBooking(_ context.Context, bookingID uuid.UUID) ([]Dispute, error) {
	var out []Dispute
	for _, d := range f.disputes {
		if d.BookingID == bookingID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (f *fakeRepo) OpenForBooking(_ context.Context, bookingID uuid.UUID) (Dispute, bool, error) {
	for _, d := range f.disputes {
		if d.BookingID == bookingID && d.Status == StatusOpen {
			return d, true, nil
		}
	}
	return Dispute{}, false, nil
}

func (f *fakeRepo) ListQueue(_ context.Context, status string, limit, offset int) ([]QueueItem, int, error) {
	var all []Dispute
	for _, d := range f.disputes {
		if status == "" || string(d.Status) == status {
			all = append(all, d)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })

	total := len(all)
	if offset > len(all) {
		offset = len(all)
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	page := all[offset:end]

	out := make([]QueueItem, len(page))
	for i, d := range page {
		out[i] = QueueItem{Dispute: d, Booking: f.bookingCtx[d.BookingID]}
	}
	return out, total, nil
}

func (f *fakeRepo) Resolve(_ context.Context, disputeID uuid.UUID, status Status, resolution string, resolvedBy uuid.UUID) (Dispute, error) {
	d, ok := f.disputes[disputeID]
	if !ok {
		return Dispute{}, ErrDisputeNotFound
	}
	if d.Status != StatusOpen {
		return Dispute{}, ErrAlreadyResolved
	}
	at := fixedNow.Add(time.Hour)
	d.Status = status
	d.Resolution = resolution
	d.ResolvedBy = &UserRef{ID: resolvedBy, DisplayName: "Site Admin"}
	d.ResolvedAt = &at
	f.disputes[disputeID] = d
	return d, nil
}

var _ Repository = (*fakeRepo)(nil)

// fakeRefunder records the refunds a resolution asked for.
type fakeRefunder struct {
	refunded []uuid.UUID
	err      error
}

func (f *fakeRefunder) AdminRefund(_ context.Context, bookingID uuid.UUID) error {
	f.refunded = append(f.refunded, bookingID)
	return f.err
}

// harness wires the handler behind the real auth middleware on /v1/bookings and
// behind auth + a real rbac.Guard on /v1/admin — the same shape cmd/api builds.
type harness struct {
	router   *gin.Engine
	tm       *auth.TokenManager
	rbacR    *fakeRBACRepo
	repo     *fakeRepo
	refunder *fakeRefunder
	svc      *Service
}

func newHarness(repo *fakeRepo, refunder *fakeRefunder) *harness {
	gin.SetMode(gin.TestMode)
	h := &harness{
		tm:       auth.NewTokenManager("disputes-test", time.Minute),
		rbacR:    newFakeRBACRepo(),
		repo:     repo,
		refunder: refunder,
	}
	h.svc = NewService(repo, discardLogger())
	if refunder != nil {
		h.svc.SetRefunder(refunder)
	}
	handler := NewHandler(h.svc, discardLogger())

	r := gin.New()
	RegisterBookingRoutes(r.Group("/v1/bookings"), handler, auth.RequireAuth(h.tm))
	RegisterAdminRoutes(r.Group("/v1/admin", auth.RequireAuth(h.tm)), handler, rbac.NewGuard(rbac.NewService(h.rbacR)))
	h.router = r
	return h
}

// bearer issues a token for an existing account id, granting perms in RBAC.
func (h *harness) bearer(uid uuid.UUID, perms ...rbac.Permission) string {
	h.rbacR.grant(uid, perms)
	tok, err := h.tm.IssueAccess(auth.User{ID: uid, Email: "u@x", DisplayName: "U"}, time.Now())
	if err != nil {
		panic(err)
	}
	return "Bearer " + tok
}

func (h *harness) do(method, path, bearer, body string) *httptest.ResponseRecorder {
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
	h.router.ServeHTTP(w, rq)
	return w
}

// fakeRBACRepo is the minimal rbac.Repository the guard needs (permission
// resolution only).
type fakeRBACRepo struct{ perms map[uuid.UUID][]string }

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
func (f *fakeRBACRepo) DeleteRole(context.Context, uuid.UUID) error              { return nil }
func (f *fakeRBACRepo) UserExists(context.Context, uuid.UUID) (bool, error)      { return true, nil }
func (f *fakeRBACRepo) AssignRole(context.Context, uuid.UUID, uuid.UUID) error   { return nil }
func (f *fakeRBACRepo) UnassignRole(context.Context, uuid.UUID, uuid.UUID) error { return nil }

var _ rbac.Repository = (*fakeRBACRepo)(nil)
