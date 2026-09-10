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

// fakeRepo is an in-memory Repository. It recomputes an in-memory teacher
// aggregate from a baseline folded with the visible booking-tied reviews —
// exactly as RecomputeTeacherRating does — so tests can assert the aggregate
// moved on create, hide, unhide, and remove.
type fakeRepo struct {
	bookings         map[uuid.UUID]BookingRef
	reviewsByBooking map[uuid.UUID]Review
	slugToID         map[string]uuid.UUID
	list             []Review

	// the displayed aggregate; the first mutating call snapshots it as the
	// immutable baseline, matching migration 000012.
	rating      float64
	reviewCount int
	ratingBase      float64
	reviewCountBase int
	baseCaptured    bool

	// every review, for the moderation queue + the recompute fold.
	adminList []adminRow

	created   []CreateParams
	createErr error
}

// adminRow is one stored review in the fake, carrying the teacher id the
// AdminReview DTO drops.
type adminRow struct {
	rec       AdminReview
	teacherID uuid.UUID
}

func (f *fakeRepo) captureBase() {
	if !f.baseCaptured {
		f.ratingBase, f.reviewCountBase = f.rating, f.reviewCount
		f.baseCaptured = true
	}
}

// recompute folds the baseline with the visible booking-tied reviews.
func (f *fakeRepo) recompute() {
	f.captureBase()
	var sum, n float64
	for _, a := range f.adminList {
		if a.rec.BookingID != nil && !a.rec.Hidden {
			sum += float64(a.rec.Rating)
			n++
		}
	}
	f.reviewCount = f.reviewCountBase + int(n)
	if f.reviewCountBase+int(n) == 0 {
		f.rating = 0
		return
	}
	next := (f.ratingBase*float64(f.reviewCountBase) + sum) / (float64(f.reviewCountBase) + n)
	f.rating = math.Max(0, math.Min(5, math.Round(next*10)/10))
}

// addExistingReview seeds a stored review for the moderation-queue tests.
func (f *fakeRepo) addExistingReview(r AdminReview, teacherID uuid.UUID) {
	f.adminList = append(f.adminList, adminRow{rec: r, teacherID: teacherID})
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

	bid := p.BookingID
	id := uuid.New()
	f.adminList = append(f.adminList, adminRow{
		rec: AdminReview{
			ID:          id,
			TeacherSlug: p.TeacherSlug,
			StudentName: p.StudentName,
			BookingID:   &bid,
			Rating:      p.Rating,
			Comment:     p.Comment,
			CreatedAt:   fixedNow,
		},
		teacherID: p.TeacherID,
	})
	f.recompute()

	r := Review{
		ID:          id,
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

func (f *fakeRepo) ListForModeration(_ context.Context, q ModerationQuery, limit, offset int) ([]AdminReview, int, error) {
	var all []AdminReview
	for _, a := range f.adminList {
		if q.Visibility == VisibilityVisible && a.rec.Hidden {
			continue
		}
		if q.Visibility == VisibilityHidden && !a.rec.Hidden {
			continue
		}
		if q.TeacherSlug != "" && a.rec.TeacherSlug != q.TeacherSlug {
			continue
		}
		if q.MaxRating != 0 && a.rec.Rating > q.MaxRating {
			continue
		}
		all = append(all, a.rec)
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

func (f *fakeRepo) SetReviewHidden(_ context.Context, reviewID uuid.UUID, hidden bool) (AdminReview, error) {
	for i := range f.adminList {
		if f.adminList[i].rec.ID == reviewID {
			f.adminList[i].rec.Hidden = hidden
			f.recompute()
			return f.adminList[i].rec, nil
		}
	}
	return AdminReview{}, ErrReviewNotFound
}

func (f *fakeRepo) RemoveReview(_ context.Context, reviewID uuid.UUID) error {
	for i := range f.adminList {
		if f.adminList[i].rec.ID == reviewID {
			f.adminList = append(f.adminList[:i], f.adminList[i+1:]...)
			f.recompute()
			return nil
		}
	}
	return ErrReviewNotFound
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
