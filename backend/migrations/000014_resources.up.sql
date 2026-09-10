-- Learning resources (phase A1): a teacher's reusable library of materials and
-- graded tasks. These plug into scheduled lessons as homework (phase A2) and
-- into self-paced courses later; this migration only creates the library +
-- the blob store it references.
--
-- file_assets — one row per uploaded file (PDF / image / audio). The bytes live
-- in a blob store (local disk in dev, R2 in prod) behind a port; the row is the
-- app's handle to them. `key` is the store-relative object key.
--
-- resources.content is JSONB — the first in the schema, deliberate: the payload
-- is a self-contained document (quiz questions, an article body, a writing
-- prompt), always read whole, and its shape varies by `type`. The service
-- validates it on write; Postgres only guarantees it is valid JSON.

CREATE TABLE file_assets (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider     text NOT NULL CHECK (provider IN ('disk', 'r2')),
    object_key   text NOT NULL,
    filename     text NOT NULL,
    content_type text NOT NULL,
    bytes        bigint NOT NULL CHECK (bytes >= 0),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX file_assets_owner_idx ON file_assets (owner_id);

CREATE TABLE resources (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id   uuid NOT NULL REFERENCES teachers (id) ON DELETE CASCADE,

    type         text NOT NULL
        CHECK (type IN ('material', 'article', 'quiz', 'listening', 'reading', 'writing')),
    title        text NOT NULL,
    instructions text NOT NULL DEFAULT '',

    -- Type-specific payload. See the package doc for the shape per type.
    content      jsonb NOT NULL DEFAULT '{}'::jsonb,

    status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    archived_at  timestamptz,

    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- The library list: a teacher's own resources, newest first, filtered by
-- type / status / archived.
CREATE INDEX resources_teacher_idx ON resources (teacher_id, created_at DESC);
