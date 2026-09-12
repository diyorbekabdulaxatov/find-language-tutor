DROP INDEX IF EXISTS submissions_enrollment_idx;
DROP INDEX IF EXISTS submissions_course_uniq;
ALTER TABLE submissions DROP COLUMN IF EXISTS enrollment_id;

DROP INDEX IF EXISTS course_payment_events_payment_idx;
DROP TABLE IF EXISTS course_payment_events;

DROP INDEX IF EXISTS course_payments_status_idx;
DROP TABLE IF EXISTS course_payments;

DROP INDEX IF EXISTS course_item_progress_enrollment_idx;
DROP TABLE IF EXISTS course_item_progress;

DROP INDEX IF EXISTS course_enrollments_course_idx;
DROP INDEX IF EXISTS course_enrollments_student_idx;
DROP TABLE IF EXISTS course_enrollments;
