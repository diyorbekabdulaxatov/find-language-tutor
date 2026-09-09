-- Booking module: concrete scheduled lessons. Times are UTC timestamptz. The
-- recurring weekly availability is read via the availability queries; here we
-- only need the teacher context, existing bookings for overlap checks, and the
-- booking rows themselves (joined to a light teacher + student summary so the
-- frontend lists avoid N+1 calls).

-- name: GetBookingTeacherContext :one
-- Slug -> everything the booking flow needs: identity, timezone, pricing, and
-- the owning account (drives the "can't book yourself" check).
-- Only an approved (publicly visible) teacher can be booked. A non-approved slug
-- returns no rows here, so the booking flow treats it as "no such teacher": 404
-- on GET /v1/teachers/{slug}/slots and 404 teacher_not_found on POST /v1/bookings.
SELECT id, slug, display_name, timezone, avatar_url,
       price_per_hour_minor, trial_price_minor, currency, user_id, meeting_url
FROM teachers
WHERE slug = $1 AND status = 'approved';

-- name: GetTeacherIDByOwner :one
-- The teacher profile owned by an account (one per user), or no rows.
SELECT id FROM teachers WHERE user_id = $1;

-- name: ListTeacherBookingIntervals :many
-- Non-cancelled bookings for a teacher that overlap the [from, to) window, for
-- server-side slot generation and the pre-insert bookability re-check.
SELECT start_at, end_at
FROM bookings
WHERE teacher_id = $1
  AND status <> 'cancelled'
  AND start_at < sqlc.arg('window_end')
  AND end_at   > sqlc.arg('window_start')
ORDER BY start_at;

-- name: CreateBooking :one
INSERT INTO bookings (
    teacher_id, student_id, start_at, end_at,
    duration_minutes, status, price_minor, currency, is_trial
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9
)
RETURNING id;

-- name: GetBookingByID :one
SELECT
    b.id, b.teacher_id, b.student_id, b.start_at, b.end_at,
    b.duration_minutes, b.status, b.price_minor, b.currency, b.is_trial,
    b.cancelled_at, b.cancellation_reason, b.created_at, b.updated_at,
    b.meeting_url_override,
    b.no_show_party,
    t.slug            AS teacher_slug,
    t.display_name    AS teacher_display_name,
    t.timezone        AS teacher_timezone,
    t.avatar_url      AS teacher_avatar_url,
    t.user_id         AS teacher_user_id,
    t.meeting_url     AS teacher_meeting_url,
    COALESCE(tu.email, '')::text AS teacher_email,
    u.display_name    AS student_display_name,
    u.email::text     AS student_email
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
JOIN users    u ON u.id = b.student_id
LEFT JOIN users tu ON tu.id = t.user_id
WHERE b.id = $1;

-- name: ListBookings :many
-- Bookings the caller participates in. Pass the caller's user id as
-- student_filter and/or the caller-owned teacher id as teacher_filter; use the
-- all-zero uuid for a dimension that should not match. Newest lesson first.
SELECT
    b.id, b.teacher_id, b.student_id, b.start_at, b.end_at,
    b.duration_minutes, b.status, b.price_minor, b.currency, b.is_trial,
    b.cancelled_at, b.cancellation_reason, b.created_at, b.updated_at,
    b.meeting_url_override,
    b.no_show_party,
    t.slug            AS teacher_slug,
    t.display_name    AS teacher_display_name,
    t.timezone        AS teacher_timezone,
    t.avatar_url      AS teacher_avatar_url,
    t.user_id         AS teacher_user_id,
    t.meeting_url     AS teacher_meeting_url,
    COALESCE(tu.email, '')::text AS teacher_email,
    u.display_name    AS student_display_name,
    u.email::text     AS student_email
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
JOIN users    u ON u.id = b.student_id
LEFT JOIN users tu ON tu.id = t.user_id
WHERE (b.student_id = sqlc.arg('student_filter') OR b.teacher_id = sqlc.arg('teacher_filter'))
  AND (sqlc.narg('status')::text IS NULL OR b.status = sqlc.narg('status')::text)
ORDER BY b.start_at DESC, b.id;

-- name: SetBookingStatus :exec
UPDATE bookings SET status = $2, updated_at = now() WHERE id = $1;

-- name: SetBookingMeetingLinkOverride :exec
-- Per-booking meeting link override. An empty string clears it (fall back to the
-- teacher's default meeting_url).
UPDATE bookings SET meeting_url_override = $2, updated_at = now() WHERE id = $1;

-- name: SetBookingNoShowParty :exec
-- Records who missed the lesson. The status transition (completed on a student
-- no-show, cancelled on a teacher no-show) is applied separately via
-- SetBookingStatus / CancelBooking so the capture / refund path is reused.
UPDATE bookings SET no_show_party = $2, updated_at = now() WHERE id = $1;

-- name: CancelBooking :exec
UPDATE bookings
SET status = 'cancelled', cancelled_at = now(), cancellation_reason = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteAllBookings :exec
-- Seed-only. bookings.teacher_id / student_id reference teachers / users with
-- no ON DELETE CASCADE, so the seed must clear bookings before those tables.
DELETE FROM bookings;
