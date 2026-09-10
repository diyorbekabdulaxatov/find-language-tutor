-- Payouts module (phase E): the operator payout dashboard and the payout run
-- that settles what teachers are owed.
--
-- The clearing window is derived, not materialised: a ledger row is payable when
-- its state is still open (`held`, or the legacy `available` written before
-- migration 000011) AND its available_at deadline has passed. Money is integer
-- minor units everywhere.

-- name: AdminListOwedPayouts :many
-- What the platform owes right now, one row per teacher: the sum of the ledger
-- rows past their clearing deadline and not yet paid out, biggest first.
SELECT t.slug,
       t.display_name,
       pl.currency,
       sum(pl.amount_minor)::bigint      AS available_minor,
       min(pl.available_at)::timestamptz AS oldest_available_at
FROM payout_ledger pl
JOIN teachers t ON t.id = pl.teacher_id
WHERE pl.state IN ('held', 'available') AND pl.available_at <= now()
GROUP BY t.slug, t.display_name, pl.currency
ORDER BY sum(pl.amount_minor) DESC, t.slug;

-- name: AdminPayoutTotals :one
-- Platform-wide ledger totals: payable now, still inside the clearing window,
-- and already disbursed. Reversed rows count towards none of them.
SELECT
    coalesce(sum(amount_minor) FILTER (
        WHERE state IN ('held', 'available') AND available_at <= now()), 0)::bigint AS available_total_minor,
    coalesce(sum(amount_minor) FILTER (
        WHERE state IN ('held', 'available') AND available_at > now()), 0)::bigint  AS held_total_minor,
    coalesce(sum(amount_minor) FILTER (WHERE state = 'paid'), 0)::bigint            AS paid_total_minor,
    coalesce(max(currency), '')::text                                               AS currency
FROM payout_ledger;

-- name: AdminListPayoutBatches :many
-- Past payout runs, newest first.
SELECT b.id, b.created_by, u.display_name AS created_by_display_name,
       b.status, b.total_minor, b.currency, b.teacher_count, b.line_count,
       b.created_at, b.completed_at
FROM payout_batches b
JOIN users u ON u.id = b.created_by
ORDER BY b.created_at DESC, b.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountPayoutBatches :one
SELECT count(*) FROM payout_batches;

-- name: AdminGetPayoutBatch :one
SELECT b.id, b.created_by, u.display_name AS created_by_display_name,
       b.status, b.total_minor, b.currency, b.teacher_count, b.line_count,
       b.created_at, b.completed_at
FROM payout_batches b
JOIN users u ON u.id = b.created_by
WHERE b.id = $1;

-- name: AdminListPayoutBatchLines :many
-- One line per teacher paid by a batch, biggest first.
SELECT t.slug,
       t.display_name,
       pl.currency,
       sum(pl.amount_minor)::bigint AS amount_minor,
       count(*)::int                AS lesson_count
FROM payout_ledger pl
JOIN teachers t ON t.id = pl.teacher_id
WHERE pl.payout_batch_id = $1
GROUP BY t.slug, t.display_name, pl.currency
ORDER BY sum(pl.amount_minor) DESC, t.slug;

-- name: LockPayablePayoutLedger :many
-- The payout run's row selection, inside the run's transaction.
--
-- FOR UPDATE SKIP LOCKED is the double-run guard: two operators running at the
-- same time get disjoint row sets — the second skips the rows the first has
-- locked rather than blocking on them or settling them a second time — and the
-- loser typically ends up with nothing to pay (409 nothing_to_pay).
SELECT id, teacher_id, amount_minor, currency
FROM payout_ledger
WHERE state IN ('held', 'available') AND available_at <= now()
ORDER BY id
FOR UPDATE SKIP LOCKED;

-- name: InsertPayoutBatch :one
-- The fake provider settles in-process, so a batch is written already
-- 'completed' in the same transaction that marks its rows paid. A real
-- disbursement API would insert 'processing' here and flip it afterwards.
INSERT INTO payout_batches (created_by, status, total_minor, currency,
                            teacher_count, line_count, completed_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
RETURNING id, created_at, completed_at;

-- name: MarkLedgerRowsPaid :exec
-- Settles exactly the rows the run locked. The state guard is belt-and-braces:
-- FOR UPDATE SKIP LOCKED already makes the set exclusive.
UPDATE payout_ledger
SET state = 'paid', payout_batch_id = sqlc.arg('payout_batch_id'),
    paid_at = now(), updated_at = now()
WHERE id = ANY (sqlc.arg('ids')::uuid[]) AND state <> 'paid';

-- name: SeedInsertPayoutLedgerRow :exec
-- Seed-only: a ledger row for a booking the seed created directly in the
-- `completed` state, with its clearing deadline placed in the past so the
-- payout dashboard is not empty on a fresh database. A non-null batch id also
-- marks the row paid.
INSERT INTO payout_ledger (teacher_id, booking_id, amount_minor, currency, state,
                           available_at, paid_at, payout_batch_id)
SELECT b.teacher_id, b.id,
       sqlc.arg('amount_minor')::bigint,
       sqlc.arg('currency')::text,
       sqlc.arg('state')::text,
       now() - make_interval(days => sqlc.arg('cleared_days_ago')::int),
       CASE WHEN sqlc.narg('payout_batch_id')::uuid IS NULL THEN NULL ELSE now() END,
       sqlc.narg('payout_batch_id')::uuid
FROM bookings b
WHERE b.id = sqlc.arg('booking_id');

-- name: DeleteAllPayoutBatches :exec
-- Seed-only. payout_ledger.payout_batch_id references payout_batches with no
-- ON DELETE CASCADE, so the seed clears the ledger first, and payout_batches
-- before users (created_by).
DELETE FROM payout_batches;
