-- Phase D2: course ratings and reviews.
--
-- A catalog with no stars gives a student no reason to trust an unknown
-- teacher's course, so this is what makes the phase-C2 storefront
-- self-sorting. It mirrors the lesson-review module (internal/reviews) in
-- shape — rating 1-5 + comment, a `hidden` moderation flag, and a derived
-- display aggregate on the parent row — with three deliberate differences:
--
--  1. Eligibility is the ENROLLMENT, not a completed lesson. `enrollment_id`
--     is UNIQUE and is both the proof the reviewer bought the course and the
--     one-review-per-buyer gate; a duplicate raises 23505, which the
--     repository maps to "already reviewed" (insert-first, never
--     check-then-insert, same idiom as the rest of the codebase). Requiring
--     progress before reviewing was considered and rejected: it is what
--     Udemy does, it cannot be gamed any better than enrollment can, and it
--     would silence the buyers most worth hearing from (the ones who bounced
--     off the course).
--
--  2. A review is EDITABLE by its author (`updated_at`). A lesson review is a
--     one-shot judgement of a past event; a course review is a standing
--     opinion of a thing the student keeps using, and a student who changes
--     their mind halfway through should be able to say so rather than being
--     stuck with their first impression.
--
--  3. There is no rating_base / review_count_base pair. `teachers` carries
--     those because the seed ships historical sample reviews that predate the
--     live ones; courses have no such baseline, so the aggregate is purely
--     derived and the recompute is a plain rebuild from the visible rows.

CREATE TABLE course_reviews (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id     uuid NOT NULL REFERENCES courses (id) ON DELETE CASCADE,

    -- The buyer's access grant. UNIQUE = one review per purchase. ON DELETE
    -- CASCADE because a review without the enrollment that justified it has
    -- no standing.
    enrollment_id uuid NOT NULL UNIQUE REFERENCES course_enrollments (id) ON DELETE CASCADE,

    -- Denormalised from the enrollment so the public list and the moderation
    -- queue can join straight to users for a display name.
    student_id    uuid NOT NULL REFERENCES users (id),

    rating        smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment       text NOT NULL DEFAULT '',

    -- Operator takedown, exactly like reviews.hidden: a hidden review keeps
    -- its row (so the author still counts as having reviewed, and cannot slip
    -- a second one past the UNIQUE) but drops out of the public list and the
    -- rating aggregate.
    hidden        boolean NOT NULL DEFAULT false,

    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- The public list: one course's visible reviews, newest first.
CREATE INDEX course_reviews_course_idx
    ON course_reviews (course_id, created_at DESC)
    WHERE NOT hidden;

-- The moderation queue: every review regardless of visibility, newest first.
CREATE INDEX course_reviews_moderation_idx ON course_reviews (created_at DESC);

-- The display aggregate, recomputed in the same transaction as any write that
-- can change it (create, edit, hide, unhide, delete). Purely derived — see the
-- header note on why there is no immutable baseline here.
ALTER TABLE courses
    ADD COLUMN rating       real    NOT NULL DEFAULT 0,
    ADD COLUMN review_count integer NOT NULL DEFAULT 0;

ALTER TABLE courses
    ADD CONSTRAINT courses_rating_range CHECK (rating >= 0 AND rating <= 5),
    ADD CONSTRAINT courses_review_count_non_negative CHECK (review_count >= 0);
