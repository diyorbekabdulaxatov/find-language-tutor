-- Booking module: a concrete, scheduled lesson between a student account and a
-- teacher profile. Times are absolute instants (timestamptz), stored and used in
-- UTC; the recurring weekly availability (teacher_availability_slots, UTC
-- minutes) is projected onto real datetimes by the service when it lists slots
-- and re-checked server-side on every create.
--
-- status lifecycle for the MVP:
--   pending_payment -> confirmed -> completed
--   pending_payment | confirmed -> cancelled   (any time; who/when recorded)
-- ('completed' transitions and reminders arrive in a later phase.)

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE bookings (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    teacher_id           uuid NOT NULL REFERENCES teachers (id),
    student_id           uuid NOT NULL REFERENCES users (id),

    start_at             timestamptz NOT NULL,
    end_at               timestamptz NOT NULL,
    CHECK (end_at > start_at),

    duration_minutes     integer NOT NULL CHECK (duration_minutes > 0),

    status               text NOT NULL
        CHECK (status IN ('pending_payment', 'confirmed', 'completed', 'cancelled')),

    price_minor          bigint NOT NULL CHECK (price_minor >= 0),
    currency             text NOT NULL,

    is_trial             boolean NOT NULL DEFAULT false,

    cancelled_at         timestamptz,
    cancellation_reason  text NOT NULL DEFAULT '',

    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),

    -- Double-booking prevention at the DB layer. A teacher can hold at most one
    -- non-cancelled lesson over any instant; likewise a student cannot be in two
    -- lessons at once. tstzrange defaults to '[)' (start inclusive, end
    -- exclusive) so back-to-back lessons (prev end == next start) do not collide.
    CONSTRAINT bookings_teacher_no_overlap
        EXCLUDE USING gist (
            teacher_id WITH =,
            tstzrange(start_at, end_at) WITH &&
        ) WHERE (status <> 'cancelled'),

    CONSTRAINT bookings_student_no_overlap
        EXCLUDE USING gist (
            student_id WITH =,
            tstzrange(start_at, end_at) WITH &&
        ) WHERE (status <> 'cancelled')
);

CREATE INDEX bookings_teacher_start_idx ON bookings (teacher_id, start_at);
CREATE INDEX bookings_student_start_idx ON bookings (student_id, start_at);
