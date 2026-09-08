package reviews

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func decodeErr(t *testing.T, body []byte) string {
	t.Helper()
	var e struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	return e.Error.Code
}

// completedBooking registers a completed booking owned by student `student` for
// a teacher and returns its id.
func completedBooking(repo *fakeRepo, student uuid.UUID) uuid.UUID {
	id := uuid.New()
	repo.bookings[id] = BookingRef{
		ID:          id,
		TeacherID:   uuid.New(),
		TeacherSlug: "nodira-karimova",
		StudentID:   student,
		StudentName: "Aziz",
		Status:      "completed",
	}
	return id
}

func TestCreate_OK_BumpsAggregate(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	repo.rating, repo.reviewCount = 4.9, 214
	bid := completedBooking(repo, student)
	tm := testTokenManager()

	w := do(newTestRouter(NewService(repo, discardLogger()), tm), http.MethodPost,
		"/v1/bookings/"+bid.String()+"/review", `{"rating":5,"comment":"  Superb.  "}`, bearerFor(tm, student))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body createdReviewDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Rating != 5 || body.Comment != "Superb." || body.TeacherSlug != "nodira-karimova" || body.Student.DisplayName != "Aziz" {
		t.Fatalf("unexpected body: %+v", body)
	}
	if len(repo.created) != 1 {
		t.Fatalf("CreateReview called %d times, want 1", len(repo.created))
	}
	// (4.9*214 + 5) / 215 = 4.9004… -> 4.9, review_count 214 -> 215
	if repo.reviewCount != 215 || repo.rating != 4.9 {
		t.Errorf("aggregate not bumped correctly: rating=%v count=%d", repo.rating, repo.reviewCount)
	}
}

func TestCreate_RaisesAggregate(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	repo.rating, repo.reviewCount = 4.0, 1
	bid := completedBooking(repo, student)
	tm := testTokenManager()

	w := do(newTestRouter(NewService(repo, discardLogger()), tm), http.MethodPost,
		"/v1/bookings/"+bid.String()+"/review", `{"rating":5}`, bearerFor(tm, student))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	// (4.0*1 + 5) / 2 = 4.5
	if repo.rating != 4.5 || repo.reviewCount != 2 {
		t.Errorf("rating=%v count=%d, want 4.5 / 2", repo.rating, repo.reviewCount)
	}
}

