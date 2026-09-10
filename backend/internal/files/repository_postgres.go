package files

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

type repositoryPostgres struct{ q *sqlc.Queries }

// NewPostgresRepository builds a Repository over the given pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{q: sqlc.New(pool)}
}

func (r *repositoryPostgres) Create(ctx context.Context, a Asset) (Asset, error) {
	row, err := r.q.CreateFileAsset(ctx, sqlc.CreateFileAssetParams{
		OwnerID:     a.OwnerID,
		Provider:    a.Provider,
		ObjectKey:   a.ObjectKey,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		Bytes:       a.Bytes,
	})
	if err != nil {
		return Asset{}, fmt.Errorf("create file asset: %w", err)
	}
	return toAsset(row), nil
}

func (r *repositoryPostgres) ByID(ctx context.Context, id uuid.UUID) (Asset, error) {
	row, err := r.q.GetFileAsset(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Asset{}, ErrAssetNotFound
		}
		return Asset{}, fmt.Errorf("get file asset: %w", err)
	}
	return toAsset(row), nil
}

func toAsset(row sqlc.FileAsset) Asset {
	return Asset{
		ID:          row.ID,
		OwnerID:     row.OwnerID,
		Provider:    row.Provider,
		ObjectKey:   row.ObjectKey,
		Filename:    row.Filename,
		ContentType: row.ContentType,
		Bytes:       row.Bytes,
		CreatedAt:   row.CreatedAt.Time.UTC(),
	}
}
