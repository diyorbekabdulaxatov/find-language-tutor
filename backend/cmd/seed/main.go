// Command seed loads the demo teacher catalog. It is idempotent: it deletes all
// teachers and re-inserts them inside one transaction.
//
//	go run ./cmd/seed
package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/i18n"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/rbac"
)

// demoPassword is the password for every seeded demo account. Demo data only —
// documented in the README. Never use this pattern for real accounts.
const demoPassword = "password"

func main() {
	log.SetFlags(0)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	q := sqlc.New(tx)

	// FK order: disputes -> bookings/users, reviews -> bookings/teachers/users,
	// payout_ledger -> payout_batches / bookings / teachers / course_enrollments,
	// payment_events -> payments -> bookings -> teachers/users, payout_batches.
	// created_by -> users, teachers.user_id -> users. Only disputes.booking_id
	// cascades, so the seed clears every table explicitly deepest-first. We
	// seed bookings, sample reviews, one open dispute, and the payments +
	// payout rows behind the operator payout dashboard.
	//
	// Courses (phases C1-C3) and lesson resources (phase A2/A3) join this same
	// clearing pass even though neither is seeded with data of its own (both
	// are created ad hoc through the app/API today): a submission may point at
	// either a booking or a course_enrollment, so it — and booking_resources —
	// must clear before DeleteAllBookings, and course_item_progress /
	// course_payment_events / course_payments / course_enrollments before
	// DeleteAllCourses, which itself must precede DeleteAllTeachers. Without
	// this, `make seed` / `make db-reset` fails with a foreign-key violation
	// the moment any lesson homework or course/purchase exists in the database.
	if err := q.DeleteAllDisputes(ctx); err != nil {
		log.Fatalf("clear disputes: %v", err)
	}
	if err := q.DeleteAllReviews(ctx); err != nil {
		log.Fatalf("clear reviews: %v", err)
	}
	if err := q.DeleteAllPayoutLedger(ctx); err != nil {
		log.Fatalf("clear payout ledger: %v", err)
	}
	if err := q.DeleteAllPayoutBatches(ctx); err != nil {
		log.Fatalf("clear payout batches: %v", err)
	}
	if err := q.DeleteAllPaymentEvents(ctx); err != nil {
		log.Fatalf("clear payment events: %v", err)
	}
	if err := q.DeleteAllPayments(ctx); err != nil {
		log.Fatalf("clear payments: %v", err)
	}
	if err := q.DeleteAllSubmissions(ctx); err != nil {
		log.Fatalf("clear submissions: %v", err)
	}
	if err := q.DeleteAllBookingResources(ctx); err != nil {
		log.Fatalf("clear booking resources: %v", err)
	}
	if err := q.DeleteAllBookings(ctx); err != nil {
		log.Fatalf("clear bookings: %v", err)
	}
	if err := q.DeleteAllCourseItemProgress(ctx); err != nil {
		log.Fatalf("clear course item progress: %v", err)
	}
	if err := q.DeleteAllCourseEnrollments(ctx); err != nil {
		log.Fatalf("clear course enrollments: %v", err)
	}
	if err := q.DeleteAllCoursePaymentEvents(ctx); err != nil {
		log.Fatalf("clear course payment events: %v", err)
	}
	if err := q.DeleteAllCoursePayments(ctx); err != nil {
		log.Fatalf("clear course payments: %v", err)
	}
	if err := q.DeleteAllCourseItems(ctx); err != nil {
		log.Fatalf("clear course items: %v", err)
	}
	if err := q.DeleteAllCourseSections(ctx); err != nil {
		log.Fatalf("clear course sections: %v", err)
	}
	if err := q.DeleteAllCourses(ctx); err != nil {
		log.Fatalf("clear courses: %v", err)
	}
	// lesson_type_prices -> lesson_types -> teachers, and bookings (already
	// cleared above) point at lesson_types.
	if err := q.DeleteAllLessonTypePrices(ctx); err != nil {
		log.Fatalf("clear lesson type prices: %v", err)
	}
	if err := q.DeleteAllLessonTypes(ctx); err != nil {
		log.Fatalf("clear lesson types: %v", err)
	}
	if err := q.DeleteAllTeachers(ctx); err != nil {
		log.Fatalf("clear teachers: %v", err)
	}
	// RBAC: user_roles -> users/roles, role_permissions -> roles.
	if err := q.DeleteAllUserRoles(ctx); err != nil {
		log.Fatalf("clear user roles: %v", err)
	}
	if err := q.DeleteAllRolePermissions(ctx); err != nil {
		log.Fatalf("clear role permissions: %v", err)
	}
	if err := q.DeleteAllRoles(ctx); err != nil {
		log.Fatalf("clear roles: %v", err)
	}
	if err := q.DeleteAllUsers(ctx); err != nil {
		log.Fatalf("clear users: %v", err)
	}

	// Every demo teacher gets a matching account (<firstname>@example.com /
	// "password") so the frontend can sign in and exercise owner-gated routes.
	passwordHash, err := auth.HashPassword(demoPassword)
	if err != nil {
		log.Fatalf("hash demo password: %v", err)
	}

	// Populated as teachers are inserted; used for the sample-review and
	// booking passes below.
	teacherIDBySlug := make(map[string]uuid.UUID, len(seedTeachers))
	teacherPriceBySlug := make(map[string]int64, len(seedTeachers))
	userIDByFirstName := make(map[string]uuid.UUID, len(seedTeachers))

	for _, t := range seedTeachers {
		user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
			Email:        demoEmail(t.DisplayName),
			PasswordHash: passwordHash,
			DisplayName:  t.DisplayName,
			Locale:       i18n.Default,
		})
		if err != nil {
			log.Fatalf("create user for %s: %v", t.Slug, err)
		}
		userIDByFirstName[strings.ToLower(firstName(t.DisplayName))] = user.ID
		// Spread the demo accounts back over the last two months so the admin
		// dashboard's sign-ups series has a shape on a fresh database.
		if err := q.SeedBackdateUser(ctx, sqlc.SeedBackdateUserParams{
			ID:        user.ID,
			CreatedAt: pgtype.Timestamptz{Time: signupDate(len(userIDByFirstName)), Valid: true},
		}); err != nil {
			log.Fatalf("backdate user for %s: %v", t.Slug, err)
		}

		status := t.Status
		if status == "" {
			status = "approved"
		}
		id, err := q.CreateTeacher(ctx, sqlc.CreateTeacherParams{
			UserID:            uuid.NullUUID{UUID: user.ID, Valid: true},
			Status:            status,
			Verified:          t.Verified,
			Slug:              t.Slug,
			DisplayName:       t.DisplayName,
			Headline:          t.Headline,
			Kind:              sqlc.TeacherKind(t.Kind),
			CountryCode:       t.CountryCode,
			CountryName:       t.CountryName,
			City:              t.City,
			Timezone:          t.Timezone,
			PricePerHourMinor: t.PriceMinor,
			TrialPriceMinor:   nullInt8(t.TrialMinor),
			Currency:          sqlc.CurrencyCodeUZS,
			Rating:            t.Rating,
			ReviewCount:       t.ReviewCount,
			LessonsCompleted:  t.LessonsCompleted,
			StudentCount:      t.StudentCount,
			ResponseTimeHours: t.ResponseTimeHours,
			AcceptingStudents: t.Accepting,
			AvatarUrl:         t.AvatarURL,
			VideoThumbnailUrl: t.VideoThumbnailURL,
			IntroVideoUrl:     t.IntroVideoURL,
			About:             t.About,
			TeachingStyle:     t.TeachingStyle,
		})
		if err != nil {
			log.Fatalf("create %s: %v", t.Slug, err)
		}
		teacherIDBySlug[t.Slug] = id
		teacherPriceBySlug[t.Slug] = t.PriceMinor
		seedLessonTypes(ctx, q, id, t)

		for i, l := range t.Teaches {
			addLang(ctx, q, id, sqlc.LanguageRoleTeaches, l, i)
		}
		for i, l := range t.AlsoSpeaks {
			addLang(ctx, q, id, sqlc.LanguageRoleAlsoSpeaks, l, i)
		}
		for i, tag := range t.Focus {
			if err := q.AddTeacherFocus(ctx, sqlc.AddTeacherFocusParams{
				TeacherID: id, Tag: tag, Position: int32(i),
			}); err != nil {
				log.Fatalf("focus %s/%s: %v", t.Slug, tag, err)
			}
		}
		for i, e := range t.Experience {
			if err := q.AddTeacherExperience(ctx, sqlc.AddTeacherExperienceParams{
				TeacherID: id, Title: e.Title, Org: e.Org, Period: e.Period, Position: int32(i),
			}); err != nil {
				log.Fatalf("experience %s: %v", t.Slug, err)
			}
		}
		for _, sl := range seedAvailability[t.Slug] {
			if err := q.AddAvailabilitySlot(ctx, sqlc.AddAvailabilitySlotParams{
				TeacherID:   id,
				Weekday:     int16(sl.Weekday),
				StartMinute: int32(sl.Start),
				EndMinute:   int32(sl.End),
			}); err != nil {
				log.Fatalf("availability %s: %v", t.Slug, err)
			}
		}
	}

	// Stamp the hand-set rating / review_count as the immutable baseline that
	// review moderation folds visible reviews onto (migration 000012). The
	// migration already does this, but it runs against an empty teachers table.
	if err := q.SeedSnapshotRatingBaselines(ctx); err != nil {
		log.Fatalf("snapshot rating baselines: %v", err)
	}

	// RBAC seed: the system 'superadmin' role (always the full catalog), two
	// non-system example roles, and one admin account holding superadmin.
	// admin@findtutor.local / "admin" — no teacher profile. Documented in README.
	superadmin, err := q.CreateRole(ctx, sqlc.CreateRoleParams{
		Name: rbac.SuperadminRoleName, IsSystem: true,
		Description: "Full access to every admin capability. System role — cannot be edited or deleted.",
	})
	if err != nil {
		log.Fatalf("create superadmin role: %v", err)
	}
	for _, p := range rbac.AllPermissions {
		if err := q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: superadmin.ID, Permission: string(p)}); err != nil {
			log.Fatalf("grant %s to superadmin: %v", p, err)
		}
	}
	seedRole(ctx, q, "support", "Read-only: dashboard, users, teachers, bookings.",
		rbac.PermMetricsView, rbac.PermUsersView, rbac.PermTeachersView, rbac.PermBookingsView)
	seedRole(ctx, q, "moderator", "Teacher and review moderation.",
		rbac.PermTeachersView, rbac.PermTeachersModerate, rbac.PermTeachersVerify, rbac.PermReviewsModerate)

	adminHash, err := auth.HashPassword("admin")
	if err != nil {
		log.Fatalf("hash admin password: %v", err)
	}
	adminUser, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Email:        "admin@findtutor.local",
		PasswordHash: adminHash,
		DisplayName:  "Site Admin",
		Locale:       i18n.Default,
	})
	if err != nil {
		log.Fatalf("create admin user: %v", err)
	}
	if err := q.AssignRoleToUser(ctx, sqlc.AssignRoleToUserParams{UserID: adminUser.ID, RoleID: superadmin.ID}); err != nil {
		log.Fatalf("assign superadmin: %v", err)
	}

	// Sample reviews: booking-less rows (booking_id NULL) that display as a
	// portion of each teacher's review history. These do NOT touch
	// teachers.rating / review_count — the hand-set values stand.
	reviewCount, hiddenReviewCount := 0, 0
	insertSampleReviews := func(src map[string][]seedReview, hidden bool) {
		for slug, rs := range src {
			teacherID, ok := teacherIDBySlug[slug]
			if !ok {
				log.Fatalf("sample reviews have slug %q with no matching teacher", slug)
			}
			for _, r := range rs {
				studentID, ok := userIDByFirstName[r.StudentFirstName]
				if !ok {
					log.Fatalf("sample review %s: no demo account for %q", slug, r.StudentFirstName)
				}
				if err := q.SeedInsertReview(ctx, sqlc.SeedInsertReviewParams{
					TeacherID: teacherID,
					StudentID: studentID,
					Rating:    int16(r.Rating),
					Comment:   r.Comment,
					Hidden:    hidden,
				}); err != nil {
					log.Fatalf("seed review %s/%s: %v", slug, r.StudentFirstName, err)
				}
				if hidden {
					hiddenReviewCount++
				} else {
					reviewCount++
				}
			}
		}
	}
	insertSampleReviews(seedReviews, false)
	insertSampleReviews(seedHiddenReviews, true)

	// Demo bookings + the one open dispute + the settled lessons behind the
	// payout dashboard, so the phase-D/E operator surfaces (/v1/admin/bookings,
	// /v1/admin/disputes, /v1/admin/payouts) all have something to show on a
	// fresh database. Start times are pinned to the hour so repeated seeds stay
	// deterministic within the hour and never collide with each other.
	now := time.Now().UTC().Truncate(time.Hour)
	disputeCount := 0
	// Bookings whose earning is already disbursed. Collected here because the
	// batch row they point at can only be written once its totals are known.
	var paidOut []seedPayoutRow
	for i, b := range seedBookings {
		teacherID, ok := teacherIDBySlug[b.TeacherSlug]
		if !ok {
			log.Fatalf("seedBookings has slug %q with no matching teacher", b.TeacherSlug)
		}
		studentID, ok := userIDByFirstName[b.StudentFirstName]
		if !ok {
			log.Fatalf("seedBookings %s: no demo account for %q", b.TeacherSlug, b.StudentFirstName)
		}
		start := now.Add(time.Duration(b.StartOffsetHours) * time.Hour)
		end := start.Add(time.Duration(b.DurationMinutes) * time.Minute)
		// Same half-up hourly proration the booking service applies.
		priceMinor := (teacherPriceBySlug[b.TeacherSlug]*int64(b.DurationMinutes) + 30) / 60

		bookingID, err := q.SeedInsertBooking(ctx, sqlc.SeedInsertBookingParams{
			TeacherID:       teacherID,
			StudentID:       studentID,
			StartAt:         pgtype.Timestamptz{Time: start, Valid: true},
			EndAt:           pgtype.Timestamptz{Time: end, Valid: true},
			DurationMinutes: int32(b.DurationMinutes),
			Status:          b.Status,
			PriceMinor:      priceMinor,
			Currency:        string(sqlc.CurrencyCodeUZS),
			CreatedAt:       pgtype.Timestamptz{Time: bookedOn(start, i, now), Valid: true},
		})
		if err != nil {
			log.Fatalf("seed booking %s/%s: %v", b.TeacherSlug, b.StudentFirstName, err)
		}

		if b.Dispute != "" {
			if err := q.SeedInsertDispute(ctx, sqlc.SeedInsertDisputeParams{
				BookingID: bookingID,
				RaisedBy:  studentID,
				Reason:    b.Dispute,
			}); err != nil {
				log.Fatalf("seed dispute %s/%s: %v", b.TeacherSlug, b.StudentFirstName, err)
			}
			disputeCount++
		}

		if b.Payout == "" {
			continue
		}
		// A settled lesson: the captured intent behind the money, then the
		// ledger row. "paid" rows wait for the batch id (below); "available"
		// ones stay `held` with a deadline already in the past, which is exactly
		// how the clearing window reads a cleared earning.
		if err := q.SeedInsertCapturedPayment(ctx, sqlc.SeedInsertCapturedPaymentParams{
			BookingID:   bookingID,
			ProviderRef: pgtype.Text{String: "seed_" + bookingID.String()[:8], Valid: true},
			AmountMinor: priceMinor,
			Currency:    string(sqlc.CurrencyCodeUZS),
		}); err != nil {
			log.Fatalf("seed payment %s/%s: %v", b.TeacherSlug, b.StudentFirstName, err)
		}
		row := seedPayoutRow{
			bookingID:      bookingID,
			teacherID:      teacherID,
			amountMinor:    priceMinor,
			state:          "held",
			clearedDaysAgo: b.PayoutClearedDaysAgo,
		}
		if b.Payout == "paid" {
			row.state = "paid"
			paidOut = append(paidOut, row)
			continue
		}
		if err := insertSeedPayoutRow(ctx, q, row, uuid.NullUUID{}); err != nil {
			log.Fatalf("seed payout ledger %s/%s: %v", b.TeacherSlug, b.StudentFirstName, err)
		}
	}

	// The completed payout batch the seeded `paid` rows belong to, run by the
	// demo admin — so GET /v1/admin/payouts has run history and
	// GET /v1/admin/payouts/batches/{id} has a batch to open.
	if len(paidOut) > 0 {
		var total int64
		teachers := map[uuid.UUID]struct{}{}
		for _, r := range paidOut {
			total += r.amountMinor
			teachers[r.teacherID] = struct{}{}
		}
		batch, err := q.InsertPayoutBatch(ctx, sqlc.InsertPayoutBatchParams{
			CreatedBy:    adminUser.ID,
			Status:       "completed",
			TotalMinor:   total,
			Currency:     string(sqlc.CurrencyCodeUZS),
			TeacherCount: int32(len(teachers)),
			LineCount:    int32(len(paidOut)),
		})
		if err != nil {
			log.Fatalf("seed payout batch: %v", err)
		}
		for _, r := range paidOut {
			if err := insertSeedPayoutRow(ctx, q, r, uuid.NullUUID{UUID: batch.ID, Valid: true}); err != nil {
				log.Fatalf("seed paid payout ledger row: %v", err)
			}
		}
	}

	// Every demo account counts as email-verified so the "confirm your email"
	// banner is quiet in the seeded app.
	if err := q.SeedMarkEmailVerified(ctx); err != nil {
		log.Fatalf("mark demo accounts verified: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}

	payoutCount := 0
	for _, b := range seedBookings {
		if b.Payout != "" {
			payoutCount++
		}
	}
	log.Printf("seeded %d teachers (+ %d demo accounts, password %q), 3 roles, 1 admin (admin@findtutor.local / \"admin\", superadmin), %d sample reviews (%d hidden), %d bookings, %d open disputes, %d settled lessons (%d already paid out)",
		len(seedTeachers), len(seedTeachers), demoPassword, reviewCount, hiddenReviewCount, len(seedBookings), disputeCount,
		payoutCount, len(paidOut))
}

// seedPayoutRow is one payout_ledger row the seed is about to write. `paid`
// rows are buffered until the batch that settled them exists.
type seedPayoutRow struct {
	bookingID      uuid.UUID
	teacherID      uuid.UUID
	amountMinor    int64
	state          string
	clearedDaysAgo int
}

func insertSeedPayoutRow(ctx context.Context, q *sqlc.Queries, r seedPayoutRow, batchID uuid.NullUUID) error {
	return q.SeedInsertPayoutLedgerRow(ctx, sqlc.SeedInsertPayoutLedgerRowParams{
		BookingID:      r.bookingID,
		AmountMinor:    r.amountMinor,
		Currency:       string(sqlc.CurrencyCodeUZS),
		State:          r.state,
		ClearedDaysAgo: int32(r.clearedDaysAgo),
		PayoutBatchID:  batchID,
	})
}

// seedRole creates a non-system role with the given permissions.
func seedRole(ctx context.Context, q *sqlc.Queries, name, description string, perms ...rbac.Permission) {
	role, err := q.CreateRole(ctx, sqlc.CreateRoleParams{Name: name, Description: description})
	if err != nil {
		log.Fatalf("create role %s: %v", name, err)
	}
	for _, p := range perms {
		if err := q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: role.ID, Permission: string(p)}); err != nil {
			log.Fatalf("grant %s to %s: %v", p, name, err)
		}
	}
}

