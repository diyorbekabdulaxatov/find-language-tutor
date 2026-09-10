-- Auth module: user accounts and refresh-token sessions.

-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name)
VALUES ($1, $2, $3)
RETURNING id, email, password_hash, display_name, email_verified_at, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, display_name, email_verified_at, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, display_name, email_verified_at, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpdateUser :one
-- Edit the caller's own account. Email is immutable here (changing it needs a
-- verification flow that does not exist yet).
UPDATE users
SET display_name = $2, updated_at = now()
WHERE id = $1
RETURNING id, email, password_hash, display_name, email_verified_at, created_at, updated_at;

-- name: SetUserPassword :exec
-- Password reset: replace the hash and bump updated_at. The caller also revokes
-- every session in the same transaction.
UPDATE users
SET password_hash = $2, updated_at = now()
WHERE id = $1;

-- name: MarkUserEmailVerified :exec
-- Idempotent: keeps the first verification time if the row is already verified.
UPDATE users
SET email_verified_at = coalesce(email_verified_at, now()), updated_at = now()
WHERE id = $1;

-- name: SeedMarkEmailVerified :exec
-- Seed-only: stamp every demo account verified so the nudge banner is quiet.
UPDATE users SET email_verified_at = now() WHERE email_verified_at IS NULL;

-- --- account-recovery link tokens (migration 000013) ---

-- name: CreateAuthToken :one
INSERT INTO auth_tokens (user_id, purpose, token_sha256, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetLiveAuthToken :one
-- A token that can still be redeemed: matches the hash + purpose, not consumed,
-- not expired.
SELECT id, user_id
FROM auth_tokens
WHERE token_sha256 = $1 AND purpose = $2
  AND consumed_at IS NULL AND expires_at > now();

-- name: ConsumeAuthToken :exec
-- Spend a token by its hash (the redeem flows already hold the hash, not the id).
UPDATE auth_tokens SET consumed_at = now()
WHERE token_sha256 = $1 AND consumed_at IS NULL;

-- name: ConsumeUserAuthTokens :exec
-- Invalidate a user's outstanding tokens of a purpose before issuing a new one,
-- so only the newest link works.
UPDATE auth_tokens SET consumed_at = now()
WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL;

-- name: LatestAuthTokenAt :one
-- The most recent still-live token of a purpose for a user — drives the "one was
-- just sent, don't send another" cooldown. No row -> zero time.
SELECT coalesce(max(created_at), 'epoch'::timestamptz)::timestamptz AS latest
FROM auth_tokens
WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL;

-- name: DeleteAllUsers :exec
-- Seed-only. teachers.user_id references users, so callers must clear teachers
-- first.
DELETE FROM users;

-- name: CreateSession :one
INSERT INTO sessions (user_id, refresh_token_hash, expires_at, user_agent)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, refresh_token_hash, expires_at, revoked_at, replaced_by, user_agent, created_at;

-- name: GetSessionByRefreshHash :one
SELECT id, user_id, refresh_token_hash, expires_at, revoked_at, replaced_by, user_agent, created_at
FROM sessions
WHERE refresh_token_hash = $1;

-- name: RevokeSession :exec
-- Marks a session revoked and records the session that replaced it (rotation).
-- No-op if it was already revoked.
UPDATE sessions
SET revoked_at = now(), replaced_by = sqlc.narg('replaced_by')
WHERE id = sqlc.arg('id') AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :exec
-- Reuse-detection hammer: kills every still-active session for a user.
UPDATE sessions
SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;
