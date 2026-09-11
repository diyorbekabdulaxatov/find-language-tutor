-- Reverse of 000015_lesson_resources.up.sql.
DROP INDEX IF EXISTS submissions_status_idx;
DROP INDEX IF EXISTS submissions_booking_idx;
DROP INDEX IF EXISTS submissions_lesson_uniq;
DROP TABLE IF EXISTS submissions;

DROP INDEX IF EXISTS booking_resources_resource_idx;
DROP INDEX IF EXISTS booking_resources_booking_idx;
DROP TABLE IF EXISTS booking_resources;
