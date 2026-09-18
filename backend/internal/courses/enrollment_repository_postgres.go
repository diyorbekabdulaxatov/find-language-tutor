package courses

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// pgUniqueViolation is SQLSTATE 23505 — raised by course_enrollments' UNIQUE
// (course_id, student_id) on a duplicate enrollment attempt. Mapped by
// EnsureEnrollment to load-and-return the existing row — race-safe, never a
// check-then-insert.
const pgUniqueViolation = "23505"

// --- catalog ---

func (r *repositoryPostgres) TeacherOwnerID(ctx context.Context, teacherID uuid.UUID) (uuid.UUID, bool, error) {
	id, err := r.q.GetTeacherOwnerID(ctx, teacherID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("get teacher owner id: %w", err)
	}
	if !id.Valid {
		return uuid.Nil, false, nil
	}
	return id.UUID, true, nil
}

func (r *repositoryPostgres) TeacherSummaryByID(ctx context.Context, teacherID uuid.UUID) (TeacherSummary, error) {
	row, err := r.q.GetTeacherSummary(ctx, teacherID)
	if err != nil {
		return TeacherSummary{}, fmt.Errorf("get teacher summary: %w", err)
	}
	return TeacherSummary{ID: row.ID, DisplayName: row.DisplayName, Slug: row.Slug, Approved: row.Approved}, nil
}

func (r *repositoryPostgres) CatalogList(ctx context.Context, q CatalogQuery) ([]CatalogEntry, int, error) {
	qArg := nullText(q.Q)
	priceArg := pgtype.Int8{}
	if q.MaxPriceMinor != nil {
		priceArg = pgtype.Int8{Int64: *q.MaxPriceMinor, Valid: true}
	}

	rows, err := r.q.CatalogListCourses(ctx, sqlc.CatalogListCoursesParams{
		Q: qArg, MaxPriceMinor: priceArg, Sort: q.Sort,
		PageLimit: int32(q.PageSize), PageOffset: int32((q.Page - 1) * q.PageSize),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog list courses: %w", err)
	}
	total, err := r.q.CountCatalogCourses(ctx, sqlc.CountCatalogCoursesParams{Q: qArg, MaxPriceMinor: priceArg})
	if err != nil {
		return nil, 0, fmt.Errorf("count catalog courses: %w", err)
	}

	out := make([]CatalogEntry, len(rows))
	for i, row := range rows {
		out[i] = CatalogEntry{
			Course: Course{
				ID: row.ID, TeacherID: row.TeacherID, Title: row.Title, Subtitle: row.Subtitle, Description: row.Description,
				PriceAmountMinor: row.PriceAmountMinor, PriceCurrency: string(row.PriceCurrency),
				Status: Status(row.Status), EverPublished: row.EverPublished,
				Rating: float64(row.Rating), ReviewCount: int(row.ReviewCount),
				CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
			},
			Teacher:      TeacherSummary{ID: row.TeacherID, DisplayName: row.TeacherDisplayName, Slug: row.TeacherSlug},
			SectionCount:         int(row.SectionCount),
			ItemCount:            int(row.ItemCount),
			TotalDurationSeconds: row.TotalDurationSeconds,
			HasPreview:           row.HasPreview,
		}
		if row.CoverAssetID.Valid {
			v := row.CoverAssetID.UUID
			out[i].Course.CoverAssetID = &v
		}
		if row.ArchivedAt.Valid {
			t := row.ArchivedAt.Time.UTC()
			out[i].Course.ArchivedAt = &t
		}
	}
	return out, int(total), nil
}

// PreviewItem resolves an item plus the storefront state of the course it
// belongs to, for the public preview stream (phase D1). An item id that isn't
// part of courseID simply doesn't match the query's join, so it reads as
// ErrItemNotFound like any unknown id.
func (r *repositoryPostgres) PreviewItem(ctx context.Context, courseID, itemID uuid.UUID) (PreviewRef, error) {
	row, err := r.q.GetPreviewItem(ctx, sqlc.GetPreviewItemParams{ItemID: itemID, CourseID: courseID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PreviewRef{}, ErrItemNotFound
		}
		return PreviewRef{}, fmt.Errorf("get preview item: %w", err)
	}
	ref := PreviewRef{
		ItemID: row.ID, CourseID: row.CourseID, Title: row.Title,
		IsPreview: row.IsPreview, DurationSeconds: int(row.DurationSeconds),
		OnStorefront: row.OnStorefront.Valid && row.OnStorefront.Bool,
	}
	if row.VideoAssetID.Valid {
		v := row.VideoAssetID.UUID
		ref.VideoAssetID = &v
	}
	return ref, nil
}

// --- enrollment ---

func (r *repositoryPostgres) EnsureEnrollment(ctx context.Context, courseID, studentID uuid.UUID, source EnrollmentSource, amountPaidMinor int64, currency string) (Enrollment, error) {
	row, err := r.q.InsertEnrollment(ctx, sqlc.InsertEnrollmentParams{
		CourseID: courseID, StudentID: studentID, Source: string(source),
		AmountPaidMinor: amountPaidMinor, Currency: sqlc.CurrencyCode(currency),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			existing, eOk, eErr := r.EnrollmentByCourseAndStudent(ctx, courseID, studentID)
			if eErr != nil {
				return Enrollment{}, eErr
			}
			if !eOk {
				return Enrollment{}, fmt.Errorf("ensure enrollment: unique violation but no existing row for course %s student %s", courseID, studentID)
			}
			return existing, nil
		}
		return Enrollment{}, fmt.Errorf("insert enrollment: %w", err)
	}
	return toEnrollment(row), nil
}

func (r *repositoryPostgres) EnrollmentByCourseAndStudent(ctx context.Context, courseID, studentID uuid.UUID) (Enrollment, bool, error) {
	row, err := r.q.GetEnrollmentByCourseAndStudent(ctx, sqlc.GetEnrollmentByCourseAndStudentParams{CourseID: courseID, StudentID: studentID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Enrollment{}, false, nil
		}
		return Enrollment{}, false, fmt.Errorf("get enrollment by course and student: %w", err)
	}
	return toEnrollment(row), true, nil
}

func (r *repositoryPostgres) EnrollmentParticipants(ctx context.Context, enrollmentID uuid.UUID) (uuid.UUID, uuid.UUID, bool, error) {
	row, err := r.q.GetEnrollmentParticipants(ctx, enrollmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, uuid.Nil, false, nil
		}
		return uuid.Nil, uuid.Nil, false, fmt.Errorf("get enrollment participants: %w", err)
	}
	if !row.TeacherOwnerID.Valid {
		return uuid.Nil, uuid.Nil, false, nil
	}
	return row.TeacherOwnerID.UUID, row.StudentID, true, nil
}

