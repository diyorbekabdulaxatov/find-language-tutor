package admin

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
)

// bookingFixtures builds three bookings across the lifecycle: a confirmed one
// (force-cancellable), a completed one carrying a resolved + an open dispute,
// and an already-cancelled one.
func bookingFixtures() (*fakeRepo, uuid.UUID, uuid.UUID, uuid.UUID) {
	confirmed := uuid.New()
	completed := uuid.New()
	cancelled := uuid.New()
	studentID := uuid.New()
	raiser := uuid.New()
	resolver := uuid.New()

	base := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	resolvedAt := base.Add(48 * time.Hour)

	row := func(id uuid.UUID, status string, start time.Time, slug, email string, hasOpen bool) BookingRow {
		return BookingRow{
			ID:             id,
			Status:         status,
			StartAt:        start,
			EndAt:          start.Add(time.Hour),
			Teacher:        BookingTeacher{Slug: slug, DisplayName: "Nodira Karimova"},
			Student:        BookingStudent{ID: studentID, Email: email, DisplayName: "Sardor"},
			PriceMinor:     9_000_000,
			Currency:       "UZS",
			PaymentStatus:  "authorized",
			CreatedAt:      start.Add(-72 * time.Hour),
			HasOpenDispute: hasOpen,
		}
	}

	repo := &fakeRepo{bookings: map[uuid.UUID]BookingDetail{
		confirmed: {
			BookingRow:      row(confirmed, BookingConfirmed, base, "nodira-karimova", "sardor@example.com", false),
			DurationMinutes: 60,
			Payment:         &BookingPayment{Status: "authorized", AmountMinor: 9_000_000, Currency: "UZS"},
			MeetingURL:      "https://meet.example.com/nodira",
			Disputes:        []BookingDispute{},
		},
		completed: {
			BookingRow:      row(completed, BookingCompleted, base.Add(-240*time.Hour), "elena-kim", "jasur@example.com", true),
			DurationMinutes: 60,
			Payment:         &BookingPayment{Status: "captured", AmountMinor: 9_000_000, Currency: "UZS"},
			Disputes: []BookingDispute{
				{
					ID: uuid.New(), Status: "open", Reason: "Lesson ended early",
					RaisedBy: UserRef{ID: raiser, DisplayName: "Sardor"}, CreatedAt: base,
				},
				{
					ID: uuid.New(), Status: "rejected", Reason: "Wrong link", Resolution: "Link was correct.",
					RaisedBy:   UserRef{ID: raiser, DisplayName: "Sardor"},
					ResolvedBy: &UserRef{ID: resolver, DisplayName: "Site Admin"},
					CreatedAt:  base.Add(-24 * time.Hour), ResolvedAt: &resolvedAt,
				},
			},
		},
		cancelled: {
			BookingRow:      row(cancelled, BookingCancelled, base.Add(-500*time.Hour), "kim-min-jun", "bekzod@example.com", false),
			DurationMinutes: 30,
			CancelledBy:     "student",
			Disputes:        []BookingDispute{},
		},
	}}
	return repo, confirmed, completed, cancelled
}

