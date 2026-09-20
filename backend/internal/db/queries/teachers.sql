-- name: ListTeachers :many
-- Page of teachers matching the optional filters, ordered by the requested sort.
-- Child collections (languages, focus, experience) are loaded separately by the
-- repository using the returned ids.
--
-- from_price_minor is derived, not stored: the price filter and the price sorts
-- run on it so that what a student filters by is what they can actually book.
SELECT t.*, COALESCE(lp.min_price_minor, t.price_per_hour_minor)::bigint AS from_price_minor
FROM teachers t
LEFT JOIN LATERAL (
    -- The cheapest live, non-trial offering: what a card means by "from
    -- 45,000 so'm". Trials are excluded so one cheap taster lesson can't make
    -- a whole profile look cheap, and min() over no rows is NULL, which the
    -- COALESCE below turns back into the teacher's headline hourly rate (a
    -- profile whose offerings are all archived, or one made before them).
    SELECT min(p.price_minor) AS min_price_minor
    FROM lesson_types lt
    JOIN lesson_type_prices p ON p.lesson_type_id = lt.id
    WHERE lt.teacher_id = t.id AND NOT lt.archived AND NOT lt.is_trial
) lp ON true
WHERE
    t.status = 'approved'
    AND (sqlc.narg('kind')::teacher_kind IS NULL OR t.kind = sqlc.narg('kind')::teacher_kind)
    AND (sqlc.narg('max_price_minor')::bigint IS NULL OR COALESCE(lp.min_price_minor, t.price_per_hour_minor) <= sqlc.narg('max_price_minor')::bigint)
    AND (
        sqlc.narg('language')::text IS NULL
        OR EXISTS (
            SELECT 1 FROM teacher_languages tl
            WHERE tl.teacher_id = t.id AND tl.role = 'teaches' AND tl.code = sqlc.narg('language')::text
        )
    )
    AND (
        sqlc.narg('q')::text IS NULL
        OR t.display_name ILIKE '%' || sqlc.narg('q') || '%'
        OR t.headline ILIKE '%' || sqlc.narg('q') || '%'
        OR EXISTS (
            SELECT 1 FROM teacher_focus tf
            WHERE tf.teacher_id = t.id AND tf.tag ILIKE '%' || sqlc.narg('q') || '%'
        )
    )
ORDER BY
    CASE WHEN sqlc.arg('sort')::text = 'recommended' THEN t.accepting_students END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'recommended' THEN t.rating END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'rating_desc' THEN t.rating END DESC,
    -- Spelled out rather than reusing the from_price_minor output alias: an
    -- alias is only visible to ORDER BY as a bare item, not inside a CASE.
    CASE WHEN sqlc.arg('sort')::text = 'price_asc' THEN COALESCE(lp.min_price_minor, t.price_per_hour_minor) END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'price_desc' THEN COALESCE(lp.min_price_minor, t.price_per_hour_minor) END DESC,
    t.review_count DESC,
    t.id
LIMIT sqlc.arg('page_limit')::int
OFFSET sqlc.arg('page_offset')::int;

-- name: CountTeachers :one
SELECT count(*)
FROM teachers t
LEFT JOIN LATERAL (
    -- The cheapest live, non-trial offering: what a card means by "from
    -- 45,000 so'm". Trials are excluded so one cheap taster lesson can't make
    -- a whole profile look cheap, and min() over no rows is NULL, which the
    -- COALESCE below turns back into the teacher's headline hourly rate (a
    -- profile whose offerings are all archived, or one made before them).
    SELECT min(p.price_minor) AS min_price_minor
    FROM lesson_types lt
    JOIN lesson_type_prices p ON p.lesson_type_id = lt.id
    WHERE lt.teacher_id = t.id AND NOT lt.archived AND NOT lt.is_trial
) lp ON true
WHERE
    t.status = 'approved'
    AND (sqlc.narg('kind')::teacher_kind IS NULL OR t.kind = sqlc.narg('kind')::teacher_kind)
    AND (sqlc.narg('max_price_minor')::bigint IS NULL OR COALESCE(lp.min_price_minor, t.price_per_hour_minor) <= sqlc.narg('max_price_minor')::bigint)
    AND (
        sqlc.narg('language')::text IS NULL
        OR EXISTS (
            SELECT 1 FROM teacher_languages tl
            WHERE tl.teacher_id = t.id AND tl.role = 'teaches' AND tl.code = sqlc.narg('language')::text
        )
    )
    AND (
        sqlc.narg('q')::text IS NULL
        OR t.display_name ILIKE '%' || sqlc.narg('q') || '%'
        OR t.headline ILIKE '%' || sqlc.narg('q') || '%'
        OR EXISTS (
            SELECT 1 FROM teacher_focus tf
            WHERE tf.teacher_id = t.id AND tf.tag ILIKE '%' || sqlc.narg('q') || '%'
        )
    );

-- name: GetTeacherBySlug :one
SELECT * FROM teachers WHERE slug = $1;

-- name: ListLanguagesForTeachers :many
SELECT teacher_id, role, code, name, level, position
FROM teacher_languages
WHERE teacher_id = ANY(sqlc.arg('teacher_ids')::uuid[])
ORDER BY teacher_id, role, position, code;

-- name: ListFocusForTeachers :many
SELECT teacher_id, tag, position
FROM teacher_focus
WHERE teacher_id = ANY(sqlc.arg('teacher_ids')::uuid[])
ORDER BY teacher_id, position, tag;

-- name: ListExperienceForTeachers :many
SELECT teacher_id, title, org, period, position
FROM teacher_experience
WHERE teacher_id = ANY(sqlc.arg('teacher_ids')::uuid[])
ORDER BY teacher_id, position;

-- name: LanguageFacets :many
-- Count of teachers per taught language across the whole catalog. Drives the
-- language filter in the UI, so it is intentionally unfiltered.
SELECT code, name, count(DISTINCT teacher_id)::bigint AS teacher_count
FROM teacher_languages
WHERE role = 'teaches'
GROUP BY code, name
ORDER BY name;

-- name: ListTeacherSlugs :many
-- Public: the frontend's static-generation slug list. Non-approved teachers are
-- not public, so they are excluded here too.
SELECT slug FROM teachers WHERE status = 'approved' ORDER BY slug;