func (r *repositoryPostgres) ListEnrollmentsForStudent(ctx context.Context, studentID uuid.UUID) ([]EnrollmentSummary, error) {
	rows, err := r.q.ListEnrollmentsForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("list enrollments for student: %w", err)
	}
	out := make([]EnrollmentSummary, len(rows))
	for i, row := range rows {
		c := Course{
			ID: row.CourseID, TeacherID: row.CourseTeacherID, Title: row.CourseTitle, Subtitle: row.CourseSubtitle,
			Description: row.CourseDescription, PriceAmountMinor: row.CoursePriceAmountMinor,
			PriceCurrency: string(row.CoursePriceCurrency), Status: Status(row.CourseStatus),
			EverPublished: row.CourseEverPublished,
			CreatedAt:     row.CourseCreatedAt.Time.UTC(), UpdatedAt: row.CourseUpdatedAt.Time.UTC(),
		}
		if row.CourseCoverAssetID.Valid {
			v := row.CourseCoverAssetID.UUID
			c.CoverAssetID = &v
		}
		if row.CourseArchivedAt.Valid {
			t := row.CourseArchivedAt.Time.UTC()
			c.ArchivedAt = &t
		}
		out[i] = EnrollmentSummary{
			Enrollment: Enrollment{
				ID: row.ID, CourseID: row.CourseID, StudentID: row.StudentID, Source: EnrollmentSource(row.Source),
				AmountPaidMinor: row.AmountPaidMinor, Currency: string(row.Currency), CreatedAt: row.CreatedAt.Time.UTC(),
			},
			Course: c, TotalItems: int(row.TotalItems), CompletedItems: int(row.CompletedItems),
		}
	}
	return out, nil
}

