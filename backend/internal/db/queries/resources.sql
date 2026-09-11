-- Learning resources library (phase A1).

-- name: CreateResource :one
INSERT INTO resources (teacher_id, type, title, instructions, content, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, teacher_id, type, title, instructions, content, status, archived_at, created_at, updated_at;

-- name: GetResource :one
SELECT id, teacher_id, type, title, instructions, content, status, archived_at, created_at, updated_at
FROM resources
WHERE id = $1;

-- name: ListTeacherResources :many
-- The library list. Filters are optional: type, status ('draft'|'published'),
-- and whether to include archived rows.
SELECT id, teacher_id, type, title, instructions, content, status, archived_at, created_at, updated_at
FROM resources
WHERE teacher_id = $1
  AND (sqlc.narg('type')::text   IS NULL OR type = sqlc.narg('type')::text)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.arg('include_archived')::bool OR archived_at IS NULL)
ORDER BY created_at DESC, id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountTeacherResources :one
SELECT count(*)
FROM resources
WHERE teacher_id = $1
  AND (sqlc.narg('type')::text   IS NULL OR type = sqlc.narg('type')::text)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.arg('include_archived')::bool OR archived_at IS NULL);

-- name: UpdateResource :one
-- Partial edit: title / instructions / content are replaced wholesale when
-- provided (the service passes the current value for fields it isn't changing).
UPDATE resources
SET title = $2, instructions = $3, content = $4, updated_at = now()
WHERE id = $1
RETURNING id, teacher_id, type, title, instructions, content, status, archived_at, created_at, updated_at;

-- name: SetResourceStatus :one
UPDATE resources
SET status = sqlc.arg('status'), updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, teacher_id, type, title, instructions, content, status, archived_at, created_at, updated_at;

-- name: SetResourceArchived :one
UPDATE resources
SET archived_at = CASE WHEN sqlc.arg('archived')::bool THEN now() ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, teacher_id, type, title, instructions, content, status, archived_at, created_at, updated_at;

-- name: DeleteResource :exec
DELETE FROM resources WHERE id = $1;

-- name: IsResourceAssigned :one
-- Phase A2: can this resource be deleted? True once it has ever been attached
-- to a booking (detaching does not clear this — attach it once, and it is
-- "in use" for delete purposes forever; archive instead).
SELECT EXISTS(SELECT 1 FROM booking_resources WHERE resource_id = $1);

-- name: DeleteAllResources :exec
-- Seed-only. resources.teacher_id -> teachers cascades; kept explicit + ordered.
DELETE FROM resources;

-- Lesson resources (phase A2/A3): attaching a resource to a booking, and the
-- student's submissions against it.

-- name: AttachBookingResource :one
-- position is the current attachment count for the booking, computed here so
-- the insert stays a single statement. A duplicate (booking_id, resource_id)
-- raises SQLSTATE 23505 on the table's UNIQUE constraint, mapped by the
-- repository to ErrAlreadyAttached — race-safe, never a check-then-insert.
WITH ins AS (
    INSERT INTO booking_resources (booking_id, resource_id, kind, assigned_by, due_at, position)
    VALUES (
        sqlc.arg('booking_id'), sqlc.arg('resource_id'), sqlc.arg('kind'),
        sqlc.arg('assigned_by'), sqlc.narg('due_at'),
        COALESCE((SELECT max(position) + 1 FROM booking_resources WHERE booking_id = sqlc.arg('booking_id')), 0)
    )
    RETURNING id, booking_id, resource_id, kind, position, assigned_by, due_at, created_at
)
SELECT ins.id, ins.booking_id, ins.resource_id, ins.kind, ins.position, ins.assigned_by, ins.due_at, ins.created_at,
       r.type, r.title, r.instructions, r.status, r.content
FROM ins
JOIN resources r ON r.id = ins.resource_id;

-- name: DetachBookingResource :execrows
DELETE FROM booking_resources WHERE id = $1 AND booking_id = $2;

-- name: ListBookingResources :many
-- A booking's attachments, teacher-ordered, each with its resource's display
-- fields joined in (title/type/status/content) so the caller never N+1s.
SELECT br.id, br.booking_id, br.resource_id, br.kind, br.position, br.assigned_by, br.due_at, br.created_at,
       r.type, r.title, r.instructions, r.status, r.content
FROM booking_resources br
JOIN resources r ON r.id = br.resource_id
WHERE br.booking_id = $1
ORDER BY br.position, br.id;

-- name: GetBookingResourceByPair :one
-- Is this resource attached to this booking, and as what kind? Backs the
-- "homework assigned on this booking" check before a submission may start.
SELECT br.id, br.booking_id, br.resource_id, br.kind, br.position, br.assigned_by, br.due_at, br.created_at,
       r.type, r.title, r.instructions, r.status, r.content
FROM booking_resources br
JOIN resources r ON r.id = br.resource_id
WHERE br.booking_id = $1 AND br.resource_id = $2;

-- name: StartOrGetSubmission :one
-- Idempotent "start homework": a second call for the same
-- (resource_id, student_id, booking_id) returns the existing row rather than
-- erroring — the no-op ON CONFLICT DO UPDATE is required to get RETURNING on
-- a conflict (DO NOTHING skips it).
INSERT INTO submissions (resource_id, student_id, booking_id, context)
VALUES (sqlc.arg('resource_id'), sqlc.arg('student_id'), sqlc.arg('booking_id'), 'lesson')
ON CONFLICT (resource_id, student_id, booking_id) WHERE booking_id IS NOT NULL
DO UPDATE SET updated_at = submissions.updated_at
RETURNING id, resource_id, student_id, context, booking_id, status, answers,
          auto_score, auto_max, teacher_score, teacher_feedback, graded_by, graded_at,
          submitted_at, created_at, updated_at;

-- name: SaveSubmissionAnswers :one
UPDATE submissions
SET answers = sqlc.arg('answers'), updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, resource_id, student_id, context, booking_id, status, answers,
          auto_score, auto_max, teacher_score, teacher_feedback, graded_by, graded_at,
          submitted_at, created_at, updated_at;

-- name: SubmitSubmission :one
-- Writes the outcome of POST /v1/submissions/{id}/submit. For an
-- auto-gradable type the caller passes status='graded' with the computed
-- auto_score/auto_max; for `writing` it passes status='submitted' with both
-- null.
UPDATE submissions
SET status       = sqlc.arg('status'),
    auto_score   = sqlc.narg('auto_score'),
    auto_max     = sqlc.narg('auto_max'),
    submitted_at = sqlc.arg('submitted_at'),
    updated_at   = now()
WHERE id = sqlc.arg('id')
RETURNING id, resource_id, student_id, context, booking_id, status, answers,
          auto_score, auto_max, teacher_score, teacher_feedback, graded_by, graded_at,
          submitted_at, created_at, updated_at;

-- name: GradeSubmission :one
-- Only a `writing` submission reaches this (enforced in the service);
-- teacher_score is nullable — a teacher may grade with feedback only.
UPDATE submissions
SET teacher_score    = sqlc.narg('teacher_score'),
    teacher_feedback = sqlc.arg('teacher_feedback'),
    graded_by        = sqlc.arg('graded_by'),
    graded_at        = sqlc.arg('graded_at'),
    status           = 'graded',
    updated_at       = now()
WHERE id = sqlc.arg('id')
RETURNING id, resource_id, student_id, context, booking_id, status, answers,
          auto_score, auto_max, teacher_score, teacher_feedback, graded_by, graded_at,
          submitted_at, created_at, updated_at;

-- name: GetSubmissionByID :one
SELECT id, resource_id, student_id, context, booking_id, status, answers,
       auto_score, auto_max, teacher_score, teacher_feedback, graded_by, graded_at,
       submitted_at, created_at, updated_at
FROM submissions
WHERE id = $1;

-- name: ListSubmissionsForBooking :many
-- Every submission filed against one booking's homeworks. A booking has
-- exactly one student, so this never needs a student filter of its own.
SELECT id, resource_id, student_id, context, booking_id, status, answers,
       auto_score, auto_max, teacher_score, teacher_feedback, graded_by, graded_at,
       submitted_at, created_at, updated_at
FROM submissions
WHERE booking_id = $1;

-- name: TeacherSubmissionInbox :many
-- The grading inbox: a teacher's own resources' submissions, filtered by
-- status (default 'submitted' in the service), newest-submitted-first.
SELECT s.id, s.resource_id, s.student_id, s.context, s.booking_id, s.status, s.answers,
       s.auto_score, s.auto_max, s.teacher_score, s.teacher_feedback, s.graded_by, s.graded_at,
       s.submitted_at, s.created_at, s.updated_at
FROM submissions s
JOIN resources r ON r.id = s.resource_id
WHERE r.teacher_id = sqlc.arg('teacher_id')
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status')::text)
ORDER BY s.submitted_at DESC NULLS LAST, s.id
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountTeacherSubmissionInbox :one
SELECT count(*)
FROM submissions s
JOIN resources r ON r.id = s.resource_id
WHERE r.teacher_id = sqlc.arg('teacher_id')
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status')::text);

-- name: FileAssetAccessibleToStudent :one
-- Widens files.Download beyond owner-only: true iff some resource whose
-- content carries this file_asset_id / audio_asset_id is attached to a
-- booking belonging to this student.
SELECT EXISTS(
    SELECT 1
    FROM booking_resources br
    JOIN resources res ON res.id = br.resource_id
    JOIN bookings b ON b.id = br.booking_id
    WHERE b.student_id = sqlc.arg('requester_id')
      AND (
          (res.content ->> 'file_asset_id') = sqlc.arg('file_asset_id')::text
          OR (res.content ->> 'audio_asset_id') = sqlc.arg('file_asset_id')::text
      )
) AS accessible;

-- name: GetUserContact :one
-- A plain contact lookup, used only for the grading-done email.
SELECT email::text AS email, display_name FROM users WHERE id = $1;

-- name: DeleteAllSubmissions :exec
-- Seed-only. submissions.booking_id cascades, but resource_id / student_id
-- reference resources / users with no cascade, so the seed clears submissions
-- before those tables.
DELETE FROM submissions;

-- name: DeleteAllBookingResources :exec
-- Seed-only. booking_resources.booking_id cascades; resource_id has no
-- cascade, so the seed clears attachments before resources.
DELETE FROM booking_resources;
