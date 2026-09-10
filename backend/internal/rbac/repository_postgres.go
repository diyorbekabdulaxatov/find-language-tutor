package rbac

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

const pgUniqueViolation = "23505"

type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
}

func (r *repositoryPostgres) PermissionKeysForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	keys, err := r.q.PermissionsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("permissions for user: %w", err)
	}
	if keys == nil {
		keys = []string{}
	}
	return keys, nil
}

func (r *repositoryPostgres) RoleRefsForUser(ctx context.Context, userID uuid.UUID) ([]RoleRef, error) {
	rows, err := r.q.RolesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("roles for user: %w", err)
	}
	out := make([]RoleRef, len(rows))
	for i, row := range rows {
		out[i] = RoleRef{ID: row.ID, Name: row.Name}
	}
	return out, nil
}

func (r *repositoryPostgres) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.q.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	roles := make([]Role, len(rows))
	ids := make([]uuid.UUID, len(rows))
	byID := make(map[uuid.UUID]*Role, len(rows))
	for i, row := range rows {
		roles[i] = Role{
			ID: row.ID, Name: row.Name, Description: row.Description,
			IsSystem: row.IsSystem, CreatedAt: row.CreatedAt.Time.UTC(),
			UserCount: row.UserCount, Permissions: []Permission{},
		}
		ids[i] = row.ID
		byID[row.ID] = &roles[i]
	}
	if err := r.attachPermissions(ctx, ids, byID); err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *repositoryPostgres) GetRole(ctx context.Context, id uuid.UUID) (Role, error) {
	row, err := r.q.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Role{}, ErrRoleNotFound
		}
		return Role{}, fmt.Errorf("get role: %w", err)
	}
	count, err := r.q.CountUsersWithRole(ctx, id)
	if err != nil {
		return Role{}, fmt.Errorf("count users with role: %w", err)
	}
	role := Role{
		ID: row.ID, Name: row.Name, Description: row.Description,
		IsSystem: row.IsSystem, CreatedAt: row.CreatedAt.Time.UTC(),
		UserCount: count, Permissions: []Permission{},
	}
	byID := map[uuid.UUID]*Role{id: &role}
	if err := r.attachPermissions(ctx, []uuid.UUID{id}, byID); err != nil {
		return Role{}, err
	}
	return role, nil
}

func (r *repositoryPostgres) attachPermissions(ctx context.Context, ids []uuid.UUID, byID map[uuid.UUID]*Role) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.q.ListPermissionsForRoles(ctx, ids)
	if err != nil {
		return fmt.Errorf("list permissions for roles: %w", err)
	}
	for _, row := range rows {
		if role := byID[row.RoleID]; role != nil {
			role.Permissions = append(role.Permissions, Permission(row.Permission))
		}
	}
	return nil
}

func (r *repositoryPostgres) CreateRole(ctx context.Context, name, description string, isSystem bool, permissions []string) (Role, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Role{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := r.q.WithTx(tx)
	row, err := qtx.CreateRole(ctx, sqlc.CreateRoleParams{Name: name, Description: description, IsSystem: isSystem})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return Role{}, ErrRoleExists
		}
		return Role{}, fmt.Errorf("create role: %w", err)
	}
	for _, p := range permissions {
		if err := qtx.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: row.ID, Permission: p}); err != nil {
			return Role{}, fmt.Errorf("add role permission: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Role{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetRole(ctx, row.ID)
}

func (r *repositoryPostgres) UpdateRole(ctx context.Context, id uuid.UUID, description string, permissions *[]string) (Role, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Role{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := r.q.WithTx(tx)
	if err := qtx.UpdateRoleDescription(ctx, sqlc.UpdateRoleDescriptionParams{ID: id, Description: description}); err != nil {
		return Role{}, fmt.Errorf("update role description: %w", err)
	}
	if permissions != nil {
		if err := qtx.DeleteRolePermissions(ctx, id); err != nil {
			return Role{}, fmt.Errorf("clear role permissions: %w", err)
		}
		for _, p := range *permissions {
			if err := qtx.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: id, Permission: p}); err != nil {
				return Role{}, fmt.Errorf("add role permission: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Role{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetRole(ctx, id)
}

func (r *repositoryPostgres) DeleteRole(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteRole(ctx, id); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	ok, err := r.q.UserExists(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("user exists: %w", err)
	}
	return ok, nil
}

func (r *repositoryPostgres) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	if err := r.q.AssignRoleToUser(ctx, sqlc.AssignRoleToUserParams{UserID: userID, RoleID: roleID}); err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) UnassignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	if err := r.q.UnassignRoleFromUser(ctx, sqlc.UnassignRoleFromUserParams{UserID: userID, RoleID: roleID}); err != nil {
		return fmt.Errorf("unassign role: %w", err)
	}
	return nil
}
