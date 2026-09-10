package teachers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestService_Create_SetsPendingAndUnverified(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	got, err := svc.Create(context.Background(), uuid.New(), validProfileInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Status != StatusPending {
		t.Errorf("status = %q, want pending", got.Status)
	}
	if got.Verified {
		t.Error("new profile should not be verified")
	}
}

func TestService_GetBySlug_HidesNonApproved(t *testing.T) {
	suspended := &Teacher{ID: uuid.New(), Slug: "suspended-t", Status: StatusSuspended}
	svc := NewService(&fakeRepo{bySlug: map[string]*Teacher{"suspended-t": suspended}})

	if _, err := svc.GetBySlug(context.Background(), "suspended-t"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("public GetBySlug on suspended = %v, want ErrNotFound", err)
	}
	// ...but the admin read still returns it.
	got, err := svc.GetForAdmin(context.Background(), "suspended-t")
	if err != nil || got.Status != StatusSuspended {
		t.Fatalf("GetForAdmin = (%+v, %v)", got, err)
	}
}

func TestHandler_GetBySlug_404ForSuspended(t *testing.T) {
	repo := &fakeRepo{bySlug: map[string]*Teacher{
		"suspended-t": {ID: uuid.New(), Slug: "suspended-t", Kind: KindProfessional, Status: StatusSuspended},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/suspended-t", nil)
	newTestRouter(repo).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", w.Code, w.Body.String())
	}
}

func TestHandler_GetMine_ReturnsNonApprovedWithStatus(t *testing.T) {
	tm := testTokenManager()
	owner := uuid.New()
	repo := &fakeRepo{
		bySlug: map[string]*Teacher{"mine": {
			ID: uuid.New(), Slug: "mine", DisplayName: "Me", Kind: KindProfessional,
			Status: StatusPending, ModerationNote: "",
		}},
		ownerBySlug: map[string]uuid.UUID{"mine": owner},
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/me", nil)
	req.Header.Set("Authorization", bearerFor(tm, owner))
	newTestRouterTM(repo, tm).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var body profileDTO
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "pending" {
		t.Errorf("me status = %q, want pending", body.Status)
	}
}

func validProfileInput() ProfileInput {
	return ProfileInput{
		DisplayName: "New Teacher", Headline: "Teaches things", Kind: KindProfessional,
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		Currency: CurrencyUZS, PricePerHourMinor: 5_000_000,
	}
}
