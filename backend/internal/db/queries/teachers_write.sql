-- Write queries — currently only used by cmd/seed. Real teacher-profile
-- mutations (create/update from the dashboard) will live here too.

-- name: CreateTeacher :one
INSERT INTO teachers (
    slug, display_name, headline, kind,
    country_code, country_name, city, timezone,
    price_per_hour_minor, trial_price_minor, currency,
    rating, review_count, lessons_completed, student_count,
    response_time_hours, accepting_students,
    avatar_url, video_thumbnail_url, intro_video_url, about, teaching_style,
    user_id
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, $10, $11,
    $12, $13, $14, $15,
    $16, $17,
    $18, $19, $20, $21, $22,
    $23
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
