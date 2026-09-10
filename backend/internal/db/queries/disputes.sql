-- Disputes module (phase D): a participant contests a confirmed / completed
-- lesson, an operator with `disputes.resolve` closes it.
--
-- Like the reviews module, this file reads the booking context it needs
-- directly (the participant + status check) rather than routing through the
-- bookings service — the Go packages stay decoupled, the SQL joins once.

-- name: GetDisputeBookingContext :one
-- Everything POST/GET /v1/bookings/{id}/disputes needs to authorize the caller:
-- the booking's student, the account owning the teacher profile, and the status
-- (only confirmed / completed lessons can be disputed).
SELECT b.id, b.status, b.student_id,
       t.user_id AS teacher_user_id
FROM bookings b
JOIN teachers t ON t.id = b.teacher_id
WHERE b.id = $1;

-- name: InsertDispute :one
-- A second OPEN dispute for the same booking raises SQLSTATE 23505 on
-- disputes_one_open_per_booking, which the repository maps to ErrDisputeExists
-- (race-safe, never a check-then-insert).
INSERT INTO disputes (booking_id, raised_by, reason)
VALUES ($1, $2, $3)
RETURNING id, booking_id, raised_by, reason, status, resolution, resolved_by, created_at, resolved_at;

-- name: ListDisputesForBooking :many
-- The whole dispute thread for one booking, newest first.
SELECT d.id, d.booking_id, d.raised_by, d.reason, d.status, d.resolution,
       d.resolved_by, d.created_at, d.resolved_at,
       ru.display_name              AS raised_by_display_name,
       COALESCE(su.display_name, '')::text AS resolved_by_display_name
FROM disputes d
JOIN users ru ON ru.id = d.raised_by
LEFT JOIN users su ON su.id = d.resolved_by
WHERE d.booking_id = $1
ORDER BY d.created_at DESC, d.id;

-- name: GetOpenDisputeForBooking :one
-- The booking's open dispute, if any. Backs the `open_dispute` /
-- `can_raise_dispute` fields the participant sees on a booking.
SELECT d.id, d.booking_id, d.raised_by, d.reason, d.status, d.resolution,
       d.resolved_by, d.created_at, d.resolved_at,
       ru.display_name AS raised_by_display_name
FROM disputes d
JOIN users ru ON ru.id = d.raised_by
WHERE d.booking_id = $1 AND d.status = 'open';

-- name: GetDisputeByID :one
SELECT d.id, d.booking_id, d.raised_by, d.reason, d.status, d.resolution,
       d.resolved_by, d.created_at, d.resolved_at,
       ru.display_name                     AS raised_by_display_name,
       COALESCE(su.display_name, '')::text AS resolved_by_display_name
FROM disputes d
JOIN users ru ON ru.id = d.raised_by
LEFT JOIN users su ON su.id = d.resolved_by
WHERE d.id = $1;

-- name: ResolveDispute :one
-- Guarded UPDATE: only an OPEN dispute moves. No rows back means either "no such
-- dispute" or "already resolved" — the repository re-reads the row to tell the
-- two apart, so a lost race renders 409 already_resolved rather than clobbering
-- another operator's resolution.
UPDATE disputes
SET status      = sqlc.arg('status'),
    resolution  = sqlc.arg('resolution'),
    resolved_by = sqlc.arg('resolved_by')::uuid,
    resolved_at = now()
WHERE id = sqlc.arg('id') AND status = 'open'
RETURNING id, booking_id, raised_by, reason, status, resolution, resolved_by, created_at, resolved_at;

-- name: AdminListDisputes :many
-- The operator queue: disputes filtered by status (default 'open' in the
-- service), each with the booking + parties context the list view renders.
SELECT d.id, d.booking_id, d.raised_by, d.reason, d.status, d.resolution,
       d.resolved_by, d.created_at, d.resolved_at,
       ru.display_name                     AS raised_by_display_name,
       COALESCE(su.display_name, '')::text AS resolved_by_display_name,
       b.status                            AS booking_status,
       b.start_at                          AS booking_start_at,
       b.price_minor                       AS booking_price_minor,
       b.currency                          AS booking_currency,
       t.slug                              AS teacher_slug,
       t.display_name                      AS teacher_display_name,
       s.id                                AS student_id,
       s.email::text                       AS student_email,
       s.display_name                      AS student_display_name
FROM disputes d
JOIN users    ru ON ru.id = d.raised_by
LEFT JOIN users su ON su.id = d.resolved_by
JOIN bookings b  ON b.id = d.booking_id
JOIN teachers t  ON t.id = b.teacher_id
JOIN users    s  ON s.id = b.student_id
WHERE (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status')::text)
ORDER BY d.created_at DESC, d.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountDisputes :one
SELECT count(*)
FROM disputes d
WHERE (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status')::text);

-- name: SeedInsertDispute :exec
-- Seed-only: an open dispute on a seeded booking so /v1/admin/disputes is not
-- empty on a fresh database.
INSERT INTO disputes (booking_id, raised_by, reason)
VALUES ($1, $2, $3);

-- name: DeleteAllDisputes :exec
-- Seed-only. disputes.booking_id cascades, but raised_by / resolved_by
-- reference users with no cascade, so the seed clears disputes explicitly
-- before bookings and users.
DELETE FROM disputes;
