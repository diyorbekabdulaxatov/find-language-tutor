-- Payments module: one payment intent per booking, the webhook-event log that
-- guards idempotency, and the simplified teacher-earnings ledger. Money is
-- integer minor units everywhere.

-- name: CreatePayment :one
-- Eagerly created right after a booking is inserted. Idempotent: a second call
-- for the same booking is a no-op and still returns the existing row.
INSERT INTO payments (booking_id, provider, status, amount_minor, currency)
VALUES ($1, $2, 'requires_payment', $3, $4)
ON CONFLICT (booking_id) DO UPDATE SET booking_id = payments.booking_id
RETURNING id, booking_id, provider, provider_ref, status, amount_minor, currency,
          last_error, authorized_at, captured_at, refunded_at, created_at, updated_at;

-- name: GetPaymentByBooking :one
SELECT id, booking_id, provider, provider_ref, status, amount_minor, currency,
       last_error, authorized_at, captured_at, refunded_at, created_at, updated_at
FROM payments
WHERE booking_id = $1;

-- name: GetPaymentByID :one
SELECT id, booking_id, provider, provider_ref, status, amount_minor, currency,
       last_error, authorized_at, captured_at, refunded_at, created_at, updated_at
FROM payments
WHERE id = $1;

-- name: MarkPaymentAuthorized :exec
UPDATE payments
SET status = 'authorized', provider_ref = $2, last_error = '',
    authorized_at = now(), updated_at = now()
WHERE id = $1;

-- name: MarkPaymentCaptured :exec
UPDATE payments
SET status = 'captured', captured_at = now(), updated_at = now()
WHERE id = $1;

-- name: MarkPaymentRefunded :exec
UPDATE payments
SET status = 'refunded', refunded_at = now(), updated_at = now()
WHERE id = $1;

-- name: MarkPaymentFailed :exec
UPDATE payments
SET status = 'failed', last_error = $2, updated_at = now()
WHERE id = $1;

-- name: InsertPaymentEvent :exec
-- The idempotency gate. A duplicate event_id raises SQLSTATE 23505, which the
-- repository treats as "already processed".
INSERT INTO payment_events (event_id, payment_id, type)
VALUES ($1, $2, $3);

-- name: ConfirmBookingForPayment :exec
-- System transition (no participant check): pending_payment -> confirmed on a
-- successful authorization webhook. Guarded so a replay cannot resurrect a
-- cancelled booking.
UPDATE bookings
SET status = 'confirmed', updated_at = now()
WHERE id = $1 AND status = 'pending_payment';

-- name: InsertLedgerHeld :exec
-- One row per captured booking. teacher_id is copied from the booking, and
-- available_at opens the clearing window: capture time + PAYOUTS_CLEARING_DAYS.
-- The row stays 'held' until a payout run settles it (phase E) — nothing flips
-- it to 'available'; that state is derived from available_at at read time.
INSERT INTO payout_ledger (teacher_id, booking_id, amount_minor, currency, state, available_at)
SELECT b.teacher_id, b.id,
       sqlc.arg('amount_minor')::bigint,
       sqlc.arg('currency')::text,
       'held',
       now() + make_interval(days => sqlc.arg('clearing_days')::int)
FROM bookings b
WHERE b.id = sqlc.arg('booking_id')
ON CONFLICT (booking_id) DO NOTHING;

-- name: MarkLedgerReversed :exec
-- A refund reverses the teacher's earning — unless a payout batch already paid
-- it out, which cannot be un-paid from here (the money has left the platform).
UPDATE payout_ledger
SET state = 'reversed', updated_at = now()
WHERE booking_id = $1 AND state NOT IN ('reversed', 'paid');

-- name: InsertCourseLedgerHeld :exec
-- Phase C3: the course-purchase sibling of InsertLedgerHeld. One row per
-- captured course sale, keyed on course_enrollment_id instead of booking_id
-- (see payout_ledger's booking_id/course_enrollment_id CHECK, migration
-- 000018). teacher_id is derived by joining the enrollment to its course;
-- amount_minor is the teacher's already-computed revenue-share (Go arithmetic
-- in payments.CourseService.CreditCourseSale, never derived here), and
-- available_at opens the same clearing window as a lesson's earning: capture
-- time + PAYOUTS_CLEARING_DAYS.
--
-- Called by courses.Service.Purchase (via the courses.PaymentGateway port)
-- right after the enrollment row is created, NOT from this package's own
-- ApplyCourseEvent webhook handler on payment.captured — unlike a booking
-- (which exists before its payment), a course_enrollment does not exist yet
-- at capture time, so there is no enrollment id to attach a ledger row to
-- until courses.Service creates one. See CreditCourseSale's doc comment.
--
-- ON CONFLICT (course_enrollment_id) DO NOTHING is the idempotency guard: a
-- retried/duplicate call for the same enrollment (e.g. a concurrent
-- double-submit racing EnsureEnrollment) never double-credits the teacher.
INSERT INTO payout_ledger (teacher_id, course_enrollment_id, amount_minor, currency, state, available_at)
SELECT co.teacher_id, ce.id,
       sqlc.arg('amount_minor')::bigint,
       sqlc.arg('currency')::text,
       'held',
       now() + make_interval(days => sqlc.arg('clearing_days')::int)
FROM course_enrollments ce
JOIN courses co ON co.id = ce.course_id
WHERE ce.id = sqlc.arg('course_enrollment_id')
ON CONFLICT (course_enrollment_id) DO NOTHING;

-- name: ListTeacherEarnings :many
-- Phase C3 widens this to a teacher's course-sale earnings alongside their
-- lesson earnings: LEFT JOIN both sources (a row's booking_id XOR
-- course_enrollment_id is set, per the ledger's own CHECK) and coalesce the
-- "when" and "counterparty name" columns so a client sees one uniform shape.
-- course_title is NULL for a lesson row, non-NULL for a course row — the
-- discriminator a client uses to render each line differently.
SELECT pl.booking_id, pl.course_enrollment_id, pl.amount_minor, pl.currency, pl.state, pl.available_at,
       coalesce(b.start_at, ce.created_at) AS start_at,
       coalesce(bu.display_name, cu.display_name) AS student_display_name,
       co.title AS course_title
FROM payout_ledger pl
LEFT JOIN bookings b ON b.id = pl.booking_id
LEFT JOIN users bu ON bu.id = b.student_id
LEFT JOIN course_enrollments ce ON ce.id = pl.course_enrollment_id
LEFT JOIN courses co ON co.id = ce.course_id
LEFT JOIN users cu ON cu.id = ce.student_id
WHERE pl.teacher_id = $1
ORDER BY coalesce(b.start_at, ce.created_at) DESC, pl.id;

-- name: SeedInsertCapturedPayment :exec
-- Seed-only: a settled payment for a booking the seed created directly in the
-- `completed` state, so the payout ledger row it carries has a matching intent.
INSERT INTO payments (booking_id, provider, provider_ref, status, amount_minor,
                      currency, authorized_at, captured_at)
VALUES ($1, 'fake', $2, 'captured', $3, $4, now(), now());

-- name: DeleteAllPayments :exec
-- Seed-only. payments.booking_id references bookings with no ON DELETE CASCADE,
-- so the seed must clear payment rows (and the event log / ledger that
-- reference them) before bookings.
DELETE FROM payments;

-- name: DeleteAllPaymentEvents :exec
DELETE FROM payment_events;

-- name: DeleteAllPayoutLedger :exec
DELETE FROM payout_ledger;
