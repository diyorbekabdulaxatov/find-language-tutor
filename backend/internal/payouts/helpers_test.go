package payouts

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

// ledgerRow is the slice of a payout_ledger row the fake repository needs to
// reproduce the clearing window and the payout run.
type ledgerRow struct {
	id          uuid.UUID
	teacher     TeacherRef
	teacherID   uuid.UUID
	amountMinor int64
	currency    string
	state       string // "held" | "available" | "paid" | "reversed"
	availableAt time.Time
	batchID     uuid.UUID
}

// fakeRepo is an in-memory Repository. Run reproduces the transactional recipe
// in repository_postgres.go — select what has cleared, insert the batch, mark
// exactly those rows paid — so the service tests exercise the same rules.
type fakeRepo struct {
	now     time.Time
	rows    []*ledgerRow
	batches map[uuid.UUID]Batch
	seq     int

	owedErr error
	runErr  error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{now: fixedNow, batches: map[uuid.UUID]Batch{}}
}

// addLedgerRow registers one ledger row. clearedAgo is how long ago the row's
// clearing window closed — negative means the deadline is still in the future.
func (f *fakeRepo) addLedgerRow(t TeacherRef, teacherID uuid.UUID, amountMinor int64, state string, clearedAgo time.Duration) *ledgerRow {
	f.seq++
	row := &ledgerRow{
		id:          uuid.New(),
		teacher:     t,
		teacherID:   teacherID,
		amountMinor: amountMinor,
		currency:    "UZS",
		state:       state,
		availableAt: f.now.Add(-clearedAgo),
	}
	f.rows = append(f.rows, row)
	return row
}

// payable mirrors the SQL predicate: an open row past its clearing deadline.
func (f *fakeRepo) payable(r *ledgerRow) bool {
	return (r.state == "held" || r.state == "available") && !r.availableAt.After(f.now)
}

func (f *fakeRepo) Owed(context.Context) ([]OwedRow, error) {
	if f.owedErr != nil {
		return nil, f.owedErr
	}
	byTeacher := map[string]*OwedRow{}
	for _, r := range f.rows {
		if !f.payable(r) {
			continue
		}
		cur, ok := byTeacher[r.teacher.Slug]
		if !ok {
			byTeacher[r.teacher.Slug] = &OwedRow{
				Teacher: r.teacher, AvailableMinor: r.amountMinor,
				Currency: r.currency, OldestAvailableAt: r.availableAt,
			}
			continue
		}
		cur.AvailableMinor += r.amountMinor
		if r.availableAt.Before(cur.OldestAvailableAt) {
			cur.OldestAvailableAt = r.availableAt
		}
	}
	out := make([]OwedRow, 0, len(byTeacher))
	for _, o := range byTeacher {
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AvailableMinor != out[j].AvailableMinor {
			return out[i].AvailableMinor > out[j].AvailableMinor
		}
		return out[i].Teacher.Slug < out[j].Teacher.Slug
	})
	return out, nil
}

func (f *fakeRepo) Totals(context.Context) (Totals, error) {
	var t Totals
	for _, r := range f.rows {
		t.Currency = r.currency
		switch {
		case r.state == "paid":
			t.PaidTotalMinor += r.amountMinor
		case r.state == "reversed":
		case f.payable(r):
			t.AvailableTotalMinor += r.amountMinor
		default:
			t.HeldTotalMinor += r.amountMinor
		}
	}
	return t, nil
}

func (f *fakeRepo) ListBatches(_ context.Context, limit, offset int) ([]Batch, int, error) {
	all := make([]Batch, 0, len(f.batches))
	for _, b := range f.batches {
		all = append(all, b)
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
	return all[offset:end], total, nil
}

func (f *fakeRepo) Batch(_ context.Context, id uuid.UUID) (BatchDetail, error) {
	b, ok := f.batches[id]
	if !ok {
		return BatchDetail{}, ErrBatchNotFound
	}
	return BatchDetail{Batch: b, Lines: f.linesFor(id)}, nil
}

func (f *fakeRepo) linesFor(batchID uuid.UUID) []BatchLine {
	byTeacher := map[string]*BatchLine{}
	for _, r := range f.rows {
		if r.batchID != batchID {
			continue
		}
		cur, ok := byTeacher[r.teacher.Slug]
		if !ok {
			byTeacher[r.teacher.Slug] = &BatchLine{
				Teacher: r.teacher, AmountMinor: r.amountMinor, Currency: r.currency, LessonCount: 1,
			}
			continue
		}
		cur.AmountMinor += r.amountMinor
		cur.LessonCount++
	}
	out := make([]BatchLine, 0, len(byTeacher))
	for _, l := range byTeacher {
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AmountMinor != out[j].AmountMinor {
			return out[i].AmountMinor > out[j].AmountMinor
		}
		return out[i].Teacher.Slug < out[j].Teacher.Slug
	})
	return out
}

func (f *fakeRepo) Run(_ context.Context, adminID uuid.UUID) (BatchDetail, error) {
	if f.runErr != nil {
		return BatchDetail{}, f.runErr
	}

	var picked []*ledgerRow
	for _, r := range f.rows {
		if f.payable(r) {
			picked = append(picked, r)
		}
	}
	if len(picked) == 0 {
		return BatchDetail{}, ErrNothingToPay
	}

	teachers := map[uuid.UUID]struct{}{}
	var total int64
	for _, r := range picked {
		teachers[r.teacherID] = struct{}{}
		total += r.amountMinor
	}

	f.seq++
	completed := f.now.Add(time.Duration(f.seq) * time.Minute)
	b := Batch{
		ID:           uuid.New(),
		CreatedBy:    UserRef{ID: adminID, DisplayName: "Site Admin"},
		Status:       StatusCompleted,
		TotalMinor:   total,
		Currency:     picked[0].currency,
		TeacherCount: len(teachers),
		LineCount:    len(picked),
		CreatedAt:    completed,
		CompletedAt:  &completed,
	}
	for _, r := range picked {
		r.state = "paid"
		r.batchID = b.ID
	}
	f.batches[b.ID] = b
	return BatchDetail{Batch: b, Lines: f.linesFor(b.ID)}, nil
}

var _ Repository = (*fakeRepo)(nil)

// harness wires the handler behind auth + a real rbac.Guard on /v1/admin — the
// same shape cmd/api builds.
type harness struct {
	router *gin.Engine
	tm     *auth.TokenManager
	rbacR  *fakeRBACRepo
	repo   *fakeRepo
	svc    *Service
}

func newHarness(repo *fakeRepo) *harness {
	gin.SetMode(gin.TestMode)
	h := &harness{
		tm:    auth.NewTokenManager("payouts-test", time.Minute),
		rbacR: newFakeRBACRepo(),
		repo:  repo,
	}
	h.svc = NewService(repo, discardLogger())
	handler := NewHandler(h.svc, discardLogger())

	r := gin.New()
	RegisterAdminRoutes(r.Group("/v1/admin", auth.RequireAuth(h.tm)), handler, rbac.NewGuard(rbac.NewService(h.rbacR)))
	h.router = r
	return h
}

// bearer issues a token for an account id, granting perms in RBAC.
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
