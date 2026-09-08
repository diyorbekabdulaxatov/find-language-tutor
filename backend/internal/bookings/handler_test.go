package bookings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

// newTestRouter mounts the booking routes plus stand-ins for the teacher /
// availability routes that share the /v1/teachers group, so route-ordering
// regressions (a gin wildcard-conflict panic on /:slug vs /:slug/slots) surface
// here.
func newTestRouter(repo Repository, tm *auth.TokenManager) *gin.Engine {
	return newTestRouterGW(repo, tm, nil)
}

func newTestRouterGW(repo Repository, tm *auth.TokenManager, gw PaymentGateway) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := &Service{repo: repo, now: func() time.Time { return fixedNow }, logger: discardLogger()}
	svc.payments = gw
	h := NewHandler(svc, discardLogger())

	tg := r.Group("/v1/teachers")
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	tg.GET("", ok)
	tg.GET("/me", ok)
	tg.GET("/:slug", ok)
	tg.GET("/:slug/availability", ok)
	RegisterTeacherSlotRoute(tg, h)

	RegisterRoutes(r.Group("/v1/bookings"), h, auth.RequireAuth(tm))
	return r
}

func do(r *gin.Engine, method, path, body, bearer string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	r.ServeHTTP(w, req)
	return w
}

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Error.Code
}

// --- slots ---

