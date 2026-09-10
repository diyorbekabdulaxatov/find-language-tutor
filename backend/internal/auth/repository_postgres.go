package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// pgUniqueViolation is SQLSTATE 23505.
const pgUniqueViolation = "23505"

// repositoryPostgres implements Repository over the sqlc-generated queries. It
// holds the pool as well as a Queries so the recovery flows can run their
// multi-statement steps in a transaction.
type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
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
	return userFromRow(row.ID, row.Email, row.DisplayName, row.EmailVerifiedAt, row.CreatedAt, row.UpdatedAt), nil
}

func (r *repositoryPostgres) UserWithHashByEmail(ctx context.Context, email string) (User, string, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "", ErrUserNotFound
		}
		return User{}, "", fmt.Errorf("get user by email: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.EmailVerifiedAt, row.CreatedAt, row.UpdatedAt), row.PasswordHash, nil
}

func (r *repositoryPostgres) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.EmailVerifiedAt, row.CreatedAt, row.UpdatedAt), nil
}

func (r *repositoryPostgres) UpdateUser(ctx context.Context, id uuid.UUID, displayName string) (User, error) {
	row, err := r.q.UpdateUser(ctx, sqlc.UpdateUserParams{ID: id, DisplayName: displayName})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("update user: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.EmailVerifiedAt, row.CreatedAt, row.UpdatedAt), nil
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

// --- account recovery ---

func (r *repositoryPostgres) UserByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return userFromRow(row.ID, row.Email, row.DisplayName, row.EmailVerifiedAt, row.CreatedAt, row.UpdatedAt), nil
}

func (r *repositoryPostgres) SetUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	if err := r.q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: userID, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("set user password: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	if err := r.q.MarkUserEmailVerified(ctx, userID); err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) CreateAuthToken(ctx context.Context, userID uuid.UUID, purpose TokenPurpose, tokenHash []byte, expiresAt time.Time) error {
	_, err := r.q.CreateAuthToken(ctx, sqlc.CreateAuthTokenParams{
		UserID:      userID,
		Purpose:     string(purpose),
		TokenSha256: tokenHash,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create auth token: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) LiveAuthToken(ctx context.Context, tokenHash []byte, purpose TokenPurpose) (uuid.UUID, error) {
	row, err := r.q.GetLiveAuthToken(ctx, sqlc.GetLiveAuthTokenParams{TokenSha256: tokenHash, Purpose: string(purpose)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrInvalidToken
		}
		return uuid.Nil, fmt.Errorf("get live auth token: %w", err)
	}
	return row.UserID, nil
}

func (r *repositoryPostgres) ConsumeUserAuthTokens(ctx context.Context, userID uuid.UUID, purpose TokenPurpose) error {
	if err := r.q.ConsumeUserAuthTokens(ctx, sqlc.ConsumeUserAuthTokensParams{UserID: userID, Purpose: string(purpose)}); err != nil {
		return fmt.Errorf("consume user auth tokens: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) LatestAuthTokenAt(ctx context.Context, userID uuid.UUID, purpose TokenPurpose) (time.Time, error) {
	row, err := r.q.LatestAuthTokenAt(ctx, sqlc.LatestAuthTokenAtParams{UserID: userID, Purpose: string(purpose)})
	if err != nil {
		return time.Time{}, fmt.Errorf("latest auth token at: %w", err)
	}
	if !row.Valid || row.Time.Unix() <= 0 {
		return time.Time{}, nil
	}
	return row.Time, nil
}

// ResetPassword: new hash + consume token + kill sessions, atomically.
func (r *repositoryPostgres) ResetPassword(ctx context.Context, userID uuid.UUID, tokenHash []byte, passwordHash string) error {
	return r.inTx(ctx, func(q *sqlc.Queries) error {
		if err := q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{ID: userID, PasswordHash: passwordHash}); err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if err := q.ConsumeAuthToken(ctx, tokenHash); err != nil {
			return fmt.Errorf("consume token: %w", err)
		}
		if err := q.RevokeAllUserSessions(ctx, userID); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return nil
	})
}

// ConfirmEmail: stamp verified + consume token, atomically.
func (r *repositoryPostgres) ConfirmEmail(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	return r.inTx(ctx, func(q *sqlc.Queries) error {
		if err := q.MarkUserEmailVerified(ctx, userID); err != nil {
			return fmt.Errorf("mark verified: %w", err)
		}
		if err := q.ConsumeAuthToken(ctx, tokenHash); err != nil {
			return fmt.Errorf("consume token: %w", err)
		}
		return nil
	})
}

func (r *repositoryPostgres) inTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	if err := fn(r.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func userFromRow(id uuid.UUID, email, displayName string, emailVerifiedAt, createdAt, updatedAt pgtype.Timestamptz) User {
	return User{
		ID:            id,
		Email:         email,
		DisplayName:   displayName,
		EmailVerified: emailVerifiedAt.Valid,
		CreatedAt:     createdAt.Time,
		UpdatedAt:     updatedAt.Time,
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
