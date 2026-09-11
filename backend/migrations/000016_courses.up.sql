-- Phase C1: self-paced video courses, authoring only. A teacher builds a
-- course's curriculum here — ordered sections, each holding ordered items
-- that are either an uploaded video or a reference to one of the teacher's
-- own published resources (internal/resources' library, reused rather than
-- duplicated: a quiz or reading passage already built for a lesson can be
-- dropped straight into a course). There is no purchase, catalog, enrollment,
-- payout, or student-facing surface yet — see CLAUDE.md / the phase plan for
-- scope. `price_amount_minor` / `price_currency` are stored now so the later
-- catalog phase doesn't need a schema change, but nothing reads them yet; 0 is
-- a valid, meaningful "free course" price, not a placeholder for "unset".
--
-- `ever_published` tracks whether the course has ever left draft, kept
-- separate from `status` because `status` is reversible (unpublish moves a
-- published course back to `draft`, mirroring resources' publish/unpublish).
-- Delete is blocked once ever_published is true — archive instead. That's a
-- simpler rule than resources.IsAssigned (which checks a real cross-module
-- attachment) because nothing outside this module references a course yet in
-- this phase; "was this ever live" is the only history worth protecting.

CREATE TABLE courses (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id          uuid NOT NULL REFERENCES teachers (id) ON DELETE CASCADE,

    title               text NOT NULL,
    subtitle            text NOT NULL DEFAULT '',
    description         text NOT NULL DEFAULT '',
    cover_asset_id      uuid REFERENCES file_assets (id),

    price_amount_minor  bigint NOT NULL DEFAULT 0 CHECK (price_amount_minor >= 0),
    price_currency      currency_code NOT NULL DEFAULT 'UZS',

    status              text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    ever_published      boolean NOT NULL DEFAULT false,
    archived_at         timestamptz,

    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- The authoring library list: a teacher's own courses, newest first. Mirrors
-- resources_teacher_idx.
CREATE INDEX courses_teacher_idx ON courses (teacher_id, created_at DESC);

-- course_sections — the curriculum's ordered top level. `position` is a plain
-- integer rewritten 0..n-1 on every reorder (no fractional-position scheme:
-- a course has at most a few dozen sections, so a full rewrite in one
-- statement is cheap and keeps the ordering exact and gap-free).
CREATE TABLE course_sections (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id    uuid NOT NULL REFERENCES courses (id) ON DELETE CASCADE,

    title        text NOT NULL,
    position     integer NOT NULL DEFAULT 0,

    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX course_sections_course_idx ON course_sections (course_id, position);

-- course_items — a section's ordered leaves. Exactly one of video_asset_id /
-- resource_id is set, matching `kind` — enforced by the CHECK below, not just
-- application convention, so a bad row can never land regardless of which
-- code path wrote it. `title` is an optional display-title override: '' falls
-- back to the video's filename or the resource's own title in the UI, so most
-- items never need one typed in by hand.
CREATE TABLE course_items (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    section_id      uuid NOT NULL REFERENCES course_sections (id) ON DELETE CASCADE,

    kind            text NOT NULL CHECK (kind IN ('video', 'resource')),
    title           text NOT NULL DEFAULT '',
    video_asset_id  uuid REFERENCES file_assets (id),
    resource_id     uuid REFERENCES resources (id),
    position        integer NOT NULL DEFAULT 0,

    created_at      timestamptz NOT NULL DEFAULT now(),

    CHECK (
        (kind = 'video'    AND video_asset_id IS NOT NULL AND resource_id IS NULL)
        OR
        (kind = 'resource' AND resource_id IS NOT NULL AND video_asset_id IS NULL)
    )
);

CREATE INDEX course_items_section_idx ON course_items (section_id, position);
