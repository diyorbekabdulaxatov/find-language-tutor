package availability

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// repositoryPostgres implements Repository over the sqlc-generated queries. It
// holds the pool directly (not just sqlc.DBTX) because ReplaceSlots runs inside
// a transaction.
type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
}

func (r *repositoryPostgres) TeacherContext(ctx context.Context, slug string) (TeacherRef, error) {
	row, err := r.q.GetTeacherAvailabilityContext(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TeacherRef{}, ErrTeacherNotFound
		}
		return TeacherRef{}, fmt.Errorf("get teacher context: %w", err)
	}
	return TeacherRef{ID: row.ID, Slug: row.Slug, Timezone: row.Timezone}, nil
}

func (r *repositoryPostgres) ListSlots(ctx context.Context, teacherID uuid.UUID) ([]Slot, error) {
	rows, err := r.q.ListAvailabilitySlots(ctx, teacherID)
	if err != nil {
		return nil, fmt.Errorf("list slots: %w", err)
	}
	slots := make([]Slot, len(rows))
	for i, row := range rows {
		slots[i] = Slot{
			Weekday:     Weekday(row.Weekday),
			StartMinute: int(row.StartMinute),
			EndMinute:   int(row.EndMinute),
		}
	}
	return slots, nil
}

func (r *repositoryPostgres) ReplaceSlots(ctx context.Context, teacherID uuid.UUID, slots []Slot) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteAvailabilitySlots(ctx, teacherID); err != nil {
		return fmt.Errorf("clear slots: %w", err)
	}
	for _, s := range slots {
		if err := qtx.AddAvailabilitySlot(ctx, sqlc.AddAvailabilitySlotParams{
			TeacherID:   teacherID,
			Weekday:     int16(s.Weekday),
			StartMinute: int32(s.StartMinute),
			EndMinute:   int32(s.EndMinute),
		}); err != nil {
			return fmt.Errorf("insert slot: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
