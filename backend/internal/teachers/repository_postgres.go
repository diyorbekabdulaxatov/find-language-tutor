package teachers

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// repositoryPostgres implements Repository over the sqlc-generated queries.
type repositoryPostgres struct {
	q *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(db sqlc.DBTX) Repository {
	return &repositoryPostgres{q: sqlc.New(db)}
}

func (r *repositoryPostgres) List(ctx context.Context, p ListParams) ([]Teacher, int, error) {
	arg := sqlc.ListTeachersParams{
		Kind:          nullKind(p.Kind),
		MaxPriceMinor: nullInt8(p.MaxPriceMinor),
		Language:      nullText(p.Language),
		Q:             nullText(p.Q),
		Sort:          string(p.Sort),
		PageOffset:    p.offset(),
		PageLimit:     int32(p.PageSize),
	}

	rows, err := r.q.ListTeachers(ctx, arg)
	if err != nil {
		return nil, 0, fmt.Errorf("list teachers: %w", err)
	}

	total, err := r.q.CountTeachers(ctx, sqlc.CountTeachersParams{
		Kind:          arg.Kind,
		MaxPriceMinor: arg.MaxPriceMinor,
		Language:      arg.Language,
		Q:             arg.Q,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count teachers: %w", err)
	}

	teachers := make([]Teacher, len(rows))
	ids := make([]uuid.UUID, len(rows))
	byID := make(map[uuid.UUID]*Teacher, len(rows))
	for i := range rows {
		teachers[i] = rowToTeacher(rows[i])
		ids[i] = rows[i].ID
		byID[rows[i].ID] = &teachers[i]
	}

	// The list view needs languages and focus tags, but not experience.
	if err := r.attachLanguages(ctx, ids, byID); err != nil {
		return nil, 0, err
	}
	if err := r.attachFocus(ctx, ids, byID); err != nil {
		return nil, 0, err
	}

	return teachers, int(total), nil
}

func (r *repositoryPostgres) GetBySlug(ctx context.Context, slug string) (*Teacher, error) {
	row, err := r.q.GetTeacherBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get teacher: %w", err)
	}

	t := rowToTeacher(row)
	ids := []uuid.UUID{t.ID}
	byID := map[uuid.UUID]*Teacher{t.ID: &t}

	if err := r.attachLanguages(ctx, ids, byID); err != nil {
		return nil, err
	}
	if err := r.attachFocus(ctx, ids, byID); err != nil {
		return nil, err
	}
	if err := r.attachExperience(ctx, ids, byID); err != nil {
		return nil, err
	}

	return &t, nil
}

func (r *repositoryPostgres) LanguageFacets(ctx context.Context) ([]LanguageFacet, error) {
	rows, err := r.q.LanguageFacets(ctx)
	if err != nil {
		return nil, fmt.Errorf("language facets: %w", err)
	}
	out := make([]LanguageFacet, len(rows))
	for i, row := range rows {
		out[i] = LanguageFacet{Code: row.Code, Name: row.Name, Count: int(row.TeacherCount)}
	}
	return out, nil
}

func (r *repositoryPostgres) Slugs(ctx context.Context) ([]string, error) {
	slugs, err := r.q.ListTeacherSlugs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list slugs: %w", err)
	}
	return slugs, nil
}

func (r *repositoryPostgres) attachLanguages(ctx context.Context, ids []uuid.UUID, byID map[uuid.UUID]*Teacher) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.q.ListLanguagesForTeachers(ctx, ids)
	if err != nil {
		return fmt.Errorf("list languages: %w", err)
	}
	for _, row := range rows {
		t := byID[row.TeacherID]
		if t == nil {
			continue
		}
		lang := Language{Code: row.Code, Name: row.Name, Level: Level(row.Level)}
		if row.Role == sqlc.LanguageRoleTeaches {
			t.Teaches = append(t.Teaches, lang)
		} else {
			t.AlsoSpeaks = append(t.AlsoSpeaks, lang)
		}
	}
	return nil
}

func (r *repositoryPostgres) attachFocus(ctx context.Context, ids []uuid.UUID, byID map[uuid.UUID]*Teacher) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.q.ListFocusForTeachers(ctx, ids)
	if err != nil {
		return fmt.Errorf("list focus: %w", err)
	}
	for _, row := range rows {
		if t := byID[row.TeacherID]; t != nil {
			t.Focus = append(t.Focus, row.Tag)
		}
	}
	return nil
}

func (r *repositoryPostgres) attachExperience(ctx context.Context, ids []uuid.UUID, byID map[uuid.UUID]*Teacher) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.q.ListExperienceForTeachers(ctx, ids)
	if err != nil {
		return fmt.Errorf("list experience: %w", err)
	}
	for _, row := range rows {
		if t := byID[row.TeacherID]; t != nil {
			t.Experience = append(t.Experience, Experience{
				Title: row.Title, Org: row.Org, Period: row.Period,
			})
		}
	}
	return nil
}

// rowToTeacher maps a sqlc teacher row onto the domain aggregate (without
// children — the caller attaches those).
func rowToTeacher(row sqlc.Teacher) Teacher {
	t := Teacher{
		ID:          row.ID,
		Slug:        row.Slug,
		DisplayName: row.DisplayName,
		Headline:    row.Headline,
		Kind:        Kind(row.Kind),
		CountryCode: row.CountryCode,
		CountryName: row.CountryName,
		City:        row.City,
		Timezone:    row.Timezone,
		PricePerHour: Money{
			AmountMinor: row.PricePerHourMinor,
			Currency:    Currency(row.Currency),
		},
		// stored as float4; round back to the one decimal it represents so the
		// JSON reads 4.7, not 4.699999809265137.
		Rating:            math.Round(float64(row.Rating)*10) / 10,
		ReviewCount:       int(row.ReviewCount),
		LessonsCompleted:  int(row.LessonsCompleted),
		StudentCount:      int(row.StudentCount),
		ResponseTimeHours: int(row.ResponseTimeHours),
		AcceptingStudents: row.AcceptingStudents,
		AvatarURL:         row.AvatarUrl,
		VideoThumbnailURL: row.VideoThumbnailUrl,
		IntroVideoURL:     row.IntroVideoUrl,
		About:             row.About,
		TeachingStyle:     row.TeachingStyle,
	}
	if row.TrialPriceMinor.Valid {
		t.TrialPrice = &Money{
			AmountMinor: row.TrialPriceMinor.Int64,
			Currency:    Currency(row.Currency),
		}
	}
	return t
}

func nullText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func nullInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func nullKind(k Kind) sqlc.NullTeacherKind {
	if k == "" {
		return sqlc.NullTeacherKind{}
	}
	return sqlc.NullTeacherKind{TeacherKind: sqlc.TeacherKind(k), Valid: true}
}
