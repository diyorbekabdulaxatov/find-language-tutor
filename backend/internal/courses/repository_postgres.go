package courses

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

type repositoryPostgres struct{ q *sqlc.Queries }

// NewPostgresRepository builds a Repository over the given pool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &repositoryPostgres{q: sqlc.New(pool)}
}

func (r *repositoryPostgres) TeacherIDByOwner(ctx context.Context, userID uuid.UUID) (uuid.UUID, bool, error) {
	id, err := r.q.GetTeacherIDByOwner(ctx, uuid.NullUUID{UUID: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("teacher id by owner: %w", err)
	}
	return id, true, nil
}

// --- courses ---

func (r *repositoryPostgres) Create(ctx context.Context, p CreateParams) (Course, error) {
	row, err := r.q.CreateCourse(ctx, sqlc.CreateCourseParams{
		TeacherID:        p.TeacherID,
		Title:            p.Title,
		Subtitle:         p.Subtitle,
		Description:      p.Description,
		PriceAmountMinor: p.PriceAmountMinor,
		PriceCurrency:    sqlc.CurrencyCode(p.PriceCurrency),
	})
	if err != nil {
		return Course{}, fmt.Errorf("create course: %w", err)
	}
	return toCourse(row), nil
}

func (r *repositoryPostgres) ByID(ctx context.Context, id uuid.UUID) (Course, error) {
	row, err := r.q.GetCourse(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Course{}, ErrNotFound
		}
		return Course{}, fmt.Errorf("get course: %w", err)
	}
	return toCourse(row), nil
}

func (r *repositoryPostgres) List(ctx context.Context, teacherID uuid.UUID, q ListQuery) ([]Course, int, error) {
	statusArg := nullText(string(q.Status))

	rows, err := r.q.ListTeacherCourses(ctx, sqlc.ListTeacherCoursesParams{
		TeacherID:       teacherID,
		Status:          statusArg,
		IncludeArchived: q.IncludeArchived,
		PageLimit:       int32(q.PageSize),
		PageOffset:      int32((q.Page - 1) * q.PageSize),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list courses: %w", err)
	}
	total, err := r.q.CountTeacherCourses(ctx, sqlc.CountTeacherCoursesParams{
		TeacherID:       teacherID,
		Status:          statusArg,
		IncludeArchived: q.IncludeArchived,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count courses: %w", err)
	}

	out := make([]Course, len(rows))
	for i, row := range rows {
		out[i] = toCourse(row)
	}
	return out, int(total), nil
}

func (r *repositoryPostgres) Update(ctx context.Context, id uuid.UUID, p UpdateParams) (Course, error) {
	row, err := r.q.UpdateCourse(ctx, sqlc.UpdateCourseParams{
		ID: id, Title: p.Title, Subtitle: p.Subtitle, Description: p.Description,
		CoverAssetID:     toNullUUID(p.CoverAssetID),
		PriceAmountMinor: p.PriceAmountMinor,
		PriceCurrency:    sqlc.CurrencyCode(p.PriceCurrency),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Course{}, ErrNotFound
		}
		return Course{}, fmt.Errorf("update course: %w", err)
	}
	return toCourse(row), nil
}

func (r *repositoryPostgres) SetStatus(ctx context.Context, id uuid.UUID, status Status) (Course, error) {
	row, err := r.q.SetCourseStatus(ctx, sqlc.SetCourseStatusParams{ID: id, Status: string(status)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Course{}, ErrNotFound
		}
		return Course{}, fmt.Errorf("set course status: %w", err)
	}
	return toCourse(row), nil
}

func (r *repositoryPostgres) SetArchived(ctx context.Context, id uuid.UUID, archived bool) (Course, error) {
	row, err := r.q.SetCourseArchived(ctx, sqlc.SetCourseArchivedParams{ID: id, Archived: archived})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Course{}, ErrNotFound
		}
		return Course{}, fmt.Errorf("set course archived: %w", err)
	}
	return toCourse(row), nil
}

func (r *repositoryPostgres) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteCourse(ctx, id); err != nil {
		return fmt.Errorf("delete course: %w", err)
	}
	return nil
}

// --- sections ---

func (r *repositoryPostgres) AddSection(ctx context.Context, courseID uuid.UUID, title string) (Section, error) {
	row, err := r.q.AddCourseSection(ctx, sqlc.AddCourseSectionParams{CourseID: courseID, Title: title})
	if err != nil {
		return Section{}, fmt.Errorf("add course section: %w", err)
	}
	return toSection(row), nil
}

func (r *repositoryPostgres) SectionByID(ctx context.Context, id uuid.UUID) (Section, error) {
	row, err := r.q.GetCourseSection(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Section{}, ErrSectionNotFound
		}
		return Section{}, fmt.Errorf("get course section: %w", err)
	}
	return toSection(row), nil
}

func (r *repositoryPostgres) ListSections(ctx context.Context, courseID uuid.UUID) ([]Section, error) {
	rows, err := r.q.ListCourseSections(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list course sections: %w", err)
	}
	out := make([]Section, len(rows))
	for i, row := range rows {
		out[i] = toSection(row)
	}
	return out, nil
}

