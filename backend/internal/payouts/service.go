package payouts

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// Repository is the persistence port. The concrete implementation
// (repositoryPostgres) lives alongside; tests use a fake.
type Repository interface {
	// Owed returns what the platform currently owes, one row per teacher:
	// ledger rows past their clearing deadline that no batch has settled.
	Owed(ctx context.Context) ([]OwedRow, error)

	// Totals returns the platform-wide available / held / paid ledger sums.
	Totals(ctx context.Context) (Totals, error)

	// ListBatches returns a page of past runs, newest first, and the total
	// count.
	ListBatches(ctx context.Context, limit, offset int) (batches []Batch, total int, err error)

	// Batch returns one batch with its per-teacher lines, or ErrBatchNotFound.
	Batch(ctx context.Context, id uuid.UUID) (BatchDetail, error)

	// Run settles every cleared, unpaid ledger row in ONE transaction: it locks
	// them FOR UPDATE SKIP LOCKED, inserts the batch, and marks the rows paid
	// with the batch id. Two concurrent runs therefore work on disjoint sets
	// instead of paying a lesson twice. ErrNothingToPay when there is nothing
	// cleared to settle (no batch row is written).
	Run(ctx context.Context, adminID uuid.UUID) (BatchDetail, error)
}

// Service holds the payout business rules. Handlers call it; it never sees a
// *gin.Context.
type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, logger: logger}
}

// Dashboard returns the operator payout dashboard: what is owed per teacher, the
// platform totals, and one page of past runs (newest first). page defaults to 1,
// pageSize to 20 (capped at 100).
func (s *Service) Dashboard(ctx context.Context, page, pageSize int) (Dashboard, error) {
	owed, err := s.repo.Owed(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	totals, err := s.repo.Totals(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	limit, offset := normalizePage(page, pageSize)
	batches, total, err := s.repo.ListBatches(ctx, limit, offset)
	if err != nil {
		return Dashboard{}, err
	}

	if owed == nil {
		owed = []OwedRow{}
	}
	if batches == nil {
		batches = []Batch{}
	}
	if totals.Currency == "" {
		totals.Currency = defaultCurrency
	}
	return Dashboard{Owed: owed, Totals: totals, Batches: batches, BatchesTotal: total}, nil
}

// Batch returns one past run with its per-teacher lines.
func (s *Service) Batch(ctx context.Context, id uuid.UUID) (BatchDetail, error) {
	d, err := s.repo.Batch(ctx, id)
	if err != nil {
		return BatchDetail{}, err
	}
	if d.Lines == nil {
		d.Lines = []BatchLine{}
	}
	return d, nil
}

// Run settles everything currently payable and returns the created batch.
// ErrNothingToPay (409 nothing_to_pay) when no earning has cleared its window:
// an empty batch would only add noise to the run history.
//
// The repository does the whole thing in one transaction, so the batch and the
// `paid` ledger rows commit together and a second operator running at the same
// moment cannot settle the same lesson (FOR UPDATE SKIP LOCKED).
func (s *Service) Run(ctx context.Context, adminID uuid.UUID) (BatchDetail, error) {
	d, err := s.repo.Run(ctx, adminID)
	if err != nil {
		return BatchDetail{}, err
	}
	if d.Lines == nil {
		d.Lines = []BatchLine{}
	}

	// The money-leaves step. The MVP's provider is the deterministic in-process
	// fake, which has no disbursement API, so settling is the ledger write above
	// plus this record of it.
	// TODO(payouts): real disbursement via the provider — hand each line to the
	// Payme / Click payout API here, insert the batch as `processing`, and flip
	// it to `completed` / `failed` on the provider's answer.
	s.logger.Info("payout batch settled",
		slog.String("batch_id", d.Batch.ID.String()),
		slog.String("created_by", adminID.String()),
		slog.Int64("total_minor", d.Batch.TotalMinor),
		slog.String("currency", d.Batch.Currency),
		slog.Int("teacher_count", d.Batch.TeacherCount),
		slog.Int("line_count", d.Batch.LineCount))

	return d, nil
}
