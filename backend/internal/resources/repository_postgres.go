package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db/sqlc"
)

// pgUniqueViolation is SQLSTATE 23505 — raised by booking_resources' UNIQUE
// (booking_id, resource_id) when the same resource is attached to the same
// booking twice. Mapped to ErrAlreadyAttached — race-safe, never a
// check-then-insert.
const pgUniqueViolation = "23505"

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

func (r *repositoryPostgres) Create(ctx context.Context, p CreateParams) (Resource, error) {
	blob, err := marshalContent(p.Content)
	if err != nil {
		return Resource{}, err
	}
	row, err := r.q.CreateResource(ctx, sqlc.CreateResourceParams{
		TeacherID:    p.TeacherID,
		Type:         string(p.Type),
		Title:        p.Title,
		Instructions: p.Instructions,
		Content:      blob,
		Status:       string(p.Status),
	})
	if err != nil {
		return Resource{}, fmt.Errorf("create resource: %w", err)
	}
	return toResource(row)
}

func (r *repositoryPostgres) ByID(ctx context.Context, id uuid.UUID) (Resource, error) {
	row, err := r.q.GetResource(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Resource{}, ErrNotFound
		}
		return Resource{}, fmt.Errorf("get resource: %w", err)
	}
	return toResource(row)
}

func (r *repositoryPostgres) List(ctx context.Context, teacherID uuid.UUID, q ListQuery) ([]Resource, int, error) {
	typeArg := nullText(string(q.Type))
	statusArg := nullText(string(q.Status))

	rows, err := r.q.ListTeacherResources(ctx, sqlc.ListTeacherResourcesParams{
		TeacherID:       teacherID,
		Type:            typeArg,
		Status:          statusArg,
		IncludeArchived: q.IncludeArchived,
		PageLimit:       int32(q.PageSize),
		PageOffset:      int32((q.Page - 1) * q.PageSize),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list resources: %w", err)
	}
	total, err := r.q.CountTeacherResources(ctx, sqlc.CountTeacherResourcesParams{
		TeacherID:       teacherID,
		Type:            typeArg,
		Status:          statusArg,
		IncludeArchived: q.IncludeArchived,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count resources: %w", err)
	}

	out := make([]Resource, len(rows))
	for i, row := range rows {
		res, err := toResource(row)
		if err != nil {
			return nil, 0, err
		}
		out[i] = res
	}
	return out, int(total), nil
}

func (r *repositoryPostgres) Update(ctx context.Context, id uuid.UUID, title, instructions string, content Content) (Resource, error) {
	blob, err := marshalContent(content)
	if err != nil {
		return Resource{}, err
	}
	row, err := r.q.UpdateResource(ctx, sqlc.UpdateResourceParams{
		ID: id, Title: title, Instructions: instructions, Content: blob,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Resource{}, ErrNotFound
		}
		return Resource{}, fmt.Errorf("update resource: %w", err)
	}
	return toResource(row)
}

func (r *repositoryPostgres) SetStatus(ctx context.Context, id uuid.UUID, status Status) (Resource, error) {
	row, err := r.q.SetResourceStatus(ctx, sqlc.SetResourceStatusParams{ID: id, Status: string(status)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Resource{}, ErrNotFound
		}
		return Resource{}, fmt.Errorf("set resource status: %w", err)
	}
	return toResource(row)
}

func (r *repositoryPostgres) SetArchived(ctx context.Context, id uuid.UUID, archived bool) (Resource, error) {
	row, err := r.q.SetResourceArchived(ctx, sqlc.SetResourceArchivedParams{ID: id, Archived: archived})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Resource{}, ErrNotFound
		}
		return Resource{}, fmt.Errorf("set resource archived: %w", err)
	}
	return toResource(row)
}

func (r *repositoryPostgres) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteResource(ctx, id); err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	return nil
}

// IsAssigned reports whether the resource has ever been attached to a booking.
func (r *repositoryPostgres) IsAssigned(ctx context.Context, id uuid.UUID) (bool, error) {
	ok, err := r.q.IsResourceAssigned(ctx, id)
	if err != nil {
		return false, fmt.Errorf("is resource assigned: %w", err)
	}
	return ok, nil
}

// --- phase A2: booking attachment ---

func (r *repositoryPostgres) AttachResource(ctx context.Context, bookingID, resourceID uuid.UUID, kind string, assignedBy uuid.UUID, dueAt *time.Time) (BookingResource, error) {
	row, err := r.q.AttachBookingResource(ctx, sqlc.AttachBookingResourceParams{
		BookingID: bookingID, ResourceID: resourceID, Kind: kind,
		AssignedBy: assignedBy, DueAt: toTimestamptz(dueAt),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return BookingResource{}, ErrAlreadyAttached
		}
		return BookingResource{}, fmt.Errorf("attach booking resource: %w", err)
	}
	return bookingResourceFromRow(row.ID, row.BookingID, row.ResourceID, row.Kind, row.Position,
		row.AssignedBy, row.DueAt, row.CreatedAt, row.Type, row.Title, row.Instructions, row.Status, row.Content)
}

