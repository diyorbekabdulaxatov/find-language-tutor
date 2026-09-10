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

-- name: DeleteAllResources :exec
-- Seed-only. resources.teacher_id -> teachers cascades; kept explicit + ordered.
DELETE FROM resources;
