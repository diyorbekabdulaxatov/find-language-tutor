-- Phase 6 (reviews): a student's rating + comment for a completed lesson.
--
-- A real review is tied to exactly one completed booking (booking_id set); the
-- demo seed also inserts sample rows with booking_id NULL that stand in for the
-- historical review history behind a teacher's hand-set rating / review_count
-- aggregate ("showing 4 of 214").
--
--   POST /v1/bookings/{id}/review     — student-only, booking must be completed,
--                                       one review per booking
--   GET  /v1/teachers/{slug}/reviews  — public, newest first, paginated
--
-- On a real (booking-tied) review the teachers aggregate is nudged in the same
-- transaction:
--   review_count = review_count + 1
--   rating       = round((rating*review_count + new_rating) / (review_count+1), 1)
-- clamped to [0, 5]. The seeded rows deliberately do NOT touch the aggregate.

CREATE TABLE reviews (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    teacher_id  uuid NOT NULL REFERENCES teachers (id),
    student_id  uuid NOT NULL REFERENCES users (id),

    -- Nullable: a real review references the completed booking it is about; the
    -- seeded sample reviews have none.
    booking_id  uuid REFERENCES bookings (id),

    rating      smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment     text NOT NULL DEFAULT '',

    created_at  timestamptz NOT NULL DEFAULT now()
);

-- One review per booking. Partial so the many seeded booking-less rows do not
-- collide. This unique index is the race-safe "already reviewed" gate — the
-- repository inserts and maps SQLSTATE 23505 to already_reviewed, never a
-- check-then-insert.
CREATE UNIQUE INDEX reviews_booking_uniq ON reviews (booking_id) WHERE booking_id IS NOT NULL;

-- Hot path: a teacher's reviews, newest first.
CREATE INDEX reviews_teacher_created_idx ON reviews (teacher_id, created_at DESC);
