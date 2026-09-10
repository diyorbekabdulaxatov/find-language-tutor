package teachers

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// repositoryPostgres implements Repository over the sqlc-generated queries. It
// holds the pool directly (not just sqlc.DBTX) because Create and Update run
// inside a transaction.
type repositoryPostgres struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

// NewPostgresRepository builds a Repository backed by the given pgx pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{pool: pool, q: sqlc.New(pool)}
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
		ID:             row.ID,
		Slug:           row.Slug,
		DisplayName:    row.DisplayName,
		Headline:       row.Headline,
		Kind:           Kind(row.Kind),
		CountryCode:    row.CountryCode,
		CountryName:    row.CountryName,
		City:           row.City,
		Timezone:       row.Timezone,
		Status:         Status(row.Status),
		Verified:       row.Verified,
		ModerationNote: row.ModerationNote,
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
		MeetingURL:        row.MeetingUrl,
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

func (r *repositoryPostgres) RefBySlug(ctx context.Context, slug string) (Ref, error) {
	row, err := r.q.TeacherRefBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Ref{}, ErrNotFound
		}
		return Ref{}, fmt.Errorf("teacher ref by slug: %w", err)
	}
	ref := Ref{ID: row.ID, Slug: row.Slug}
	if row.UserID.Valid {
		ref.OwnerID = row.UserID.UUID
	}
	return ref, nil
}

func (r *repositoryPostgres) RefByOwner(ctx context.Context, ownerID uuid.UUID) (Ref, error) {
	row, err := r.q.TeacherRefByOwner(ctx, uuid.NullUUID{UUID: ownerID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Ref{}, ErrNotFound
		}
		return Ref{}, fmt.Errorf("teacher ref by owner: %w", err)
	}
	ref := Ref{ID: row.ID, Slug: row.Slug}
	if row.UserID.Valid {
		ref.OwnerID = row.UserID.UUID
	}
	return ref, nil
}

func (r *repositoryPostgres) SlugExists(ctx context.Context, slug string) (bool, error) {
	exists, err := r.q.TeacherSlugExists(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("teacher slug exists: %w", err)
	}
	return exists, nil
}

func (r *repositoryPostgres) Create(ctx context.Context, ownerID uuid.UUID, slug string, in ProfileInput) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	qtx := r.q.WithTx(tx)
	id, err := qtx.CreateTeacher(ctx, sqlc.CreateTeacherParams{
		Slug:              slug,
		DisplayName:       in.DisplayName,
		Headline:          in.Headline,
		Kind:              sqlc.TeacherKind(in.Kind),
		CountryCode:       in.CountryCode,
		CountryName:       in.CountryName,
		City:              in.City,
		Timezone:          in.Timezone,
		PricePerHourMinor: in.PricePerHourMinor,
		TrialPriceMinor:   nullInt8(in.TrialPriceMinor),
		Currency:          sqlc.CurrencyCode(in.Currency),
		// Server-controlled aggregates start at zero; the profile is open to
		// students by default.
		Rating:            0,
		ReviewCount:       0,
		LessonsCompleted:  0,
		StudentCount:      0,
		ResponseTimeHours: 0,
		AcceptingStudents: true,
		AvatarUrl:         in.AvatarURL,
		VideoThumbnailUrl: in.VideoThumbnailURL,
		IntroVideoUrl:     in.IntroVideoURL,
		About:             in.About,
		TeachingStyle:     in.TeachingStyle,
		UserID:            uuid.NullUUID{UUID: ownerID, Valid: true},
		// New profiles are not public until an admin approves them, and the
		// verified badge is never self-granted.
		Status:   string(StatusPending),
		Verified: false,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create teacher: %w", err)
	}

	if err := insertChildren(ctx, qtx, id, in); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return id, nil
}

