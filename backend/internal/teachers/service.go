package teachers

import (
	"context"
	"errors"
)

// ErrNotFound is returned by GetBySlug when no teacher has the given slug.
var ErrNotFound = errors.New("teacher not found")

// Repository is the persistence port for this module. The concrete
// implementation (repositoryPostgres) lives alongside; tests use a fake.
type Repository interface {
	// List returns one page of teachers matching params (already normalized)
	// together with the total count of matches, ignoring pagination.
	List(ctx context.Context, params ListParams) (page []Teacher, total int, err error)

	// GetBySlug returns the full aggregate for one teacher, or ErrNotFound.
	GetBySlug(ctx context.Context, slug string) (*Teacher, error)

	// LanguageFacets returns the taught-language counts across the whole catalog.
	LanguageFacets(ctx context.Context) ([]LanguageFacet, error)

	// Slugs returns every teacher slug, for the frontend's static generation.
	Slugs(ctx context.Context) ([]string, error)
}

// Service holds the teacher-profiles business logic. Handlers call it; it never
// sees an *gin.Context.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List runs a teacher search and attaches the filter facets.
func (s *Service) List(ctx context.Context, params ListParams) (ListResult, error) {
	params = params.normalized()

	page, total, err := s.repo.List(ctx, params)
	if err != nil {
		return ListResult{}, err
	}

	facets, err := s.repo.LanguageFacets(ctx)
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Teachers: page,
		Total:    total,
		Facets:   Facets{Languages: facets},
	}, nil
}

// GetBySlug returns one full profile. Propagates ErrNotFound.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Teacher, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// Slugs lists every teacher slug.
func (s *Service) Slugs(ctx context.Context) ([]string, error) {
	return s.repo.Slugs(ctx)
}