func (r *repositoryPostgres) EnrollmentGrantsResource(ctx context.Context, enrollmentID, resourceID uuid.UUID) (bool, error) {
	ok, err := r.q.EnrollmentGrantsResource(ctx, sqlc.EnrollmentGrantsResourceParams{
		EnrollmentID: enrollmentID, ResourceID: toNullUUID(&resourceID),
	})
	if err != nil {
		return false, fmt.Errorf("enrollment grants resource: %w", err)
	}
	return ok, nil
}

func (r *repositoryPostgres) StudentResourceAccess(ctx context.Context, resourceID, studentID uuid.UUID) (bool, error) {
	ok, err := r.q.StudentHasResourceAccess(ctx, sqlc.StudentHasResourceAccessParams{
		StudentID: studentID, ResourceID: toNullUUID(&resourceID),
	})
	if err != nil {
		return false, fmt.Errorf("student has resource access: %w", err)
	}
	return ok, nil
}

func (r *repositoryPostgres) StudentHasVideoAccess(ctx context.Context, fileAssetID, studentID uuid.UUID) (bool, error) {
	ok, err := r.q.StudentHasVideoAccess(ctx, sqlc.StudentHasVideoAccessParams{
		StudentID: studentID, FileAssetID: toNullUUID(&fileAssetID),
	})
	if err != nil {
		return false, fmt.Errorf("student has video access: %w", err)
	}
	return ok, nil
}

func (r *repositoryPostgres) ItemForEnrollmentResource(ctx context.Context, enrollmentID, resourceID uuid.UUID) (Item, bool, error) {
	row, err := r.q.GetCourseItemForEnrollmentResource(ctx, sqlc.GetCourseItemForEnrollmentResourceParams{
		EnrollmentID: enrollmentID, ResourceID: toNullUUID(&resourceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, false, nil
		}
		return Item{}, false, fmt.Errorf("get course item for enrollment resource: %w", err)
	}
	return toItem(row), true, nil
}

// --- progress ---

func (r *repositoryPostgres) UpsertItemProgress(ctx context.Context, enrollmentID, itemID uuid.UUID, positionSeconds *int, completed *bool) (ItemProgress, error) {
	row, err := r.q.UpsertItemProgress(ctx, sqlc.UpsertItemProgressParams{
		EnrollmentID: enrollmentID, ItemID: itemID,
		VideoPositionSeconds: toInt4(positionSeconds), Completed: toBool(completed),
	})
	if err != nil {
		return ItemProgress{}, fmt.Errorf("upsert item progress: %w", err)
	}
	return toItemProgress(row), nil
}

func (r *repositoryPostgres) CompleteItemProgress(ctx context.Context, enrollmentID, itemID uuid.UUID, completedAt time.Time) (ItemProgress, error) {
	row, err := r.q.CompleteItemProgress(ctx, sqlc.CompleteItemProgressParams{
		EnrollmentID: enrollmentID, ItemID: itemID, CompletedAt: pgtype.Timestamptz{Time: completedAt.UTC(), Valid: true},
	})
	if err != nil {
		return ItemProgress{}, fmt.Errorf("complete item progress: %w", err)
	}
	return toItemProgress(row), nil
}

func (r *repositoryPostgres) ListItemProgressForEnrollment(ctx context.Context, enrollmentID uuid.UUID) ([]ItemProgress, error) {
	rows, err := r.q.ListItemProgressForEnrollment(ctx, enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("list item progress for enrollment: %w", err)
	}
	out := make([]ItemProgress, len(rows))
	for i, row := range rows {
		out[i] = toItemProgress(row)
	}
	return out, nil
}

// --- mapping helpers ---

func toEnrollment(row sqlc.CourseEnrollment) Enrollment {
	return Enrollment{
		ID: row.ID, CourseID: row.CourseID, StudentID: row.StudentID, Source: EnrollmentSource(row.Source),
		AmountPaidMinor: row.AmountPaidMinor, Currency: string(row.Currency), CreatedAt: row.CreatedAt.Time.UTC(),
	}
}

func toItemProgress(row sqlc.CourseItemProgress) ItemProgress {
	p := ItemProgress{
		ID: row.ID, EnrollmentID: row.EnrollmentID, ItemID: row.ItemID, Status: ItemStatus(row.Status),
		VideoPositionSeconds: int(row.VideoPositionSeconds), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time.UTC()
		p.CompletedAt = &t
	}
	return p
}

func toInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func toBool(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}
