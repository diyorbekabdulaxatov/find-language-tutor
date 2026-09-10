-- File assets (phase A1): the app's handle to a blob in the store.

-- name: CreateFileAsset :one
INSERT INTO file_assets (owner_id, provider, object_key, filename, content_type, bytes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, owner_id, provider, object_key, filename, content_type, bytes, created_at;

-- name: GetFileAsset :one
SELECT id, owner_id, provider, object_key, filename, content_type, bytes, created_at
FROM file_assets
WHERE id = $1;

-- name: DeleteAllFileAssets :exec
-- Seed-only. file_assets.owner_id -> users cascades, but resources may still
-- reference an asset id in JSONB (no FK), so the seed clears this before users.
DELETE FROM file_assets;
