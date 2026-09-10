-- Phase E: teacher payouts — a clearing window on the earnings ledger and the
-- operator payout run that settles what is owed.
--
-- 1. The clearing window. Until now a captured lesson landed in payout_ledger
--    as 'held' and was flipped to 'available' in the same transaction, so every
--    tiyin was withdrawable the instant the lesson completed. A row now carries
--    `available_at` — capture time + PAYOUTS_CLEARING_DAYS (default 7) — and the
--    window is *derived* from it rather than materialised by a background job:
--
--      held      state IN ('held','available') AND available_at >  now()
--      available state IN ('held','available') AND available_at <= now()
--      paid      state = 'paid'      (settled by a payout batch)
--      reversed  state = 'reversed'  (refunded to the student)
--
--    Storing the deadline rather than recomputing `created_at + interval` at
--    read time means changing the configured window never retroactively moves
--    money that has already been promised to a teacher. Rows written before
--    this migration were payable immediately, which the `DEFAULT now()`
--    backfill preserves exactly; the default is then dropped so every new row
--    states its own deadline.
--
--    'available' stays a legal state value for those pre-existing rows (the two
--    are read identically), but nothing writes it any more: a row is created
--    'held' and leaves as 'paid' or 'reversed'.
--
-- 2. payout_batches — one row per POST /v1/admin/payouts/run. The MVP's payment
--    provider is the in-process fake, which settles synchronously, so a batch is
--    written 'completed' in the same transaction that marks its ledger rows
--    'paid'. The 'processing' / 'failed' states are in the CHECK for the real
--    Payme / Click disbursement call that replaces the fake, which cannot
--    settle inside the transaction.
--
--    The run is race-safe against two operators clicking at once: it selects the
--    payable rows FOR UPDATE SKIP LOCKED, so a concurrent run works on a
--    disjoint set (and finds nothing, 409 nothing_to_pay) instead of paying the
--    same lesson twice.

CREATE TABLE payout_batches (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    -- The operator who ran it (holds `payouts.run`).
    created_by    uuid NOT NULL REFERENCES users (id),

    status        text NOT NULL
        CHECK (status IN ('processing', 'completed', 'failed')),

    -- Totals of the ledger rows the batch settled, in integer minor units.
    total_minor   bigint NOT NULL CHECK (total_minor >= 0),
    currency      text NOT NULL,

    -- How many distinct teachers were paid and how many ledger rows (lessons)
    -- the batch covered. Denormalised so the batch list needs no join.
    teacher_count int NOT NULL CHECK (teacher_count >= 0),
    line_count    int NOT NULL CHECK (line_count >= 0),

    created_at    timestamptz NOT NULL DEFAULT now(),
    completed_at  timestamptz
);

-- The batch list is newest first.
CREATE INDEX payout_batches_created_idx ON payout_batches (created_at DESC);

ALTER TABLE payout_ledger
    ADD COLUMN available_at    timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN paid_at         timestamptz,
    ADD COLUMN payout_batch_id uuid REFERENCES payout_batches (id);

-- Existing rows keep the "payable now" semantics they had; new rows must state
-- their own clearing deadline.
ALTER TABLE payout_ledger ALTER COLUMN available_at DROP DEFAULT;

ALTER TABLE payout_ledger DROP CONSTRAINT payout_ledger_state_check;
ALTER TABLE payout_ledger ADD CONSTRAINT payout_ledger_state_check
    CHECK (state IN ('held', 'available', 'paid', 'reversed'));

-- Hot path: the payout run and the operator dashboard both scan what is payable.
CREATE INDEX payout_ledger_payable_idx ON payout_ledger (available_at)
    WHERE state IN ('held', 'available');

-- The batch detail page groups a batch's ledger rows by teacher.
CREATE INDEX payout_ledger_batch_idx ON payout_ledger (payout_batch_id);