func TestCreate_RequiresAuth(t *testing.T) {
	repo := newFakeRepo()
	w := do(newTestRouter(NewService(repo, discardLogger()), testTokenManager()), http.MethodPost,
		"/v1/bookings/"+uuid.New().String()+"/review", `{"rating":5}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestCreate_NonStudent403(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	bid := completedBooking(repo, student)
	tm := testTokenManager()

	w := do(newTestRouter(NewService(repo, discardLogger()), tm), http.MethodPost,
		"/v1/bookings/"+bid.String()+"/review", `{"rating":5}`, bearerFor(tm, uuid.New()))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", w.Code, w.Body.String())
	}
}

func TestCreate_NotCompleted409(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	bid := completedBooking(repo, student)
	b := repo.bookings[bid]
	b.Status = "confirmed"
	repo.bookings[bid] = b
	tm := testTokenManager()

	w := do(newTestRouter(NewService(repo, discardLogger()), tm), http.MethodPost,
		"/v1/bookings/"+bid.String()+"/review", `{"rating":5}`, bearerFor(tm, student))
	if w.Code != http.StatusConflict || decodeErr(t, w.Body.Bytes()) != "booking_not_completed" {
		t.Fatalf("status = %d code = %s, want 409 booking_not_completed", w.Code, decodeErr(t, w.Body.Bytes()))
	}
}

func TestCreate_Duplicate409(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	bid := completedBooking(repo, student)
	tm := testTokenManager()
	r := newTestRouter(NewService(repo, discardLogger()), tm)

	first := do(r, http.MethodPost, "/v1/bookings/"+bid.String()+"/review", `{"rating":5}`, bearerFor(tm, student))
	if first.Code != http.StatusCreated {
		t.Fatalf("first review status = %d", first.Code)
	}
	second := do(r, http.MethodPost, "/v1/bookings/"+bid.String()+"/review", `{"rating":4}`, bearerFor(tm, student))
	if second.Code != http.StatusConflict || decodeErr(t, second.Body.Bytes()) != "already_reviewed" {
		t.Fatalf("status = %d code = %s, want 409 already_reviewed", second.Code, decodeErr(t, second.Body.Bytes()))
	}
}

func TestCreate_BookingNotFound404(t *testing.T) {
	student := uuid.New()
	tm := testTokenManager()
	w := do(newTestRouter(NewService(newFakeRepo(), discardLogger()), tm), http.MethodPost,
		"/v1/bookings/"+uuid.New().String()+"/review", `{"rating":5}`, bearerFor(tm, student))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestCreate_RatingOutOfRange400(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	bid := completedBooking(repo, student)
	tm := testTokenManager()
	r := newTestRouter(NewService(repo, discardLogger()), tm)

	for _, rating := range []string{"0", "6", "-1"} {
		w := do(r, http.MethodPost, "/v1/bookings/"+bid.String()+"/review",
			fmt.Sprintf(`{"rating":%s}`, rating), bearerFor(tm, student))
		if w.Code != http.StatusBadRequest {
			t.Errorf("rating %s: status = %d, want 400", rating, w.Code)
		}
	}
}

func TestCreate_CommentTooLong400(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	bid := completedBooking(repo, student)
	tm := testTokenManager()

	long := strings.Repeat("x", MaxCommentLen+1)
	w := do(newTestRouter(NewService(repo, discardLogger()), tm), http.MethodPost,
		"/v1/bookings/"+bid.String()+"/review",
		fmt.Sprintf(`{"rating":5,"comment":%q}`, long), bearerFor(tm, student))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if len(repo.created) != 0 {
		t.Errorf("review was created despite over-long comment")
	}
}

func TestListForTeacher_NewestFirstAndPaged(t *testing.T) {
	repo := newFakeRepo()
	tid := uuid.New()
	repo.slugToID["nodira-karimova"] = tid
	for i := 0; i < 5; i++ {
		repo.list = append(repo.list, Review{
			ID:          uuid.New(),
			TeacherID:   tid,
			StudentName: fmt.Sprintf("S%d", i),
			Rating:      5 - i%2,
			Comment:     fmt.Sprintf("c%d", i),
			CreatedAt:   fixedNow.Add(time.Duration(i) * time.Hour),
		})
	}
	r := newTestRouter(NewService(repo, discardLogger()), testTokenManager())

	w := do(r, http.MethodGet, "/v1/teachers/nodira-karimova/reviews?page=1&page_size=2", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body reviewListDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Total != 5 || len(body.Reviews) != 2 {
		t.Fatalf("total=%d len=%d, want 5 / 2", body.Total, len(body.Reviews))
	}
	if !body.Reviews[0].CreatedAt.After(body.Reviews[1].CreatedAt) {
		t.Errorf("not newest-first: %v then %v", body.Reviews[0].CreatedAt, body.Reviews[1].CreatedAt)
	}
	if body.Reviews[0].StudentDisplayName != "S4" {
		t.Errorf("first review student = %q, want S4", body.Reviews[0].StudentDisplayName)
	}

	w2 := do(r, http.MethodGet, "/v1/teachers/nodira-karimova/reviews?page=3&page_size=2", "", "")
	var page3 reviewListDTO
	_ = json.Unmarshal(w2.Body.Bytes(), &page3)
	if len(page3.Reviews) != 1 || page3.Total != 5 {
		t.Errorf("page 3: len=%d total=%d, want 1 / 5", len(page3.Reviews), page3.Total)
	}
}

func TestListForTeacher_PageSizeCapped(t *testing.T) {
	repo := newFakeRepo()
	tid := uuid.New()
	repo.slugToID["x"] = tid
	// 60 reviews; page_size=999 must be capped at 50.
	for i := 0; i < 60; i++ {
		repo.list = append(repo.list, Review{ID: uuid.New(), TeacherID: tid, CreatedAt: fixedNow.Add(time.Duration(i) * time.Minute)})
	}
	w := do(newTestRouter(NewService(repo, discardLogger()), testTokenManager()), http.MethodGet,
		"/v1/teachers/x/reviews?page_size=999", "", "")
	var body reviewListDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Reviews) != 50 {
		t.Fatalf("returned %d reviews, want 50 (capped)", len(body.Reviews))
	}
}

func TestListForTeacher_UnknownSlug404(t *testing.T) {
	w := do(newTestRouter(NewService(newFakeRepo(), discardLogger()), testTokenManager()), http.MethodGet,
		"/v1/teachers/ghost/reviews", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestListForTeacher_EmptyIsEmptyArray(t *testing.T) {
	repo := newFakeRepo()
	repo.slugToID["nodira-karimova"] = uuid.New()
	w := do(newTestRouter(NewService(repo, discardLogger()), testTokenManager()), http.MethodGet,
		"/v1/teachers/nodira-karimova/reviews", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"reviews":[]`) {
		t.Errorf("empty list should serialize reviews as [], got %s", w.Body.String())
	}
}
