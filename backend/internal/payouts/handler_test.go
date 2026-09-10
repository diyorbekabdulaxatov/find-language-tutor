package payouts

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
)

// errCode pulls the error envelope's code out of a response body.
func errCode(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error envelope: %v (%s)", err, body)
	}
	return env.Error.Code
}

func TestHandler_Dashboard_OK(t *testing.T) {
	repo, _, _ := seedLedger()
	h := newHarness(repo)
	tok := h.bearer(uuid.New(), rbac.PermPayoutsView)

	w := h.do(http.MethodGet, "/v1/admin/payouts", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body dashboardDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Owed) != 2 || body.Owed[0].Teacher.Slug != nodira.Slug {
		t.Fatalf("owed = %+v", body.Owed)
	}
	if body.Owed[0].AvailableMinor != 13_500_000 || body.Owed[0].Currency != "UZS" {
		t.Errorf("owed[0] = %+v", body.Owed[0])
	}
	if body.Totals.AvailableTotalMinor != 19_500_000 || body.Totals.HeldTotalMinor != 7_000_000 {
		t.Errorf("totals = %+v", body.Totals)
	}
	if body.Batches == nil || body.BatchesTotal != 0 {
		t.Errorf("batches = %v / %d, want [] / 0", body.Batches, body.BatchesTotal)
	}
}

func TestHandler_Dashboard_RequiresPermission(t *testing.T) {
	repo, _, _ := seedLedger()
	h := newHarness(repo)

	// Authenticated but without payouts.view.
	w := h.do(http.MethodGet, "/v1/admin/payouts", h.bearer(uuid.New()), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	// Not authenticated at all.
	if w := h.do(http.MethodGet, "/v1/admin/payouts", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandler_Dashboard_BadPage(t *testing.T) {
	h := newHarness(newFakeRepo())
	tok := h.bearer(uuid.New(), rbac.PermPayoutsView)

	if w := h.do(http.MethodGet, "/v1/admin/payouts?page=nope", tok, ""); w.Code != http.StatusBadRequest {
		t.Errorf("page: status = %d, want 400", w.Code)
	}
	if w := h.do(http.MethodGet, "/v1/admin/payouts?page_size=nope", tok, ""); w.Code != http.StatusBadRequest {
		t.Errorf("page_size: status = %d, want 400", w.Code)
	}
}

func TestHandler_Dashboard_RepositoryError(t *testing.T) {
	repo := newFakeRepo()
	repo.owedErr = errors.New("boom")
	h := newHarness(repo)

	w := h.do(http.MethodGet, "/v1/admin/payouts", h.bearer(uuid.New(), rbac.PermPayoutsView), "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestHandler_Run_OK(t *testing.T) {
	repo, _, _ := seedLedger()
	h := newHarness(repo)
	admin := uuid.New()
	tok := h.bearer(admin, rbac.PermPayoutsRun, rbac.PermPayoutsView)

	w := h.do(http.MethodPost, "/v1/admin/payouts/run", tok, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body batchDetailDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalMinor != 19_500_000 || body.Status != string(StatusCompleted) {
		t.Errorf("batch = %+v", body.batchDTO)
	}
	if body.CreatedBy.ID != admin.String() {
		t.Errorf("created_by = %q, want %q", body.CreatedBy.ID, admin)
	}
	if body.CompletedAt == nil {
		t.Error("completed_at is null on a completed batch")
	}
	if len(body.Lines) != 2 || body.Lines[0].LessonCount != 2 {
		t.Errorf("lines = %+v", body.Lines)
	}

	// It now shows up in the dashboard's batch history.
	w = h.do(http.MethodGet, "/v1/admin/payouts", tok, "")
	var dash dashboardDTO
	_ = json.Unmarshal(w.Body.Bytes(), &dash)
	if dash.BatchesTotal != 1 || len(dash.Batches) != 1 || dash.Batches[0].ID != body.ID {
		t.Errorf("batch history = %+v", dash.Batches)
	}
	if len(dash.Owed) != 0 || dash.Totals.PaidTotalMinor != 19_500_000 {
		t.Errorf("after the run: owed=%+v totals=%+v", dash.Owed, dash.Totals)
	}
}

func TestHandler_Run_NothingToPay(t *testing.T) {
	repo := newFakeRepo()
	repo.addLedgerRow(nodira, uuid.New(), 9_000_000, "held", -48*time.Hour)
	h := newHarness(repo)

	w := h.do(http.MethodPost, "/v1/admin/payouts/run", h.bearer(uuid.New(), rbac.PermPayoutsRun), "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	if code := errCode(t, w.Body.Bytes()); code != "nothing_to_pay" {
		t.Errorf("code = %q, want nothing_to_pay", code)
	}
}

// TestHandler_Run_NeedsRunPermission: payouts.view is not enough to move money.
func TestHandler_Run_NeedsRunPermission(t *testing.T) {
	repo, _, _ := seedLedger()
	h := newHarness(repo)

	w := h.do(http.MethodPost, "/v1/admin/payouts/run", h.bearer(uuid.New(), rbac.PermPayoutsView), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if len(repo.batches) != 0 {
		t.Error("a forbidden run still created a batch")
	}
}

func TestHandler_Batch_OK(t *testing.T) {
	repo, _, _ := seedLedger()
	h := newHarness(repo)
	tok := h.bearer(uuid.New(), rbac.PermPayoutsView, rbac.PermPayoutsRun)

	created, err := h.svc.Run(ctx(), uuid.New())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	w := h.do(http.MethodGet, "/v1/admin/payouts/batches/"+created.Batch.ID.String(), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body batchDetailDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != created.Batch.ID.String() || len(body.Lines) != 2 {
		t.Errorf("batch = %+v", body)
	}
	if body.Lines[0].Teacher.DisplayName != nodira.DisplayName || body.Lines[0].AmountMinor != 13_500_000 {
		t.Errorf("lines[0] = %+v", body.Lines[0])
	}
}

func TestHandler_Batch_NotFound(t *testing.T) {
	h := newHarness(newFakeRepo())
	tok := h.bearer(uuid.New(), rbac.PermPayoutsView)

	w := h.do(http.MethodGet, "/v1/admin/payouts/batches/"+uuid.NewString(), tok, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if code := errCode(t, w.Body.Bytes()); code != "batch_not_found" {
		t.Errorf("code = %q, want batch_not_found", code)
	}
}

func TestHandler_Batch_BadID(t *testing.T) {
	h := newHarness(newFakeRepo())
	tok := h.bearer(uuid.New(), rbac.PermPayoutsView)

	if w := h.do(http.MethodGet, "/v1/admin/payouts/batches/not-a-uuid", tok, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
