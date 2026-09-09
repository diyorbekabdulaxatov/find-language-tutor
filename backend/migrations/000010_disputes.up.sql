-- Phase D: the bookings admin surface (force-cancel) and lesson disputes.
--
-- 1. bookings.cancelled_by — until now a cancellation only recorded when it
--    happened and the free-text reason; who pulled the trigger was implicit in
--    the request. The admin force-cancel override makes that ambiguous (an
--    operator can now cancel a booking neither participant asked to cancel), so
--    the actor is recorded explicitly:
--      ''        — not cancelled
--      'student' — the student cancelled
--      'teacher' — the teacher-owner cancelled (includes a teacher no-show)
--      'admin'   — an operator force-cancelled it (POST /v1/admin/bookings/{id}/force-cancel)
--
-- 2. disputes — a participant (student or teacher-owner) contests a confirmed or
--    completed lesson; an admin with `disputes.resolve` resolves or rejects it.
--
--      POST /v1/bookings/{id}/disputes        — participant raises one
--      GET  /v1/bookings/{id}/disputes        — participant reads the thread
--      GET  /v1/admin/disputes                — the operator queue (?status=)
--      POST /v1/admin/disputes/{id}/resolve   — resolve / reject (+ optional refund)
--
--    status lifecycle: open -> resolved | rejected (terminal).
--
--    A booking may have at most ONE open dispute, enforced by the partial unique
--    index below: the repository inserts and maps SQLSTATE 23505 to
--    "dispute_exists" — race-safe, never a check-then-insert (same pattern as
--    payment_events and reviews_booking_uniq). Resolved / rejected rows are
--    exempt, so the same booking can be disputed again after a resolution and
--    the thread keeps its full history.

ALTER TABLE bookings
    ADD COLUMN cancelled_by text NOT NULL DEFAULT ''
        CHECK (cancelled_by IN ('', 'student', 'teacher', 'admin'));

CREATE TABLE disputes (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    -- A dispute is meaningless without its booking, and the seed / an account
    -- teardown removes bookings wholesale, so this is the one cascade in the
    -- schema.
    booking_id   uuid NOT NULL REFERENCES bookings (id) ON DELETE CASCADE,

    -- The participant who raised it (student or teacher-owner account).
    raised_by    uuid NOT NULL REFERENCES users (id),

    reason       text NOT NULL,

    status       text NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'resolved', 'rejected')),

    -- The operator's closing note. Empty while the dispute is open.
    resolution   text NOT NULL DEFAULT '',
    resolved_by  uuid REFERENCES users (id),

    created_at   timestamptz NOT NULL DEFAULT now(),
    resolved_at  timestamptz
);

-- At most one OPEN dispute per booking. Partial, so a booking can be disputed
-- again once an earlier dispute is closed. This index is the race-safe
-- "already disputed" gate.
CREATE UNIQUE INDEX disputes_one_open_per_booking ON disputes (booking_id) WHERE status = 'open';

-- Hot path: the admin queue filters by status (default 'open'), newest first.
CREATE INDEX disputes_status_idx ON disputes (status);

-- The booking-detail thread read (participant view and admin booking detail).
CREATE INDEX disputes_booking_idx ON disputes (booking_id, created_at DESC);
