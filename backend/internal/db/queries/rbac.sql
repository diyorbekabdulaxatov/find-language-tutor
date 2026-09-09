-- RBAC module: roles, the permissions they grant, and role assignments. A
-- user's effective permissions are the union across their roles, resolved per
-- request (never from the JWT).

-- name: PermissionsForUser :many
-- The caller's effective permission keys, deduped and sorted. One indexed join.
SELECT DISTINCT rp.permission
FROM user_roles ur
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE ur.user_id = $1
ORDER BY rp.permission;

-- name: RolesForUser :many
SELECT r.id, r.name
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.name;

-- name: ListRoles :many
SELECT
    r.id, r.name, r.description, r.is_system, r.created_at,
    (SELECT count(*) FROM user_roles ur WHERE ur.role_id = r.id)::bigint AS user_count
FROM roles r
ORDER BY r.is_system DESC, r.name;

-- name: ListPermissionsForRoles :many
SELECT role_id, permission
FROM role_permissions
WHERE role_id = ANY(sqlc.arg('role_ids')::uuid[])
ORDER BY role_id, permission;

-- name: GetRole :one
SELECT id, name, description, is_system, created_at FROM roles WHERE id = $1;

-- name: GetRoleByName :one
SELECT id, name, description, is_system, created_at FROM roles WHERE name = $1;

-- name: CreateRole :one
INSERT INTO roles (name, description, is_system)
VALUES ($1, $2, $3)
RETURNING id, name, description, is_system, created_at;

-- name: UpdateRoleDescription :exec
UPDATE roles SET description = $2 WHERE id = $1;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission)
VALUES ($1, $2)
ON CONFLICT (role_id, permission) DO NOTHING;

-- name: CountUsersWithRole :one
SELECT count(*) FROM user_roles WHERE role_id = $1;

-- name: AssignRoleToUser :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: UnassignRoleFromUser :exec
DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2;

-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE id = $1);

-- name: DeleteAllUserRoles :exec
-- Seed-only. Clear assignments before roles / users.
DELETE FROM user_roles;

-- name: DeleteAllRolePermissions :exec
DELETE FROM role_permissions;

-- name: DeleteAllRoles :exec
DELETE FROM roles;
