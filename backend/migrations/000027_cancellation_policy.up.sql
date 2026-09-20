-- Cancellation policy: what happened to the money when a booking was
-- cancelled. The rule itself (free cancellation until N hours before the
-- start, default 24; a later student cancellation forfeits the fee to the
-- teacher; a teacher cancellation always refunds) lives in the booking
-- service — the column only records the outcome so the booking page, the
-- admin console and the payout ledger tell the same story.
--
--   ''          not cancelled, or cancelled before this migration / by an
--               operator who settled the money elsewhere
--   'refunded'  the student's hold was released or the capture refunded
--   'forfeited' cancelled inside the window: captured and paid to the teacher
--   'unpaid'    no payment had been taken, nothing to move
ALTER TABLE bookings
    ADD COLUMN cancellation_outcome text NOT NULL DEFAULT ''
        CHECK (cancellation_outcome IN ('', 'refunded', 'forfeited', 'unpaid'));
