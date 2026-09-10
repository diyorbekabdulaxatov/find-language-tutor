package payouts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
}

func (r *repositoryPostgres) Owed(ctx context.Context) ([]OwedRow, error) {
	rows, err := r.q.AdminListOwedPayouts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list owed payouts: %w", err)
	}
	out := make([]OwedRow, len(rows))
	for i, row := range rows {
		out[i] = OwedRow{
			Teacher:           TeacherRef{Slug: row.Slug, DisplayName: row.DisplayName},
			AvailableMinor:    row.AvailableMinor,
			Currency:          row.Currency,
			OldestAvailableAt: row.OldestAvailableAt.Time.UTC(),
		}
	}
	return out, nil
}

func (r *repositoryPostgres) Totals(ctx context.Context) (Totals, error) {
	row, err := r.q.AdminPayoutTotals(ctx)
	if err != nil {
		return Totals{}, fmt.Errorf("payout totals: %w", err)
	}
	return Totals{
		AvailableTotalMinor: row.AvailableTotalMinor,
		HeldTotalMinor:      row.HeldTotalMinor,
		PaidTotalMinor:      row.PaidTotalMinor,
		Currency:            row.Currency,
	}, nil
}

func (r *repositoryPostgres) ListBatches(ctx context.Context, limit, offset int) ([]Batch, int, error) {
	rows, err := r.q.AdminListPayoutBatches(ctx, sqlc.AdminListPayoutBatchesParams{
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list payout batches: %w", err)
	}
	total, err := r.q.AdminCountPayoutBatches(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count payout batches: %w", err)
	}

	out := make([]Batch, len(rows))
	for i, row := range rows {
		out[i] = Batch{
			ID:           row.ID,
			CreatedBy:    UserRef{ID: row.CreatedBy, DisplayName: row.CreatedByDisplayName},
			Status:       BatchStatus(row.Status),
			TotalMinor:   row.TotalMinor,
			Currency:     row.Currency,
			TeacherCount: int(row.TeacherCount),
			LineCount:    int(row.LineCount),
			CreatedAt:    row.CreatedAt.Time.UTC(),
			CompletedAt:  utcPtr(row.CompletedAt),
		}
	}
	return out, int(total), nil
}

func (r *repositoryPostgres) Batch(ctx context.Context, id uuid.UUID) (BatchDetail, error) {
	return r.batchDetail(ctx, r.q, id)
}

// batchDetail loads one batch plus its lines through the given query set (the
// pool's, or a transaction's inside Run).
func (r *repositoryPostgres) batchDetail(ctx context.Context, q *sqlc.Queries, id uuid.UUID) (BatchDetail, error) {
	row, err := q.AdminGetPayoutBatch(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BatchDetail{}, ErrBatchNotFound
		}
		return BatchDetail{}, fmt.Errorf("get payout batch: %w", err)
	}
	lines, err := r.linesFor(ctx, q, id)
	if err != nil {
		return BatchDetail{}, err
	}
	return BatchDetail{
		Batch: Batch{
			ID:           row.ID,
			CreatedBy:    UserRef{ID: row.CreatedBy, DisplayName: row.CreatedByDisplayName},
			Status:       BatchStatus(row.Status),
			TotalMinor:   row.TotalMinor,
			Currency:     row.Currency,
			TeacherCount: int(row.TeacherCount),
			LineCount:    int(row.LineCount),
			CreatedAt:    row.CreatedAt.Time.UTC(),
			CompletedAt:  utcPtr(row.CompletedAt),
		},
		Lines: lines,
	}, nil
}

// Run — see the Repository interface doc. Everything below happens in one
// transaction: the payable rows are locked FOR UPDATE SKIP LOCKED (so a
// concurrent run gets a disjoint set rather than double-paying a lesson), the
// batch is inserted with the totals those rows add up to, and the rows are
// flipped to `paid` pointing at it. Nothing payable means no batch row at all.
func (r *repositoryPostgres) Run(ctx context.Context, adminID uuid.UUID) (BatchDetail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BatchDetail{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	qtx := r.q.WithTx(tx)

	payable, err := qtx.LockPayablePayoutLedger(ctx)
	if err != nil {
		return BatchDetail{}, fmt.Errorf("lock payable ledger rows: %w", err)
	}
	if len(payable) == 0 {
		return BatchDetail{}, ErrNothingToPay
	}

	// The platform is single-currency for the MVP; the first row's currency
	// names the batch.
	ids := make([]uuid.UUID, len(payable))
	teachers := make(map[uuid.UUID]struct{}, len(payable))
	var total int64
	currency := payable[0].Currency
	for i, row := range payable {
		ids[i] = row.ID
		teachers[row.TeacherID] = struct{}{}
		total += row.AmountMinor
	}

	batch, err := qtx.InsertPayoutBatch(ctx, sqlc.InsertPayoutBatchParams{
		CreatedBy:    adminID,
		Status:       string(StatusCompleted),
		TotalMinor:   total,
		Currency:     currency,
		TeacherCount: int32(len(teachers)),
		LineCount:    int32(len(ids)),
	})
	if err != nil {
		return BatchDetail{}, fmt.Errorf("insert payout batch: %w", err)
	}

	if err := qtx.MarkLedgerRowsPaid(ctx, sqlc.MarkLedgerRowsPaidParams{
		PayoutBatchID: uuid.NullUUID{UUID: batch.ID, Valid: true},
		Ids:           ids,
	}); err != nil {
		return BatchDetail{}, fmt.Errorf("mark ledger rows paid: %w", err)
	}

	// Read the batch and its per-teacher lines back inside the transaction, so
	// the response describes exactly what was committed (and carries the
	// operator's display name from the join).
	detail, err := r.batchDetail(ctx, qtx, batch.ID)
	if err != nil {
		return BatchDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return BatchDetail{}, fmt.Errorf("commit: %w", err)
	}
	return detail, nil
}

func (r *repositoryPostgres) linesFor(ctx context.Context, q *sqlc.Queries, batchID uuid.UUID) ([]BatchLine, error) {
	rows, err := q.AdminListPayoutBatchLines(ctx, uuid.NullUUID{UUID: batchID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list payout batch lines: %w", err)
	}
	out := make([]BatchLine, len(rows))
	for i, row := range rows {
		out[i] = BatchLine{
			Teacher:     TeacherRef{Slug: row.Slug, DisplayName: row.DisplayName},
			AmountMinor: row.AmountMinor,
			Currency:    row.Currency,
			LessonCount: int(row.LessonCount),
		}
	}
	return out, nil
}

func utcPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}
