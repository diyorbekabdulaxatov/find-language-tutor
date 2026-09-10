package disputes

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
)

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
}

// errContext stands in for a payment-provider failure on the refund path.
var errContext = errors.New("provider unreachable")

func errCode(w *httptest.ResponseRecorder) string {
	var b struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return b.Error.Code
}

// --- raising a dispute (participant) ---

func TestRaise_StudentOnCompletedBooking(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	h := newHarness(repo, nil)

	w := h.do(http.MethodPost, "/v1/bookings/"+booking.String()+"/disputes",
		h.bearer(student), `{"reason":"The lesson ended 20 minutes early."}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("raise = %d (%s)", w.Code, w.Body.String())
	}
	var d disputeDTO
	decode(t, w, &d)
	if d.Status != "open" || d.BookingID != booking.String() || d.Reason != "The lesson ended 20 minutes early." {
		t.Fatalf("dispute mapped wrong: %+v", d)
	}
	if d.ResolvedBy != nil || d.ResolvedAt != nil || d.Resolution != "" {
		t.Fatalf("a new dispute must not be resolved: %+v", d)
	}
	if d.RaisedBy.ID != student.String() {
		t.Fatalf("raised_by = %q, want the caller", d.RaisedBy.ID)
	}
}

func TestRaise_TeacherOwnerCanAlsoRaise(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingConfirmed, student, teacher)
	h := newHarness(repo, nil)

	if w := h.do(http.MethodPost, "/v1/bookings/"+booking.String()+"/disputes",
		h.bearer(teacher), `{"reason":"The student was abusive."}`); w.Code != http.StatusCreated {
		t.Fatalf("teacher raise = %d (%s)", w.Code, w.Body.String())
	}
}

func TestRaise_NonParticipantIs403(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	h := newHarness(repo, nil)

	w := h.do(http.MethodPost, "/v1/bookings/"+booking.String()+"/disputes",
		h.bearer(uuid.New()), `{"reason":"nosy"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("stranger raise = %d, want 403", w.Code)
	}
	if len(repo.disputes) != 0 {
		t.Fatal("a stranger's dispute was recorded")
	}
}

func TestRaise_WrongBookingStateIs409(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	h := newHarness(repo, nil)

	for _, status := range []string{"pending_payment", "cancelled"} {
		booking := repo.addBooking(status, student, teacher)
		w := h.do(http.MethodPost, "/v1/bookings/"+booking.String()+"/disputes",
			h.bearer(student), `{"reason":"x"}`)
		if w.Code != http.StatusConflict || errCode(w) != "dispute_not_allowed" {
			t.Fatalf("%s: %d / %s", status, w.Code, w.Body.String())
		}
	}
}

func TestRaise_SecondOpenDisputeIs409(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	h := newHarness(repo, nil)

	if w := h.do(http.MethodPost, "/v1/bookings/"+booking.String()+"/disputes",
		h.bearer(student), `{"reason":"first"}`); w.Code != http.StatusCreated {
		t.Fatalf("first raise = %d", w.Code)
	}
	// The DB's partial unique index is the gate; the repo surfaces it as
	// ErrDisputeExists whoever files the second one.
	w := h.do(http.MethodPost, "/v1/bookings/"+booking.String()+"/disputes",
		h.bearer(teacher), `{"reason":"second"}`)
	if w.Code != http.StatusConflict || errCode(w) != "dispute_exists" {
		t.Fatalf("second raise: %d / %s", w.Code, w.Body.String())
	}

	// Once the first is closed, a new one is allowed again.
	for id := range repo.disputes {
		d := repo.disputes[id]
		d.Status = StatusRejected
		repo.disputes[id] = d
	}
	if w := h.do(http.MethodPost, "/v1/bookings/"+booking.String()+"/disputes",
		h.bearer(student), `{"reason":"again"}`); w.Code != http.StatusCreated {
		t.Fatalf("re-raise after close = %d (%s)", w.Code, w.Body.String())
	}
}

func TestRaise_ValidationAndAuth(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	h := newHarness(repo, nil)
	path := "/v1/bookings/" + booking.String() + "/disputes"

	if w := h.do(http.MethodPost, path, h.bearer(student), `{"reason":"   "}`); w.Code != http.StatusBadRequest {
		t.Fatalf("blank reason = %d, want 400", w.Code)
	}
	long := `{"reason":"` + strings.Repeat("x", MaxReasonLen+1) + `"}`
	if w := h.do(http.MethodPost, path, h.bearer(student), long); w.Code != http.StatusBadRequest {
		t.Fatalf("over-long reason = %d, want 400", w.Code)
	}
	if w := h.do(http.MethodPost, path, h.bearer(student), `not json`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", w.Code)
	}
	if w := h.do(http.MethodPost, path, "", `{"reason":"x"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", w.Code)
	}
	if w := h.do(http.MethodPost, "/v1/bookings/"+uuid.New().String()+"/disputes",
		h.bearer(student), `{"reason":"x"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown booking = %d, want 404", w.Code)
	}
	if w := h.do(http.MethodPost, "/v1/bookings/nope/disputes",
		h.bearer(student), `{"reason":"x"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad uuid = %d, want 400", w.Code)
	}
}

// --- the participant thread ---

func TestListForBooking_ParticipantsOnlyNewestFirst(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	older := repo.addDispute(Dispute{BookingID: booking, Status: StatusRejected, Reason: "older",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	newer := repo.addDispute(Dispute{BookingID: booking, Status: StatusOpen, Reason: "newer",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	h := newHarness(repo, nil)
	path := "/v1/bookings/" + booking.String() + "/disputes"

	var list disputeListDTO
	decode(t, h.do(http.MethodGet, path, h.bearer(student), ""), &list)
	if len(list.Disputes) != 2 || list.Disputes[0].ID != newer.String() || list.Disputes[1].ID != older.String() {
		t.Fatalf("thread order: %+v", list)
	}
	// The teacher-owner sees the same thread.
	decode(t, h.do(http.MethodGet, path, h.bearer(teacher), ""), &list)
	if len(list.Disputes) != 2 {
		t.Fatalf("teacher thread: %+v", list)
	}
	if w := h.do(http.MethodGet, path, h.bearer(uuid.New()), ""); w.Code != http.StatusForbidden {
		t.Fatalf("stranger thread = %d, want 403", w.Code)
	}
	if w := h.do(http.MethodGet, path, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", w.Code)
	}
}

func TestListForBooking_EmptyThreadIsAnEmptyArray(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingConfirmed, student, teacher)
	h := newHarness(repo, nil)

	w := h.do(http.MethodGet, "/v1/bookings/"+booking.String()+"/disputes", h.bearer(student), "")
	if w.Code != http.StatusOK {
		t.Fatalf("empty thread = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"disputes":[]`) {
		t.Fatalf("expected an empty array, got %s", w.Body.String())
	}
}

// --- the operator queue ---

func queueFixtures() (*fakeRepo, uuid.UUID) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	b1 := repo.addBooking(bookingCompleted, student, teacher)
	b2 := repo.addBooking(bookingConfirmed, student, teacher)
	repo.addDispute(Dispute{BookingID: b1, Status: StatusOpen, Reason: "open one",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	repo.addDispute(Dispute{BookingID: b2, Status: StatusOpen, Reason: "open two",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	repo.addDispute(Dispute{BookingID: b1, Status: StatusResolved, Reason: "closed one",
		Resolution: "Refunded.", RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	return repo, b1
}

func TestListQueue_DefaultsToOpenAndPaginates(t *testing.T) {
	repo, _ := queueFixtures()
	h := newHarness(repo, nil)
	admin := h.bearer(uuid.New(), rbac.PermDisputesResolve)

	var page queuePageDTO
	decode(t, h.do(http.MethodGet, "/v1/admin/disputes", admin, ""), &page)
	if page.Total != 2 || len(page.Disputes) != 2 {
		t.Fatalf("default status filter: total=%d len=%d", page.Total, len(page.Disputes))
	}
	for _, d := range page.Disputes {
		if d.Status != "open" {
			t.Fatalf("default queue leaked a %q dispute", d.Status)
		}
	}
	// Each row carries the booking + parties context.
	if page.Disputes[0].Booking.Teacher.Slug != "nodira-karimova" ||
		page.Disputes[0].Booking.Price.AmountMinor != 9_000_000 ||
		page.Disputes[0].Booking.Student.Email != "sardor@example.com" {
		t.Fatalf("booking context: %+v", page.Disputes[0].Booking)
	}

	decode(t, h.do(http.MethodGet, "/v1/admin/disputes?status=resolved", admin, ""), &page)
	if page.Total != 1 || page.Disputes[0].Resolution != "Refunded." {
		t.Fatalf("resolved filter: %+v", page)
	}
	decode(t, h.do(http.MethodGet, "/v1/admin/disputes?status=all", admin, ""), &page)
	if page.Total != 3 {
		t.Fatalf("status=all: total=%d", page.Total)
	}

	decode(t, h.do(http.MethodGet, "/v1/admin/disputes?status=all&page=2&page_size=2", admin, ""), &page)
	if page.Total != 3 || len(page.Disputes) != 1 {
		t.Fatalf("page 2: total=%d len=%d", page.Total, len(page.Disputes))
	}
	// page_size is capped, not honoured verbatim.
	decode(t, h.do(http.MethodGet, "/v1/admin/disputes?status=all&page_size=9999", admin, ""), &page)
	if len(page.Disputes) != 3 {
		t.Fatalf("capped page size: len=%d", len(page.Disputes))
	}

	if w := h.do(http.MethodGet, "/v1/admin/disputes?status=bogus", admin, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, want 400", w.Code)
	}
	if w := h.do(http.MethodGet, "/v1/admin/disputes?page=x", admin, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad page = %d, want 400", w.Code)
	}
}

func TestQueue_RequiresDisputesResolve(t *testing.T) {
	repo, _ := queueFixtures()
	h := newHarness(repo, nil)

	other := h.bearer(uuid.New(), rbac.PermBookingsView)
	if w := h.do(http.MethodGet, "/v1/admin/disputes", other, ""); w.Code != http.StatusForbidden {
		t.Fatalf("scoped token = %d, want 403", w.Code)
	}
	if w := h.do(http.MethodGet, "/v1/admin/disputes", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", w.Code)
	}
	var id uuid.UUID
	for k := range repo.disputes {
		id = k
		break
	}
	if w := h.do(http.MethodPost, "/v1/admin/disputes/"+id.String()+"/resolve", other,
		`{"outcome":"resolved","resolution":"ok"}`); w.Code != http.StatusForbidden {
		t.Fatalf("scoped resolve = %d, want 403", w.Code)
	}
}

// --- resolving ---

func TestResolve_ResolvedWithRefund(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	id := repo.addDispute(Dispute{BookingID: booking, Status: StatusOpen, Reason: "no show",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	refunder := &fakeRefunder{}
	h := newHarness(repo, refunder)
	adminID := uuid.New()

	w := h.do(http.MethodPost, "/v1/admin/disputes/"+id.String()+"/resolve",
		h.bearer(adminID, rbac.PermDisputesResolve),
		`{"outcome":"resolved","resolution":"Refunded in full.","refund":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("resolve = %d (%s)", w.Code, w.Body.String())
	}
	var d disputeDTO
	decode(t, w, &d)
	if d.Status != "resolved" || d.Resolution != "Refunded in full." {
		t.Fatalf("resolve result: %+v", d)
	}
	if d.ResolvedBy == nil || d.ResolvedBy.ID != adminID.String() || d.ResolvedAt == nil {
		t.Fatalf("resolver not recorded: %+v", d)
	}
	if len(refunder.refunded) != 1 || refunder.refunded[0] != booking {
		t.Fatalf("refund not routed to the booking: %+v", refunder.refunded)
	}
}

func TestResolve_RejectedNeverRefunds(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	id := repo.addDispute(Dispute{BookingID: booking, Status: StatusOpen, Reason: "x",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	refunder := &fakeRefunder{}
	h := newHarness(repo, refunder)

	// refund:true is ignored for a rejected outcome — the operator sided with
	// the teacher.
	w := h.do(http.MethodPost, "/v1/admin/disputes/"+id.String()+"/resolve",
		h.bearer(uuid.New(), rbac.PermDisputesResolve),
		`{"outcome":"rejected","resolution":"The recording shows a full lesson.","refund":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reject = %d (%s)", w.Code, w.Body.String())
	}
	var d disputeDTO
	decode(t, w, &d)
	if d.Status != "rejected" {
		t.Fatalf("status = %q", d.Status)
	}
	if len(refunder.refunded) != 0 {
		t.Fatalf("a rejected dispute must not refund: %+v", refunder.refunded)
	}
}

func TestResolve_NoRefundRequested(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	id := repo.addDispute(Dispute{BookingID: booking, Status: StatusOpen, Reason: "x",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	refunder := &fakeRefunder{}
	h := newHarness(repo, refunder)

	if w := h.do(http.MethodPost, "/v1/admin/disputes/"+id.String()+"/resolve",
		h.bearer(uuid.New(), rbac.PermDisputesResolve),
		`{"outcome":"resolved","resolution":"Credited a free lesson instead."}`); w.Code != http.StatusOK {
		t.Fatalf("resolve = %d", w.Code)
	}
	if len(refunder.refunded) != 0 {
		t.Fatalf("refund defaulted to true: %+v", refunder.refunded)
	}
}

func TestResolve_AlreadyResolvedAndUnknown(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	id := repo.addDispute(Dispute{BookingID: booking, Status: StatusResolved, Reason: "x",
		Resolution: "done", RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	h := newHarness(repo, &fakeRefunder{})
	admin := h.bearer(uuid.New(), rbac.PermDisputesResolve)
	body := `{"outcome":"resolved","resolution":"again"}`

	w := h.do(http.MethodPost, "/v1/admin/disputes/"+id.String()+"/resolve", admin, body)
	if w.Code != http.StatusConflict || errCode(w) != "already_resolved" {
		t.Fatalf("already resolved: %d / %s", w.Code, w.Body.String())
	}
	w = h.do(http.MethodPost, "/v1/admin/disputes/"+uuid.New().String()+"/resolve", admin, body)
	if w.Code != http.StatusNotFound || errCode(w) != "dispute_not_found" {
		t.Fatalf("unknown dispute: %d / %s", w.Code, w.Body.String())
	}
}

func TestResolve_Validation(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	id := repo.addDispute(Dispute{BookingID: booking, Status: StatusOpen, Reason: "x",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	h := newHarness(repo, &fakeRefunder{})
	admin := h.bearer(uuid.New(), rbac.PermDisputesResolve)
	path := "/v1/admin/disputes/" + id.String() + "/resolve"

	for name, body := range map[string]string{
		"unknown outcome":  `{"outcome":"maybe","resolution":"x"}`,
		"open outcome":     `{"outcome":"open","resolution":"x"}`,
		"missing outcome":  `{"resolution":"x"}`,
		"blank resolution": `{"outcome":"resolved","resolution":"  "}`,
		"long resolution":  `{"outcome":"resolved","resolution":"` + strings.Repeat("y", MaxResolutionLen+1) + `"}`,
		"not json":         `[]`,
	} {
		if w := h.do(http.MethodPost, path, admin, body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
	if repo.disputes[id].Status != StatusOpen {
		t.Fatal("a rejected request changed the dispute")
	}
	if w := h.do(http.MethodPost, "/v1/admin/disputes/not-a-uuid/resolve", admin,
		`{"outcome":"resolved","resolution":"x"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad uuid = %d, want 400", w.Code)
	}
}

// A refund failure must not lose the operator's decision: the dispute stays
// resolved and the endpoint still returns 200.
func TestResolve_RefundFailureDoesNotFailTheRequest(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	id := repo.addDispute(Dispute{BookingID: booking, Status: StatusOpen, Reason: "x",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	refunder := &fakeRefunder{err: errContext}
	h := newHarness(repo, refunder)

	w := h.do(http.MethodPost, "/v1/admin/disputes/"+id.String()+"/resolve",
		h.bearer(uuid.New(), rbac.PermDisputesResolve),
		`{"outcome":"resolved","resolution":"Refund the student.","refund":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("resolve with a failing refund = %d (%s)", w.Code, w.Body.String())
	}
	if repo.disputes[id].Status != StatusResolved {
		t.Fatal("the resolution was rolled back by a refund failure")
	}
}

// A nil Refunder (cmd/api without payments, or a unit test) makes refund:true a
// logged no-op rather than a panic.
func TestResolve_NilRefunderIsSafe(t *testing.T) {
	repo := newFakeRepo()
	student, teacher := uuid.New(), uuid.New()
	booking := repo.addBooking(bookingCompleted, student, teacher)
	id := repo.addDispute(Dispute{BookingID: booking, Status: StatusOpen, Reason: "x",
		RaisedBy: UserRef{ID: student, DisplayName: "Sardor"}})
	h := newHarness(repo, nil)

	if w := h.do(http.MethodPost, "/v1/admin/disputes/"+id.String()+"/resolve",
		h.bearer(uuid.New(), rbac.PermDisputesResolve),
		`{"outcome":"resolved","resolution":"x","refund":true}`); w.Code != http.StatusOK {
		t.Fatalf("nil refunder = %d (%s)", w.Code, w.Body.String())
	}
}
