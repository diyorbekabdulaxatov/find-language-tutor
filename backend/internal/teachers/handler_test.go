package teachers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func newTestRouter(repo Repository) *gin.Engine {
	return newTestRouterTM(repo, testTokenManager())
}

func newTestRouterTM(repo Repository, tm *auth.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(repo), discardLogger())
	RegisterRoutes(r.Group("/v1/teachers"), h, auth.RequireAuth(tm))
	return r
}

func TestHandler_List_OK(t *testing.T) {
	repo := &fakeRepo{
		list: []Teacher{{
			ID: uuid.New(), Slug: "nodira-karimova", DisplayName: "Nodira Karimova",
			Kind: KindProfessional, PricePerHour: Money{9_000_000, CurrencyUZS},
			Teaches: []Language{{Code: "en", Name: "English", Level: "c2"}},
		}},
		total:  1,
		facets: []LanguageFacet{{Code: "en", Name: "English", Count: 1}},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers?language=en&sort=price_asc", nil)
	newTestRouter(repo).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var body listResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 || len(body.Teachers) != 1 {
		t.Fatalf("unexpected body: %+v", body)
	}
	got := body.Teachers[0]
	if got.Slug != "nodira-karimova" || got.PricePerHour.AmountMinor != 9_000_000 || got.PricePerHour.Currency != "UZS" {
		t.Errorf("summary mapped wrong: %+v", got)
	}
	if got.Focus == nil {
		t.Error("focus should serialize as [] not null")
	}
	if len(body.Facets.Languages) != 1 {
		t.Errorf("facets missing: %+v", body.Facets)
	}
	if repo.lastParams.Language != "en" || repo.lastParams.Sort != SortPriceAsc {
		t.Errorf("params not passed through: %+v", repo.lastParams)
	}
}

func TestHandler_List_RejectsBadKind(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers?kind=wizard", nil)
	newTestRouter(&fakeRepo{}).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Error.Code != "bad_request" {
		t.Errorf("error code = %q", body.Error.Code)
	}
}

func TestHandler_GetBySlug_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/ghost", nil)
	newTestRouter(&fakeRepo{bySlug: map[string]*Teacher{}}).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandler_GetBySlug_OK(t *testing.T) {
	trial := Money{3_000_000, CurrencyUZS}
	repo := &fakeRepo{bySlug: map[string]*Teacher{
		"nodira-karimova": {
			ID: uuid.New(), Slug: "nodira-karimova", DisplayName: "Nodira Karimova",
			Kind: KindProfessional, PricePerHour: Money{9_000_000, CurrencyUZS}, TrialPrice: &trial,
			About:      "para one\n\npara two",
			Experience: []Experience{{Title: "IELTS instructor", Org: "Cambridge", Period: "2019 – present"}},
		},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/nodira-karimova", nil)
	newTestRouter(repo).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body profileDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "nodira-karimova" {
		t.Errorf("slug = %q", body.Slug)
	}
	if body.TrialPrice == nil || body.TrialPrice.AmountMinor != 3_000_000 {
		t.Errorf("trial price mapped wrong: %+v", body.TrialPrice)
	}
	if len(body.Experience) != 1 {
		t.Errorf("experience missing: %+v", body.Experience)
	}
}

// --- write endpoints ---

const validCreateBody = `{
	"display_name": "Nodira Karimova",
	"headline": "IELTS coach",
	"kind": "professional",
	"country_code": "UZ",
	"country_name": "Uzbekistan",
	"city": "Tashkent",
	"timezone": "Asia/Tashkent",
	"price_per_hour_minor": 9000000,
	"trial_price_minor": 3000000,
	"currency": "UZS",
	"about": "about",
	"teaching_style": "structured",
	"languages": [
		{"role": "teaches", "code": "en", "name": "English", "level": "c2"},
		{"role": "also_speaks", "code": "uz", "name": "Uzbek", "level": "native"}
	],
	"focus": ["IELTS"],
	"experience": [{"title": "Instructor", "org": "Cambridge", "period": "2019 - now"}]
}`

func TestHandler_Create_RequiresAuth(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/teachers", strings.NewReader(validCreateBody))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter(&fakeRepo{}).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandler_Create_OK(t *testing.T) {
	tm := testTokenManager()
	repo := &fakeRepo{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/teachers", strings.NewReader(validCreateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, uuid.New()))
	newTestRouterTM(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body profileDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "nodira-karimova" {
		t.Errorf("slug = %q", body.Slug)
	}
	if len(body.Teaches) != 1 || len(body.AlsoSpeaks) != 1 {
		t.Errorf("languages split wrong: %+v / %+v", body.Teaches, body.AlsoSpeaks)
	}
	if repo.createdInput == nil || repo.createdInput.DisplayName != "Nodira Karimova" {
		t.Errorf("repo input wrong: %+v", repo.createdInput)
	}
}

func TestHandler_Create_Conflict(t *testing.T) {
	tm := testTokenManager()
	owner := uuid.New()
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"x": {ID: uuid.New(), Slug: "x"}},
		ownerBySlug: map[string]uuid.UUID{"x": owner},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/teachers", strings.NewReader(validCreateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, owner))
	newTestRouterTM(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", w.Code, w.Body.String())
	}
	var e struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Error.Code != "profile_exists" {
		t.Errorf("error code = %q", e.Error.Code)
	}
}

func TestHandler_Create_ValidationError(t *testing.T) {
	tm := testTokenManager()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/teachers",
		strings.NewReader(`{"display_name":"","headline":"h","kind":"professional"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, uuid.New()))
	newTestRouterTM(&fakeRepo{}, tm).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestHandler_Update_OK(t *testing.T) {
	tm := testTokenManager()
	owner := uuid.New()
	repo := &fakeRepo{
		bySlug: map[string]*Teacher{"nodira-karimova": {
			ID: uuid.New(), Slug: "nodira-karimova", DisplayName: "Nodira Karimova",
			Headline: "old", Kind: KindProfessional, CountryCode: "UZ", CountryName: "Uzbekistan",
			City: "Tashkent", Timezone: "Asia/Tashkent", PricePerHour: Money{9_000_000, CurrencyUZS},
		}},
		ownerBySlug: map[string]uuid.UUID{"nodira-karimova": owner},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/teachers/nodira-karimova",
		strings.NewReader(`{"headline":"new headline","price_per_hour_minor":12000000}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, owner))
	newTestRouterTM(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body profileDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Headline != "new headline" || body.PricePerHour.AmountMinor != 12_000_000 {
		t.Errorf("update not applied: %+v", body)
	}
}

func TestHandler_Update_Forbidden(t *testing.T) {
	tm := testTokenManager()
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"nodira-karimova": {ID: uuid.New(), Slug: "nodira-karimova"}},
		ownerBySlug: map[string]uuid.UUID{"nodira-karimova": uuid.New()},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/teachers/nodira-karimova",
		strings.NewReader(`{"headline":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, uuid.New()))
	newTestRouterTM(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if repo.lastUpdate != nil {
		t.Error("Update should not run for a non-owner")
	}
}

func TestHandler_Update_NotFound(t *testing.T) {
	tm := testTokenManager()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/teachers/ghost", strings.NewReader(`{"headline":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, uuid.New()))
	newTestRouterTM(&fakeRepo{bySlug: map[string]*Teacher{}}, tm).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandler_Update_RequiresAuth(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/teachers/nodira-karimova", strings.NewReader(`{"headline":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter(&fakeRepo{}).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandler_GetMine_OK(t *testing.T) {
	tm := testTokenManager()
	owner := uuid.New()
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"mine": {ID: uuid.New(), Slug: "mine", DisplayName: "Me"}},
		ownerBySlug: map[string]uuid.UUID{"mine": owner},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/me", nil)
	req.Header.Set("Authorization", bearerFor(tm, owner))
	newTestRouterTM(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body profileDTO
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Slug != "mine" {
		t.Errorf("slug = %q", body.Slug)
	}
}

func TestHandler_GetMine_404WhenNoProfile(t *testing.T) {
	tm := testTokenManager()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/me", nil)
	req.Header.Set("Authorization", bearerFor(tm, uuid.New()))
	newTestRouterTM(&fakeRepo{}, tm).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
	var e struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Error.Code != "not_found" {
		t.Errorf("error code = %q", e.Error.Code)
	}
}

func TestHandler_GetMine_RequiresAuth(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/me", nil)
	newTestRouter(&fakeRepo{}).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
