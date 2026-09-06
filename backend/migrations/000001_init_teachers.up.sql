-- Teachers module: a teacher profile plus its child collections
-- (languages, focus tags, experience entries).

CREATE TYPE teacher_kind AS ENUM ('professional', 'community');
CREATE TYPE language_role AS ENUM ('teaches', 'also_speaks');
CREATE TYPE language_level AS ENUM ('native', 'c2', 'c1', 'b2', 'b1', 'a2', 'a1');
CREATE TYPE currency_code AS ENUM ('UZS', 'USD');

CREATE TABLE teachers (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                  text NOT NULL UNIQUE,
    display_name          text NOT NULL,
    headline              text NOT NULL,
    kind                  teacher_kind NOT NULL,

    country_code          text NOT NULL,          -- ISO 3166-1 alpha-2
    country_name          text NOT NULL,
    city                  text NOT NULL,
    timezone              text NOT NULL,          -- IANA name, e.g. Asia/Tashkent

    price_per_hour_minor  bigint NOT NULL CHECK (price_per_hour_minor >= 0),
    trial_price_minor     bigint CHECK (trial_price_minor >= 0), -- NULL = no trial offered
    currency              currency_code NOT NULL DEFAULT 'UZS',

    -- real (not numeric): rating is a display-only aggregate to one decimal;
    -- float4 avoids the awkward pgtype.Numeric mapping in Go.
    rating                real NOT NULL DEFAULT 0 CHECK (rating >= 0 AND rating <= 5),
    review_count          integer NOT NULL DEFAULT 0 CHECK (review_count >= 0),
    lessons_completed     integer NOT NULL DEFAULT 0 CHECK (lessons_completed >= 0),
    student_count         integer NOT NULL DEFAULT 0 CHECK (student_count >= 0),
    response_time_hours   integer NOT NULL DEFAULT 24 CHECK (response_time_hours >= 0),
    accepting_students    boolean NOT NULL DEFAULT true,

    avatar_url            text NOT NULL DEFAULT '',
    video_thumbnail_url   text NOT NULL DEFAULT '',
    intro_video_url       text NOT NULL DEFAULT '',
    about                 text NOT NULL DEFAULT '',
    teaching_style        text NOT NULL DEFAULT '',

    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE teacher_languages (
    teacher_id  uuid NOT NULL REFERENCES teachers (id) ON DELETE CASCADE,
    role        language_role NOT NULL,
    code        text NOT NULL,          -- ISO 639-1
    name        text NOT NULL,
    level       language_level NOT NULL,
    position    integer NOT NULL DEFAULT 0,
    PRIMARY KEY (teacher_id, role, code)
);

CREATE TABLE teacher_focus (
    teacher_id  uuid NOT NULL REFERENCES teachers (id) ON DELETE CASCADE,
    tag         text NOT NULL,
    position    integer NOT NULL DEFAULT 0,
    PRIMARY KEY (teacher_id, tag)
);

CREATE TABLE teacher_experience (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    teacher_id  uuid NOT NULL REFERENCES teachers (id) ON DELETE CASCADE,
    title       text NOT NULL,
    org         text NOT NULL,
    period      text NOT NULL,          -- free text, e.g. "2019 – present"
    position    integer NOT NULL DEFAULT 0
);

-- Filtering by the language a teacher *teaches* is the hot path for search.
CREATE INDEX teacher_languages_teaches_code_idx
    ON teacher_languages (code)
    WHERE role = 'teaches';

CREATE INDEX teacher_experience_teacher_idx ON teacher_experience (teacher_id, position);
CREATE INDEX teachers_accepting_rating_idx ON teachers (accepting_students, rating DESC);
