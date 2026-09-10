-- Admin module (phase A/B): read-across-tables queries for the ops dashboard and
-- the moderation write path. This is an internal tool behind auth.RequireAdmin,
-- so it reads other modules' tables directly rather than routing through their
-- services.

-- The dashboard counters (GET /v1/admin/metrics) are six small aggregate reads,
-- one per area. Each is cheap (a single sequential scan of one table); the
-- dashboard is a rare, operator-only call.

-- name: AdminUserStats :one
SELECT
    count(*)                                                       AS total,
    count(*) FILTER (WHERE created_at >= now() - interval '7 days') AS this_week
FROM users;

-- name: AdminTeacherStats :one
SELECT
    count(*)                                     AS total,
    count(*) FILTER (WHERE status = 'pending')   AS pending,
    count(*) FILTER (WHERE status = 'approved')  AS approved,
    count(*) FILTER (WHERE verified)             AS verified
FROM teachers;

-- name: AdminBookingStats :one
-- gmv_minor = booking price that actually flowed: bookings that reached
-- confirmed or completed. this_week = created in the last 7 days. upcoming =
-- confirmed and not yet started. active_students = distinct people who booked.
SELECT
    count(*)                                                                                 AS total,
    count(*) FILTER (WHERE created_at >= now() - interval '7 days')                           AS this_week,
    count(*) FILTER (WHERE status = 'confirmed' AND start_at > now())                         AS upcoming,
    count(*) FILTER (WHERE status = 'completed')                                              AS completed,
    count(*) FILTER (WHERE status = 'cancelled')                                              AS cancelled,
    count(DISTINCT student_id)                                                                AS active_students,
    coalesce(sum(price_minor) FILTER (WHERE status IN ('confirmed', 'completed')), 0)::bigint AS gmv_minor
FROM bookings;

-- name: AdminPaymentStats :one
-- Money the platform actually moved: captured = collected from students,
-- refunded = returned to them.
SELECT
    coalesce(sum(amount_minor) FILTER (WHERE status = 'captured'), 0)::bigint AS captured_minor,
    coalesce(sum(amount_minor) FILTER (WHERE status = 'refunded'), 0)::bigint AS refunded_minor
FROM payments;

-- name: AdminPayoutStats :one
-- owed_minor = earned by teachers but not yet disbursed (held or cleared);
-- paid_minor = disbursed by past payout runs. Reversed rows count towards none.
SELECT
    coalesce(sum(amount_minor) FILTER (WHERE state IN ('held', 'available')), 0)::bigint AS owed_minor,
    coalesce(sum(amount_minor) FILTER (WHERE state = 'paid'), 0)::bigint                 AS paid_minor
FROM payout_ledger;

-- name: AdminReviewStats :one
-- Visible reviews only (hidden ones are off the platform). average_rating is
-- rounded to one decimal, 0 when there are none.
SELECT
    count(*) FILTER (WHERE NOT hidden)                                                   AS visible_total,
    coalesce(round(avg(rating) FILTER (WHERE NOT hidden), 1), 0)::double precision        AS average_rating
FROM reviews;

-- name: AdminOpenDisputeCount :one
SELECT count(*) FROM disputes WHERE status = 'open';

-- name: AdminListUsers :many
SELECT
    u.id, u.email, u.display_name, u.created_at,
    EXISTS (SELECT 1 FROM teachers t WHERE t.user_id = u.id)          AS is_teacher,
    (SELECT count(*) FROM bookings b WHERE b.student_id = u.id)::bigint AS booking_count
FROM users u
WHERE (
    sqlc.narg('q')::text IS NULL
    OR u.email ILIKE '%' || sqlc.narg('q') || '%'
    OR u.display_name ILIKE '%' || sqlc.narg('q') || '%'
)
ORDER BY u.created_at DESC, u.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountUsers :one
SELECT count(*)
FROM users u
WHERE (
    sqlc.narg('q')::text IS NULL
    OR u.email ILIKE '%' || sqlc.narg('q') || '%'
    OR u.display_name ILIKE '%' || sqlc.narg('q') || '%'
);

-- name: AdminGetUser :one
SELECT id, email, display_name, created_at FROM users WHERE id = $1;

-- name: AdminGetUserTeacherProfile :one
SELECT slug, status, verified FROM teachers WHERE user_id = $1;

-- name: AdminUserRoles :many
SELECT r.id, r.name
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.name;

-- name: AdminListUserBookings :many
-- The 50 newest bookings the user takes part in, as student or as teacher-owner.
SELECT
    b.id, b.status, b.start_at, b.price_minor, b.currency,
    b.student_id,
    t.user_id       AS teacher_user_id,
    t.display_name  AS teacher_display_name,
    s.display_name  AS student_display_name
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
JOIN users    s ON s.id = b.student_id
WHERE b.student_id = $1 OR t.user_id = $1
ORDER BY b.created_at DESC, b.id
LIMIT 50;

-- name: AdminUserPaymentsSummary :one
-- The user's payments as the paying student, bucketed by current payment state.
SELECT
    coalesce(sum(p.amount_minor) FILTER (WHERE p.status = 'authorized'), 0)::bigint AS authorized_minor,
    coalesce(sum(p.amount_minor) FILTER (WHERE p.status = 'captured'),   0)::bigint AS captured_minor,
    coalesce(sum(p.amount_minor) FILTER (WHERE p.status = 'refunded'),   0)::bigint AS refunded_minor,
    coalesce(max(p.currency), 'UZS')::text                                          AS currency
FROM payments p
JOIN bookings b ON b.id = p.booking_id
WHERE b.student_id = $1;

-- name: AdminListTeachers :many
SELECT
    t.slug, t.display_name, t.status, t.verified, t.headline, t.country_name, t.created_at,
    u.id    AS owner_id,
    u.email AS owner_email
