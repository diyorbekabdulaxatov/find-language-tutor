package resources

import (
	"context"
	"encoding/json"
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

// IsAssigned is always false in phase A1 — nothing references a resource yet.
func (r *repositoryPostgres) IsAssigned(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func marshalContent(c Content) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal content: %w", err)
	}
	return b, nil
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
