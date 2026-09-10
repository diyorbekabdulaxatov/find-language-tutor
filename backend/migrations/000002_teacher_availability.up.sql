-- Availability module: a teacher's weekly recurring availability, stored in UTC.
--
-- A slot is a span on one weekday, expressed as minutes from 00:00 UTC so the
-- Go side never touches pgtype.Time and overlap math stays integer-only (same
-- rationale as the float4 rating column on teachers). Booking will convert these
-- to a concrete date/time using the teacher's IANA timezone (teachers.timezone).
--
-- weekday: 0 = Sunday ... 6 = Saturday (matches JavaScript Date.getDay() and
-- Postgres EXTRACT(DOW), which the frontend grid relies on).

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE teacher_availability_slots (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    teacher_id    uuid NOT NULL REFERENCES teachers (id) ON DELETE CASCADE,
    weekday       smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),

    -- minutes from 00:00 UTC; 0..1440, aligned to a 15-minute grid.
    start_minute  integer NOT NULL CHECK (start_minute >= 0 AND start_minute < 1440 AND start_minute % 15 = 0),
    end_minute    integer NOT NULL CHECK (end_minute > 0 AND end_minute <= 1440 AND end_minute % 15 = 0),
    CHECK (start_minute < end_minute),

    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    -- No two slots for the same teacher on the same weekday may overlap.
    -- Touching slots (prev end == next start) are allowed.
    CONSTRAINT teacher_availability_slots_no_overlap
        EXCLUDE USING gist (
            teacher_id WITH =,
            weekday WITH =,
            int4range(start_minute, end_minute) WITH &&
        )
);

CREATE INDEX teacher_availability_slots_teacher_idx
    ON teacher_availability_slots (teacher_id, weekday, start_minute);
