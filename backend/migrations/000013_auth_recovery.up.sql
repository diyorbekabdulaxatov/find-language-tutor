-- Account recovery: email verification + password reset.
--
-- users.email_verified_at — NULL until the account confirms its address via the
-- link mailed on registration. Nothing is *blocked* on verification yet (the
-- frontend just shows a nudge banner); the column is here so a future gate can
-- key off it. Existing rows stay NULL; the seed stamps the demo accounts
-- verified so they don't all show the banner.
ALTER TABLE users
    ADD COLUMN email_verified_at timestamptz;

-- One table for both single-use link tokens. Only the SHA-256 of the token is
-- stored (same as sessions.refresh_token_hash) — the raw value lives only in
-- the emailed URL. A token is spent by stamping consumed_at; expires_at bounds
-- its life even if never used.
CREATE TABLE auth_tokens (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose      text NOT NULL
        CHECK (purpose IN ('password_reset', 'email_verification')),
    token_sha256 bytea NOT NULL,
    expires_at   timestamptz NOT NULL,
    consumed_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- The lookup on redeem: a raw token hashes to exactly one row.
CREATE UNIQUE INDEX auth_tokens_sha_idx ON auth_tokens (token_sha256);

-- Invalidating a user's outstanding tokens of a purpose when a new one is
-- issued, and the "was one just sent?" cooldown check.
CREATE INDEX auth_tokens_user_purpose_idx
    ON auth_tokens (user_id, purpose, created_at DESC)
    WHERE consumed_at IS NULL;
