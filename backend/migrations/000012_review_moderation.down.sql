-- Reverse of 000012_review_moderation.up.sql.
--
-- rating / review_count keep whatever RecomputeTeacherRating last wrote — always
-- a valid folded value, and the same number the old incremental path would have
-- produced for the visible set — so there is nothing to restore there.

DROP INDEX IF EXISTS reviews_teacher_visible_idx;

ALTER TABLE teachers
    DROP COLUMN IF EXISTS review_count_base,
    DROP COLUMN IF EXISTS rating_base;

ALTER TABLE reviews
    DROP COLUMN IF EXISTS hidden;
