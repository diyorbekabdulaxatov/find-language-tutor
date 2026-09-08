package availability

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

func testTokenManager() *auth.TokenManager {
	return auth.NewTokenManager("availability-test-secret", time.Minute)
}

func bearerFor(tm *auth.TokenManager, id uuid.UUID) string {
	tok, err := tm.IssueAccess(auth.User{ID: id, Email: "demo@example.com", DisplayName: "Demo"}, time.Now())
	if err != nil {
		panic(err)
	}
	return "Bearer " + tok
}

func newTestRouter(repo Repository, tm *auth.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(repo), discardLogger())
	RegisterRoutes(r.Group("/v1/teachers"), h, auth.RequireAuth(tm))
	return r
}

func TestHandler_Get_OK(t *testing.T) {
	repo := &fakeRepo{
		ref: teacherRef(),
		stored: []Slot{
			{Weekday: Monday, StartMinute: 540, EndMinute: 600},
			{Weekday: Wednesday, StartMinute: 600, EndMinute: 660},
		},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/nodira-karimova/availability", nil)
	newTestRouter(repo, testTokenManager()).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body availabilityDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Timezone != "Asia/Tashkent" || body.TeacherSlug != "nodira-karimova" {
		t.Errorf("unexpected body: %+v", body)
	}
	if body.GranularityMinutes != SlotGranularityMinutes {
		t.Errorf("granularity = %d", body.GranularityMinutes)
	}
	if len(body.Slots) != 2 || body.Slots[0].Weekday != int(Monday) || body.Slots[0].StartMinute != 540 {
		t.Errorf("slots mapped wrong: %+v", body.Slots)
	}
}

func TestHandler_Get_TeacherNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/ghost/availability", nil)
	newTestRouter(&fakeRepo{refErr: ErrTeacherNotFound}, testTokenManager()).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandler_Replace_RequiresAuth(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/teachers/nodira-karimova/availability",
		strings.NewReader(`{"slots":[]}`))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter(&fakeRepo{ref: teacherRef()}, testTokenManager()).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Error.Code != "unauthorized" {
		t.Errorf("error code = %q", body.Error.Code)
	}
}

func TestHandler_Replace_RejectsNonOwner(t *testing.T) {
	tm := testTokenManager()
	repo := &fakeRepo{ref: teacherRef()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/teachers/nodira-karimova/availability",
		strings.NewReader(`{"slots":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, uuid.New())) // not the owner
	newTestRouter(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", w.Code, w.Body.String())
	}
	if repo.replaceHit {
		t.Error("ReplaceSlots should not run for a non-owner")
	}
}

func TestHandler_Replace_OK(t *testing.T) {
	tm := testTokenManager()
	repo := &fakeRepo{ref: teacherRef()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/teachers/nodira-karimova/availability",
		strings.NewReader(`{"slots":[
			{"weekday":3,"start_minute":600,"end_minute":660},
			{"weekday":1,"start_minute":540,"end_minute":600}
		]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, demoOwnerID))
	newTestRouter(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body availabilityDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Slots) != 2 || body.Slots[0].Weekday != int(Monday) {
		t.Errorf("response not sorted / wrong: %+v", body.Slots)
	}
	if !repo.replaceHit || len(repo.replaced) != 2 {
		t.Errorf("ReplaceSlots not called with 2 slots: %+v", repo.replaced)
	}
}

func TestHandler_Replace_RejectsOverlap(t *testing.T) {
	tm := testTokenManager()
	repo := &fakeRepo{ref: teacherRef()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/teachers/nodira-karimova/availability",
		strings.NewReader(`{"slots":[
			{"weekday":1,"start_minute":540,"end_minute":660},
			{"weekday":1,"start_minute":600,"end_minute":720}
		]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, demoOwnerID))
	newTestRouter(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
	if repo.replaceHit {
		t.Error("ReplaceSlots should not run for an invalid set")
	}
}

func TestHandler_Replace_TeacherNotFound(t *testing.T) {
	tm := testTokenManager()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/teachers/ghost/availability",
		strings.NewReader(`{"slots":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, demoOwnerID))
	newTestRouter(&fakeRepo{refErr: ErrTeacherNotFound}, tm).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
