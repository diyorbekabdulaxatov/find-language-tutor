package teachers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newTestRouter(repo Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(repo), discardLogger())
	RegisterRoutes(r.Group("/v1/teachers"), h)
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
