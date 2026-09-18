package courses

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeReviewRepo is an in-memory ReviewRepository. It reproduces the two
// behaviours the SQL guarantees and the service depends on: one review per
// enrollment (the UNIQUE), and a rating aggregate rebuilt from the VISIBLE
// rows on every write.
type fakeReviewRepo struct {
	reviews map[uuid.UUID]Review
	courses *fakeRepo // to write the recomputed aggregate back onto the course
}

func newFakeReviewRepo(courses *fakeRepo) *fakeReviewRepo {
	return &fakeReviewRepo{reviews: map[uuid.UUID]Review{}, courses: courses}
}

func (r *fakeReviewRepo) recompute(courseID uuid.UUID) {
	var sum, n int
	for _, rv := range r.reviews {
		if rv.CourseID == courseID && !rv.Hidden {
			sum += rv.Rating
			n++
		}
	}
	c := r.courses.courses[courseID]
	c.ReviewCount = n
	if n == 0 {
		c.Rating = 0
	} else {
		// Match the SQL's round-to-one-decimal.
		c.Rating = float64(int((float64(sum)/float64(n))*10+0.5)) / 10
	}
	r.courses.courses[courseID] = c
}

func (r *fakeReviewRepo) CreateReview(_ context.Context, p CreateReviewParams) (Review, error) {
	for _, rv := range r.reviews {
		if rv.EnrollmentID == p.EnrollmentID {
			return Review{}, ErrAlreadyReviewed
		}
	}
	rv := Review{
		ID: uuid.New(), CourseID: p.CourseID, EnrollmentID: p.EnrollmentID,
		StudentID: p.StudentID, Rating: p.Rating, Comment: p.Comment,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	r.reviews[rv.ID] = rv
	r.recompute(p.CourseID)
	return rv, nil
}

func (r *fakeReviewRepo) UpdateReview(_ context.Context, reviewID, studentID uuid.UUID, rating int, comment string) (Review, error) {
	rv, ok := r.reviews[reviewID]
	if !ok || rv.StudentID != studentID {
		return Review{}, ErrReviewNotFound
	}
	rv.Rating, rv.Comment, rv.UpdatedAt = rating, comment, time.Now()
	r.reviews[reviewID] = rv
	r.recompute(rv.CourseID)
	return rv, nil
}

func (r *fakeReviewRepo) ReviewByEnrollment(_ context.Context, enrollmentID uuid.UUID) (Review, bool, error) {
	for _, rv := range r.reviews {
		if rv.EnrollmentID == enrollmentID {
			return rv, true, nil
		}
	}
	return Review{}, false, nil
}

func (r *fakeReviewRepo) ListReviews(_ context.Context, courseID uuid.UUID, limit, offset int) (ReviewPage, error) {
	var visible []Review
	var breakdown [5]int
	for _, rv := range r.reviews {
		if rv.CourseID == courseID && !rv.Hidden {
			visible = append(visible, rv)
			breakdown[rv.Rating-1]++
		}
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].CreatedAt.After(visible[j].CreatedAt) })
	total := len(visible)
	if offset > len(visible) {
		offset = len(visible)
	}
	end := offset + limit
	if end > len(visible) {
		end = len(visible)
	}
	return ReviewPage{Reviews: visible[offset:end], Total: total, Breakdown: breakdown}, nil
}

func (r *fakeReviewRepo) AdminListReviews(_ context.Context, q AdminReviewQuery, limit, offset int) ([]AdminReview, int, error) {
	var out []AdminReview
	for _, rv := range r.reviews {
		if q.Visibility == ReviewVisibilityVisible && rv.Hidden {
			continue
		}
		if q.Visibility == ReviewVisibilityHidden && !rv.Hidden {
			continue
		}
		if q.CourseID != nil && rv.CourseID != *q.CourseID {
			continue
		}
		if q.MaxRating > 0 && rv.Rating > q.MaxRating {
			continue
		}
		out = append(out, AdminReview{
			ID: rv.ID, CourseID: rv.CourseID, Rating: rv.Rating,
			Comment: rv.Comment, Hidden: rv.Hidden, CreatedAt: rv.CreatedAt,
		})
	}
	return out, len(out), nil
}

func (r *fakeReviewRepo) SetReviewHidden(_ context.Context, reviewID uuid.UUID, hidden bool) (AdminReview, error) {
	rv, ok := r.reviews[reviewID]
	if !ok {
		return AdminReview{}, ErrReviewNotFound
	}
	rv.Hidden = hidden
	r.reviews[reviewID] = rv
	r.recompute(rv.CourseID)
	return AdminReview{ID: rv.ID, CourseID: rv.CourseID, Rating: rv.Rating, Hidden: hidden}, nil
}

// reviewEnv is a published course, its owner, and a buyer already enrolled.
type reviewEnv struct {
	*testEnv
	rrepo    *fakeReviewRepo
	owner    uuid.UUID
	buyer    uuid.UUID
	courseID uuid.UUID
}

