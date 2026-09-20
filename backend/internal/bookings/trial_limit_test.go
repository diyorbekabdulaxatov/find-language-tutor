package bookings

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// One trial per student per teacher. The repository's unique index is the
// authority (covered in internal/dbtest); these pin the service's early
// answer, the race mapping, and the eligibility read the checkout uses.

func trialService(repo *fakeRepo) (*Service, uuid.UUID) {
	id := uuid.New()
	svc := newService(repo)
	svc.SetLessonTypes(&fakeLessonTypes{offerings: map[uuid.UUID]map[int]Offering{
		id: {30: {ID: id, TeacherID: repo.tc.ID, Title: "Trial lesson", IsTrial: true, Price: Money{AmountMinor: 3_000_000, Currency: "UZS"}}},
	}})
	return svc, id
}

func TestCreate_SecondTrialWithTheSameTeacherIsRefused(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	repo.trialHeld = uuid.New()
	svc, trialID := trialService(repo)

	// Through the offering…
	_, err := svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), DurationMinutes: 30, LessonTypeID: trialID,
	})
	if !errors.Is(err, ErrTrialAlreadyBooked) {
		t.Fatalf("offering path: err = %v, want ErrTrialAlreadyBooked", err)
	}
	// …and through the legacy is_trial flag.
	_, err = svc.Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), IsTrial: true,
	})
	if !errors.Is(err, ErrTrialAlreadyBooked) {
		t.Fatalf("legacy path: err = %v, want ErrTrialAlreadyBooked", err)
	}
	if repo.created != nil {
		t.Fatal("no insert should have been attempted")
	}
}

func TestCreate_AHeldTrialDoesNotBlockARegularLesson(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	repo.trialHeld = uuid.New()

	if _, err := newService(repo).Create(ctx(), uuid.New(), CreateInput{
		TeacherSlug: "nodira-karimova", StartAt: bookableStart(), DurationMinutes: 60,
	}); err != nil {
		t.Fatalf("regular lesson: %v", err)
	}
	if repo.created == nil || repo.created.IsTrial {
		t.Fatalf("expected a regular booking to be inserted, got %+v", repo.created)
	}
}

func TestHandler_Create_TrialRaceIs409(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	repo.spans = stitchedSpans()
	repo.createErr = ErrTrialAlreadyBooked // the unique index won the race
	tm := testTokenManager()

	w := do(newTestRouter(repo, tm), http.MethodPost, "/v1/bookings",
		`{"teacher_slug":"nodira-karimova","start_at":"2026-01-05T23:30:00Z","duration_minutes":30,"is_trial":true}`,
		bearerFor(tm, uuid.New()))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if c := errCode(t, w); c != "trial_already_booked" {
		t.Errorf("error code = %q, want trial_already_booked", c)
	}
}

func TestHandler_TrialEligibility(t *testing.T) {
	tm := testTokenManager()
	held := uuid.New()

	cases := []struct {
		name      string
		trialHeld uuid.UUID
		caller    func(tc TeacherContext) uuid.UUID
		want      trialEligibilityDTO
	}{
		{
			name:   "fresh student",
			caller: func(TeacherContext) uuid.UUID { return uuid.New() },
			want:   trialEligibilityDTO{Eligible: true},
		},
		{
			name:      "already booked",
			trialHeld: held,
			caller:    func(TeacherContext) uuid.UUID { return uuid.New() },
			want:      trialEligibilityDTO{Reason: ptr("already_booked"), BookingID: ptr(held.String())},
		},
		{
			name:   "own profile",
			caller: func(tc TeacherContext) uuid.UUID { return tc.OwnerID },
			want:   trialEligibilityDTO{Reason: ptr("own_profile")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.tc = teacherCtx()
			repo.trialHeld = tc.trialHeld

			w := do(newTestRouter(repo, tm), http.MethodGet,
				"/v1/teachers/nodira-karimova/trial-eligibility", "", bearerFor(tm, tc.caller(repo.tc)))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			var got trialEligibilityDTO
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Eligible != tc.want.Eligible || deref(got.Reason) != deref(tc.want.Reason) || deref(got.BookingID) != deref(tc.want.BookingID) {
				t.Fatalf("got %s, want %+v", w.Body.String(), tc.want)
			}
		})
	}
}

func TestHandler_TrialEligibility_RequiresAuth(t *testing.T) {
	repo := newFakeRepo()
	repo.tc = teacherCtx()
	w := do(newTestRouter(repo, testTokenManager()), http.MethodGet,
		"/v1/teachers/nodira-karimova/trial-eligibility", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func ptr(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
