-- Course reviews (phase D2): a buyer's rating + comment for a course they are
-- enrolled in, plus the derived display aggregate on courses.rating /
-- review_count. See migration 000024 for the design notes.

-- name: InsertCourseReview :one
-- A duplicate (enrollment_id) raises SQLSTATE 23505 on the table's UNIQUE
-- constraint, which the repository maps to ErrAlreadyReviewed — race-safe,
-- never a check-then-insert.
INSERT INTO course_reviews (course_id, enrollment_id, student_id, rating, comment)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, course_id, enrollment_id, student_id, rating, comment, hidden, created_at, updated_at;

-- name: UpdateCourseReview :one
-- The author edits their own standing opinion. Scoped by student_id so an
-- edit can never touch someone else's row even if the id leaked.
UPDATE course_reviews
SET rating = sqlc.arg('rating'),
    comment = sqlc.arg('comment'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND student_id = sqlc.arg('student_id')
RETURNING id, course_id, enrollment_id, student_id, rating, comment, hidden, created_at, updated_at;

-- name: RecomputeCourseRating :exec
-- Rebuild courses.rating / review_count from the course's currently VISIBLE
-- reviews. Idempotent and order-independent — run it inside the same
-- transaction as any create / edit / hide / unhide / delete. A course with no
-- visible reviews reads as 0/0, which every surface renders as "no ratings
-- yet" rather than as a zero-star course.
WITH v AS (
    SELECT COALESCE(sum(rating), 0)::numeric AS sum_r,
           count(*)                          AS n
    FROM course_reviews
    WHERE course_id = sqlc.arg('course_id') AND NOT hidden
)
UPDATE courses c
SET review_count = v.n,
    rating = CASE
        WHEN v.n = 0 THEN 0
        ELSE LEAST(5, GREATEST(0, round(v.sum_r / v.n, 1)))::real
    END,
    updated_at = now()
FROM v
WHERE c.id = sqlc.arg('course_id');

-- name: GetCourseReviewByEnrollment :one
SELECT id, course_id, enrollment_id, student_id, rating, comment, hidden, created_at, updated_at
FROM course_reviews
WHERE enrollment_id = $1;

-- name: GetCourseReviewByID :one
SELECT id, course_id, enrollment_id, student_id, rating, comment, hidden, created_at, updated_at
FROM course_reviews
WHERE id = $1;

-- name: ListCourseReviews :many
-- One course's public review list: visible rows only, newest first.
SELECT cr.id, cr.rating, cr.comment, cr.created_at, cr.updated_at,
       u.display_name AS student_display_name
FROM course_reviews cr
JOIN users u ON u.id = cr.student_id
WHERE cr.course_id = sqlc.arg('course_id') AND NOT cr.hidden
ORDER BY cr.created_at DESC, cr.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountCourseReviews :one
SELECT count(*) FROM course_reviews
WHERE course_id = sqlc.arg('course_id') AND NOT hidden;

-- name: CourseRatingBreakdown :many
-- The "5★ ▓▓▓ 12" histogram under a course's rating. Visible rows only, and
-- only the star values that actually occur — the caller fills 1..5 with zeros.
SELECT rating, count(*)::bigint AS n
FROM course_reviews
WHERE course_id = sqlc.arg('course_id') AND NOT hidden
GROUP BY rating;

-- name: AdminListCourseReviews :many
-- The moderation queue: every course review regardless of visibility, newest
-- first, optionally narrowed to one visibility, one course, or a rating
-- ceiling (to surface the low-star reviews an operator is looking for).
SELECT cr.id, cr.course_id, cr.rating, cr.comment, cr.hidden, cr.created_at,
       c.title AS course_title,
       u.display_name AS student_display_name
FROM course_reviews cr
JOIN courses c ON c.id = cr.course_id
JOIN users u ON u.id = cr.student_id
WHERE (sqlc.narg('hidden')::boolean IS NULL OR cr.hidden = sqlc.narg('hidden')::boolean)
  AND (sqlc.narg('course_id')::uuid IS NULL OR cr.course_id = sqlc.narg('course_id')::uuid)
  AND (sqlc.narg('max_rating')::int IS NULL OR cr.rating <= sqlc.narg('max_rating')::int)
ORDER BY cr.created_at DESC, cr.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountCourseReviews :one
SELECT count(*)
FROM course_reviews cr
WHERE (sqlc.narg('hidden')::boolean IS NULL OR cr.hidden = sqlc.narg('hidden')::boolean)
  AND (sqlc.narg('course_id')::uuid IS NULL OR cr.course_id = sqlc.narg('course_id')::uuid)
  AND (sqlc.narg('max_rating')::int IS NULL OR cr.rating <= sqlc.narg('max_rating')::int);

-- name: SetCourseReviewHidden :one
UPDATE course_reviews
SET hidden = sqlc.arg('hidden'), updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, course_id, enrollment_id, student_id, rating, comment, hidden, created_at, updated_at;

-- name: DeleteAllCourseReviews :exec
DELETE FROM course_reviews;

-- name: AdminGetCourseReview :one
-- One moderation-queue row by id, with the course title and student name the
-- bare course_reviews row can't carry. Used to build the response after a
-- hide/unhide, whose RETURNING clause only sees the one table.
SELECT cr.id, cr.course_id, cr.rating, cr.comment, cr.hidden, cr.created_at,
       c.title AS course_title,
       u.display_name AS student_display_name
FROM course_reviews cr
JOIN courses c ON c.id = cr.course_id
JOIN users u ON u.id = cr.student_id
WHERE cr.id = $1;
