-- Reverse of 000010_disputes.up.sql. Dropping the table takes its three indexes
-- with it; the DROP INDEX statements are belt-and-braces for a partially applied
-- up.
DROP INDEX IF EXISTS disputes_booking_idx;
DROP INDEX IF EXISTS disputes_status_idx;
DROP INDEX IF EXISTS disputes_one_open_per_booking;
DROP TABLE IF EXISTS disputes;

ALTER TABLE bookings DROP COLUMN IF EXISTS cancelled_by;
