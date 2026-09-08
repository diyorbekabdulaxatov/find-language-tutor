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
-- One row per captured booking. teacher_id is copied from the booking.
INSERT INTO payout_ledger (teacher_id, booking_id, amount_minor, currency, state)
SELECT b.teacher_id, b.id, $2, $3, 'held'
FROM bookings b
WHERE b.id = $1
ON CONFLICT (booking_id) DO NOTHING;

-- name: MarkLedgerAvailable :exec
-- MVP: 'held' clears to 'available' immediately (no hold period).
-- TODO(payouts): real clearing window.
UPDATE payout_ledger
SET state = 'available', updated_at = now()
WHERE booking_id = $1 AND state = 'held';

-- name: MarkLedgerReversed :exec
UPDATE payout_ledger
SET state = 'reversed', updated_at = now()
WHERE booking_id = $1 AND state <> 'reversed';

-- name: ListTeacherEarnings :many
SELECT pl.booking_id, pl.amount_minor, pl.currency, pl.state,
       b.start_at,
       u.display_name AS student_display_name
FROM payout_ledger pl
JOIN bookings b ON b.id = pl.booking_id
JOIN users    u ON u.id = b.student_id
WHERE pl.teacher_id = $1
ORDER BY b.start_at DESC, pl.booking_id;

-- name: DeleteAllPayments :exec
-- Seed-only. payments.booking_id references bookings with no ON DELETE CASCADE,
-- so the seed must clear payment rows (and the event log / ledger that
-- reference them) before bookings.
DELETE FROM payments;

-- name: DeleteAllPaymentEvents :exec
DELETE FROM payment_events;

-- name: DeleteAllPayoutLedger :exec
DELETE FROM payout_ledger;
