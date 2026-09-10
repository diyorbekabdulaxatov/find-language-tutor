package payments

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func newTestRouter(s *Service, secret string, tm *auth.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/v1/payments"), NewHandler(s, secret, discardLogger()), auth.RequireAuth(tm))
	return r
}

func do(r *gin.Engine, method, path, body, bearer string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_Webhook_IdempotentOn200(t *testing.T) {
	s, repo := newTestService()
	bid := uuid.New()
	pid := repo.seedPayment(bid, StatusRequiresPayment, 9_000_000, "UZS")
	r := newTestRouter(s, "", testTokenManager())

	body := fmt.Sprintf(`{"event_id":"evt_1","type":"payment.authorized","payment_id":%q,"provider_ref":"ref_1"}`, pid)

	w := do(r, http.MethodPost, "/v1/payments/webhook", body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("first: status = %d, body = %s", w.Code, w.Body.String())
	}
	var got webhookResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if !got.Applied {
		t.Error("first delivery should be applied")
	}

	w = do(r, http.MethodPost, "/v1/payments/webhook", body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("replay: status = %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Applied {
		t.Error("replay should not re-apply")
	}
	if len(repo.applied) != 1 {
		t.Errorf("effect ran %d times, want 1", len(repo.applied))
	}
}

func TestHandler_Webhook_BadBody400(t *testing.T) {
	s, _ := newTestService()
	r := newTestRouter(s, "", testTokenManager())
	w := do(r, http.MethodPost, "/v1/payments/webhook", `not json`, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandler_Webhook_BadSignature401(t *testing.T) {
	s, repo := newTestService()
	pid := repo.seedPayment(uuid.New(), StatusRequiresPayment, 100, "UZS")
	r := newTestRouter(s, "shhh", testTokenManager())
	body := fmt.Sprintf(`{"event_id":"e","type":"payment.failed","payment_id":%q}`, pid)
	w := do(r, http.MethodPost, "/v1/payments/webhook", body, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandler_Earnings_OK(t *testing.T) {
	s, repo := newTestService()
	owner := uuid.New()
	tid := uuid.New()
	repo.teacherByOwner[owner] = tid
	cleared := time.Now().UTC().Add(-24 * time.Hour)
	repo.earnings[tid] = []EarningLine{
		{BookingID: uuid.New(), StudentDisplayName: "Aziz", AmountMinor: 9_000_000, Currency: "UZS", State: LedgerHeld, AvailableAt: cleared},
		{BookingID: uuid.New(), StudentDisplayName: "Bek", AmountMinor: 4_000_000, Currency: "UZS", State: LedgerPaid, AvailableAt: cleared},
	}
	tm := testTokenManager()
	r := newTestRouter(s, "", tm)

	w := do(r, http.MethodGet, "/v1/payments/me", "", bearerFor(tm, owner))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body earningsDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.AvailableMinor != 9_000_000 || body.PaidMinor != 4_000_000 ||
		body.TotalEarnedMinor != 13_000_000 || len(body.Lessons) != 2 {
		t.Errorf("unexpected body: %+v", body)
	}
	if body.Lessons[0].State != string(LedgerAvailable) {
		t.Errorf("cleared lesson state = %q, want available", body.Lessons[0].State)
	}
	if body.Lessons[0].StudentDisplayName != "Aziz" {
		t.Errorf("student name = %q", body.Lessons[0].StudentDisplayName)
	}
}

func TestHandler_Earnings_RequiresAuth(t *testing.T) {
	s, _ := newTestService()
	w := do(newTestRouter(s, "", testTokenManager()), http.MethodGet, "/v1/payments/me", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandler_Earnings_NoProfile404(t *testing.T) {
	s, _ := newTestService()
	tm := testTokenManager()
	w := do(newTestRouter(s, "", tm), http.MethodGet, "/v1/payments/me", "", bearerFor(tm, uuid.New()))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
