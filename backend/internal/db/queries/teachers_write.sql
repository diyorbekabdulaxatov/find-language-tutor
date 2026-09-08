-- Write queries: the demo seed plus the dashboard's create/update-profile flow.

-- name: TeacherRefBySlug :one
-- Slug -> id + owning user, for the ownership check on PATCH /v1/teachers/{slug}.
SELECT id, slug, user_id FROM teachers WHERE slug = $1;

-- name: TeacherRefByOwner :one
-- The teacher profile owned by a user (one per user), or no rows.
SELECT id, slug, user_id FROM teachers WHERE user_id = $1;

-- name: TeacherSlugExists :one
SELECT EXISTS (SELECT 1 FROM teachers WHERE slug = $1);

-- name: UpdateTeacher :exec
-- Edit the caller-editable profile fields. Server-controlled aggregates (rating,
-- review_count, lessons_completed, student_count, response_time_hours,
-- accepting_students) and the slug are intentionally left untouched.
UPDATE teachers SET
    display_name         = $2,
    headline             = $3,
    kind                 = $4,
    country_code         = $5,
    country_name         = $6,
    city                 = $7,
    timezone             = $8,
    price_per_hour_minor = $9,
    trial_price_minor    = $10,
    currency             = $11,
    about                = $12,
    teaching_style       = $13,
    avatar_url           = $14,
    video_thumbnail_url  = $15,
    intro_video_url      = $16,
    meeting_url          = $17,
    updated_at           = now()
WHERE id = $1;

-- name: DeleteTeacherLanguages :exec
DELETE FROM teacher_languages WHERE teacher_id = $1;

-- name: DeleteTeacherFocus :exec
DELETE FROM teacher_focus WHERE teacher_id = $1;

-- name: DeleteTeacherExperience :exec
DELETE FROM teacher_experience WHERE teacher_id = $1;

-- name: CreateTeacher :one
-- Both the demo seed and POST /v1/teachers insert through here. The seed passes
-- status = 'approved'; the API create path passes 'pending' so a new profile is
-- not public until an admin approves it. verified defaults to false.
INSERT INTO teachers (
    slug, display_name, headline, kind,
    country_code, country_name, city, timezone,
    price_per_hour_minor, trial_price_minor, currency,
    rating, review_count, lessons_completed, student_count,
    response_time_hours, accepting_students,
    avatar_url, video_thumbnail_url, intro_video_url, about, teaching_style,
    user_id, status, verified
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, $10, $11,
    $12, $13, $14, $15,
    $16, $17,
    $18, $19, $20, $21, $22,
    $23, $24, $25
)
RETURNING id;

-- name: AddTeacherLanguage :exec
INSERT INTO teacher_languages (teacher_id, role, code, name, level, position)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: AddTeacherFocus :exec
INSERT INTO teacher_focus (teacher_id, tag, position)
VALUES ($1, $2, $3);

-- name: AddTeacherExperience :exec
INSERT INTO teacher_experience (teacher_id, title, org, period, position)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteAllTeachers :exec
DELETE FROM teachers;
