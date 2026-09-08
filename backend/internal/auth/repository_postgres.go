package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// pgUniqueViolation is SQLSTATE 23505.
const pgUniqueViolation = "23505"

// repositoryPostgres implements Repository over the sqlc-generated queries.
type repositoryPostgres struct {
	q *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(db sqlc.DBTX) Repository {
	return &repositoryPostgres{q: sqlc.New(db)}
}

func (r *repositoryPostgres) CreateUser(ctx context.Context, in NewUser) (User, error) {
	row, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		Email:        in.Email,
		PasswordHash: in.PasswordHash,
		DisplayName:  in.DisplayName,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.CreatedAt, row.UpdatedAt), nil
}

func (r *repositoryPostgres) UserWithHashByEmail(ctx context.Context, email string) (User, string, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "", ErrUserNotFound
		}
		return User{}, "", fmt.Errorf("get user by email: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.CreatedAt, row.UpdatedAt), row.PasswordHash, nil
}

func (r *repositoryPostgres) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.CreatedAt, row.UpdatedAt), nil
}

func (r *repositoryPostgres) UpdateUser(ctx context.Context, id uuid.UUID, displayName string) (User, error) {
	row, err := r.q.UpdateUser(ctx, sqlc.UpdateUserParams{ID: id, DisplayName: displayName})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("update user: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.CreatedAt, row.UpdatedAt), nil
}

func (r *repositoryPostgres) CreateSession(ctx context.Context, in NewSession) (Session, error) {
	row, err := r.q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:           in.UserID,
		RefreshTokenHash: in.RefreshTokenHash,
		ExpiresAt:        pgtype.Timestamptz{Time: in.ExpiresAt, Valid: true},
		UserAgent:        in.UserAgent,
	})
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return sessionFromRow(row), nil
}

func (r *repositoryPostgres) SessionByRefreshHash(ctx context.Context, hash []byte) (Session, error) {
	row, err := r.q.GetSessionByRefreshHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrInvalidRefreshToken
		}
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	return sessionFromRow(row), nil
}

func (r *repositoryPostgres) RevokeSession(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error {
	arg := sqlc.RevokeSessionParams{ID: id}
	if replacedBy != nil {
		arg.ReplacedBy = uuid.NullUUID{UUID: *replacedBy, Valid: true}
	}
	if err := r.q.RevokeSession(ctx, arg); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	if err := r.q.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

func userFromRow(id uuid.UUID, email, displayName string, createdAt, updatedAt pgtype.Timestamptz) User {
	return User{
		ID:          id,
		Email:       email,
		DisplayName: displayName,
		CreatedAt:   createdAt.Time,
		UpdatedAt:   updatedAt.Time,
	}
}

func sessionFromRow(row sqlc.Session) Session {
	s := Session{
		ID:        row.ID,
		UserID:    row.UserID,
		ExpiresAt: row.ExpiresAt.Time,
		UserAgent: row.UserAgent,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		s.RevokedAt = &t
	}
	if row.ReplacedBy.Valid {
		id := row.ReplacedBy.UUID
		s.ReplacedBy = &id
	}
	return s
}
