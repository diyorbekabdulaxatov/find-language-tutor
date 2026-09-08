// Command seed loads the demo teacher catalog. It is idempotent: it deletes all
// teachers and re-inserts them inside one transaction.
//
//	go run ./cmd/seed
package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db"
	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
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

	// teachers.user_id references users, so clear teachers before users.
	if err := q.DeleteAllTeachers(ctx); err != nil {
		log.Fatalf("clear teachers: %v", err)
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

	for _, t := range seedTeachers {
		user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
			Email:        demoEmail(t.DisplayName),
			PasswordHash: passwordHash,
			DisplayName:  t.DisplayName,
		})
		if err != nil {
			log.Fatalf("create user for %s: %v", t.Slug, err)
		}

		id, err := q.CreateTeacher(ctx, sqlc.CreateTeacherParams{
			UserID:            uuid.NullUUID{UUID: user.ID, Valid: true},
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

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}

	log.Printf("seeded %d teachers (+ %d demo accounts, password %q)", len(seedTeachers), len(seedTeachers), demoPassword)
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