func (r *repositoryPostgres) DetachResource(ctx context.Context, bookingID, attachmentID uuid.UUID) error {
	n, err := r.q.DetachBookingResource(ctx, sqlc.DetachBookingResourceParams{ID: attachmentID, BookingID: bookingID})
	if err != nil {
		return fmt.Errorf("detach booking resource: %w", err)
	}
	if n == 0 {
		return ErrAttachmentNotFound
	}
	return nil
}

func (r *repositoryPostgres) ListBookingResources(ctx context.Context, bookingID uuid.UUID) ([]BookingResource, error) {
	rows, err := r.q.ListBookingResources(ctx, bookingID)
	if err != nil {
		return nil, fmt.Errorf("list booking resources: %w", err)
	}
	out := make([]BookingResource, len(rows))
	for i, row := range rows {
		br, err := bookingResourceFromRow(row.ID, row.BookingID, row.ResourceID, row.Kind, row.Position,
			row.AssignedBy, row.DueAt, row.CreatedAt, row.Type, row.Title, row.Instructions, row.Status, row.Content)
		if err != nil {
			return nil, err
		}
		out[i] = br
	}
	return out, nil
}

func (r *repositoryPostgres) GetBookingResourceByPair(ctx context.Context, bookingID, resourceID uuid.UUID) (BookingResource, bool, error) {
	row, err := r.q.GetBookingResourceByPair(ctx, sqlc.GetBookingResourceByPairParams{BookingID: bookingID, ResourceID: resourceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BookingResource{}, false, nil
		}
		return BookingResource{}, false, fmt.Errorf("get booking resource by pair: %w", err)
	}
	br, err := bookingResourceFromRow(row.ID, row.BookingID, row.ResourceID, row.Kind, row.Position,
		row.AssignedBy, row.DueAt, row.CreatedAt, row.Type, row.Title, row.Instructions, row.Status, row.Content)
	if err != nil {
		return BookingResource{}, false, err
	}
	return br, true, nil
}

// --- phase A3: submissions ---

func (r *repositoryPostgres) StartSubmission(ctx context.Context, resourceID, studentID, bookingID uuid.UUID) (Submission, error) {
	row, err := r.q.StartOrGetSubmission(ctx, sqlc.StartOrGetSubmissionParams{
		ResourceID: resourceID, StudentID: studentID, BookingID: toNullUUID(bookingID),
	})
	if err != nil {
		return Submission{}, fmt.Errorf("start submission: %w", err)
	}
	return toSubmission(row)
}

func (r *repositoryPostgres) SaveSubmissionAnswers(ctx context.Context, id uuid.UUID, answers map[string][]string) (Submission, error) {
	blob, err := marshalAnswers(answers)
	if err != nil {
		return Submission{}, err
	}
	row, err := r.q.SaveSubmissionAnswers(ctx, sqlc.SaveSubmissionAnswersParams{Answers: blob, ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, ErrSubmissionNotFound
		}
		return Submission{}, fmt.Errorf("save submission answers: %w", err)
	}
	return toSubmission(row)
}

func (r *repositoryPostgres) SubmitSubmission(ctx context.Context, id uuid.UUID, p SubmitParams) (Submission, error) {
	row, err := r.q.SubmitSubmission(ctx, sqlc.SubmitSubmissionParams{
		Status: p.Status, AutoScore: toInt4(p.AutoScore), AutoMax: toInt4(p.AutoMax),
		SubmittedAt: toTimestamptz(&p.SubmittedAt), ID: id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, ErrSubmissionNotFound
		}
		return Submission{}, fmt.Errorf("submit submission: %w", err)
	}
	return toSubmission(row)
}

func (r *repositoryPostgres) GradeSubmission(ctx context.Context, id uuid.UUID, score *int, feedback string, gradedBy uuid.UUID, gradedAt time.Time) (Submission, error) {
	row, err := r.q.GradeSubmission(ctx, sqlc.GradeSubmissionParams{
		TeacherScore: toInt4(score), TeacherFeedback: feedback,
		GradedBy: toNullUUID(gradedBy), GradedAt: toTimestamptz(&gradedAt), ID: id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, ErrSubmissionNotFound
		}
		return Submission{}, fmt.Errorf("grade submission: %w", err)
	}
	return toSubmission(row)
}

func (r *repositoryPostgres) GetSubmission(ctx context.Context, id uuid.UUID) (Submission, error) {
	row, err := r.q.GetSubmissionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, ErrSubmissionNotFound
		}
		return Submission{}, fmt.Errorf("get submission: %w", err)
	}
	return toSubmission(row)
}