// firstName is the first whitespace/hyphen-delimited token of a display name,
// matching demoEmail's derivation ("Kim Min-jun" -> "Kim").
func firstName(displayName string) string {
	if i := strings.IndexAny(displayName, " -"); i > 0 {
		return displayName[:i]
	}
	return displayName
}

func addLang(ctx context.Context, q *sqlc.Queries, id uuid.UUID, role sqlc.LanguageRole, l seedLang, pos int) {
	if err := q.AddTeacherLanguage(ctx, sqlc.AddTeacherLanguageParams{
		TeacherID: id,
		Role:      role,
		Code:      l.Code,
		Name:      l.Name,
		Level:     sqlc.LanguageLevel(l.Level),
		Position:  int32(pos),
	}); err != nil {
		log.Fatalf("language %s: %v", l.Code, err)
	}
}

func nullInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

// signupDate spreads the demo accounts over the last two months, oldest first,
// so the dashboard's sign-ups series has a shape instead of one spike on the
// day the database was seeded. Deterministic in the account's ordinal.
func signupDate(ordinal int) time.Time {
	const spreadDays = 60
	back := spreadDays - (ordinal*5)%spreadDays
	return time.Now().UTC().Add(-time.Duration(back)*24*time.Hour).Truncate(time.Hour)
}

