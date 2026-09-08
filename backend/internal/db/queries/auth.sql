-- Auth module: user accounts and refresh-token sessions.

-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name)
VALUES ($1, $2, $3)
RETURNING id, email, password_hash, display_name, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, display_name, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, display_name, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpdateUser :one
-- Edit the caller's own account. Email is immutable here (changing it needs a
-- verification flow that does not exist yet).
UPDATE users
SET display_name = $2, updated_at = now()
WHERE id = $1
RETURNING id, email, password_hash, display_name, created_at, updated_at;

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
