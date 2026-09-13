-- Reverse of 000018_course_payout_ledger.up.sql.
--
-- Lossy in one scenario: if any course-sourced rows (course_enrollment_id NOT
-- NULL) have already been written, restoring `booking_id NOT NULL` would
-- reject them outright. A real rollback after real course-ledger rows exist
-- would need those rows deleted first (same caveat pattern as
-- 000011_payouts.down.sql's 'paid' -> 'available' rewrite) — this down
-- migration assumes it is run immediately after the up, before any such row
-- exists, which is exactly how it is tested here.

ALTER TABLE payout_ledger DROP CONSTRAINT IF EXISTS payout_ledger_course_enrollment_uniq;
ALTER TABLE payout_ledger DROP CONSTRAINT IF EXISTS payout_ledger_source_check;

ALTER TABLE payout_ledger DROP COLUMN IF EXISTS course_enrollment_id;

ALTER TABLE payout_ledger ALTER COLUMN booking_id SET NOT NULL;