func newReviewEnv(t *testing.T) *reviewEnv {
	t.Helper()
	ctx := context.Background()
	e := newTestEnv()
	rr := newFakeReviewRepo(e.repo)
	e.svc.SetReviewRepository(rr)

	owner, _ := e.seedTeacher()
	d := mustCreate(t, e, owner, "Reviewable course")
	// Publishing needs a non-empty curriculum.
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	videoID := uuid.New()
	e.file.put(videoID, owner, "video/mp4")
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, d.Sections[0].Section.ID,
		ItemKindVideo, "Lesson", &videoID, nil, false, 120); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if _, err := e.svc.SetPublished(ctx, owner, d.Course.ID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}

	buyer := uuid.New()
	if _, err := e.repo.EnsureEnrollment(ctx, d.Course.ID, buyer, EnrollmentFree, 0, "UZS"); err != nil {
		t.Fatalf("enroll: %v", err)
	}
	return &reviewEnv{testEnv: e, rrepo: rr, owner: owner, buyer: buyer, courseID: d.Course.ID}
}

func TestService_CreateReview_Eligibility(t *testing.T) {
	ctx := context.Background()

	t.Run("an enrolled buyer can review", func(t *testing.T) {
		e := newReviewEnv(t)
		r, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 5, "  Excellent  ")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if r.Rating != 5 || r.Comment != "Excellent" {
			t.Errorf("comment should be trimmed: %+v", r)
		}
	})

	t.Run("a non-buyer cannot", func(t *testing.T) {
		e := newReviewEnv(t)
		if _, err := e.svc.CreateReview(ctx, uuid.New(), e.courseID, 5, ""); !errors.Is(err, ErrNotEnrolled) {
			t.Errorf("want ErrNotEnrolled, got %v", err)
		}
	})

	t.Run("the owner cannot review their own course", func(t *testing.T) {
		e := newReviewEnv(t)
		if _, err := e.svc.CreateReview(ctx, e.owner, e.courseID, 5, ""); !errors.Is(err, ErrCannotReviewOwnCourse) {
			t.Errorf("want ErrCannotReviewOwnCourse, got %v", err)
		}
	})

	t.Run("a second review is a conflict, not a duplicate row", func(t *testing.T) {
		e := newReviewEnv(t)
		if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 4, "first"); err != nil {
			t.Fatalf("first: %v", err)
		}
		if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 1, "second"); !errors.Is(err, ErrAlreadyReviewed) {
			t.Errorf("want ErrAlreadyReviewed, got %v", err)
		}
	})

	t.Run("rating and comment are validated", func(t *testing.T) {
		e := newReviewEnv(t)
		for _, bad := range []int{0, 6, -1} {
			if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, bad, ""); err == nil {
				t.Errorf("rating %d should be rejected", bad)
			} else {
				asValidationError(t, err)
			}
		}
		long := make([]rune, MaxReviewCommentLen+1)
		for i := range long {
			long[i] = 'a'
		}
		if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 5, string(long)); err == nil {
			t.Error("an over-long comment should be rejected")
		} else {
			asValidationError(t, err)
		}
	})
}

func TestService_UpdateReview(t *testing.T) {
	ctx := context.Background()
	e := newReviewEnv(t)

	if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 2, "rough start"); err != nil {
		t.Fatalf("create: %v", err)
	}
	updated, err := e.svc.UpdateReview(ctx, e.buyer, e.courseID, 5, "it grew on me")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Rating != 5 || updated.Comment != "it grew on me" {
		t.Errorf("unexpected: %+v", updated)
	}

	// A buyer who hasn't written one yet gets 404, not a silent insert.
	e2 := newReviewEnv(t)
	if _, err := e2.svc.UpdateReview(ctx, e2.buyer, e2.courseID, 5, ""); !errors.Is(err, ErrReviewNotFound) {
		t.Errorf("want ErrReviewNotFound, got %v", err)
	}
}

