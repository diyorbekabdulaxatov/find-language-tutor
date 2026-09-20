//go:build integration

package dbtest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// The catalog's "from" price is derived in SQL (a LEFT JOIN LATERAL over the
// live, non-trial lesson types, falling back to the hourly rate), so no unit
// test can reach it — the first version of that query was accepted by the Go
// compiler and by sqlc, then failed against a real server because an output
// alias is not visible inside a CASE in ORDER BY. These tests run the real
// repository against a real database.
func TestTeacherSearch_FromPriceIsTheCheapestBookableLesson(t *testing.T) {
	pool := Pool(t)
	repo := teachers.NewPostgresRepository(pool)

	// hourly 6,000,000; a 4,000,000 regular lesson; a 1,000,000 trial that must
	// not count; an archived 500,000 lesson that must not count either.
	id := approvedTeacher(t, pool, "priced", 6_000_000)
	lessonType(t, pool, id, "Regular", false, false, map[int]int64{60: 5_000_000, 30: 4_000_000})
	lessonType(t, pool, id, "Trial", true, false, map[int]int64{30: 1_000_000})
	lessonType(t, pool, id, "Retired", false, true, map[int]int64{30: 500_000})

	// No offerings at all: the hourly rate stands in.
	approvedTeacher(t, pool, "bare", 7_000_000)

	got := bySlug(t, repo, teachers.ListParams{Sort: teachers.SortPriceAsc, Page: 1, PageSize: 50})

	if p := got["priced"].FromPrice.AmountMinor; p != 4_000_000 {
		t.Errorf("from price = %d, want 4000000 (cheapest live non-trial lesson)", p)
	}
	if p := got["priced"].PricePerHour.AmountMinor; p != 6_000_000 {
		t.Errorf("hourly rate = %d, want it left alone at 6000000", p)
	}
	if p := got["bare"].FromPrice.AmountMinor; p != 7_000_000 {
		t.Errorf("no offerings: from price = %d, want the hourly 7000000", p)
	}
	if got["priced"].FromPrice.Currency == "" || got["bare"].FromPrice.Currency == "" {
		t.Error("from price must carry the currency")
	}
}

func TestTeacherSearch_PriceFilterAndSortUseTheDerivedPrice(t *testing.T) {
	pool := Pool(t)
	ctx := context.Background()
	repo := teachers.NewPostgresRepository(pool)

	// Both teachers are expensive by the hour but cheap per lesson, so a filter
	// or sort still running on price_per_hour_minor would get this wrong.
	cheap := approvedTeacher(t, pool, "cheap", 9_000_000)
	lessonType(t, pool, cheap, "Regular", false, false, map[int]int64{30: 1_000_000})
	dear := approvedTeacher(t, pool, "dear", 8_000_000)
	lessonType(t, pool, dear, "Regular", false, false, map[int]int64{30: 3_000_000})

	asc, total, err := repo.List(ctx, teachers.ListParams{Sort: teachers.SortPriceAsc, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if asc[0].Slug != "cheap" || asc[1].Slug != "dear" {
		t.Errorf("price_asc order = %s, %s; want cheap, dear", asc[0].Slug, asc[1].Slug)
	}

	desc, _, err := repo.List(ctx, teachers.ListParams{Sort: teachers.SortPriceDesc, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if desc[0].Slug != "dear" {
		t.Errorf("price_desc first = %s, want dear", desc[0].Slug)
	}

	// A 2,000,000 ceiling keeps only the teacher whose cheapest lesson is under
	// it — by the hourly rate neither would have matched.
	ceiling := int64(2_000_000)
	filtered, count, err := repo.List(ctx, teachers.ListParams{MaxPriceMinor: &ceiling, Sort: teachers.SortPriceAsc, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(filtered) != 1 || filtered[0].Slug != "cheap" {
		t.Fatalf("max_price filter: count=%d rows=%d, want just \"cheap\"", count, len(filtered))
	}
}

func approvedTeacher(t *testing.T, pool *pgxpool.Pool, slug string, hourlyMinor int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO teachers (slug, display_name, headline, kind, country_code, country_name, city,
		                      timezone, price_per_hour_minor, currency, status)
		VALUES ($1, $1, 'h', 'community', 'UZ', 'Uzbekistan', 'Tashkent', 'Asia/Tashkent', $2, 'UZS', 'approved')
		RETURNING id`, slug, hourlyMinor).Scan(&id); err != nil {
		t.Fatalf("insert teacher %s: %v", slug, err)
	}
	return id
}

func lessonType(t *testing.T, pool *pgxpool.Pool, teacherID uuid.UUID, title string, isTrial, archived bool, prices map[int]int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO lesson_types (teacher_id, title, is_trial, archived)
		VALUES ($1, $2, $3, $4) RETURNING id`, teacherID, title, isTrial, archived).Scan(&id); err != nil {
		t.Fatalf("insert lesson type %s: %v", title, err)
	}
	for minutes, minor := range prices {
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO lesson_type_prices (lesson_type_id, duration_minutes, price_minor)
			VALUES ($1, $2, $3)`, id, minutes, minor); err != nil {
			t.Fatalf("insert price %d: %v", minutes, err)
		}
	}
	return id
}

func bySlug(t *testing.T, repo teachers.Repository, p teachers.ListParams) map[string]teachers.Teacher {
	t.Helper()
	rows, _, err := repo.List(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]teachers.Teacher, len(rows))
	for _, r := range rows {
		out[r.Slug] = r
	}
	return out
}
