-- Course authoring (phase C1): a teacher's courses, and the curriculum tree
-- (sections -> items) each one owns.

-- name: CreateCourse :one
INSERT INTO courses (teacher_id, title, subtitle, description, price_amount_minor, price_currency)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, teacher_id, title, subtitle, description, cover_asset_id,
          price_amount_minor, price_currency, status, ever_published, archived_at,
          created_at, updated_at, suspended_at;

-- name: GetCourse :one
SELECT id, teacher_id, title, subtitle, description, cover_asset_id,
       price_amount_minor, price_currency, status, ever_published, archived_at,
       created_at, updated_at, suspended_at
FROM courses
WHERE id = $1;

-- name: ListTeacherCourses :many
-- The authoring library list. Filters are optional: status ('draft'|'published')
-- and whether to include archived rows.
SELECT id, teacher_id, title, subtitle, description, cover_asset_id,
       price_amount_minor, price_currency, status, ever_published, archived_at,
       created_at, updated_at, suspended_at
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
          created_at, updated_at, suspended_at;

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
          created_at, updated_at, suspended_at;

-- name: SetCourseArchived :one
UPDATE courses
SET archived_at = CASE WHEN sqlc.arg('archived')::bool THEN now() ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, teacher_id, title, subtitle, description, cover_asset_id,
          price_amount_minor, price_currency, status, ever_published, archived_at,
          created_at, updated_at, suspended_at;

-- name: DeleteCourse :exec
DELETE FROM courses WHERE id = $1;

-- Phase C3: admin moderation.

-- name: AdminListCourses :many
-- The operator moderation queue: every course regardless of status/teacher/
-- suspension, newest first. Filters mirror reviews' ModerationQuery shape:
-- status ('draft'|'published'|'archived', "archived" meaning archived_at IS NOT
-- NULL regardless of the underlying status column), suspended ('true'|'false'),
-- teacher_slug, and a title substring q — each NULL/empty means "any".
SELECT
    c.id, c.teacher_id, c.title, c.subtitle, c.description, c.cover_asset_id,
    c.price_amount_minor, c.price_currency, c.status, c.ever_published, c.archived_at,
    c.suspended_at, c.created_at, c.updated_at,
    t.slug AS teacher_slug, t.display_name AS teacher_display_name
FROM courses c
JOIN teachers t ON t.id = c.teacher_id
WHERE (sqlc.narg('status')::text IS NULL
        OR (sqlc.narg('status')::text = 'archived' AND c.archived_at IS NOT NULL)
        OR (sqlc.narg('status')::text <> 'archived' AND c.status = sqlc.narg('status')::text AND c.archived_at IS NULL))
  AND (sqlc.narg('suspended')::bool IS NULL
        OR (sqlc.narg('suspended')::bool AND c.suspended_at IS NOT NULL)
        OR (NOT sqlc.narg('suspended')::bool AND c.suspended_at IS NULL))
  AND (sqlc.narg('teacher_slug')::text IS NULL OR t.slug = sqlc.narg('teacher_slug')::text)
  AND (sqlc.narg('q')::text IS NULL OR c.title ILIKE '%' || sqlc.narg('q')::text || '%')
ORDER BY c.created_at DESC, c.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: AdminCountCourses :one
SELECT count(*)
FROM courses c
JOIN teachers t ON t.id = c.teacher_id
WHERE (sqlc.narg('status')::text IS NULL
        OR (sqlc.narg('status')::text = 'archived' AND c.archived_at IS NOT NULL)
        OR (sqlc.narg('status')::text <> 'archived' AND c.status = sqlc.narg('status')::text AND c.archived_at IS NULL))
  AND (sqlc.narg('suspended')::bool IS NULL
        OR (sqlc.narg('suspended')::bool AND c.suspended_at IS NOT NULL)
        OR (NOT sqlc.narg('suspended')::bool AND c.suspended_at IS NULL))
  AND (sqlc.narg('teacher_slug')::text IS NULL OR t.slug = sqlc.narg('teacher_slug')::text)
  AND (sqlc.narg('q')::text IS NULL OR c.title ILIKE '%' || sqlc.narg('q')::text || '%');

