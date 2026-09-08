-- RBAC (phase A): roles, the permissions each role grants, and the roles each
-- user holds. Authorization is resolved per request from these tables — never
-- from the JWT — so revoking a role takes effect immediately rather than at the
-- next access-token refresh.
--
--   roles              — a named bundle of permissions. is_system roles
--                        (e.g. 'superadmin') cannot be renamed, re-permissioned,
--                        or deleted through the API.
--   role_permissions   — the permission strings a role grants. Permission keys
--                        are validated against internal/rbac's catalog on write.
--   user_roles         — which roles a user holds. A user's effective
--                        permission set is the union across their roles.

CREATE TABLE roles (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL UNIQUE,
    description text NOT NULL DEFAULT '',
    is_system   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission text NOT NULL,
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE user_roles (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

-- Hot path: resolve a caller's permissions on every /v1/admin request.
CREATE INDEX user_roles_user_id_idx ON user_roles (user_id);
