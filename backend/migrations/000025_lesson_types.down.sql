DROP INDEX IF EXISTS bookings_lesson_type_idx;
ALTER TABLE bookings DROP COLUMN IF EXISTS lesson_type_id;
DROP TABLE IF EXISTS lesson_type_prices;
DROP INDEX IF EXISTS lesson_types_teacher_idx;
DROP INDEX IF EXISTS lesson_types_one_trial_idx;
DROP TABLE IF EXISTS lesson_types;