func TestAdminListBookings_FilterSearchAndPagination(t *testing.T) {
	repo, _, _, _ := bookingFixtures()
	r, h, full := newRouter(repo, fakeProfiles{})

	var page bookingsPageDTO
	decode(t, req(r, http.MethodGet, "/v1/admin/bookings", full, ""), &page)
	if page.Total != 3 || len(page.Bookings) != 3 {
		t.Fatalf("unfiltered: total=%d len=%d", page.Total, len(page.Bookings))
	}
	// Newest lesson first, and the row carries the parties + money + dispute flag.
	if page.Bookings[0].Status != BookingConfirmed {
		t.Fatalf("expected the newest lesson first, got %q", page.Bookings[0].Status)
	}
	if page.Bookings[0].Price.AmountMinor != 9_000_000 || page.Bookings[0].Price.Currency != "UZS" {
		t.Fatalf("price mapped wrong: %+v", page.Bookings[0].Price)
	}
	if page.Bookings[0].PaymentStatus == nil || *page.Bookings[0].PaymentStatus != "authorized" {
		t.Fatalf("payment_status mapped wrong: %+v", page.Bookings[0].PaymentStatus)
	}

	decode(t, req(r, http.MethodGet, "/v1/admin/bookings?status=completed", full, ""), &page)
	if page.Total != 1 || !page.Bookings[0].HasOpenDispute {
		t.Fatalf("status filter: %+v", page)
	}

	// q matches the teacher slug / display name and the student email.
	decode(t, req(r, http.MethodGet, "/v1/admin/bookings?q=ELENA", full, ""), &page)
	if page.Total != 1 || page.Bookings[0].Teacher.Slug != "elena-kim" {
		t.Fatalf("q by slug: %+v", page)
	}
	decode(t, req(r, http.MethodGet, "/v1/admin/bookings?q=bekzod@example.com", full, ""), &page)
	if page.Total != 1 || page.Bookings[0].Status != BookingCancelled {
		t.Fatalf("q by student email: %+v", page)
	}

	decode(t, req(r, http.MethodGet, "/v1/admin/bookings?page=2&page_size=2", full, ""), &page)
	if page.Total != 3 || len(page.Bookings) != 1 {
		t.Fatalf("page 2: total=%d len=%d", page.Total, len(page.Bookings))
	}
	if repo.lastLimit != 2 || repo.lastOffset != 2 {
		t.Fatalf("limit/offset: %d/%d", repo.lastLimit, repo.lastOffset)
	}

	if w := req(r, http.MethodGet, "/v1/admin/bookings?status=bogus", full, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, want 400", w.Code)
	}
	if w := req(r, http.MethodGet, "/v1/admin/bookings?page=x", full, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad page = %d, want 400", w.Code)
	}
	// bookings.view is required; another admin permission is not enough.
	if w := req(r, http.MethodGet, "/v1/admin/bookings", h.bearerWith(rbac.PermUsersView), ""); w.Code != http.StatusForbidden {
		t.Fatalf("scoped token = %d, want 403", w.Code)
	}
	if w := req(r, http.MethodGet, "/v1/admin/bookings", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", w.Code)
	}
}

func TestAdminGetBooking_DetailShapeAndNotFound(t *testing.T) {
	repo, confirmed, completed, _ := bookingFixtures()
	r, _, full := newRouter(repo, fakeProfiles{})

	var d bookingDetailDTO
	decode(t, req(r, http.MethodGet, "/v1/admin/bookings/"+confirmed.String(), full, ""), &d)
	if d.ID != confirmed.String() || d.DurationMinutes != 60 {
		t.Fatalf("detail core: %+v", d)
	}
	if d.Payment == nil || d.Payment.Status != "authorized" {
		t.Fatalf("payment: %+v", d.Payment)
	}
	if d.MeetingURL != "https://meet.example.com/nodira" {
		t.Fatalf("meeting_url: %q", d.MeetingURL)
	}
	if len(d.Disputes) != 0 {
		t.Fatalf("expected no disputes, got %d", len(d.Disputes))
	}

	// The completed booking carries the whole thread, open and closed.
	decode(t, req(r, http.MethodGet, "/v1/admin/bookings/"+completed.String(), full, ""), &d)
	if len(d.Disputes) != 2 {
		t.Fatalf("expected 2 disputes, got %d", len(d.Disputes))
	}
	if d.Disputes[0].Status != "open" || d.Disputes[0].ResolvedBy != nil || d.Disputes[0].ResolvedAt != nil {
		t.Fatalf("open dispute mapped wrong: %+v", d.Disputes[0])
	}
	if d.Disputes[1].Status != "rejected" || d.Disputes[1].ResolvedBy == nil ||
		d.Disputes[1].ResolvedBy.DisplayName != "Site Admin" || d.Disputes[1].ResolvedAt == nil {
		t.Fatalf("closed dispute mapped wrong: %+v", d.Disputes[1])
	}

	w := req(r, http.MethodGet, "/v1/admin/bookings/"+uuid.New().String(), full, "")
	if w.Code != http.StatusNotFound || errCode(w) != "booking_not_found" {
		t.Fatalf("unknown booking: %d / %s", w.Code, w.Body.String())
	}
	if w := req(r, http.MethodGet, "/v1/admin/bookings/not-a-uuid", full, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad uuid = %d, want 400", w.Code)
	}
}

