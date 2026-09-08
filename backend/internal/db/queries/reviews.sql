-- Reviews module (Phase 6): a student's rating + comment for a completed lesson.
-- A real review carries booking_id; the demo seed inserts booking-less samples.

-- name: GetReviewBookingContext :one
-- Everything the POST /v1/bookings/{id}/review flow needs to authorize the
-- caller and build the review: the booking's student + status and the teacher
-- it is for, plus the display names embedded in the response.
SELECT b.id, b.status, b.teacher_id, b.student_id,
       t.slug            AS teacher_slug,
       u.display_name    AS student_display_name
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
JOIN users    u ON u.id = b.student_id
WHERE b.id = $1;

-- name: InsertReview :one
-- A real, booking-tied review. A duplicate for the same booking_id raises
-- SQLSTATE 23505 on reviews_booking_uniq, which the repository maps to
-- ErrAlreadyReviewed (race-safe, never a check-then-insert).
INSERT INTO reviews (teacher_id, student_id, booking_id, rating, comment)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, teacher_id, student_id, booking_id, rating, comment, created_at;

-- name: BumpTeacherRatingForReview :exec
-- Incrementally fold one new rating into the teacher's display aggregate,
-- keeping the hand-set historical values as the baseline. All references to the
-- current row see the pre-UPDATE values, so review_count is the old count in
-- both expressions. Result clamped to [0, 5].
UPDATE teachers
SET rating = LEAST(5, GREATEST(0,
        round(((rating::numeric * review_count) + sqlc.arg('new_rating')::int)
              / (review_count + 1), 1)))::real,
    review_count = review_count + 1,
    updated_at = now()
WHERE id = sqlc.arg('teacher_id');

-- name: GetReviewByBooking :one
-- The review for a booking (for embedding in a BookingDTO). No rows -> the
-- booking has not been reviewed.
SELECT r.id, r.teacher_id, r.student_id, r.booking_id, r.rating, r.comment, r.created_at,
       t.slug            AS teacher_slug,
       u.display_name    AS student_display_name
FROM reviews r
JOIN teachers t ON t.id = r.teacher_id
JOIN users    u ON u.id = r.student_id
WHERE r.booking_id = $1;

-- name: ListTeacherReviews :many
-- A page of a teacher's reviews, newest first.
SELECT r.id, r.rating, r.comment, r.created_at,
       u.display_name AS student_display_name
FROM reviews r
JOIN users u ON u.id = r.student_id
WHERE r.teacher_id = $1
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountTeacherReviews :one
SELECT count(*) FROM reviews WHERE teacher_id = $1;

-- name: SeedInsertReview :exec
-- Seed-only: a booking-less sample review. Does NOT touch the teacher aggregate.
INSERT INTO reviews (teacher_id, student_id, booking_id, rating, comment)
VALUES ($1, $2, NULL, $3, $4);

-- name: DeleteAllReviews :exec
-- Seed-only. reviews references teachers / users / bookings with no cascade, so
-- the seed clears it before all three.
DELETE FROM reviews;
