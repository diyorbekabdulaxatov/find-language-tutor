package reviews

import (
	"context"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testTokenManager() *auth.TokenManager {
	return auth.NewTokenManager("reviews-test-secret", time.Minute)
}

func bearerFor(tm *auth.TokenManager, id uuid.UUID) string {
	tok, err := tm.IssueAccess(auth.User{ID: id, Email: "demo@example.com", DisplayName: "Demo"}, time.Now())
	if err != nil {
		panic(err)
	}
	return "Bearer " + tok
}

var fixedNow = time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)

// fakeRepo is an in-memory Repository. CreateReview also folds the new rating
// into an in-memory teacher aggregate exactly as the real transaction does, so
// tests can assert the aggregate moved.
type fakeRepo struct {
	bookings         map[uuid.UUID]BookingRef
	reviewsByBooking map[uuid.UUID]Review
	slugToID         map[string]uuid.UUID
	list             []Review

	// the aggregate the CreateReview transaction bumps
	rating      float64
	reviewCount int

	created   []CreateParams
	createErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		bookings:         map[uuid.UUID]BookingRef{},
		reviewsByBooking: map[uuid.UUID]Review{},
		slugToID:         map[string]uuid.UUID{},
	}
}

func (f *fakeRepo) BookingForReview(_ context.Context, id uuid.UUID) (BookingRef, error) {
	b, ok := f.bookings[id]
	if !ok {
		return BookingRef{}, ErrBookingNotFound
	}
	return b, nil
}

func (f *fakeRepo) CreateReview(_ context.Context, p CreateParams) (Review, error) {
	if f.createErr != nil {
		return Review{}, f.createErr
	}
	if _, dup := f.reviewsByBooking[p.BookingID]; dup {
		return Review{}, ErrAlreadyReviewed
	}
	f.created = append(f.created, p)

	// incremental aggregate bump, clamped to [0, 5]
	next := (f.rating*float64(f.reviewCount) + float64(p.Rating)) / float64(f.reviewCount+1)
	next = math.Round(next*10) / 10
	f.rating = math.Max(0, math.Min(5, next))
	f.reviewCount++

	bid := p.BookingID
	r := Review{
		ID:          uuid.New(),
		TeacherID:   p.TeacherID,
		TeacherSlug: p.TeacherSlug,
		StudentID:   p.StudentID,
		StudentName: p.StudentName,
		BookingID:   &bid,
		Rating:      p.Rating,
		Comment:     p.Comment,
		CreatedAt:   fixedNow,
	}
	f.reviewsByBooking[p.BookingID] = r
	return r, nil
}

func (f *fakeRepo) ReviewByBooking(_ context.Context, id uuid.UUID) (Review, bool, error) {
	r, ok := f.reviewsByBooking[id]
	return r, ok, nil
}

func (f *fakeRepo) TeacherIDBySlug(_ context.Context, slug string) (uuid.UUID, error) {
	id, ok := f.slugToID[slug]
	if !ok {
		return uuid.Nil, ErrTeacherNotFound
	}
	return id, nil
}

func (f *fakeRepo) ListByTeacher(_ context.Context, teacherID uuid.UUID, limit, offset int) ([]Review, int, error) {
	var all []Review
	for _, r := range f.list {
		if r.TeacherID == teacherID {
			all = append(all, r)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	total := len(all)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func newTestRouter(s *Service, tm *auth.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(s, discardLogger())

	tg := r.Group("/v1/teachers")
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	tg.GET("/:slug", ok)
	tg.GET("/:slug/slots", ok)
	RegisterTeacherRoutes(tg, h)

	RegisterBookingRoutes(r.Group("/v1/bookings"), h, auth.RequireAuth(tm))
	return r
}

func do(r *gin.Engine, method, path, body, bearer string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	r.ServeHTTP(w, req)
	return w
}
