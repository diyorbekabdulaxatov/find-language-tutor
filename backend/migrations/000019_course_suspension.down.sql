-- Reverse of 000019_course_suspension.up.sql.
ALTER TABLE courses DROP COLUMN IF EXISTS suspended_at;
