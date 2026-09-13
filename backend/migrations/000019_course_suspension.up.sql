-- Phase C3 (part 2): admin course moderation. An operator-only takedown flag,
-- independent of the teacher's own draft/published/archived state (courses.go's
-- Status / archived_at) — a suspended course is pulled from the storefront
-- (catalog, catalog detail, cover image, new purchases) but an already-enrolled
-- student keeps their access; this is a takedown, not a deletion or refund.
-- NULL = not suspended, matching archived_at's own null-is-the-default idiom.

ALTER TABLE courses ADD COLUMN suspended_at timestamptz;

-- The catalog list filters on this alongside status/archived_at/teacher
-- approval; index it the same way archived_at's implicit lookups already are
-- (no dedicated index there either — the catalog's WHERE is led by
-- status = 'published', which is far more selective than this ever would be).
