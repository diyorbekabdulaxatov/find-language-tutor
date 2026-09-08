package bookings

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

// fakeReviewReader is an in-memory bookings.ReviewReader.
type fakeReviewReader struct {
	byBooking map[uuid.UUID]*BookingReview
	err       error
}

func newFakeReviewReader() *fakeReviewReader {
	return &fakeReviewReader{byBooking: map[uuid.UUID]*BookingReview{}}
}

func (f *fakeReviewReader) ForBooking(_ context.Context, id uuid.UUID) (*BookingReview, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	r, ok := f.byBooking[id]
	return r, ok, nil
}

// newTestRouterReviews mounts the booking routes with a wired ReviewReader.
func newTestRouterReviews(repo Repository, tm *auth.TokenManager, gw PaymentGateway, rr ReviewReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := &Service{repo: repo, now: func() time.Time { return fixedNow }, logger: discardLogger()}
	svc.payments = gw
	svc.reviews = rr
	h := NewHandler(svc, discardLogger())
	RegisterRoutes(r.Group("/v1/bookings"), h, auth.RequireAuth(tm))
	return r
}

func completedBooking(repo *fakeRepo, student, owner uuid.UUID) uuid.UUID {
	id := uuid.New()
	repo.store[id] = Booking{
		ID:             id,
		Status:         StatusCompleted,
		StartAt:        fixedNow.Add(-2 * time.Hour),
		EndAt:          fixedNow.Add(-time.Hour),
		Student:        StudentSummary{ID: student, DisplayName: "Student"},
		TeacherOwnerID: owner,
	}
	return id
}

// --- withReview pure logic ---

func TestWithReview_CanReviewTransitions(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	b := Booking{Status: StatusCompleted, Student: StudentSummary{ID: student}, TeacherOwnerID: owner}

	if !withReview(bookingDTO{}, b, student, nil).CanReview {
		t.Error("completed + student viewer + no review: can_review should be true")
	}
	if withReview(bookingDTO{}, b, owner, nil).CanReview {
		t.Error("teacher viewer: can_review must be false")
	}
	if withReview(bookingDTO{}, b, student, &BookingReview{Rating: 5}).CanReview {
		t.Error("already reviewed: can_review must be false")
	}

	b.Status = StatusConfirmed
	if withReview(bookingDTO{}, b, student, nil).CanReview {
		t.Error("not completed: can_review must be false")
	}
}

func TestWithReview_EmbedsReview(t *testing.T) {
	student := uuid.New()
	b := Booking{Status: StatusCompleted, Student: StudentSummary{ID: student}}
	rv := &BookingReview{Rating: 4, Comment: "good", CreatedAt: fixedNow}

	dto := withReview(bookingDTO{}, b, student, rv)
	if dto.Review == nil || dto.Review.Rating != 4 || dto.Review.Comment != "good" {
		t.Fatalf("review not embedded: %+v", dto.Review)
	}
}

// --- handler: GET /v1/bookings/{id} ---

func TestHandler_Get_ReviewEmbeddedForBothParticipants(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := completedBooking(repo, student, owner)
	rr := newFakeReviewReader()
	rr.byBooking[id] = &BookingReview{Rating: 5, Comment: "great lesson", CreatedAt: fixedNow}
	tm := testTokenManager()
	r := newTestRouterReviews(repo, tm, newFakeGateway(repo), rr)

	for _, viewer := range []uuid.UUID{student, owner} {
		w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, viewer))
		if w.Code != http.StatusOK {
			t.Fatalf("viewer %s: status = %d, body = %s", viewer, w.Code, w.Body.String())
		}
		var body bookingDTO
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body.Review == nil || body.Review.Rating != 5 {
			t.Errorf("viewer %s: review not embedded: %+v", viewer, body.Review)
		}
		if body.CanReview {
			t.Errorf("viewer %s: can_review must be false once reviewed", viewer)
		}
	}
}

func TestHandler_Get_CanReviewTrueForStudentOnly(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := completedBooking(repo, student, owner)
	tm := testTokenManager()
	r := newTestRouterReviews(repo, tm, newFakeGateway(repo), newFakeReviewReader())

	ws := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, student))
	var studentView bookingDTO
	_ = json.Unmarshal(ws.Body.Bytes(), &studentView)
	if !studentView.CanReview || studentView.Review != nil {
		t.Errorf("student: can_review=%v review=%+v, want true / nil", studentView.CanReview, studentView.Review)
	}

	wo := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, owner))
	var ownerView bookingDTO
	_ = json.Unmarshal(wo.Body.Bytes(), &ownerView)
	if ownerView.CanReview {
		t.Errorf("teacher owner: can_review must be false")
	}
}

func TestHandler_List_EmbedsCanReviewAndReview(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	completedID := uuid.New()
	confirmedID := uuid.New()
	repo.listResult = []Booking{
		{ID: completedID, Status: StatusCompleted, StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour),
			Student: StudentSummary{ID: student}},
		{ID: confirmedID, Status: StatusConfirmed, StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour),
			Student: StudentSummary{ID: student}},
	}
	rr := newFakeReviewReader() // no reviews yet
	tm := testTokenManager()
	r := newTestRouterReviews(repo, tm, nil, rr)

	w := do(r, http.MethodGet, "/v1/bookings?role=student", "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body bookingListDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Bookings) != 2 {
		t.Fatalf("got %d bookings", len(body.Bookings))
	}
	if !body.Bookings[0].CanReview {
		t.Errorf("completed booking should have can_review true")
	}
	if body.Bookings[1].CanReview {
		t.Errorf("confirmed booking should have can_review false")
	}
}

// A ReviewReader error must not fail the booking read.
func TestHandler_Get_ReviewReaderErrorIsIgnored(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := completedBooking(repo, student, owner)
	rr := newFakeReviewReader()
	rr.err = context.DeadlineExceeded
	tm := testTokenManager()
	r := newTestRouterReviews(repo, tm, newFakeGateway(repo), rr)

	w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite review lookup error", w.Code)
	}
	var body bookingDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Review != nil {
		t.Errorf("review should be nil on lookup error")
	}
}