func (r *repositoryPostgres) RenameSection(ctx context.Context, id uuid.UUID, title string) (Section, error) {
	row, err := r.q.RenameCourseSection(ctx, sqlc.RenameCourseSectionParams{ID: id, Title: title})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Section{}, ErrSectionNotFound
		}
		return Section{}, fmt.Errorf("rename course section: %w", err)
	}
	return toSection(row), nil
}

func (r *repositoryPostgres) DeleteSection(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteCourseSection(ctx, id); err != nil {
		return fmt.Errorf("delete course section: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) ReorderSections(ctx context.Context, courseID uuid.UUID, orderedIDs []uuid.UUID) error {
	n, err := r.q.ReorderCourseSections(ctx, sqlc.ReorderCourseSectionsParams{SectionIds: orderedIDs, CourseID: courseID})
	if err != nil {
		return fmt.Errorf("reorder course sections: %w", err)
	}
	if int(n) != len(orderedIDs) {
		return fmt.Errorf("reorder course sections: expected %d rows updated, got %d", len(orderedIDs), n)
	}
	return nil
}

// --- items ---

func (r *repositoryPostgres) AddItem(ctx context.Context, p AddItemParams) (Item, error) {
	row, err := r.q.AddCourseItem(ctx, sqlc.AddCourseItemParams{
		SectionID: p.SectionID, Kind: string(p.Kind), Title: p.Title,
		VideoAssetID: toNullUUID(p.VideoAssetID), ResourceID: toNullUUID(p.ResourceID),
	})
	if err != nil {
		return Item{}, fmt.Errorf("add course item: %w", err)
	}
	return toItem(row), nil
}

func (r *repositoryPostgres) ItemByID(ctx context.Context, id uuid.UUID) (Item, error) {
	row, err := r.q.GetCourseItem(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrItemNotFound
		}
		return Item{}, fmt.Errorf("get course item: %w", err)
	}
	return toItem(row), nil
}

func (r *repositoryPostgres) ListItems(ctx context.Context, sectionID uuid.UUID) ([]Item, error) {
	rows, err := r.q.ListCourseItems(ctx, sectionID)
	if err != nil {
		return nil, fmt.Errorf("list course items: %w", err)
	}
	out := make([]Item, len(rows))
	for i, row := range rows {
		out[i] = toItem(row)
	}
	return out, nil
}

func (r *repositoryPostgres) ListItemsByCourse(ctx context.Context, courseID uuid.UUID) ([]Item, error) {
	rows, err := r.q.ListCourseItemsByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list course items by course: %w", err)
	}
	out := make([]Item, len(rows))
	for i, row := range rows {
		out[i] = toItem(row)
	}
	return out, nil
}

func (r *repositoryPostgres) RenameItem(ctx context.Context, id uuid.UUID, title string) (Item, error) {
	row, err := r.q.RenameCourseItem(ctx, sqlc.RenameCourseItemParams{ID: id, Title: title})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrItemNotFound
		}
		return Item{}, fmt.Errorf("rename course item: %w", err)
	}
	return toItem(row), nil
}

func (r *repositoryPostgres) DeleteItem(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteCourseItem(ctx, id); err != nil {
		return fmt.Errorf("delete course item: %w", err)
	}
	return nil
}

func (r *repositoryPostgres) ReorderItems(ctx context.Context, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	n, err := r.q.ReorderCourseItems(ctx, sqlc.ReorderCourseItemsParams{ItemIds: orderedIDs, SectionID: sectionID})
	if err != nil {
		return fmt.Errorf("reorder course items: %w", err)
	}
	if int(n) != len(orderedIDs) {
		return fmt.Errorf("reorder course items: expected %d rows updated, got %d", len(orderedIDs), n)
	}
	return nil
}

// --- mapping helpers ---

func toCourse(row sqlc.Course) Course {
	c := Course{
		ID: row.ID, TeacherID: row.TeacherID, Title: row.Title, Subtitle: row.Subtitle, Description: row.Description,
		PriceAmountMinor: row.PriceAmountMinor, PriceCurrency: string(row.PriceCurrency),
		Status: Status(row.Status), EverPublished: row.EverPublished,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
	if row.CoverAssetID.Valid {
		v := row.CoverAssetID.UUID
		c.CoverAssetID = &v
	}
	if row.ArchivedAt.Valid {
		t := row.ArchivedAt.Time.UTC()
		c.ArchivedAt = &t
	}
	return c
}

func toSection(row sqlc.CourseSection) Section {
	return Section{
		ID: row.ID, CourseID: row.CourseID, Title: row.Title, Position: int(row.Position),
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func toItem(row sqlc.CourseItem) Item {
	it := Item{
		ID: row.ID, SectionID: row.SectionID, Kind: ItemKind(row.Kind), Title: row.Title,
		Position: int(row.Position), CreatedAt: row.CreatedAt.Time.UTC(),
	}
	if row.VideoAssetID.Valid {
		v := row.VideoAssetID.UUID
		it.VideoAssetID = &v
	}
	if row.ResourceID.Valid {
		v := row.ResourceID.UUID
		it.ResourceID = &v
	}
	return it
}

func toNullUUID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

func nullText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
