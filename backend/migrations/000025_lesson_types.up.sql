-- Lesson types (italki's shape): a teacher offers several named 1-on-1 lessons
-- rather than one hourly rate. Each carries its own description and a price per
-- duration, and at most one of them is the trial.
--
-- teachers.price_per_hour_minor stays: it is the headline "from" price the
-- search cards sort and filter on, and the fallback a booking is priced at when
-- no lesson type is chosen (pre-L1 clients).

CREATE TABLE lesson_types (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id   uuid NOT NULL REFERENCES teachers (id),

    title        text NOT NULL CHECK (length(btrim(title)) > 0),
    description  text NOT NULL DEFAULT '',

    -- The trial is a lesson type like any other, flagged so the UI can badge it
    -- and the booking carries is_trial through to reporting. Note that nothing
    -- limits how many trials a student may book with one teacher -- that rule
    -- has never existed here; only the offering itself is unique (below).
    is_trial     boolean NOT NULL DEFAULT false,

    archived     boolean NOT NULL DEFAULT false,
    position     integer NOT NULL DEFAULT 0,

    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- One live trial offering per teacher.
CREATE UNIQUE INDEX lesson_types_one_trial_idx
    ON lesson_types (teacher_id)
    WHERE is_trial AND NOT archived;

CREATE INDEX lesson_types_teacher_idx ON lesson_types (teacher_id, archived, position, created_at);

-- A type's price list. The 15-minute availability grid is what allows 45.
CREATE TABLE lesson_type_prices (
    lesson_type_id   uuid NOT NULL REFERENCES lesson_types (id),
    duration_minutes integer NOT NULL CHECK (duration_minutes IN (30, 45, 60, 90, 120)),
    price_minor      bigint NOT NULL CHECK (price_minor >= 0),
    PRIMARY KEY (lesson_type_id, duration_minutes)
);

-- Which offering a booking was made against. Nullable: bookings made before
-- this migration have no type, and the legacy hourly path still works.
ALTER TABLE bookings ADD COLUMN lesson_type_id uuid REFERENCES lesson_types (id);
CREATE INDEX bookings_lesson_type_idx ON bookings (lesson_type_id);

-- Backfill: every existing teacher gets the offering they already had, so no
-- profile is left with an empty lesson list.
INSERT INTO lesson_types (teacher_id, title, description, is_trial, position)
SELECT id,
       'One-to-one lesson',
       'A regular lesson built around what you need that week.',
       false,
       0
FROM teachers;

INSERT INTO lesson_type_prices (lesson_type_id, duration_minutes, price_minor)
SELECT lt.id, d.minutes, ((t.price_per_hour_minor * d.minutes) + 30) / 60
FROM lesson_types lt
JOIN teachers t ON t.id = lt.teacher_id
CROSS JOIN (VALUES (30), (60), (90), (120)) AS d(minutes)
WHERE NOT lt.is_trial;

INSERT INTO lesson_types (teacher_id, title, description, is_trial, position)
SELECT id,
       'Trial lesson',
       'A short first lesson: we talk, I find your level and we agree a plan.',
       true,
       -1
FROM teachers
WHERE trial_price_minor IS NOT NULL;

INSERT INTO lesson_type_prices (lesson_type_id, duration_minutes, price_minor)
SELECT lt.id, 30, t.trial_price_minor
FROM lesson_types lt
JOIN teachers t ON t.id = lt.teacher_id
WHERE lt.is_trial AND t.trial_price_minor IS NOT NULL;
