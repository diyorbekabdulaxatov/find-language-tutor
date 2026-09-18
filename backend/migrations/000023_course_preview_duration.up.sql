-- Phase D1: free preview lessons + duration metadata.
--
-- `is_preview` marks a curriculum item a non-enrolled viewer may watch from
-- the course landing page — the sample that makes a sealed course sellable.
-- It is deliberately restricted to `video` items by a CHECK rather than by
-- application convention: the public preview route streams bytes, and a
-- resource item's payload (quiz answers, a writing prompt's rubric) has a
-- whole separate stripping path (resources' course gateway) that has no
-- business running for an anonymous viewer. Keeping the DB honest here means
-- no future code path can accidentally expose a resource for free.
--
-- `duration_seconds` is the video's length, supplied by the client from the
-- browser's own <video> metadata at upload time and validated (0 < d <= 24h)
-- by the service. It is deliberately NOT probed server-side: it is display
-- metadata only ("12.5 hours · 48 lectures") and never gates access, pricing
-- or payouts, so the worst a lying client achieves is misdescribing its own
-- course to its own buyers. 0 means "unknown" — a WebM upload whose duration
-- the browser could not read, or any item created before this migration —
-- and every surface omits the figure rather than rendering "0m".
--
-- A course's total duration is summed on read (same idiom as the existing
-- section_count / item_count subqueries in CatalogListCourses) rather than
-- denormalised onto `courses`, so it can never drift from the items.

ALTER TABLE course_items
    ADD COLUMN is_preview       boolean NOT NULL DEFAULT false,
    ADD COLUMN duration_seconds integer NOT NULL DEFAULT 0;

ALTER TABLE course_items
    ADD CONSTRAINT course_items_duration_non_negative
        CHECK (duration_seconds >= 0),
    ADD CONSTRAINT course_items_preview_video_only
        CHECK (NOT is_preview OR kind = 'video');

-- The landing page asks "does this course have a preview, and which item is
-- it?" on every view; a partial index keeps that a lookup over the handful of
-- preview rows rather than a scan of every item in the course.
CREATE INDEX course_items_preview_idx
    ON course_items (section_id, position)
    WHERE is_preview;
