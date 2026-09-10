package resources

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// Repository is the persistence port. Postgres impl alongside; fake in tests.
type Repository interface {
	// TeacherIDByOwner resolves the teacher profile owned by an account.
	TeacherIDByOwner(ctx context.Context, userID uuid.UUID) (uuid.UUID, bool, error)

	Create(ctx context.Context, p CreateParams) (Resource, error)
	ByID(ctx context.Context, id uuid.UUID) (Resource, error)
	List(ctx context.Context, teacherID uuid.UUID, q ListQuery) ([]Resource, int, error)
	Update(ctx context.Context, id uuid.UUID, title, instructions string, content Content) (Resource, error)
	SetStatus(ctx context.Context, id uuid.UUID, status Status) (Resource, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (Resource, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// IsAssigned reports whether the resource is attached to any lesson / course
	// (phase A2+). Phase A1 always returns false.
	IsAssigned(ctx context.Context, id uuid.UUID) (bool, error)
}

// CreateParams is the repository's insert payload.
type CreateParams struct {
	TeacherID    uuid.UUID
	Type         Type
	Title        string
	Instructions string
	Content      Content
	Status       Status
}

// Service holds the authoring rules. Handlers call it; it never sees a *gin.Context.
type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, logger: logger}
}

// Library returns a page of the caller's resources.
func (s *Service) Library(ctx context.Context, ownerID uuid.UUID, q ListQuery) (Page, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Page{}, err
	}
	if q.Type != "" && !q.Type.valid() {
		return Page{}, invalid("Unknown resource type filter.")
	}
	if q.Status != "" && q.Status != StatusDraft && q.Status != StatusPublished {
		return Page{}, invalid("Status filter must be draft or published.")
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}

	items, total, err := s.repo.List(ctx, tid, q)
	if err != nil {
		return Page{}, err
	}
	if items == nil {
		items = []Resource{}
	}
	return Page{Resources: items, Total: total}, nil
}

// Create authors a new resource (draft or published).
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, t Type, title, instructions string, content Content, publish bool) (Resource, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Resource{}, err
	}
	if !t.valid() {
		return Resource{}, invalid("Unknown resource type.")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Resource{}, invalid("Give the resource a title.")
	}
	cleaned, err := normalizeAndValidateContent(t, content)
	if err != nil {
		return Resource{}, err
	}
	status := StatusDraft
	if publish {
		status = StatusPublished
	}
	return s.repo.Create(ctx, CreateParams{
		TeacherID:    tid,
		Type:         t,
		Title:        title,
		Instructions: strings.TrimSpace(instructions),
		Content:      cleaned,
		Status:       status,
	})
}

// Get returns one of the caller's own resources.
func (s *Service) Get(ctx context.Context, ownerID, id uuid.UUID) (Resource, error) {
	return s.owned(ctx, ownerID, id)
}

// Update replaces the editable fields. Type is immutable; content is
// re-validated against the existing type.
func (s *Service) Update(ctx context.Context, ownerID, id uuid.UUID, title, instructions string, content Content) (Resource, error) {
	r, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return Resource{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Resource{}, invalid("Give the resource a title.")
	}
	cleaned, err := normalizeAndValidateContent(r.Type, content)
	if err != nil {
		return Resource{}, err
	}
	return s.repo.Update(ctx, id, title, strings.TrimSpace(instructions), cleaned)
}

// SetPublished publishes / unpublishes a resource.
func (s *Service) SetPublished(ctx context.Context, ownerID, id uuid.UUID, published bool) (Resource, error) {
	if _, err := s.owned(ctx, ownerID, id); err != nil {
		return Resource{}, err
	}
	status := StatusDraft
	if published {
		status = StatusPublished
	}
	return s.repo.SetStatus(ctx, id, status)
}

// SetArchived archives / restores a resource.
func (s *Service) SetArchived(ctx context.Context, ownerID, id uuid.UUID, archived bool) (Resource, error) {
	if _, err := s.owned(ctx, ownerID, id); err != nil {
		return Resource{}, err
	}
	return s.repo.SetArchived(ctx, id, archived)
}

// Delete removes a resource that has never been assigned. ErrInUse otherwise
// (archive it instead).
func (s *Service) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	if _, err := s.owned(ctx, ownerID, id); err != nil {
		return err
	}
	used, err := s.repo.IsAssigned(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return ErrInUse
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) teacher(ctx context.Context, ownerID uuid.UUID) (uuid.UUID, error) {
	tid, ok, err := s.repo.TeacherIDByOwner(ctx, ownerID)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, ErrNoTeacher
	}
	return tid, nil
}

func (s *Service) owned(ctx context.Context, ownerID, id uuid.UUID) (Resource, error) {
	tid, err := s.teacher(ctx, ownerID)
	if err != nil {
		return Resource{}, err
	}
	r, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Resource{}, err
	}
	if r.TeacherID != tid {
		return Resource{}, ErrForbidden
	}
	return r, nil
}
