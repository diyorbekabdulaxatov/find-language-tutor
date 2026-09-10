package payouts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func ctx() context.Context { return context.Background() }

var (
	nodira = TeacherRef{Slug: "nodira-karimova", DisplayName: "Nodira Karimova"}
	elena  = TeacherRef{Slug: "elena-kim", DisplayName: "Elena Kim"}
)

// seedLedger builds the standard fixture: two teachers with cleared money, one
// lesson still inside its clearing window, and one reversed lesson.
func seedLedger() (*fakeRepo, uuid.UUID, uuid.UUID) {
	repo := newFakeRepo()
	nodiraID, elenaID := uuid.New(), uuid.New()
	repo.addLedgerRow(nodira, nodiraID, 9_000_000, "held", 48*time.Hour)
	repo.addLedgerRow(nodira, nodiraID, 4_500_000, "held", 24*time.Hour)
	repo.addLedgerRow(elena, elenaID, 6_000_000, "held", 12*time.Hour)
	repo.addLedgerRow(elena, elenaID, 7_000_000, "held", -72*time.Hour) // still held
	repo.addLedgerRow(elena, elenaID, 3_000_000, "reversed", 24*time.Hour)
	return repo, nodiraID, elenaID
}

func newTestService(repo *fakeRepo) *Service { return NewService(repo, discardLogger()) }

func TestService_Dashboard_OwedAndTotals(t *testing.T) {
	repo, _, _ := seedLedger()
	s := newTestService(repo)

	d, err := s.Dashboard(ctx(), 0, 0)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}

	if len(d.Owed) != 2 {
		t.Fatalf("owed rows = %d, want 2 (one per teacher with cleared money)", len(d.Owed))
	}
	// Biggest first: Nodira is owed 13.5M, Elena 6M.
	if d.Owed[0].Teacher.Slug != nodira.Slug || d.Owed[0].AvailableMinor != 13_500_000 {
		t.Errorf("owed[0] = %+v, want nodira / 13_500_000", d.Owed[0])
	}
	if want := fixedNow.Add(-48 * time.Hour); !d.Owed[0].OldestAvailableAt.Equal(want) {
		t.Errorf("oldest_available_at = %s, want %s", d.Owed[0].OldestAvailableAt, want)
	}
	if d.Owed[1].Teacher.Slug != elena.Slug || d.Owed[1].AvailableMinor != 6_000_000 {
		t.Errorf("owed[1] = %+v, want elena / 6_000_000", d.Owed[1])
	}

	if d.Totals.AvailableTotalMinor != 19_500_000 {
		t.Errorf("available total = %d, want 19_500_000", d.Totals.AvailableTotalMinor)
	}
	if d.Totals.HeldTotalMinor != 7_000_000 {
		t.Errorf("held total = %d, want 7_000_000 (the lesson still in its window)", d.Totals.HeldTotalMinor)
	}
	if d.Totals.PaidTotalMinor != 0 {
		t.Errorf("paid total = %d, want 0", d.Totals.PaidTotalMinor)
	}
	if d.Totals.Currency != "UZS" {
		t.Errorf("currency = %q", d.Totals.Currency)
	}
	if len(d.Batches) != 0 || d.BatchesTotal != 0 {
		t.Errorf("batches = %d / %d, want none yet", len(d.Batches), d.BatchesTotal)
	}
}

// TestService_Dashboard_EmptyLedger: the JSON must carry empty arrays, not
// nulls, and fall back to the platform currency.
func TestService_Dashboard_EmptyLedger(t *testing.T) {
	s := newTestService(newFakeRepo())

	d, err := s.Dashboard(ctx(), 0, 0)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if d.Owed == nil || len(d.Owed) != 0 || d.Batches == nil || len(d.Batches) != 0 {
		t.Errorf("owed=%v batches=%v, want empty non-nil slices", d.Owed, d.Batches)
	}
	if d.Totals.Currency != defaultCurrency {
		t.Errorf("currency = %q, want %q", d.Totals.Currency, defaultCurrency)
	}
}

