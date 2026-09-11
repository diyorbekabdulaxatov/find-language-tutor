-- Course authoring (phase C1): a teacher's courses, and the curriculum tree
-- (sections -> items) each one owns.

-- name: CreateCourse :one
INSERT INTO courses (teacher_id, title, subtitle, description, price_amount_minor, price_currency)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, teacher_id, title, subtitle, description, cover_asset_id,
          price_amount_minor, price_currency, status, ever_published, archived_at,
          created_at, updated_at;

-- name: GetCourse :one
SELECT id, teacher_id, title, subtitle, description, cover_asset_id,
       price_amount_minor, price_currency, status, ever_published, archived_at,
       created_at, updated_at
FROM courses
WHERE id = $1;

-- name: ListTeacherCourses :many
-- The authoring library list. Filters are optional: status ('draft'|'published')
-- and whether to include archived rows.
SELECT id, teacher_id, title, subtitle, description, cover_asset_id,
       price_amount_minor, price_currency, status, ever_published, archived_at,
       created_at, updated_at
FROM courses
WHERE teacher_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.arg('include_archived')::bool OR archived_at IS NULL)
ORDER BY created_at DESC, id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountTeacherCourses :one
SELECT count(*)
FROM courses
WHERE teacher_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.arg('include_archived')::bool OR archived_at IS NULL);

-- name: UpdateCourse :one
-- Full replace of the editable fields: title / subtitle / description / cover
-- / price. The service passes the current value for anything it isn't
-- changing, same convention as UpdateResource.
UPDATE courses
SET title = $2, subtitle = $3, description = $4, cover_asset_id = $5,
    price_amount_minor = $6, price_currency = $7, updated_at = now()
WHERE id = $1
RETURNING id, teacher_id, title, subtitle, description, cover_asset_id,
          price_amount_minor, price_currency, status, ever_published, archived_at,
          created_at, updated_at;

-- name: SetCourseStatus :one
-- ever_published is a one-way latch: OR'd with "is this setting `published`",
-- so unpublishing (status back to draft) never clears it — Delete stays
-- blocked forever once a course has gone live at least once.
UPDATE courses
SET status = sqlc.arg('status'),
    ever_published = ever_published OR (sqlc.arg('status')::text = 'published'),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, teacher_id, title, subtitle, description, cover_asset_id,
          price_amount_minor, price_currency, status, ever_published, archived_at,
          created_at, updated_at;

-- name: SetCourseArchived :one
UPDATE courses
SET archived_at = CASE WHEN sqlc.arg('archived')::bool THEN now() ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, teacher_id, title, subtitle, description, cover_asset_id,
          price_amount_minor, price_currency, status, ever_published, archived_at,
          created_at, updated_at;

-- name: DeleteCourse :exec
DELETE FROM courses WHERE id = $1;

-- Sections.

-- name: AddCourseSection :one
-- position is the current section count for the course, computed here so the
-- insert stays a single statement (same idiom as AttachBookingResource).
INSERT INTO course_sections (course_id, title, position)
VALUES (
    sqlc.arg('course_id'), sqlc.arg('title'),
    COALESCE((SELECT max(position) + 1 FROM course_sections WHERE course_id = sqlc.arg('course_id')), 0)
)
RETURNING id, course_id, title, position, created_at, updated_at;

-- name: GetCourseSection :one
SELECT id, course_id, title, position, created_at, updated_at
FROM course_sections
WHERE id = $1;

-- name: ListCourseSections :many
SELECT id, course_id, title, position, created_at, updated_at
FROM course_sections
WHERE course_id = $1
ORDER BY position, id;

-- name: RenameCourseSection :one
UPDATE course_sections
SET title = $2, updated_at = now()
WHERE id = $1
RETURNING id, course_id, title, position, created_at, updated_at;

-- name: DeleteCourseSection :exec
DELETE FROM course_sections WHERE id = $1;

-- name: ReorderCourseSections :execrows
-- Rewrites positions 0..n-1 from the given ordered id array in one statement.
-- The service has already validated the array is exactly this course's
-- current section ids; the course_id filter is a defense-in-depth guard
-- against a section that raced its way into another course between the
-- service's check and this write (affected rows short of len(ids) surfaces
-- to the caller as a write failure, never a silent partial reorder).
WITH ordered AS (
    SELECT id, row_number() OVER () - 1 AS position
    FROM unnest(sqlc.arg('section_ids')::uuid[]) AS id
)
UPDATE course_sections cs
SET position = ordered.position, updated_at = now()
FROM ordered
WHERE cs.id = ordered.id AND cs.course_id = sqlc.arg('course_id');

-- Items.

-- name: AddCourseItem :one
-- position is the current item count for the section, same idiom as sections.
INSERT INTO course_items (section_id, kind, title, video_asset_id, resource_id, position)
VALUES (
    sqlc.arg('section_id'), sqlc.arg('kind'), sqlc.arg('title'),
    sqlc.narg('video_asset_id'), sqlc.narg('resource_id'),
    COALESCE((SELECT max(position) + 1 FROM course_items WHERE section_id = sqlc.arg('section_id')), 0)
)
RETURNING id, section_id, kind, title, video_asset_id, resource_id, position, created_at;

-- name: GetCourseItem :one
SELECT id, section_id, kind, title, video_asset_id, resource_id, position, created_at
FROM course_items
WHERE id = $1;

-- name: ListCourseItems :many
SELECT id, section_id, kind, title, video_asset_id, resource_id, position, created_at
FROM course_items
WHERE section_id = $1
ORDER BY position, id;

-- name: ListCourseItemsByCourse :many
-- Every item across a course's sections, in curriculum order (section
-- position, then item position) — one query for the whole tree instead of
-- N+1 per-section queries; the service groups rows by section_id in Go.
SELECT ci.id, ci.section_id, ci.kind, ci.title, ci.video_asset_id, ci.resource_id, ci.position, ci.created_at
FROM course_items ci
JOIN course_sections cs ON cs.id = ci.section_id
WHERE cs.course_id = $1
ORDER BY cs.position, cs.id, ci.position, ci.id;

-- name: RenameCourseItem :one
UPDATE course_items
SET title = $2
WHERE id = $1
RETURNING id, section_id, kind, title, video_asset_id, resource_id, position, created_at;

-- name: DeleteCourseItem :exec
DELETE FROM course_items WHERE id = $1;

-- name: ReorderCourseItems :execrows
WITH ordered AS (
    SELECT id, row_number() OVER () - 1 AS position
    FROM unnest(sqlc.arg('item_ids')::uuid[]) AS id
)
UPDATE course_items ci
SET position = ordered.position
FROM ordered
WHERE ci.id = ordered.id AND ci.section_id = sqlc.arg('section_id');
