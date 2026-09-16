//go:build integration

package dbtest

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/payments"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/resources"
)

const (
	sqlstateExclusion  = "23P01"
	sqlstateUnique     = "23505"
	sqlstateCheck      = "23514"
	sqlstateForeignKey = "23503"
)

func TestMain(m *testing.M) {
	code := m.Run()
	Terminate()
	os.Exit(code)
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// TestMigrations_EveryStepIsReversible walks every migration up, then all the
// way down, then up again. A down file that doesn't undo its up (or is
// missing a statement) breaks here rather than in a production rollback.
func TestMigrations_EveryStepIsReversible(t *testing.T) {
	u := URL(t)
	ResetSchema(t, u)
	m := Migrator(t, u)
	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	top, _, err := m.Version()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Down(); err != nil {
		t.Fatalf("down from %d: %v", top, err)
	}
	if _, _, err := m.Version(); !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("after full down: version err = %v, want ErrNilVersion", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
	again, _, _ := m.Version()
	if again != top {
		t.Fatalf("second up landed on %d, want %d", again, top)
	}
}

// fixture inserts the minimum rows the constraint tests need.
type fixture struct {
	pool                 *pgxpool.Pool
	student, other       uuid.UUID
	teacher, teacherUser uuid.UUID
}

func seed(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	f := fixture{pool: pool}
	user := func(email string) uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO users (email, password_hash, display_name) VALUES ($1, 'x', $2) RETURNING id`,
			email, email).Scan(&id); err != nil {
			t.Fatalf("insert user %s: %v", email, err)
		}
		return id
	}
	f.student = user("student@example.com")
	f.other = user("other@example.com")
	f.teacherUser = user("teacher@example.com")
	if err := pool.QueryRow(ctx, `
		INSERT INTO teachers (slug, display_name, headline, kind, country_code, country_name, city, timezone,
		                      price_per_hour_minor, user_id, status)
		VALUES ('t1', 'T', 'h', 'professional', 'UZ', 'Uzbekistan', 'Tashkent', 'Asia/Tashkent',
		        1000000, $1, 'approved')
		RETURNING id`, f.teacherUser).Scan(&f.teacher); err != nil {
		t.Fatalf("insert teacher: %v", err)
	}
	return f
}

func (f fixture) booking(t *testing.T, student uuid.UUID, start time.Time, minutes int, status string) (uuid.UUID, error) {
	t.Helper()
	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(), `
		INSERT INTO bookings (teacher_id, student_id, start_at, end_at, duration_minutes, status, price_minor, currency, is_trial)
		VALUES ($1, $2, $3, $4, $5, $6, 1000000, 'UZS', false)
		RETURNING id`,
		f.teacher, student, start, start.Add(time.Duration(minutes)*time.Minute), minutes, status).Scan(&id)
	return id, err
}

// TestBookings_ExcludeConstraintPreventsDoubleBooking is the invariant the
// whole booking flow leans on: the DB, not the service, is what makes a lost
// race a 409.
func TestBookings_ExcludeConstraintPreventsDoubleBooking(t *testing.T) {
	pool := Pool(t)
	f := seed(t, pool)
	start := time.Date(2030, 1, 6, 9, 0, 0, 0, time.UTC)

	if _, err := f.booking(t, f.student, start, 60, "confirmed"); err != nil {
		t.Fatalf("first booking: %v", err)
	}

	// Same teacher, overlapping window, a different student → excluded.
	_, err := f.booking(t, f.other, start.Add(30*time.Minute), 60, "pending_payment")
	if pgCode(err) != sqlstateExclusion {
		t.Fatalf("overlapping teacher booking: err=%v, want SQLSTATE %s", err, sqlstateExclusion)
	}

	// Back-to-back is fine: tstzrange is [) so end == next start does not overlap.
	if _, err := f.booking(t, f.other, start.Add(60*time.Minute), 60, "confirmed"); err != nil {
		t.Fatalf("back-to-back booking should be allowed: %v", err)
	}

	// A cancelled lesson frees the window (the constraint is partial on status).
	if _, err := pool.Exec(context.Background(),
		`UPDATE bookings SET status = 'cancelled' WHERE start_at = $1`, start); err != nil {
		t.Fatal(err)
	}
	if _, err := f.booking(t, f.other, start, 60, "confirmed"); err != nil {
		t.Fatalf("window should be free after cancellation: %v", err)
	}
}

func TestBookings_StudentCannotBeInTwoLessonsAtOnce(t *testing.T) {
	pool := Pool(t)
	f := seed(t, pool)
	start := time.Date(2030, 1, 6, 9, 0, 0, 0, time.UTC)

	// A second teacher so the teacher-side constraint is not what fires.
	var teacher2 uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO teachers (slug, display_name, headline, kind, country_code, country_name, city, timezone, price_per_hour_minor, status)
		VALUES ('t2', 'T2', 'h', 'community', 'UZ', 'Uzbekistan', 'Tashkent', 'Asia/Tashkent', 1, 'approved') RETURNING id`).Scan(&teacher2); err != nil {
		t.Fatal(err)
	}
	if _, err := f.booking(t, f.student, start, 60, "confirmed"); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO bookings (teacher_id, student_id, start_at, end_at, duration_minutes, status, price_minor, currency, is_trial)
		VALUES ($1, $2, $3, $4, 60, 'confirmed', 1, 'UZS', false)`,
		teacher2, f.student, start.Add(15*time.Minute), start.Add(75*time.Minute))
	if pgCode(err) != sqlstateExclusion {
		t.Fatalf("student double-booking: err=%v, want SQLSTATE %s", err, sqlstateExclusion)
	}
}

// TestPayments_WebhookReplayIsANoOp checks the insert-first idempotency
// gate end to end through the real repository: the second delivery of the
// same event id must neither error nor apply twice.
func TestPayments_WebhookReplayIsANoOp(t *testing.T) {
	pool := Pool(t)
	f := seed(t, pool)
	ctx := context.Background()
	bookingID, err := f.booking(t, f.student, time.Date(2030, 1, 6, 9, 0, 0, 0, time.UTC), 60, "pending_payment")
	if err != nil {
		t.Fatal(err)
	}
	var paymentID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO payments (booking_id, provider, status, amount_minor, currency)
		VALUES ($1, 'fake', 'requires_payment', 1000000, 'UZS') RETURNING id`, bookingID).Scan(&paymentID); err != nil {
		t.Fatal(err)
	}

	repo := payments.NewPostgresRepository(pool, 7)
	ev := payments.Event{ID: "evt_1", Type: payments.EventAuthorized, PaymentID: paymentID}

	applied, err := repo.ApplyEvent(ctx, ev)
	if err != nil || !applied {
		t.Fatalf("first delivery: applied=%v err=%v", applied, err)
	}
	applied, err = repo.ApplyEvent(ctx, ev)
	if err != nil || applied {
		t.Fatalf("replay: applied=%v err=%v, want (false, nil)", applied, err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM bookings WHERE id = $1`, bookingID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "confirmed" {
		t.Fatalf("booking status after authorized event = %q, want confirmed", status)
	}
	var events int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM payment_events WHERE payment_id = $1`, paymentID).Scan(&events)
	if events != 1 {
		t.Fatalf("payment_events rows = %d, want 1", events)
	}

	// An event for a payment we don't have is a foreign-key violation the
	// repository names, not a silent no-op.
	_, err = repo.ApplyEvent(ctx, payments.Event{ID: "evt_2", Type: payments.EventAuthorized, PaymentID: uuid.New()})
	if !errors.Is(err, payments.ErrPaymentNotFound) {
		t.Fatalf("unknown payment: err=%v, want ErrPaymentNotFound", err)
	}
}

// TestResources_ContentRoundTripsThroughJSONB: the first jsonb column in the
// schema — what goes in must come out identical, including nested question
// structures and empty optional fields.
func TestResources_ContentRoundTripsThroughJSONB(t *testing.T) {
	pool := Pool(t)
	f := seed(t, pool)
	repo := resources.NewPostgresRepository(pool)
	ctx := context.Background()

	content := resources.Content{
		Passage: "Uzbekistan is in Central Asia.",
		Questions: []resources.Question{
			{
				ID: "q1", Prompt: "Where is Uzbekistan?", Kind: "single", Points: 2,
				Choices: []resources.Choice{{ID: "a", Text: "Central Asia"}, {ID: "b", Text: "Europe"}},
				Correct: []string{"a"},
			},
			{ID: "q2", Prompt: "Capital?", Kind: "text", Points: 1, Correct: []string{"Tashkent", "Toshkent"}},
		},
	}
	created, err := repo.Create(ctx, resources.CreateParams{
		TeacherID: f.teacher, Type: resources.TypeReading, Title: "Geo", Content: content, Status: resources.StatusDraft,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Content.Passage != content.Passage || len(got.Content.Questions) != 2 {
		t.Fatalf("content differs: %+v", got.Content)
	}
	q := got.Content.Questions[0]
	if q.Kind != "single" || q.Points != 2 || len(q.Choices) != 2 || len(q.Correct) != 1 || q.Correct[0] != "a" {
		t.Fatalf("question 1 differs: %+v", q)
	}
	if got.Content.Questions[1].Correct[1] != "Toshkent" {
		t.Fatalf("question 2 accepted answers differ: %+v", got.Content.Questions[1])
	}
}

// TestPayoutLedger_ExactlyOneSource: a ledger row must point at a booking or
// a course enrollment, never both, never neither (migration 000018).
func TestPayoutLedger_ExactlyOneSource(t *testing.T) {
	pool := Pool(t)
	f := seed(t, pool)
	ctx := context.Background()
	bookingID, err := f.booking(t, f.student, time.Date(2030, 1, 6, 9, 0, 0, 0, time.UTC), 60, "completed")
	if err != nil {
		t.Fatal(err)
	}
	insert := func(booking, enrollment any) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO payout_ledger (teacher_id, booking_id, course_enrollment_id, amount_minor, currency, state, available_at)
			VALUES ($1, $2, $3, 1000000, 'UZS', 'held', now())`, f.teacher, booking, enrollment)
		return err
	}
	if err := insert(bookingID, nil); err != nil {
		t.Fatalf("booking-sourced row: %v", err)
	}
	if err := insert(nil, nil); pgCode(err) != sqlstateCheck {
		t.Fatalf("neither source: err=%v, want SQLSTATE %s", err, sqlstateCheck)
	}
	// Same booking twice is the UNIQUE, not the CHECK.
	if err := insert(bookingID, nil); pgCode(err) != sqlstateUnique {
		t.Fatalf("duplicate booking row: err=%v, want SQLSTATE %s", err, sqlstateUnique)
	}
}

// TestTeachers_OneProfilePerAccount: the service pre-checks, the database
// decides. A second profile for the same user is a unique violation; the
// unclaimed seed profiles (user_id NULL) are unaffected.
func TestTeachers_OneProfilePerAccount(t *testing.T) {
	pool := Pool(t)
	f := seed(t, pool)
	ctx := context.Background()
	insert := func(slug string, user any) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO teachers (slug, display_name, headline, kind, country_code, country_name, city, timezone, price_per_hour_minor, user_id, status)
			VALUES ($1, 'T', 'h', 'community', 'UZ', 'Uzbekistan', 'Tashkent', 'Asia/Tashkent', 1, $2, 'pending')`, slug, user)
		return err
	}
	if err := insert("dup", f.teacherUser); pgCode(err) != sqlstateUnique {
		t.Fatalf("second profile for the same user: err=%v, want SQLSTATE %s", err, sqlstateUnique)
	}
	if err := insert("unclaimed-1", nil); err != nil {
		t.Fatalf("first unclaimed profile: %v", err)
	}
	if err := insert("unclaimed-2", nil); err != nil {
		t.Fatalf("second unclaimed profile (NULL user_id must not collide): %v", err)
	}
}

func TestUsers_LocaleIsConstrained(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO users (email, password_hash, display_name, locale) VALUES ('fr@example.com', 'x', 'F', 'fr')`)
	if pgCode(err) != sqlstateCheck {
		t.Fatalf("locale=fr: err=%v, want SQLSTATE %s", err, sqlstateCheck)
	}
	var loc string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, display_name) VALUES ('d@example.com', 'x', 'D') RETURNING locale`).Scan(&loc); err != nil {
		t.Fatal(err)
	}
	if loc != "en" {
		t.Fatalf("default locale = %q, want en", loc)
	}
}
