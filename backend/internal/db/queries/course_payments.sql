-- Phase C2: course-purchase payments. A sibling of payments.sql's booking
-- flow — same status lifecycle and webhook-idempotency shape, keyed to
-- (course_id, student_id) instead of booking_id. No payout-ledger write here;
-- revenue-share for course sales is a future phase.

-- name: CreateCoursePayment :one
-- Idempotent: a second call for the same (course_id, student_id) is a no-op
-- and still returns the existing row (mirrors CreatePayment's ON CONFLICT
-- DO UPDATE no-op trick).
INSERT INTO course_payments (course_id, student_id, provider, amount_minor, currency, status)
VALUES ($1, $2, $3, $4, $5, 'requires_payment')
ON CONFLICT (course_id, student_id) DO UPDATE SET course_id = course_payments.course_id
RETURNING id, course_id, student_id, provider, provider_ref, status, amount_minor, currency,
          last_error, authorized_at, captured_at, refunded_at, created_at, updated_at;

-- name: GetCoursePaymentByCourseAndStudent :one
SELECT id, course_id, student_id, provider, provider_ref, status, amount_minor, currency,
       last_error, authorized_at, captured_at, refunded_at, created_at, updated_at
FROM course_payments
WHERE course_id = $1 AND student_id = $2;

-- name: GetCoursePaymentByID :one
SELECT id, course_id, student_id, provider, provider_ref, status, amount_minor, currency,
       last_error, authorized_at, captured_at, refunded_at, created_at, updated_at
FROM course_payments
WHERE id = $1;

-- name: InsertCoursePaymentEvent :exec
INSERT INTO course_payment_events (event_id, payment_id, type)
VALUES (sqlc.arg('event_id'), sqlc.arg('payment_id'), sqlc.arg('type'));

-- name: MarkCoursePaymentAuthorized :exec
UPDATE course_payments
SET status = 'authorized', provider_ref = sqlc.narg('provider_ref'), authorized_at = now(), updated_at = now()
WHERE id = sqlc.arg('id');

-- name: MarkCoursePaymentCaptured :exec
UPDATE course_payments SET status = 'captured', captured_at = now(), updated_at = now() WHERE id = $1;

-- name: MarkCoursePaymentRefunded :exec
UPDATE course_payments SET status = 'refunded', refunded_at = now(), updated_at = now() WHERE id = $1;

-- name: MarkCoursePaymentFailed :exec
UPDATE course_payments
SET status = 'failed', last_error = sqlc.arg('last_error'), updated_at = now()
WHERE id = sqlc.arg('id');

-- Seed-only. course_payment_events -> course_payments -> courses/users; clear
-- events before payments, and both before courses.DeleteAllCourses.

-- name: DeleteAllCoursePaymentEvents :exec
DELETE FROM course_payment_events;

-- name: DeleteAllCoursePayments :exec
DELETE FROM course_payments;
