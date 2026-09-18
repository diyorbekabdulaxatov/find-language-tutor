ALTER TABLE courses
    DROP CONSTRAINT IF EXISTS courses_review_count_non_negative,
    DROP CONSTRAINT IF EXISTS courses_rating_range;

ALTER TABLE courses
    DROP COLUMN IF EXISTS review_count,
    DROP COLUMN IF EXISTS rating;

DROP TABLE IF EXISTS course_reviews;
