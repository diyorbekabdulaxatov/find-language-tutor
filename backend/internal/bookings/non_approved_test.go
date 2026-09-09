package bookings

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// A non-approved teacher is invisible to the booking flow: the Postgres
// TeacherContextBySlug query filters status = 'approved', so the slug resolves
// to "no such teacher". These tests pin the handler behaviour when the
// teacher-context lookup reports not-found.

func TestHandler_Slots_NonApprovedTeacher_404(t *testing.T) {
	repo := newFakeRepo()
	repo.tcErr = ErrTeacherNotFound // pending / rejected / suspended -> not resolvable

	w := do(newTestRouter(repo, testTokenManager()), http.MethodGet,
		"/v1/teachers/pending-teacher/slots", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", w.Code, w.Body.String())
	}
}

func TestHandler_Create_NonApprovedTeacher_404(t *testing.T) {
	repo := newFakeRepo()
	repo.tcErr = ErrTeacherNotFound

	tm := testTokenManager()
	student := uuid.New()
	w := do(newTestRouter(repo, tm), http.MethodPost, "/v1/bookings",
		`{"teacher_slug":"pending-teacher","start_at":"2099-01-05T10:00:00Z","duration_minutes":60}`,
		bearerFor(tm, student))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", w.Code, w.Body.String())
	}
	if code := errCode(t, w); code != "not_found" {
		t.Fatalf("error code = %q, want not_found", code)
	}
}
