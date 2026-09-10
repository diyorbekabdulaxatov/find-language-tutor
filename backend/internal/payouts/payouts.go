// Package payouts is the phase-E operator payout surface: what the platform
// owes its teachers right now, the history of past payout runs, and the run
// itself, which settles every cleared earning in one transaction.
//
// Layout mirrors the other modules: domain types + errors here, a Service with
// the rules, a Repository port (Postgres impl alongside, fake in tests), gin
// handlers, and a RegisterAdminRoutes func. The Service never sees a
// *gin.Context.
//
// Boundaries:
//
//   - This module owns the payout_batches table and every WRITE that settles a
//     payout_ledger row. internal/payments keeps owning the ledger writes on the
//     money path (a row on capture, a reversal on refund) and the teacher-facing
//     GET /v1/payments/me read. The two never import each other: they share a
//     table, not Go types, and each reads it with its own SQL — the same call
//     internal/admin and internal/disputes make. Nothing here needs a port.
//   - The admin routes (/v1/admin/payouts...) mount on the admin group behind
//     the RBAC guard, like rbac.RegisterAdminRoutes and
//     disputes.RegisterAdminRoutes. internal/admin stays out of it: payouts is
//     its own module that happens to be mounted under /v1/admin.
//
// The clearing window (migration 000011). A captured lesson lands in
// payout_ledger as `held` with an `available_at` deadline — capture time plus
// PAYOUTS_CLEARING_DAYS (default 7). Nothing flips the row to `available`: that
// state is DERIVED by comparing available_at against now() in the queries that
// need it, so there is no background job and no window in which the ledger
// disagrees with the clock. The payout run reads straight past it, taking every
// row whose deadline has passed and marking it `paid`.
//
// Money is integer minor units everywhere. Never float.
package payouts

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// BatchStatus is the payout_batches lifecycle state.
//
// The MVP's provider is the in-process fake, which settles synchronously, so a
// run writes StatusCompleted directly. StatusProcessing / StatusFailed exist for
// the real Payme / Click disbursement call that replaces it, which cannot settle
// inside the transaction.
type BatchStatus string

const (
	StatusProcessing BatchStatus = "processing"
	StatusCompleted  BatchStatus = "completed"
	StatusFailed     BatchStatus = "failed"
)

// defaultCurrency is used when the ledger is empty and has no currency to
// report. The platform is UZS-only for the MVP.
const defaultCurrency = "UZS"

// TeacherRef is the light teacher summary shown on a payout row.
type TeacherRef struct {
	Slug        string
	DisplayName string
}

// UserRef is the light account summary of the operator who ran a batch.
type UserRef struct {
	ID          uuid.UUID
	DisplayName string
}

// OwedRow is one teacher's currently payable balance: ledger rows past their
// clearing deadline that no batch has settled yet.
type OwedRow struct {
	Teacher           TeacherRef
	AvailableMinor    int64
	Currency          string
	OldestAvailableAt time.Time
}

// Totals are the platform-wide ledger sums behind the dashboard.
type Totals struct {
	AvailableTotalMinor int64 // payable now
	HeldTotalMinor      int64 // still inside the clearing window
	PaidTotalMinor      int64 // already disbursed by a batch
	Currency            string
}

// Batch is one payout run.
type Batch struct {
	ID           uuid.UUID
	CreatedBy    UserRef
	Status       BatchStatus
	TotalMinor   int64
	Currency     string
	TeacherCount int
	LineCount    int
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

// BatchLine is one teacher's share of a batch.
type BatchLine struct {
	Teacher     TeacherRef
	AmountMinor int64
	Currency    string
	LessonCount int
}

// BatchDetail is a batch plus its per-teacher line items.
type BatchDetail struct {
	Batch Batch
	Lines []BatchLine
}

// Dashboard is the answer to GET /v1/admin/payouts: what is owed, the platform
// totals, and a page of past runs.
type Dashboard struct {
	Owed         []OwedRow
	Totals       Totals
	Batches      []Batch
	BatchesTotal int
}

// Domain errors. The handler maps each to an HTTP status; anything else is 500.
var (
	// ErrBatchNotFound — no payout batch has the requested id. Rendered 404
	// batch_not_found.
	ErrBatchNotFound = errors.New("payouts: payout batch not found")

	// ErrNothingToPay — a run found no cleared, unpaid ledger row. Rendered 409
	// nothing_to_pay; no batch row is created, so the history stays a log of
	// runs that actually moved money.
	ErrNothingToPay = errors.New("payouts: nothing is available to pay out")
)

// Pagination defaults, matching the other admin list endpoints.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func normalizePage(page, pageSize int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return pageSize, (page - 1) * pageSize
}
