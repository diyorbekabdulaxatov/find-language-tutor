-- Reverse of 000009_teacher_moderation.up.sql.
DROP INDEX IF EXISTS teachers_status_idx;
ALTER TABLE teachers DROP COLUMN IF EXISTS moderation_note;
ALTER TABLE teachers DROP COLUMN IF EXISTS verified;
ALTER TABLE teachers DROP COLUMN IF EXISTS status;