func (r *repositoryPostgres) Update(ctx context.Context, teacherID uuid.UUID, upd ProfileUpdate) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	qtx := r.q.WithTx(tx)
	in := upd.Fields
	if err := qtx.UpdateTeacher(ctx, sqlc.UpdateTeacherParams{
		ID:                teacherID,
		DisplayName:       in.DisplayName,
		Headline:          in.Headline,
		Kind:              sqlc.TeacherKind(in.Kind),
		CountryCode:       in.CountryCode,
		CountryName:       in.CountryName,
		City:              in.City,
		Timezone:          in.Timezone,
		PricePerHourMinor: in.PricePerHourMinor,
		TrialPriceMinor:   nullInt8(in.TrialPriceMinor),
		Currency:          sqlc.CurrencyCode(in.Currency),
		About:             in.About,
		TeachingStyle:     in.TeachingStyle,
		AvatarUrl:         in.AvatarURL,
		VideoThumbnailUrl: in.VideoThumbnailURL,
		IntroVideoUrl:     in.IntroVideoURL,
		MeetingUrl:        in.MeetingURL,
	}); err != nil {
		return fmt.Errorf("update teacher: %w", err)
	}

	if upd.ReplaceLanguages {
		if err := qtx.DeleteTeacherLanguages(ctx, teacherID); err != nil {
			return fmt.Errorf("clear languages: %w", err)
		}
		if err := insertLanguages(ctx, qtx, teacherID, in.Languages); err != nil {
			return err
		}
	}
	if upd.ReplaceFocus {
		if err := qtx.DeleteTeacherFocus(ctx, teacherID); err != nil {
			return fmt.Errorf("clear focus: %w", err)
		}
		if err := insertFocus(ctx, qtx, teacherID, in.Focus); err != nil {
			return err
		}
	}
	if upd.ReplaceExperience {
		if err := qtx.DeleteTeacherExperience(ctx, teacherID); err != nil {
			return fmt.Errorf("clear experience: %w", err)
		}
		if err := insertExperience(ctx, qtx, teacherID, in.Experience); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// insertChildren writes all three child collections for a freshly created
// teacher.
func insertChildren(ctx context.Context, q *sqlc.Queries, teacherID uuid.UUID, in ProfileInput) error {
	if err := insertLanguages(ctx, q, teacherID, in.Languages); err != nil {
		return err
	}
	if err := insertFocus(ctx, q, teacherID, in.Focus); err != nil {
		return err
	}
	return insertExperience(ctx, q, teacherID, in.Experience)
}

func insertLanguages(ctx context.Context, q *sqlc.Queries, teacherID uuid.UUID, langs []LanguageEntry) error {
	for i, l := range langs {
		if err := q.AddTeacherLanguage(ctx, sqlc.AddTeacherLanguageParams{
			TeacherID: teacherID,
			Role:      sqlc.LanguageRole(l.Role),
			Code:      l.Code,
			Name:      l.Name,
			Level:     sqlc.LanguageLevel(l.Level),
			Position:  int32(i),
		}); err != nil {
			return fmt.Errorf("insert language %q: %w", l.Code, err)
		}
	}
	return nil
}

func insertFocus(ctx context.Context, q *sqlc.Queries, teacherID uuid.UUID, tags []string) error {
	for i, tag := range tags {
		if err := q.AddTeacherFocus(ctx, sqlc.AddTeacherFocusParams{
			TeacherID: teacherID,
			Tag:       tag,
			Position:  int32(i),
		}); err != nil {
			return fmt.Errorf("insert focus %q: %w", tag, err)
		}
	}
	return nil
}

func insertExperience(ctx context.Context, q *sqlc.Queries, teacherID uuid.UUID, exp []Experience) error {
	for i, e := range exp {
		if err := q.AddTeacherExperience(ctx, sqlc.AddTeacherExperienceParams{
			TeacherID: teacherID,
			Title:     e.Title,
			Org:       e.Org,
			Period:    e.Period,
			Position:  int32(i),
		}); err != nil {
			return fmt.Errorf("insert experience %q: %w", e.Title, err)
		}
	}
	return nil
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
