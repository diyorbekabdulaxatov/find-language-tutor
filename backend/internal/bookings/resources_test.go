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

// fakeResourceReader is an in-memory bookings.ResourceReader.
type fakeResourceReader struct {
	byBooking map[uuid.UUID][]BookingResource
	err       error
}

func newFakeResourceReader() *fakeResourceReader {
	return &fakeResourceReader{byBooking: map[uuid.UUID][]BookingResource{}}
}

func (f *fakeResourceReader) ForBooking(_ context.Context, id, _ uuid.UUID) ([]BookingResource, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byBooking[id], nil
}

// newTestRouterResources mounts the booking routes with a wired ResourceReader.
func newTestRouterResources(repo Repository, tm *auth.TokenManager, gw PaymentGateway, rr ResourceReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := &Service{repo: repo, now: func() time.Time { return fixedNow }, logger: discardLogger()}
	svc.payments = gw
	svc.resources = rr
	h := NewHandler(svc, discardLogger())
	RegisterRoutes(r.Group("/v1/bookings"), h, auth.RequireAuth(tm))
	return r
}

func confirmedBookingForResources(repo *fakeRepo, student, owner uuid.UUID) uuid.UUID {
	id := uuid.New()
	repo.store[id] = Booking{
		ID:             id,
		Status:         StatusConfirmed,
		StartAt:        fixedNow.Add(time.Hour),
		EndAt:          fixedNow.Add(2 * time.Hour),
		Student:        StudentSummary{ID: student, DisplayName: "Student"},
		TeacherOwnerID: owner,
	}
	return id
}

func TestHandler_Get_EmbedsPopulatedResources(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := confirmedBookingForResources(repo, student, owner)

	dueAt := fixedNow.Add(48 * time.Hour)
	rr := newFakeResourceReader()
	rr.byBooking[id] = []BookingResource{
		{
			ID: uuid.New(), ResourceID: uuid.New(), Kind: "homework", Position: 0, DueAt: &dueAt,
			Type: "quiz", Title: "Numbers quiz", ResourceStatus: "published", SubmissionStatus: "in_progress",
		},
	}
	tm := testTokenManager()
	r := newTestRouterResources(repo, tm, newFakeGateway(repo), rr)

	w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body bookingDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Resources) != 1 {
		t.Fatalf("got %d resources, want 1: %+v", len(body.Resources), body.Resources)
	}
	got := body.Resources[0]
	if got.Kind != "homework" || got.Title != "Numbers quiz" || got.Type != "quiz" || got.SubmissionStatus != "in_progress" {
		t.Errorf("unexpected resource embed: %+v", got)
	}
	if got.DueAt == nil || !got.DueAt.Equal(dueAt) {
		t.Errorf("due_at not embedded: %+v", got.DueAt)
	}
}

// A nil ResourceReader must leave `resources` an empty array, not break the read.
func TestHandler_Get_NilResourceReaderLeavesResourcesEmpty(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := confirmedBookingForResources(repo, student, owner)
	tm := testTokenManager()
	r := newTestRouterResources(repo, tm, newFakeGateway(repo), nil)

	w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body bookingDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Resources == nil {
		t.Fatal("resources must be [] not null")
	}
	if len(body.Resources) != 0 {
		t.Errorf("got %d resources, want 0", len(body.Resources))
	}
}

// A ResourceReader error must not fail the booking read.
func TestHandler_Get_ResourceReaderErrorIsIgnored(t *testing.T) {
	student, owner := uuid.New(), uuid.New()
	repo := newFakeRepo()
	id := confirmedBookingForResources(repo, student, owner)
	rr := newFakeResourceReader()
	rr.err = context.DeadlineExceeded
	tm := testTokenManager()
	r := newTestRouterResources(repo, tm, newFakeGateway(repo), rr)

	w := do(r, http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite resources lookup error", w.Code)
	}
	var body bookingDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Resources) != 0 {
		t.Errorf("resources should be empty on lookup error, got %+v", body.Resources)
	}
}

func TestHandler_List_EmbedsResourcesPerBooking(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	idA := uuid.New()
	idB := uuid.New()
	repo.listResult = []Booking{
		{ID: idA, Status: StatusConfirmed, StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour), Student: StudentSummary{ID: student}},
		{ID: idB, Status: StatusConfirmed, StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour), Student: StudentSummary{ID: student}},
	}
	rr := newFakeResourceReader()
	rr.byBooking[idA] = []BookingResource{{ID: uuid.New(), ResourceID: uuid.New(), Kind: "material", Type: "article", Title: "Reading"}}
	// idB left with no resources.
	tm := testTokenManager()
	r := newTestRouterResources(repo, tm, nil, rr)

	w := do(r, http.MethodGet, "/v1/bookings?role=student", "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body bookingListDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Bookings) != 2 {
		t.Fatalf("got %d bookings", len(body.Bookings))
	}
	if len(body.Bookings[0].Resources) != 1 || body.Bookings[0].Resources[0].Title != "Reading" {
		t.Errorf("booking A should have 1 resource: %+v", body.Bookings[0].Resources)
	}
	if len(body.Bookings[1].Resources) != 0 {
		t.Errorf("booking B should have 0 resources: %+v", body.Bookings[1].Resources)
	}
}