// The aggregate is the whole point of D2: it must track creates, edits and
// moderation, and it must ignore hidden rows.
func TestService_RatingAggregate(t *testing.T) {
	ctx := context.Background()
	e := newReviewEnv(t)

	rating := func() (float64, int) {
		c := e.repo.courses[e.courseID]
		return c.Rating, c.ReviewCount
	}

	if r, n := rating(); r != 0 || n != 0 {
		t.Errorf("a course with no reviews must read 0/0, got %v/%d", r, n)
	}

	// Three buyers, 5 + 4 + 3 -> 4.0 average.
	buyers := []uuid.UUID{e.buyer, uuid.New(), uuid.New()}
	for _, b := range buyers[1:] {
		if _, err := e.repo.EnsureEnrollment(ctx, e.courseID, b, EnrollmentFree, 0, "UZS"); err != nil {
			t.Fatalf("enroll: %v", err)
		}
	}
	for i, b := range buyers {
		if _, err := e.svc.CreateReview(ctx, b, e.courseID, 5-i, ""); err != nil {
			t.Fatalf("review %d: %v", i, err)
		}
	}
	if r, n := rating(); r != 4.0 || n != 3 {
		t.Errorf("want 4.0/3, got %v/%d", r, n)
	}

	// An edit moves it: 1 + 4 + 3 -> 2.7.
	if _, err := e.svc.UpdateReview(ctx, buyers[0], e.courseID, 1, ""); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if r, n := rating(); r != 2.7 || n != 3 {
		t.Errorf("want 2.7/3 after the edit, got %v/%d", r, n)
	}

	// Hiding the 1★ drops it out of both the count and the average: 4 + 3 -> 3.5.
	page, err := e.svc.Reviews(ctx, e.courseID, 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var oneStar uuid.UUID
	for _, rv := range e.rrepo.reviews {
		if rv.Rating == 1 {
			oneStar = rv.ID
		}
	}
	if _, err := e.svc.SetReviewHidden(ctx, oneStar, true); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if r, n := rating(); r != 3.5 || n != 2 {
		t.Errorf("want 3.5/2 after hiding, got %v/%d", r, n)
	}

	// ...and unhiding restores it exactly.
	if _, err := e.svc.SetReviewHidden(ctx, oneStar, false); err != nil {
		t.Fatalf("unhide: %v", err)
	}
	if r, n := rating(); r != 2.7 || n != 3 {
		t.Errorf("unhide must be the exact inverse, got %v/%d", r, n)
	}

	// The public list and histogram only ever show visible rows.
	if _, err := e.svc.SetReviewHidden(ctx, oneStar, true); err != nil {
		t.Fatalf("hide: %v", err)
	}
	page, err = e.svc.Reviews(ctx, e.courseID, 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 2 || len(page.Reviews) != 2 {
		t.Errorf("hidden review must not appear: total=%d len=%d", page.Total, len(page.Reviews))
	}
	if page.Breakdown[0] != 0 {
		t.Errorf("the hidden 1★ must not be in the histogram: %v", page.Breakdown)
	}
	if page.Breakdown[3] != 1 || page.Breakdown[2] != 1 {
		t.Errorf("histogram should hold one 4★ and one 3★: %v", page.Breakdown)
	}
}

// A course that isn't on the storefront must not leak its reviews.
func TestService_Reviews_StorefrontGate(t *testing.T) {
	ctx := context.Background()
	e := newReviewEnv(t)
	if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 5, "great"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := e.svc.SetPublished(ctx, e.owner, e.courseID, false); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if _, err := e.svc.Reviews(ctx, e.courseID, 1, 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unpublished course must not serve reviews, got %v", err)
	}

	// But its existing buyer can still review/edit — they paid for it, and
	// their opinion did not stop being true.
	if _, err := e.svc.UpdateReview(ctx, e.buyer, e.courseID, 4, "still fine"); err != nil {
		t.Errorf("an existing buyer should still be able to edit: %v", err)
	}
}

// A nil ReviewRepository must fail closed, like every other guarded port here.
func TestService_Reviews_NilRepositoryFailsClosed(t *testing.T) {
	ctx := context.Background()
	e := newReviewEnv(t)
	e.svc.reviews = nil

	if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 5, ""); !errors.Is(err, ErrReviewsUnavailable) {
		t.Errorf("create: want ErrReviewsUnavailable, got %v", err)
	}
	if _, err := e.svc.Reviews(ctx, e.courseID, 1, 10); !errors.Is(err, ErrReviewsUnavailable) {
		t.Errorf("list: want ErrReviewsUnavailable, got %v", err)
	}
	if _, _, err := e.svc.MyReview(ctx, e.buyer, e.courseID); err != nil {
		t.Errorf("MyReview should degrade to 'no review', not error: %v", err)
	}
}

// The landing page hands a buyer their own review back so it can offer "edit"
// rather than a "write one" button that would 409.
func TestService_CatalogDetail_MyReview(t *testing.T) {
	ctx := context.Background()
	e := newReviewEnv(t)

	d, err := e.svc.CatalogDetail(ctx, e.buyer, e.courseID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.MyReview != nil {
		t.Error("a buyer who hasn't reviewed should have no my_review")
	}

	if _, err := e.svc.CreateReview(ctx, e.buyer, e.courseID, 4, "solid"); err != nil {
		t.Fatalf("create: %v", err)
	}
	d, err = e.svc.CatalogDetail(ctx, e.buyer, e.courseID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.MyReview == nil || d.MyReview.Rating != 4 {
		t.Errorf("want the buyer's own review back, got %+v", d.MyReview)
	}
	if d.Course.Rating != 4.0 || d.Course.ReviewCount != 1 {
		t.Errorf("detail should carry the aggregate, got %v/%d", d.Course.Rating, d.Course.ReviewCount)
	}

	// An anonymous viewer never gets one.
	d, err = e.svc.CatalogDetail(ctx, uuid.Nil, e.courseID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.MyReview != nil {
		t.Error("an anonymous viewer must not receive a my_review")
	}
}