func TestService_Run_SettlesEverythingCleared(t *testing.T) {
	repo, _, _ := seedLedger()
	s := newTestService(repo)
	admin := uuid.New()

	d, err := s.Run(ctx(), admin)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if d.Batch.TotalMinor != 19_500_000 {
		t.Errorf("total = %d, want 19_500_000", d.Batch.TotalMinor)
	}
	if d.Batch.TeacherCount != 2 || d.Batch.LineCount != 3 {
		t.Errorf("teacher_count=%d line_count=%d, want 2 / 3", d.Batch.TeacherCount, d.Batch.LineCount)
	}
	if d.Batch.Status != StatusCompleted || d.Batch.CompletedAt == nil {
		t.Errorf("status=%q completed_at=%v, want completed + a timestamp", d.Batch.Status, d.Batch.CompletedAt)
	}
	if d.Batch.CreatedBy.ID != admin {
		t.Errorf("created_by = %s, want the running operator", d.Batch.CreatedBy.ID)
	}
	if len(d.Lines) != 2 || d.Lines[0].Teacher.Slug != nodira.Slug || d.Lines[0].LessonCount != 2 {
		t.Errorf("lines = %+v, want nodira (2 lessons) first", d.Lines)
	}

	// The lesson still inside its clearing window was NOT paid.
	after, err := s.Dashboard(ctx(), 0, 0)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if len(after.Owed) != 0 {
		t.Errorf("still owed after the run: %+v", after.Owed)
	}
	if after.Totals.PaidTotalMinor != 19_500_000 || after.Totals.HeldTotalMinor != 7_000_000 {
		t.Errorf("paid=%d held=%d after the run", after.Totals.PaidTotalMinor, after.Totals.HeldTotalMinor)
	}
	if after.BatchesTotal != 1 {
		t.Errorf("batches total = %d, want 1", after.BatchesTotal)
	}
}

// TestService_Run_SecondRunPaysNothingTwice is the double-run guard seen from
// above the repository: the rows the first run settled are `paid`, so the second
// run finds nothing (in Postgres the first run's FOR UPDATE SKIP LOCKED makes
// the same thing true for two *concurrent* runs).
func TestService_Run_SecondRunPaysNothingTwice(t *testing.T) {
	repo, _, _ := seedLedger()
	s := newTestService(repo)

	first, err := s.Run(ctx(), uuid.New())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := s.Run(ctx(), uuid.New()); !errors.Is(err, ErrNothingToPay) {
		t.Fatalf("second run err = %v, want ErrNothingToPay", err)
	}

	totals, _ := repo.Totals(ctx())
	if totals.PaidTotalMinor != first.Batch.TotalMinor {
		t.Errorf("paid total = %d, want %d — the second run must not re-pay",
			totals.PaidTotalMinor, first.Batch.TotalMinor)
	}
	if len(repo.batches) != 1 {
		t.Errorf("batches = %d, want 1 — an empty run writes no batch row", len(repo.batches))
	}
}

func TestService_Run_NothingCleared(t *testing.T) {
	repo := newFakeRepo()
	repo.addLedgerRow(nodira, uuid.New(), 9_000_000, "held", -48*time.Hour) // window still open
	s := newTestService(repo)

	if _, err := s.Run(ctx(), uuid.New()); !errors.Is(err, ErrNothingToPay) {
		t.Fatalf("err = %v, want ErrNothingToPay", err)
	}
}

func TestService_Batch_NotFound(t *testing.T) {
	s := newTestService(newFakeRepo())
	if _, err := s.Batch(ctx(), uuid.New()); !errors.Is(err, ErrBatchNotFound) {
		t.Fatalf("err = %v, want ErrBatchNotFound", err)
	}
}

func TestService_Batch_Lines(t *testing.T) {
	repo, _, _ := seedLedger()
	s := newTestService(repo)
	created, err := s.Run(ctx(), uuid.New())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	got, err := s.Batch(ctx(), created.Batch.ID)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if got.Batch.ID != created.Batch.ID || len(got.Lines) != 2 {
		t.Errorf("got %+v", got)
	}
	if got.Lines[0].AmountMinor != 13_500_000 || got.Lines[1].AmountMinor != 6_000_000 {
		t.Errorf("lines = %+v", got.Lines)
	}
}

// TestService_Dashboard_Pagination checks the shared page defaults and the
// page-size cap the other admin lists use.
func TestService_Dashboard_Pagination(t *testing.T) {
	repo := newFakeRepo()
	teacherID := uuid.New()
	for i := 0; i < 3; i++ {
		repo.addLedgerRow(nodira, teacherID, 1_000_000, "held", 24*time.Hour)
		if _, err := repo.Run(ctx(), uuid.New()); err != nil {
			t.Fatalf("seed run %d: %v", i, err)
		}
	}
	s := newTestService(repo)

	d, err := s.Dashboard(ctx(), 2, 2)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if d.BatchesTotal != 3 || len(d.Batches) != 1 {
		t.Errorf("page 2 of size 2: total=%d len=%d, want 3 / 1", d.BatchesTotal, len(d.Batches))
	}

	if limit, offset := normalizePage(0, 0); limit != defaultPageSize || offset != 0 {
		t.Errorf("defaults = %d/%d, want %d/0", limit, offset, defaultPageSize)
	}
	if limit, _ := normalizePage(1, 5000); limit != maxPageSize {
		t.Errorf("page size cap = %d, want %d", limit, maxPageSize)
	}
}