FROM teachers t
LEFT JOIN users u ON u.id = t.user_id
WHERE (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status')::text)
  AND (
      sqlc.narg('q')::text IS NULL
      OR t.display_name ILIKE '%' || sqlc.narg('q') || '%'
      OR t.slug ILIKE '%' || sqlc.narg('q') || '%'
      OR u.email ILIKE '%' || sqlc.narg('q') || '%'
  )
ORDER BY t.created_at DESC, t.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountTeachers :one
SELECT count(*)
FROM teachers t
LEFT JOIN users u ON u.id = t.user_id
WHERE (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status')::text)
  AND (
      sqlc.narg('q')::text IS NULL
      OR t.display_name ILIKE '%' || sqlc.narg('q') || '%'
      OR t.slug ILIKE '%' || sqlc.narg('q') || '%'
      OR u.email ILIKE '%' || sqlc.narg('q') || '%'
  );

-- name: AdminGetTeacherModeration :one
SELECT
    t.id, t.status, t.verified, t.moderation_note,
    u.id           AS owner_id,
    u.email        AS owner_email,
    u.display_name AS owner_display_name
FROM teachers t
LEFT JOIN users u ON u.id = t.user_id
WHERE t.slug = $1;

-- name: AdminSetTeacherStatus :exec
UPDATE teachers
SET status = $2, moderation_note = $3, updated_at = now()
WHERE slug = $1;

-- name: AdminSetTeacherVerified :exec
UPDATE teachers
SET verified = $2, updated_at = now()
WHERE slug = $1;

-- --- phase D: bookings admin ---

-- name: AdminListBookings :many
-- Every booking on the platform (not just the caller's), newest lesson first.
-- `status` is an optional exact filter; `q` matches the teacher's display name
-- or slug and the student's email or display name. payment_status is '' when the
-- booking has no intent yet (payments.booking_id is UNIQUE, so the LEFT JOIN
-- cannot fan the row set out).
SELECT
    b.id, b.status, b.start_at, b.end_at, b.price_minor, b.currency, b.created_at,
    t.slug              AS teacher_slug,
    t.display_name      AS teacher_display_name,
    s.id                AS student_id,
    s.email::text       AS student_email,
    s.display_name      AS student_display_name,
    COALESCE(p.status, '')::text AS payment_status,
    EXISTS (SELECT 1 FROM disputes d WHERE d.booking_id = b.id AND d.status = 'open') AS has_open_dispute
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
JOIN users    s ON s.id = b.student_id
LEFT JOIN payments p ON p.booking_id = b.id
WHERE (sqlc.narg('status')::text IS NULL OR b.status = sqlc.narg('status')::text)
  AND (
      sqlc.narg('q')::text IS NULL
      OR t.display_name ILIKE '%' || sqlc.narg('q') || '%'
      OR t.slug ILIKE '%' || sqlc.narg('q') || '%'
      OR s.email ILIKE '%' || sqlc.narg('q') || '%'
      OR s.display_name ILIKE '%' || sqlc.narg('q') || '%'
  )
ORDER BY b.start_at DESC, b.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountBookings :one
SELECT count(*)
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
JOIN users    s ON s.id = b.student_id
WHERE (sqlc.narg('status')::text IS NULL OR b.status = sqlc.narg('status')::text)
  AND (
      sqlc.narg('q')::text IS NULL
      OR t.display_name ILIKE '%' || sqlc.narg('q') || '%'
      OR t.slug ILIKE '%' || sqlc.narg('q') || '%'
      OR s.email ILIKE '%' || sqlc.narg('q') || '%'
      OR s.display_name ILIKE '%' || sqlc.narg('q') || '%'
  );

-- name: AdminGetBooking :one
-- One booking with everything the operator detail view shows: the list-row
-- fields plus the lifecycle extras, the effective meeting link (the per-booking
-- override if set, else the teacher's default), and the payment detail.
SELECT
    b.id, b.status, b.start_at, b.end_at, b.duration_minutes, b.is_trial,
    b.price_minor, b.currency, b.created_at,
    b.cancelled_at, b.cancellation_reason, b.cancelled_by, b.no_show_party,
    COALESCE(NULLIF(b.meeting_url_override, ''), t.meeting_url)::text AS meeting_url,
    t.slug              AS teacher_slug,
    t.display_name      AS teacher_display_name,
    s.id                AS student_id,
    s.email::text       AS student_email,
    s.display_name      AS student_display_name,
    COALESCE(p.status, '')::text        AS payment_status,
    COALESCE(p.amount_minor, 0)::bigint AS payment_amount_minor,
    COALESCE(p.currency, '')::text      AS payment_currency,
    EXISTS (SELECT 1 FROM disputes d WHERE d.booking_id = b.id AND d.status = 'open') AS has_open_dispute
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
JOIN users    s ON s.id = b.student_id
LEFT JOIN payments p ON p.booking_id = b.id
WHERE b.id = $1;

-- name: AdminListBookingDisputes :many
-- The booking's full dispute thread, newest first. Read straight from the
-- disputes table: the admin surface is an ops tool, not a public API (see the
-- package doc), so it does not route this through the disputes service.
SELECT
    d.id, d.booking_id, d.raised_by, d.reason, d.status, d.resolution,
    d.resolved_by, d.created_at, d.resolved_at,
    ru.display_name                     AS raised_by_display_name,
    COALESCE(su.display_name, '')::text AS resolved_by_display_name
FROM disputes d
JOIN users ru ON ru.id = d.raised_by
LEFT JOIN users su ON su.id = d.resolved_by
WHERE d.booking_id = $1
ORDER BY d.created_at DESC, d.id;