func (r *repositoryPostgres) ListSubmissionsForBooking(ctx context.Context, bookingID uuid.UUID) ([]Submission, error) {
	rows, err := r.q.ListSubmissionsForBooking(ctx, toNullUUID(bookingID))
	if err != nil {
		return nil, fmt.Errorf("list submissions for booking: %w", err)
	}
	out := make([]Submission, len(rows))
	for i, row := range rows {
		s, err := toSubmission(row)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

func (r *repositoryPostgres) InboxSubmissions(ctx context.Context, teacherID uuid.UUID, status string, limit, offset int) ([]Submission, int, error) {
	rows, err := r.q.TeacherSubmissionInbox(ctx, sqlc.TeacherSubmissionInboxParams{
		TeacherID: teacherID, Status: nullText(status), PageOffset: int32(offset), PageLimit: int32(limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("teacher submission inbox: %w", err)
	}
	total, err := r.q.CountTeacherSubmissionInbox(ctx, sqlc.CountTeacherSubmissionInboxParams{
		TeacherID: teacherID, Status: nullText(status),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count teacher submission inbox: %w", err)
	}
	out := make([]Submission, len(rows))
	for i, row := range rows {
		s, err := toSubmission(row)
		if err != nil {
			return nil, 0, err
		}
		out[i] = s
	}
	return out, int(total), nil
}

func (r *repositoryPostgres) FileAssetAccessible(ctx context.Context, fileAssetID, requesterID uuid.UUID) (bool, error) {
	ok, err := r.q.FileAssetAccessibleToStudent(ctx, sqlc.FileAssetAccessibleToStudentParams{
		RequesterID: requesterID, FileAssetID: fileAssetID.String(),
	})
	if err != nil {
		return false, fmt.Errorf("file asset accessible: %w", err)
	}
	return ok, nil
}

func (r *repositoryPostgres) UserContact(ctx context.Context, userID uuid.UUID) (string, string, error) {
	row, err := r.q.GetUserContact(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("get user contact: %w", err)
	}
	return row.Email, row.DisplayName, nil
}

func marshalContent(c Content) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal content: %w", err)
	}
	return b, nil
}

func unmarshalContent(b []byte) (Content, error) {
	var c Content
	if len(b) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return Content{}, fmt.Errorf("unmarshal content: %w", err)
	}
	return c, nil
}

func marshalAnswers(a map[string][]string) ([]byte, error) {
	if a == nil {
		a = map[string][]string{}
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("marshal answers: %w", err)
	}
	return b, nil
}

func unmarshalAnswers(b []byte) (map[string][]string, error) {
	a := map[string][]string{}
	if len(b) == 0 {
		return a, nil
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("unmarshal answers: %w", err)
	}
	return a, nil
}

// bookingResourceFromRow builds the domain BookingResource from the identical
// column set shared by AttachBookingResourceRow / GetBookingResourceByPairRow /
// ListBookingResourcesRow (three distinct generated types, one shape).
func bookingResourceFromRow(
	id, bookingID, resourceID uuid.UUID, kind string, position int32, assignedBy uuid.UUID,
	dueAt, createdAt pgtype.Timestamptz, resType, title, instructions, status string, content []byte,
) (BookingResource, error) {
	c, err := unmarshalContent(content)
	if err != nil {
		return BookingResource{}, err
	}
	return BookingResource{
		ID: id, BookingID: bookingID, ResourceID: resourceID, Kind: kind, Position: int(position),
		AssignedBy: assignedBy, DueAt: fromTimestamptz(dueAt), CreatedAt: createdAt.Time.UTC(),
		Type: Type(resType), Title: title, Instructions: instructions, ResourceStatus: Status(status), Content: c,
	}, nil
}

func toSubmission(row sqlc.Submission) (Submission, error) {
	answers, err := unmarshalAnswers(row.Answers)
	if err != nil {
		return Submission{}, err
	}
	return Submission{
		ID: row.ID, ResourceID: row.ResourceID, StudentID: row.StudentID, Context: row.Context,
		BookingID: fromNullUUID(row.BookingID),
		Status:    row.Status, Answers: answers,
		AutoScore: fromInt4(row.AutoScore), AutoMax: fromInt4(row.AutoMax),
		TeacherScore: fromInt4(row.TeacherScore), TeacherFeedback: row.TeacherFeedback,
		GradedBy: fromNullUUID(row.GradedBy), GradedAt: fromTimestamptz(row.GradedAt),
		SubmittedAt: fromTimestamptz(row.SubmittedAt),
		CreatedAt:   row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}, nil
}

func fromTimestamptz(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

func toTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func fromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

func toInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func fromNullUUID(n uuid.NullUUID) uuid.UUID {
	if !n.Valid {
		return uuid.Nil
	}
	return n.UUID
}

func toNullUUID(id uuid.UUID) uuid.NullUUID {
	if id == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: id, Valid: true}
}

func toResource(row sqlc.Resource) (Resource, error) {
	var c Content
	if len(row.Content) > 0 {
		if err := json.Unmarshal(row.Content, &c); err != nil {
			return Resource{}, fmt.Errorf("unmarshal content for %s: %w", row.ID, err)
		}
	}
	res := Resource{
		ID:           row.ID,
		TeacherID:    row.TeacherID,
		Type:         Type(row.Type),
		Title:        row.Title,
		Instructions: row.Instructions,
		Content:      c,
		Status:       Status(row.Status),
		CreatedAt:    row.CreatedAt.Time.UTC(),
		UpdatedAt:    row.UpdatedAt.Time.UTC(),
	}
	if row.ArchivedAt.Valid {
		t := row.ArchivedAt.Time.UTC()
		res.ArchivedAt = &t
	}
	return res, nil
}

func nullText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
