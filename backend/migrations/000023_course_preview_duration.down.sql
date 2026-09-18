DROP INDEX IF EXISTS course_items_preview_idx;

ALTER TABLE course_items
    DROP CONSTRAINT IF EXISTS course_items_preview_video_only,
    DROP CONSTRAINT IF EXISTS course_items_duration_non_negative;

ALTER TABLE course_items
    DROP COLUMN IF EXISTS duration_seconds,
    DROP COLUMN IF EXISTS is_preview;
