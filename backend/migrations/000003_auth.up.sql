-- Auth module: user accounts (email + argon2id password hash) and refresh-token
-- sessions. A "session" is one issued refresh token: the row stores only the
-- SHA-256 of the opaque token, its expiry, and a rotation chain (replaced_by)
-- so a re-used token can be detected and the whole chain revoked.
--
-- teachers.user_id links a teacher profile to the account that owns it; it is
-- nullable so an unclaimed profile can exist, and ownership checks compare it to
-- the caller's authenticated user id.

-- citext gives us case-insensitive, case-preserving emails without lower()
-- everywhere.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email          citext NOT NULL UNIQUE,
    password_hash  text NOT NULL,          -- PHC string: $argon2id$v=19$m=...
    display_name   text NOT NULL,

    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,

    -- SHA-256 (32 bytes) of the opaque base64url refresh token. The raw token
    -- is only ever held by the client (in an HttpOnly cookie).
    refresh_token_hash  bytea NOT NULL,

    expires_at          timestamptz NOT NULL,
    revoked_at          timestamptz,                       -- NULL = still active
    replaced_by         uuid REFERENCES sessions (id),     -- set on rotation

    user_agent          text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_refresh_token_hash_idx ON sessions (refresh_token_hash);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

ALTER TABLE teachers
    ADD COLUMN user_id uuid REFERENCES users (id);

CREATE INDEX teachers_user_id_idx ON teachers (user_id);
