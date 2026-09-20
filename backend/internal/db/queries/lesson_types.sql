-- Lesson types: a teacher's 1-on-1 offerings and their per-duration prices.
-- Prices are read as a separate pass (by type id) rather than joined, so a
-- type with no prices yet still comes back and the caller assembles the tree.

-- name: ListLessonTypes :many
-- The teacher's live offerings, trial first, then by position.
SELECT id, teacher_id, title, description, is_trial, archived, position, created_at, updated_at
FROM lesson_types
WHERE teacher_id = $1
  AND (NOT archived OR sqlc.arg('include_archived')::bool)
ORDER BY is_trial DESC, position, created_at;

-- name: ListLessonTypePrices :many
SELECT lesson_type_id, duration_minutes, price_minor
FROM lesson_type_prices
WHERE lesson_type_id = ANY(sqlc.arg('lesson_type_ids')::uuid[])
ORDER BY lesson_type_id, duration_minutes;

-- name: GetLessonType :one
SELECT id, teacher_id, title, description, is_trial, archived, position, created_at, updated_at
FROM lesson_types
WHERE id = $1;

-- name: CreateLessonType :one
INSERT INTO lesson_types (teacher_id, title, description, is_trial, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, teacher_id, title, description, is_trial, archived, position, created_at, updated_at;

-- name: UpdateLessonType :one
UPDATE lesson_types
SET title = $2, description = $3, position = $4, updated_at = now()
WHERE id = $1
RETURNING id, teacher_id, title, description, is_trial, archived, position, created_at, updated_at;

-- name: SetLessonTypeArchived :one
UPDATE lesson_types
SET archived = $2, updated_at = now()
WHERE id = $1
RETURNING id, teacher_id, title, description, is_trial, archived, position, created_at, updated_at;

-- name: DeleteLessonTypePrices :exec
DELETE FROM lesson_type_prices WHERE lesson_type_id = $1;

-- name: InsertLessonTypePrice :exec
-- Replace is delete-then-insert; a price list is at most five rows, so the
-- repository loops rather than carrying a two-array unnest.
INSERT INTO lesson_type_prices (lesson_type_id, duration_minutes, price_minor)
VALUES ($1, $2, $3);

-- name: LessonTypeOffering :one
-- What the booking service needs to price one lesson: the offering, its owner
-- and the price for the requested duration. No row means the duration is not
-- offered (or the type is archived / belongs to another teacher).
SELECT lt.id, lt.teacher_id, lt.title, lt.is_trial, p.duration_minutes, p.price_minor
FROM lesson_types lt
JOIN lesson_type_prices p ON p.lesson_type_id = lt.id
WHERE lt.id = $1 AND NOT lt.archived AND p.duration_minutes = $2;

-- name: LessonTypeDurations :many
-- Durations a live offering can be booked for, cheapest first.
SELECT duration_minutes, price_minor
FROM lesson_type_prices p
JOIN lesson_types lt ON lt.id = p.lesson_type_id
WHERE p.lesson_type_id = $1 AND NOT lt.archived
ORDER BY duration_minutes;

-- name: DeleteAllLessonTypePrices :exec
-- Seed-only (no ON DELETE CASCADE in this schema — see cmd/seed).
DELETE FROM lesson_type_prices;

-- name: DeleteAllLessonTypes :exec
DELETE FROM lesson_types;
