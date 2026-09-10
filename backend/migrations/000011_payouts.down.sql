-- Reverse of 000011_payouts.up.sql.
--
-- 'paid' has no representation in the pre-phase-E state vocabulary, and the
-- narrowed CHECK would reject it. A paid row was 'available' the instant before
-- the payout run flipped it, so that is exactly what the run's inverse restores
-- (the batch it pointed at is dropped below along with the column).
UPDATE payout_ledger SET state = 'available', updated_at = now() WHERE state = 'paid';

DROP INDEX IF EXISTS payout_ledger_batch_idx;
DROP INDEX IF EXISTS payout_ledger_payable_idx;

ALTER TABLE payout_ledger DROP CONSTRAINT IF EXISTS payout_ledger_state_check;
ALTER TABLE payout_ledger ADD CONSTRAINT payout_ledger_state_check
    CHECK (state IN ('held', 'available', 'reversed'));

ALTER TABLE payout_ledger
    DROP COLUMN IF EXISTS payout_batch_id,
    DROP COLUMN IF EXISTS paid_at,
    DROP COLUMN IF EXISTS available_at;

-- Dropping the table takes its index with it; the DROP INDEX is belt-and-braces
-- for a partially applied up.
DROP INDEX IF EXISTS payout_batches_created_idx;
DROP TABLE IF EXISTS payout_batches;
