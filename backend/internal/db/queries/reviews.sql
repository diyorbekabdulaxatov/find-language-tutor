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

-- name: ApprovedTeacherIDBySlug :one
-- Slug -> id, but only for a publicly visible (approved) teacher. GET
-- /v1/teachers/{slug}/reviews 404s for a non-approved slug, same as the profile.
SELECT id FROM teachers WHERE slug = $1 AND status = 'approved';

-- name: InsertReview :one
-- A real, booking-tied review. A duplicate for the same booking_id raises
-- SQLSTATE 23505 on reviews_booking_uniq, which the repository maps to
-- ErrAlreadyReviewed (race-safe, never a check-then-insert).
INSERT INTO reviews (teacher_id, student_id, booking_id, rating, comment)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, teacher_id, student_id, booking_id, rating, comment, created_at;

-- name: RecomputeTeacherRating :exec
-- Rebuild a teacher's display aggregate (teachers.rating / review_count) from
-- the immutable historical baseline (rating_base / review_count_base) folded
-- with their currently VISIBLE, booking-tied reviews. Idempotent and
-- order-independent — run it after a review is created, hidden, unhidden, or
-- removed. Result clamped to [0, 5]; a teacher with no baseline and no visible
-- reviews reads as 0.
WITH v AS (
    SELECT COALESCE(sum(rating), 0)::numeric AS sum_r,
           count(*)                          AS n
    FROM reviews
    WHERE teacher_id = sqlc.arg('teacher_id')
      AND booking_id IS NOT NULL
      AND NOT hidden
)
UPDATE teachers t
SET review_count = t.review_count_base + v.n,
    rating = CASE
        WHEN t.review_count_base + v.n = 0 THEN 0
        ELSE LEAST(5, GREATEST(0, round(
            (t.rating_base::numeric * t.review_count_base + v.sum_r)
            / (t.review_count_base + v.n), 1)))::real
    END,
    updated_at = now()
FROM v
WHERE t.id = sqlc.arg('teacher_id');

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
-- A page of a teacher's PUBLIC reviews, newest first. Hidden reviews (removed
-- from display by an operator) never appear on the profile.
SELECT r.id, r.rating, r.comment, r.created_at,
       u.display_name AS student_display_name
FROM reviews r
JOIN users u ON u.id = r.student_id
WHERE r.teacher_id = $1 AND NOT r.hidden
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountTeacherReviews :one
SELECT count(*) FROM reviews WHERE teacher_id = $1 AND NOT hidden;

-- name: SeedInsertReview :exec
-- Seed-only: a booking-less sample review. Does NOT touch the teacher aggregate.
INSERT INTO reviews (teacher_id, student_id, booking_id, rating, comment, hidden)
VALUES ($1, $2, NULL, sqlc.arg('rating'), sqlc.arg('comment'), sqlc.arg('hidden'));

-- --- Admin: review moderation (phase F) ---

-- name: AdminListReviews :many
-- A page of reviews for the moderation queue, newest first. Filters are all
-- optional: visibility ('visible' | 'hidden'; NULL = both), a teacher slug, and
-- a rating ceiling (to surface the low-star reviews an operator is looking for).
SELECT r.id, r.rating, r.comment, r.created_at, r.hidden, r.booking_id,
       t.slug         AS teacher_slug,
       t.display_name AS teacher_display_name,
       u.display_name AS student_display_name
FROM reviews r
JOIN teachers t ON t.id = r.teacher_id
JOIN users    u ON u.id = r.student_id
WHERE (
        sqlc.narg('visibility')::text IS NULL
        OR (sqlc.narg('visibility')::text = 'visible' AND NOT r.hidden)
        OR (sqlc.narg('visibility')::text = 'hidden'  AND r.hidden)
      )
  AND (sqlc.narg('teacher_slug')::text IS NULL OR t.slug = sqlc.narg('teacher_slug')::text)
  AND (sqlc.narg('max_rating')::int IS NULL OR r.rating <= sqlc.narg('max_rating')::int)
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountReviews :one
-- Same filters as AdminListReviews — the total match count for pagination.
SELECT count(*)
FROM reviews r
JOIN teachers t ON t.id = r.teacher_id
WHERE (
        sqlc.narg('visibility')::text IS NULL
        OR (sqlc.narg('visibility')::text = 'visible' AND NOT r.hidden)
        OR (sqlc.narg('visibility')::text = 'hidden'  AND r.hidden)
      )
  AND (sqlc.narg('teacher_slug')::text IS NULL OR t.slug = sqlc.narg('teacher_slug')::text)
  AND (sqlc.narg('max_rating')::int IS NULL OR r.rating <= sqlc.narg('max_rating')::int);

-- name: AdminGetReview :one
SELECT r.id, r.teacher_id, r.rating, r.comment, r.created_at, r.hidden, r.booking_id,
       t.slug         AS teacher_slug,
       t.display_name AS teacher_display_name,
       u.display_name AS student_display_name
FROM reviews r
JOIN teachers t ON t.id = r.teacher_id
JOIN users    u ON u.id = r.student_id
WHERE r.id = $1;

-- name: SetReviewHidden :exec
UPDATE reviews SET hidden = sqlc.arg('hidden') WHERE id = sqlc.arg('id');

-- name: DeleteReview :exec
DELETE FROM reviews WHERE id = $1;

-- name: DeleteAllReviews :exec
-- Seed-only. reviews references teachers / users / bookings with no cascade, so
-- the seed clears it before all three.
DELETE FROM reviews;