-- name: SetCourseSuspended :one
-- Idempotent: suspending an already-suspended course (or unsuspending an
-- already-active one) is a no-op that still returns the current row — COALESCE
-- keeps the original suspended_at instead of sliding it forward on a repeat
-- call, same "don't move a timestamp that's already set" idiom as
-- UpsertItemProgress's completed_at. Returns the course row only; the
-- repository loads the teacher summary separately with GetTeacherSummary
-- (same two-query shape avoids a CTE-scoped ambiguous-column parse sqlc
-- rejects when the same query both updates and re-joins the touched row).
UPDATE courses
SET suspended_at = CASE WHEN sqlc.arg('suspended')::bool THEN COALESCE(suspended_at, now()) ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg('course_id')
RETURNING id, teacher_id, title, subtitle, description, cover_asset_id,
          price_amount_minor, price_currency, status, ever_published, archived_at,
          created_at, updated_at, suspended_at;

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

-- Phase C2: public catalog, purchase/enrollment, the student player, and
-- progress tracking.

-- name: GetTeacherOwnerID :one
-- The account that owns a teacher profile (the reverse of GetTeacherIDByOwner).
-- Used to resolve who to authorize as "the teacher" for a course (e.g. an
-- enrollment's grading authorization) without joining through users elsewhere.
SELECT user_id FROM teachers WHERE id = $1;

-- name: GetTeacherSummary :one
SELECT id, slug, display_name FROM teachers WHERE id = $1;

-- Catalog: published, non-archived, non-suspended courses only, from an
-- approved teacher (mirrors GetBookingTeacherContext's status = 'approved'
-- gate — a suspended teacher's courses shouldn't surface in the public
-- marketplace even if the course row itself is still 'published'). Phase C3
-- adds suspended_at IS NULL: an operator takedown must 404 the storefront the
-- same way an unpublished/archived course already does.

-- name: CatalogListCourses :many
-- sort: 'price_asc' | 'price_desc' | anything else (including "" / 'newest' /
-- 'recommended' — there is no rating-based ranking for courses yet) falls
-- back to newest-first. The two CASE columns are NULL for every row unless
-- their own sort is selected, so they never affect ordering otherwise and the
-- final created_at/id tiebreak always applies.
SELECT
    c.id, c.teacher_id, c.title, c.subtitle, c.description, c.cover_asset_id,
    c.price_amount_minor, c.price_currency, c.status, c.ever_published, c.archived_at,
    c.suspended_at, c.created_at, c.updated_at,
    t.slug AS teacher_slug, t.display_name AS teacher_display_name,
    (SELECT count(*) FROM course_sections cs WHERE cs.course_id = c.id) AS section_count,
    (SELECT count(*) FROM course_items ci JOIN course_sections cs2 ON cs2.id = ci.section_id WHERE cs2.course_id = c.id) AS item_count
FROM courses c
JOIN teachers t ON t.id = c.teacher_id
WHERE c.status = 'published' AND c.archived_at IS NULL AND c.suspended_at IS NULL AND t.status = 'approved'
  AND (sqlc.narg('q')::text IS NULL OR c.title ILIKE '%' || sqlc.narg('q')::text || '%' OR c.subtitle ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('max_price_minor')::bigint IS NULL OR c.price_amount_minor <= sqlc.narg('max_price_minor')::bigint)
ORDER BY
    (CASE WHEN sqlc.arg('sort')::text = 'price_asc'  THEN c.price_amount_minor END) ASC NULLS LAST,
    (CASE WHEN sqlc.arg('sort')::text = 'price_desc' THEN c.price_amount_minor END) DESC NULLS LAST,
    c.created_at DESC, c.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountCatalogCourses :one
SELECT count(*)
FROM courses c
JOIN teachers t ON t.id = c.teacher_id
WHERE c.status = 'published' AND c.archived_at IS NULL AND c.suspended_at IS NULL AND t.status = 'approved'
  AND (sqlc.narg('q')::text IS NULL OR c.title ILIKE '%' || sqlc.narg('q')::text || '%' OR c.subtitle ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('max_price_minor')::bigint IS NULL OR c.price_amount_minor <= sqlc.narg('max_price_minor')::bigint);

-- Enrollment.

-- name: InsertEnrollment :one
-- Insert-first idempotency: a duplicate (course_id, student_id) raises
-- SQLSTATE 23505 on the table's UNIQUE constraint, mapped by the repository
-- to load-and-return the existing row instead — never check-then-insert.
INSERT INTO course_enrollments (course_id, student_id, source, amount_paid_minor, currency)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, course_id, student_id, source, amount_paid_minor, currency, created_at;

-- name: GetEnrollmentByCourseAndStudent :one
SELECT id, course_id, student_id, source, amount_paid_minor, currency, created_at
FROM course_enrollments
WHERE course_id = $1 AND student_id = $2;

-- name: GetEnrollmentByID :one
SELECT id, course_id, student_id, source, amount_paid_minor, currency, created_at
FROM course_enrollments
WHERE id = $1;

-- name: ListEnrollmentsForStudent :many
-- "My learning": the student's enrollments, newest first, each with its
-- course summary and a progress percent computed from two correlated
-- subqueries (total curriculum items for the course; completed rows for this
-- specific enrollment) — cheap since a course has at most a few dozen items.
SELECT
    e.id, e.course_id, e.student_id, e.source, e.amount_paid_minor, e.currency, e.created_at,
    c.teacher_id AS course_teacher_id, c.title AS course_title, c.subtitle AS course_subtitle,
    c.description AS course_description, c.cover_asset_id AS course_cover_asset_id,
    c.price_amount_minor AS course_price_amount_minor, c.price_currency AS course_price_currency,
    c.status AS course_status, c.ever_published AS course_ever_published, c.archived_at AS course_archived_at,
    c.created_at AS course_created_at, c.updated_at AS course_updated_at,
    (SELECT count(*) FROM course_items ci JOIN course_sections cs ON cs.id = ci.section_id WHERE cs.course_id = c.id) AS total_items,
    (SELECT count(*) FROM course_item_progress cip WHERE cip.enrollment_id = e.id AND cip.status = 'completed') AS completed_items
FROM course_enrollments e
JOIN courses c ON c.id = e.course_id
WHERE e.student_id = sqlc.arg('student_id')
ORDER BY e.created_at DESC, e.id;

-- name: EnrollmentGrantsResource :one
-- Does this enrollment's course actually embed resourceID as a curriculum
-- item? The course-context equivalent of GetBookingResourceByPair's "is this
-- actually attached" check.
SELECT EXISTS(
    SELECT 1
    FROM course_enrollments e
    JOIN course_sections cs ON cs.course_id = e.course_id
    JOIN course_items ci ON ci.section_id = cs.id
    WHERE e.id = sqlc.arg('enrollment_id') AND ci.resource_id = sqlc.arg('resource_id')
) AS granted;

-- name: StudentHasResourceAccess :one
-- Does studentID have SOME active enrollment granting access to resourceID,
-- independent of which specific enrollment? Backs the course-based widening
-- of a course-embedded resource's material/audio file.
SELECT EXISTS(
    SELECT 1
    FROM course_enrollments e
    JOIN course_sections cs ON cs.course_id = e.course_id
    JOIN course_items ci ON ci.section_id = cs.id
    WHERE e.student_id = sqlc.arg('student_id') AND ci.resource_id = sqlc.arg('resource_id')
) AS accessible;

-- name: StudentHasVideoAccess :one
-- Backs files.AssigneeChecker's course-based widening: is fileAssetID a video
-- item's asset in some course the student is actively enrolled in?
SELECT EXISTS(
    SELECT 1
    FROM course_enrollments e
    JOIN course_sections cs ON cs.course_id = e.course_id
    JOIN course_items ci ON ci.section_id = cs.id
    WHERE e.student_id = sqlc.arg('student_id') AND ci.video_asset_id = sqlc.arg('file_asset_id')
) AS accessible;

-- name: GetEnrollmentParticipants :one
-- Resolves an enrollment's two participants for authorization: the course's
-- teacher's owning account, and the enrolled student.
SELECT t.user_id AS teacher_owner_id, e.student_id AS student_id
FROM course_enrollments e
JOIN courses c ON c.id = e.course_id
JOIN teachers t ON t.id = c.teacher_id
WHERE e.id = sqlc.arg('id');

-- name: GetCourseItemForEnrollmentResource :one
-- Resolves the curriculum item a course-context submission's resource
-- corresponds to, scoped to the enrollment's own course. Backs
-- resources.CourseProgress.ItemCompleted.
SELECT ci.id, ci.section_id, ci.kind, ci.title, ci.video_asset_id, ci.resource_id, ci.position, ci.created_at
FROM course_enrollments e
JOIN course_sections cs ON cs.course_id = e.course_id
JOIN course_items ci ON ci.section_id = cs.id
WHERE e.id = sqlc.arg('enrollment_id') AND ci.resource_id = sqlc.arg('resource_id')
LIMIT 1;

-- Progress.

-- name: UpsertItemProgress :one
-- Merges the given fields into a video item's progress row for a student
-- driving the player (position updates as they watch; explicit
-- complete/uncomplete). NULL args (a field the client didn't send) leave the
-- stored value unchanged. completed=true stamps completed_at once (COALESCE
-- keeps the first time it was ever set); completed=false clears it.
INSERT INTO course_item_progress (enrollment_id, item_id, video_position_seconds, status, completed_at)
VALUES (
    sqlc.arg('enrollment_id'), sqlc.arg('item_id'),
    COALESCE(sqlc.narg('video_position_seconds')::int, 0),
    CASE WHEN sqlc.narg('completed')::bool IS TRUE THEN 'completed' ELSE 'in_progress' END,
    CASE WHEN sqlc.narg('completed')::bool IS TRUE THEN now() ELSE NULL END
)
ON CONFLICT (enrollment_id, item_id) DO UPDATE SET
    video_position_seconds = COALESCE(sqlc.narg('video_position_seconds')::int, course_item_progress.video_position_seconds),
    status = CASE
        WHEN sqlc.narg('completed')::bool IS TRUE  THEN 'completed'
        WHEN sqlc.narg('completed')::bool IS FALSE THEN 'in_progress'
        ELSE course_item_progress.status
    END,
    completed_at = CASE
        WHEN sqlc.narg('completed')::bool IS TRUE  THEN COALESCE(course_item_progress.completed_at, now())
        WHEN sqlc.narg('completed')::bool IS FALSE THEN NULL
        ELSE course_item_progress.completed_at
    END,
    updated_at = now()
RETURNING id, enrollment_id, item_id, status, video_position_seconds, completed_at, updated_at;

-- name: CompleteItemProgress :one
-- Marks an item complete unconditionally (a course-embedded resource's
-- submission reaching submitted/graded, via resources.CourseProgress). Never
-- regresses video_position_seconds or an already-recorded completed_at.
INSERT INTO course_item_progress (enrollment_id, item_id, status, completed_at)
VALUES (sqlc.arg('enrollment_id'), sqlc.arg('item_id'), 'completed', sqlc.arg('completed_at'))
ON CONFLICT (enrollment_id, item_id) DO UPDATE SET
    status = 'completed',
    completed_at = COALESCE(course_item_progress.completed_at, sqlc.arg('completed_at')),
    updated_at = now()
RETURNING id, enrollment_id, item_id, status, video_position_seconds, completed_at, updated_at;

-- name: ListItemProgressForEnrollment :many
SELECT id, enrollment_id, item_id, status, video_position_seconds, completed_at, updated_at
FROM course_item_progress
WHERE enrollment_id = $1;

-- Seed-only. FK order (deepest first): course_item_progress -> course_items/
-- course_enrollments; submissions.enrollment_id -> course_enrollments (cleared
-- by resources.DeleteAllSubmissions, called by cmd/seed before
-- DeleteAllCourseEnrollments); course_enrollments -> courses/users;
-- course_items -> course_sections; course_sections -> courses; courses ->
-- teachers. cmd/seed clears all of these before DeleteAllTeachers/
-- DeleteAllUsers, same "explicit, deepest-first" discipline as its existing
-- booking/payment clearing.

-- name: DeleteAllCourseItemProgress :exec
DELETE FROM course_item_progress;

-- name: DeleteAllCourseEnrollments :exec
DELETE FROM course_enrollments;

-- name: DeleteAllCourseItems :exec
DELETE FROM course_items;

-- name: DeleteAllCourseSections :exec
DELETE FROM course_sections;

-- name: DeleteAllCourses :exec
DELETE FROM courses;
