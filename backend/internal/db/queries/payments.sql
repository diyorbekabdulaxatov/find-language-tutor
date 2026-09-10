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

-- name: ListTeacherEarnings :many
SELECT pl.booking_id, pl.amount_minor, pl.currency, pl.state, pl.available_at,
       b.start_at,
       u.display_name AS student_display_name
FROM payout_ledger pl
JOIN bookings b ON b.id = pl.booking_id
JOIN users    u ON u.id = b.student_id
WHERE pl.teacher_id = $1
ORDER BY b.start_at DESC, pl.booking_id;

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