func TestHandler_Slots_OK(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()

	from := fixedNow.Format(time.RFC3339)
	to := fixedNow.AddDate(0, 0, 3).Format(time.RFC3339)
	w := do(newTestRouter(repo, testTokenManager()), http.MethodGet,
		"/v1/teachers/nodira-karimova/slots?from="+from+"&to="+to+"&duration=60", "", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body slotsResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Timezone != "Asia/Tashkent" || body.DurationMinutes != 60 || len(body.Slots) == 0 {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestHandler_Slots_BadFrom(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	w := do(newTestRouter(repo, testTokenManager()), http.MethodGet,
		"/v1/teachers/nodira-karimova/slots?from=not-a-date", "", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandler_Slots_TeacherNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.tcErr = ErrTeacherNotFound
	w := do(newTestRouter(repo, testTokenManager()), http.MethodGet,
		"/v1/teachers/ghost/slots", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// --- create ---

func TestHandler_Create_RequiresAuth(t *testing.T) {
	w := do(newTestRouter(newFakeRepo(), testTokenManager()), http.MethodPost,
		"/v1/bookings", `{"teacher_slug":"x","start_at":"2026-01-05T23:30:00Z","duration_minutes":60}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandler_Create_OK(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	tm := testTokenManager()

	w := do(newTestRouter(repo, tm), http.MethodPost, "/v1/bookings",
		`{"teacher_slug":"nodira-karimova","start_at":"2026-01-05T23:30:00Z","duration_minutes":60}`,
		bearerFor(tm, uuid.New()))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body bookingDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "pending_payment" || body.Price.AmountMinor != 9_000_000 {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestHandler_Create_SlotTakenRace(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	repo.createErr = ErrSlotTaken
	tm := testTokenManager()

	w := do(newTestRouter(repo, tm), http.MethodPost, "/v1/bookings",
		`{"teacher_slug":"nodira-karimova","start_at":"2026-01-05T23:30:00Z","duration_minutes":60}`,
		bearerFor(tm, uuid.New()))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	if c := errCode(t, w); c != "slot_taken" {
		t.Errorf("error code = %q, want slot_taken", c)
	}
}

func TestHandler_Create_BookSelf(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	tm := testTokenManager()

	w := do(newTestRouter(repo, tm), http.MethodPost, "/v1/bookings",
		`{"teacher_slug":"nodira-karimova","start_at":"2026-01-05T23:30:00Z","duration_minutes":60}`,
		bearerFor(tm, repo.tc.OwnerID))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

// --- list / get / confirm / cancel ---

func TestHandler_List_OK(t *testing.T) {
	repo := newFakeRepo()
	repo.listResult = []Booking{{
		ID: uuid.New(), Status: StatusConfirmed,
		StartAt: fixedNow, EndAt: fixedNow.Add(time.Hour),
		Price:   Money{AmountMinor: 100, Currency: "UZS"},
		Teacher: TeacherSummary{Slug: "nodira-karimova", DisplayName: "Nodira", Timezone: "Asia/Tashkent"},
		Student: StudentSummary{ID: uuid.New(), DisplayName: "Aziz"},
	}}
	tm := testTokenManager()

	w := do(newTestRouter(repo, tm), http.MethodGet, "/v1/bookings?role=student", "", bearerFor(tm, uuid.New()))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body bookingListDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Bookings) != 1 || body.Bookings[0].Teacher.Slug != "nodira-karimova" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestHandler_List_BadRole(t *testing.T) {
	tm := testTokenManager()
	w := do(newTestRouter(newFakeRepo(), tm), http.MethodGet, "/v1/bookings?role=boss", "", bearerFor(tm, uuid.New()))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandler_Get_Forbidden(t *testing.T) {
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), uuid.New())
	tm := testTokenManager()

	w := do(newTestRouter(repo, tm), http.MethodGet, "/v1/bookings/"+id.String(), "", bearerFor(tm, uuid.New()))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	tm := testTokenManager()
	w := do(newTestRouter(newFakeRepo(), tm), http.MethodGet, "/v1/bookings/"+uuid.New().String(), "", bearerFor(tm, uuid.New()))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandler_Get_BadID(t *testing.T) {
	tm := testTokenManager()
	w := do(newTestRouter(newFakeRepo(), tm), http.MethodGet, "/v1/bookings/not-a-uuid", "", bearerFor(tm, uuid.New()))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandler_Pay_OK(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusPendingPayment, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.status[id] = "requires_payment"
	tm := testTokenManager()

	w := do(newTestRouterGW(repo, tm, gw), http.MethodPost, "/v1/bookings/"+id.String()+"/pay",
		`{"method_token":"pm_ok"}`, bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body bookingDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Status != "confirmed" || body.Payment == nil || body.Payment.Status != "authorized" {
		t.Errorf("unexpected body: %+v (payment %+v)", body, body.Payment)
	}
}

func TestHandler_Pay_Declined402(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusPendingPayment, student, uuid.New())
	gw := newFakeGateway(repo)
	gw.authErr = PaymentFailedError{Reason: "the card was declined"}
	tm := testTokenManager()

	w := do(newTestRouterGW(repo, tm, gw), http.MethodPost, "/v1/bookings/"+id.String()+"/pay",
		`{"method_token":"pm_decline"}`, bearerFor(tm, student))
	if w.Code != http.StatusPaymentRequired || errCode(t, w) != "payment_failed" {
		t.Fatalf("status = %d code = %s, want 402 payment_failed", w.Code, errCode(t, w))
	}
}

func TestHandler_Pay_DoublePay409(t *testing.T) {
	student := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, student, uuid.New())
	gw := newFakeGateway(repo)
	tm := testTokenManager()

	w := do(newTestRouterGW(repo, tm, gw), http.MethodPost, "/v1/bookings/"+id.String()+"/pay",
		`{"method_token":"pm_ok"}`, bearerFor(tm, student))
	if w.Code != http.StatusConflict || errCode(t, w) != "already_paid" {
		t.Fatalf("status = %d code = %s, want 409 already_paid", w.Code, errCode(t, w))
	}
}

func TestHandler_Complete_TooEarly409(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, uuid.New(), owner)
	gw := newFakeGateway(repo)
	tm := testTokenManager()

	w := do(newTestRouterGW(repo, tm, gw), http.MethodPost, "/v1/bookings/"+id.String()+"/complete",
		"", bearerFor(tm, owner))
	if w.Code != http.StatusConflict || errCode(t, w) != "too_early" {
		t.Fatalf("status = %d code = %s, want 409 too_early", w.Code, errCode(t, w))
	}
}

func TestHandler_Complete_OK(t *testing.T) {
	owner := uuid.New()
	repo := newFakeRepo()
	id := uuid.New()
	repo.store[id] = Booking{
		ID: id, Status: StatusConfirmed,
		StartAt: fixedNow.Add(-2 * time.Hour), EndAt: fixedNow.Add(-time.Hour),
		Student:        StudentSummary{ID: uuid.New()},
		TeacherOwnerID: owner,
	}
	gw := newFakeGateway(repo)
	gw.status[id] = "authorized"
	tm := testTokenManager()

	w := do(newTestRouterGW(repo, tm, gw), http.MethodPost, "/v1/bookings/"+id.String()+"/complete",
		"", bearerFor(tm, owner))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body bookingDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Status != "completed" {
		t.Errorf("status = %s, want completed", body.Status)
	}
}

func TestHandler_Cancel_WithAndWithoutBody(t *testing.T) {
	student := uuid.New()
	tm := testTokenManager()

	repo := newFakeRepo()
	id := seedBooking(repo, StatusConfirmed, student, uuid.New())
	w := do(newTestRouter(repo, tm), http.MethodPost, "/v1/bookings/"+id.String()+"/cancel",
		`{"reason":"schedule clash"}`, bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body bookingDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Status != "cancelled" || body.CancellationReason != "schedule clash" {
		t.Errorf("unexpected body: %+v", body)
	}

	repo2 := newFakeRepo()
	id2 := seedBooking(repo2, StatusPendingPayment, student, uuid.New())
	w = do(newTestRouter(repo2, tm), http.MethodPost, "/v1/bookings/"+id2.String()+"/cancel", "", bearerFor(tm, student))
	if w.Code != http.StatusOK {
		t.Fatalf("no-body cancel status = %d", w.Code)
	}
}
