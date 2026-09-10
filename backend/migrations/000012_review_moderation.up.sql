-- Phase F: review moderation — an operator can hide a review (reversibly) or
-- remove it, and the teacher's display aggregate follows correctly.
--
-- The problem this migration solves. Until now teachers.rating / review_count
-- were maintained INCREMENTALLY: each new review folded itself in with
--   rating = round((rating*review_count + new) / (review_count+1), 1)
-- on top of a hand-set historical baseline ("showing 4 of 214"). That fold
-- rounds at every step and cannot be inverted, so hiding a review could not
-- move the aggregate back to where it belongs.
--
-- The fix: keep the historical baseline as its own immutable pair
-- (rating_base / review_count_base) and make rating / review_count DERIVED —
-- recomputed from the baseline folded with the teacher's currently visible,
-- booking-tied reviews (see RecomputeTeacherRating). Hiding, unhiding, removing,
-- or adding a review all just re-run that one idempotent, order-independent
-- computation.

ALTER TABLE reviews
    ADD COLUMN hidden boolean NOT NULL DEFAULT false;

ALTER TABLE teachers
    ADD COLUMN rating_base       real    NOT NULL DEFAULT 0 CHECK (rating_base >= 0 AND rating_base <= 5),
    ADD COLUMN review_count_base integer NOT NULL DEFAULT 0 CHECK (review_count_base >= 0);

-- Snapshot the current aggregate as the baseline. This is exact wherever this
-- migration runs: the seed folds in no booking-tied reviews, and nothing is
-- deployed, so rating / review_count still hold only the hand-set history.
UPDATE teachers SET rating_base = rating, review_count_base = review_count;

-- Hot path: RecomputeTeacherRating sums a teacher's visible booking-tied reviews.
CREATE INDEX reviews_teacher_visible_idx ON reviews (teacher_id)
    WHERE booking_id IS NOT NULL AND NOT hidden;
