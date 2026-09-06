package teachers

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// fakeRepo is an in-memory Repository for service tests.
type fakeRepo struct {
	list       []Teacher
	total      int
	facets     []LanguageFacet
	bySlug     map[string]*Teacher
	lastParams ListParams
	listErr    error
}

func (f *fakeRepo) List(_ context.Context, p ListParams) ([]Teacher, int, error) {
	f.lastParams = p
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.list, f.total, nil
}

func (f *fakeRepo) GetBySlug(_ context.Context, slug string) (*Teacher, error) {
	if t, ok := f.bySlug[slug]; ok {
		return t, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) LanguageFacets(_ context.Context) ([]LanguageFacet, error) {
	return f.facets, nil
}

func (f *fakeRepo) Slugs(_ context.Context) ([]string, error) { return nil, nil }

func TestService_List_NormalizesParamsAndAttachesFacets(t *testing.T) {
	repo := &fakeRepo{
		list:   []Teacher{{Slug: "a"}, {Slug: "b"}},
		total:  2,
		facets: []LanguageFacet{{Code: "en", Name: "English", Count: 5}},
	}
	svc := NewService(repo)

	res, err := svc.List(context.Background(), ListParams{Sort: "bogus", Page: 0, PageSize: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// invalid sort falls back to recommended; page/size get defaults.
	if repo.lastParams.Sort != SortRecommended {
		t.Errorf("sort = %q, want recommended", repo.lastParams.Sort)
	}
	if repo.lastParams.Page != 1 || repo.lastParams.PageSize != defaultPageSize {
		t.Errorf("pagination = (%d,%d), want (1,%d)", repo.lastParams.Page, repo.lastParams.PageSize, defaultPageSize)
	}
	if res.Total != 2 || len(res.Teachers) != 2 {
		t.Errorf("got total=%d teachers=%d", res.Total, len(res.Teachers))
	}
	if len(res.Facets.Languages) != 1 || res.Facets.Languages[0].Code != "en" {
		t.Errorf("facets not attached: %+v", res.Facets)
	}
}

func TestService_List_ClampsPageSize(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.List(context.Background(), ListParams{PageSize: 10_000}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastParams.PageSize != maxPageSize {
		t.Errorf("page size = %d, want clamped to %d", repo.lastParams.PageSize, maxPageSize)
	}
}

func TestService_GetBySlug_NotFound(t *testing.T) {
	svc := NewService(&fakeRepo{bySlug: map[string]*Teacher{}})

	_, err := svc.GetBySlug(context.Background(), "missing")
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_GetBySlug_Found(t *testing.T) {
	want := &Teacher{ID: uuid.New(), Slug: "nodira-karimova"}
	svc := NewService(&fakeRepo{bySlug: map[string]*Teacher{"nodira-karimova": want}})

	got, err := svc.GetBySlug(context.Background(), "nodira-karimova")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Slug != want.Slug {
		t.Errorf("slug = %q, want %q", got.Slug, want.Slug)
	}
}