func TestAdminForceCancel_CancelsThroughThePortAndRefunds(t *testing.T) {
	repo, confirmed, completed, cancelled := bookingFixtures()
	mod := &fakeModerator{repo: repo}
	r, _, full, _ := newRouterWithModerator(repo, fakeProfiles{}, mod)

	w := req(r, http.MethodPost, "/v1/admin/bookings/"+confirmed.String()+"/force-cancel", full,
		`{"reason":"duplicate charge","refund":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("force-cancel = %d (%s)", w.Code, w.Body.String())
	}
	var d bookingDetailDTO
	decode(t, w, &d)
	if d.Status != BookingCancelled || d.CancelledBy != "admin" || d.CancellationReason != "duplicate charge" {
		t.Fatalf("force-cancel result: status=%q by=%q reason=%q", d.Status, d.CancelledBy, d.CancellationReason)
	}
	if d.CancelledAt == nil {
		t.Fatal("cancelled_at not set")
	}
	if len(mod.calls) != 1 || mod.calls[0].id != confirmed || !mod.calls[0].refund || mod.calls[0].reason != "duplicate charge" {
		t.Fatalf("port call: %+v", mod.calls)
	}

	// refund defaults to false when omitted.
	repo2, confirmed2, _, _ := bookingFixtures()
	mod2 := &fakeModerator{repo: repo2}
	r2, _, full2, _ := newRouterWithModerator(repo2, fakeProfiles{}, mod2)
	if w := req(r2, http.MethodPost, "/v1/admin/bookings/"+confirmed2.String()+"/force-cancel", full2,
		`{"reason":"teacher unreachable"}`); w.Code != http.StatusOK {
		t.Fatalf("force-cancel without refund = %d (%s)", w.Code, w.Body.String())
	}
	if len(mod2.calls) != 1 || mod2.calls[0].refund {
		t.Fatalf("refund should default to false: %+v", mod2.calls)
	}

	// A completed or already-cancelled booking is 409 invalid_state, and the
	// port is never called.
	before := len(mod.calls)
	for _, id := range []uuid.UUID{completed, cancelled} {
		w := req(r, http.MethodPost, "/v1/admin/bookings/"+id.String()+"/force-cancel", full, `{"reason":"nope"}`)
		if w.Code != http.StatusConflict || errCode(w) != "invalid_state" {
			t.Fatalf("force-cancel %s: %d / %s", id, w.Code, w.Body.String())
		}
	}
	if len(mod.calls) != before {
		t.Fatalf("port called for an un-cancellable booking: %+v", mod.calls)
	}
}

func TestAdminForceCancel_ValidationAndPermission(t *testing.T) {
	repo, confirmed, _, _ := bookingFixtures()
	mod := &fakeModerator{repo: repo}
	r, h, full, _ := newRouterWithModerator(repo, fakeProfiles{}, mod)

	if w := req(r, http.MethodPost, "/v1/admin/bookings/"+confirmed.String()+"/force-cancel", full,
		`{"reason":"   ","refund":true}`); w.Code != http.StatusBadRequest {
		t.Fatalf("blank reason = %d, want 400", w.Code)
	}
	if w := req(r, http.MethodPost, "/v1/admin/bookings/"+confirmed.String()+"/force-cancel", full,
		`{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing reason = %d, want 400", w.Code)
	}
	if w := req(r, http.MethodPost, "/v1/admin/bookings/"+uuid.New().String()+"/force-cancel", full,
		`{"reason":"x"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown booking = %d, want 404", w.Code)
	}
	if len(mod.calls) != 0 {
		t.Fatalf("port called on a rejected request: %+v", mod.calls)
	}

	// bookings.view alone can read but not force-cancel.
	viewer := h.bearerWith(rbac.PermBookingsView)
	if w := req(r, http.MethodGet, "/v1/admin/bookings/"+confirmed.String(), viewer, ""); w.Code != http.StatusOK {
		t.Fatalf("viewer GET = %d, want 200", w.Code)
	}
	if w := req(r, http.MethodPost, "/v1/admin/bookings/"+confirmed.String()+"/force-cancel", viewer,
		`{"reason":"x"}`); w.Code != http.StatusForbidden {
		t.Fatalf("viewer force-cancel = %d, want 403", w.Code)
	}
}

func TestAdminForceCancel_PortErrorIs500(t *testing.T) {
	repo, confirmed, _, _ := bookingFixtures()
	mod := &fakeModerator{repo: repo, err: errors.New("boom")}
	r, _, full, _ := newRouterWithModerator(repo, fakeProfiles{}, mod)

	w := req(r, http.MethodPost, "/v1/admin/bookings/"+confirmed.String()+"/force-cancel", full, `{"reason":"x"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("port failure = %d, want 500 (%s)", w.Code, w.Body.String())
	}
}

// Without a wired BookingModerator the endpoint fails loudly rather than
// pretending to cancel.
func TestAdminForceCancel_NoModeratorWired(t *testing.T) {
	repo, confirmed, _, _ := bookingFixtures()
	r, _, full := newRouter(repo, fakeProfiles{})

	w := req(r, http.MethodPost, "/v1/admin/bookings/"+confirmed.String()+"/force-cancel", full, `{"reason":"x"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unwired port = %d, want 500", w.Code)
	}
	if repo.bookings[confirmed].Status != BookingConfirmed {
		t.Fatal("booking changed state without a moderator")
	}
}
