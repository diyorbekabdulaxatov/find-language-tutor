-- Rescheduling: a student may move a lesson to another bookable slot of the
-- same teacher (same length, price and payment) up to the free-cancellation
-- deadline. The booking keeps its id; these columns keep the history the
-- booking page and the "lesson moved" email need. The service caps the count.
ALTER TABLE bookings
    ADD COLUMN reschedule_count integer NOT NULL DEFAULT 0 CHECK (reschedule_count >= 0),
    ADD COLUMN rescheduled_from timestamptz;
