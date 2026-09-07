-- Availability module: a teacher's weekly recurring slots (UTC minutes).

-- name: GetTeacherAvailabilityContext :one
-- Resolve a slug to the teacher id and timezone the availability endpoints need.
SELECT id, slug, timezone FROM teachers WHERE slug = $1;

-- name: ListAvailabilitySlots :many
SELECT weekday, start_minute, end_minute
FROM teacher_availability_slots
WHERE teacher_id = $1
ORDER BY weekday, start_minute;

-- name: DeleteAvailabilitySlots :exec
DELETE FROM teacher_availability_slots WHERE teacher_id = $1;

-- name: AddAvailabilitySlot :exec
INSERT INTO teacher_availability_slots (teacher_id, weekday, start_minute, end_minute)
VALUES ($1, $2, $3, $4);