// bookedOn is when a seeded booking was made: a few days before the lesson,
// varied per row and never in the future (a past lesson booked "tomorrow"
// would make the activity chart lie).
func bookedOn(start time.Time, ordinal int, now time.Time) time.Time {
	made := start.Add(-time.Duration(3+(ordinal*7)%18) * 24 * time.Hour)
	if made.After(now) {
		return now
	}
	return made
}

// seedLessonTypes gives a demo teacher the italki-shaped offerings a real one
// would list: the trial they already advertised, a lesson named after what they
// actually teach, and cheaper open-ended conversation practice. Prices hang off
// the teacher's hourly rate so the numbers stay coherent with their card.
func seedLessonTypes(ctx context.Context, q *sqlc.Queries, teacherID uuid.UUID, t seedTeacher) {
	type offering struct {
		title       string
		description string
		isTrial     bool
		position    int32
		// durations priced as a fraction of the hourly rate, rounded to
		// thousands of so'm so nothing reads like a computed number
		prices map[int32]int64
	}

	hourly := t.PriceMinor
	round := func(v int64) int64 { return (v / 500_000) * 500_000 } // to 5,000 so'm

	focus := "Lessons"
	if len(t.Focus) > 0 {
		focus = t.Focus[0]
	}

	offerings := []offering{{
		title:       focus,
		description: "A structured lesson built around " + strings.ToLower(focus) + ". We agree a goal in the first lesson and work through it week by week.",
		position:    0,
		prices: map[int32]int64{
			45: round(hourly * 3 / 4),
			60: hourly,
			90: round(hourly * 3 / 2),
		},
	}, {
		title:       "Conversation practice",
		description: "No slides, no homework — we talk, and I correct what gets in the way. Bring a topic or I will.",
		position:    1,
		prices: map[int32]int64{
			30: round(hourly * 2 / 5),
			60: round(hourly * 4 / 5),
		},
	}}

	if t.TrialMinor != nil {
		offerings = append([]offering{{
			title:       "Trial lesson",
			description: "A short first lesson: we talk, I find your level and we agree a plan. No commitment.",
			isTrial:     true,
			position:    -1,
			prices:      map[int32]int64{30: *t.TrialMinor},
		}}, offerings...)
	}

	for _, o := range offerings {
		row, err := q.CreateLessonType(ctx, sqlc.CreateLessonTypeParams{
			TeacherID:   teacherID,
			Title:       o.title,
			Description: o.description,
			IsTrial:     o.isTrial,
			Position:    o.position,
		})
		if err != nil {
			log.Fatalf("seed lesson type %s/%s: %v", t.Slug, o.title, err)
		}
		for _, minutes := range []int32{30, 45, 60, 90, 120} {
			price, ok := o.prices[minutes]
			if !ok {
				continue
			}
			if err := q.InsertLessonTypePrice(ctx, sqlc.InsertLessonTypePriceParams{
				LessonTypeID:    row.ID,
				DurationMinutes: minutes,
				PriceMinor:      price,
			}); err != nil {
				log.Fatalf("seed lesson price %s/%s/%d: %v", t.Slug, o.title, minutes, err)
			}
		}
	}
}
