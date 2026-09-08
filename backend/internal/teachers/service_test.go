package teachers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeRepo is an in-memory Repository for service + handler tests.
type fakeRepo struct {
	list       []Teacher
	total      int
	facets     []LanguageFacet
	bySlug     map[string]*Teacher
	lastParams ListParams
	listErr    error

	// write-path state
	existingSlugs map[string]bool      // seed for SlugExists
	ownerBySlug   map[string]uuid.UUID // teachers.user_id per slug
	createdSlug   string
	createdInput  *ProfileInput
	createdOwner  uuid.UUID
	lastUpdate    *ProfileUpdate
	updatedID     uuid.UUID
	createErr     error
	updateErr     error
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

func (f *fakeRepo) RefBySlug(_ context.Context, slug string) (Ref, error) {
	t, ok := f.bySlug[slug]
	if !ok {
		return Ref{}, ErrNotFound
	}
	return Ref{ID: t.ID, Slug: slug, OwnerID: f.ownerBySlug[slug]}, nil
}

func (f *fakeRepo) RefByOwner(_ context.Context, ownerID uuid.UUID) (Ref, error) {
	for slug, owner := range f.ownerBySlug {
		if owner == ownerID {
			t := f.bySlug[slug]
			id := uuid.Nil
			if t != nil {
				id = t.ID
			}
			return Ref{ID: id, Slug: slug, OwnerID: owner}, nil
		}
	}
	return Ref{}, ErrNotFound
}

func (f *fakeRepo) SlugExists(_ context.Context, slug string) (bool, error) {
	if f.existingSlugs[slug] {
		return true, nil
	}
	_, ok := f.bySlug[slug]
	return ok, nil
}

func (f *fakeRepo) Create(_ context.Context, ownerID uuid.UUID, slug string, in ProfileInput) (uuid.UUID, error) {
	if f.createErr != nil {
		return uuid.Nil, f.createErr
	}
	f.createdSlug = slug
	f.createdOwner = ownerID
	cp := in
	f.createdInput = &cp
	id := uuid.New()
	if f.bySlug == nil {
		f.bySlug = map[string]*Teacher{}
	}
	teaches, alsoSpeaks := entriesToLanguages(in.Languages)
	f.bySlug[slug] = &Teacher{
		ID: id, Slug: slug, DisplayName: in.DisplayName, Headline: in.Headline,
		Kind: in.Kind, Timezone: in.Timezone,
		PricePerHour: Money{in.PricePerHourMinor, in.Currency},
		Teaches:      teaches, AlsoSpeaks: alsoSpeaks, Focus: in.Focus,
		Experience: in.Experience, AcceptingStudents: true,
	}
	if f.ownerBySlug == nil {
		f.ownerBySlug = map[string]uuid.UUID{}
	}
	f.ownerBySlug[slug] = ownerID
	return id, nil
}

func (f *fakeRepo) Update(_ context.Context, teacherID uuid.UUID, upd ProfileUpdate) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updatedID = teacherID
	cp := upd
	f.lastUpdate = &cp
	// reflect the merged scalars + replaced collections into bySlug so the
	// service's re-fetch returns the new state.
	for slug, t := range f.bySlug {
		if t.ID != teacherID {
			continue
		}
		in := upd.Fields
		t.DisplayName, t.Headline, t.Kind = in.DisplayName, in.Headline, in.Kind
		t.Timezone = in.Timezone
		t.PricePerHour = Money{in.PricePerHourMinor, in.Currency}
		if upd.ReplaceLanguages {
			t.Teaches, t.AlsoSpeaks = entriesToLanguages(in.Languages)
		}
		if upd.ReplaceFocus {
			t.Focus = in.Focus
		}
		if upd.ReplaceExperience {
			t.Experience = in.Experience
		}
		f.bySlug[slug] = t
	}
	return nil
}

func entriesToLanguages(entries []LanguageEntry) (teaches, alsoSpeaks []Language) {
	for _, e := range entries {
		l := Language{Code: e.Code, Name: e.Name, Level: e.Level}
		if e.Role == RoleAlsoSpeaks {
			alsoSpeaks = append(alsoSpeaks, l)
		} else {
			teaches = append(teaches, l)
		}
	}
	return teaches, alsoSpeaks
}

// validCreateInput is a complete, valid ProfileInput for write-path tests.
func validCreateInput() ProfileInput {
	trial := int64(3_000_000)
	return ProfileInput{
		DisplayName:       "Nodira Karimova",
		Headline:          "IELTS coach",
		Kind:              KindProfessional,
		CountryCode:       "UZ",
		CountryName:       "Uzbekistan",
		City:              "Tashkent",
		Timezone:          "Asia/Tashkent",
		PricePerHourMinor: 9_000_000,
		TrialPriceMinor:   &trial,
		Currency:          CurrencyUZS,
		About:             "about",
		TeachingStyle:     "structured",
		Languages: []LanguageEntry{
			{Role: RoleTeaches, Code: "en", Name: "English", Level: LevelC2},
			{Role: RoleAlsoSpeaks, Code: "uz", Name: "Uzbek", Level: LevelNative},
		},
		Focus:      []string{"IELTS"},
		Experience: []Experience{{Title: "Instructor", Org: "Cambridge", Period: "2019 – now"}},
	}
}

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

// --- write path ---

func TestService_Create_OK(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	owner := uuid.New()

	got, err := svc.Create(context.Background(), owner, validCreateInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.createdOwner != owner {
		t.Errorf("owner = %v, want %v", repo.createdOwner, owner)
	}
	if repo.createdSlug != "nodira-karimova" {
		t.Errorf("slug = %q, want nodira-karimova", repo.createdSlug)
	}
	if got.Slug != "nodira-karimova" || len(got.Teaches) != 1 || len(got.AlsoSpeaks) != 1 {
		t.Errorf("returned profile wrong: %+v", got)
	}
}

func TestService_Create_DeduplicatesSlug(t *testing.T) {
	repo := &fakeRepo{existingSlugs: map[string]bool{"nodira-karimova": true, "nodira-karimova-2": true}}
	svc := NewService(repo)

	if _, err := svc.Create(context.Background(), uuid.New(), validCreateInput()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.createdSlug != "nodira-karimova-3" {
		t.Errorf("slug = %q, want nodira-karimova-3", repo.createdSlug)
	}
}

func TestService_Create_RejectsSecondProfile(t *testing.T) {
	owner := uuid.New()
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"existing": {ID: uuid.New(), Slug: "existing"}},
		ownerBySlug: map[string]uuid.UUID{"existing": owner},
	}
	svc := NewService(repo)

	_, err := svc.Create(context.Background(), owner, validCreateInput())
	if !errors.Is(err, ErrProfileExists) {
		t.Fatalf("err = %v, want ErrProfileExists", err)
	}
}

func TestService_Create_Validation(t *testing.T) {
	bad := map[string]func(in *ProfileInput){
		"empty display name": func(in *ProfileInput) { in.DisplayName = "" },
		"empty headline":     func(in *ProfileInput) { in.Headline = "" },
		"bad kind":           func(in *ProfileInput) { in.Kind = "wizard" },
		"non-UZS currency":   func(in *ProfileInput) { in.Currency = CurrencyUSD },
		"negative price":     func(in *ProfileInput) { in.PricePerHourMinor = -1 },
		"negative trial":     func(in *ProfileInput) { v := int64(-5); in.TrialPriceMinor = &v },
		"bad timezone":       func(in *ProfileInput) { in.Timezone = "Mars/Olympus" },
		"bad language role":  func(in *ProfileInput) { in.Languages[0].Role = "mutters" },
		"bad language level": func(in *ProfileInput) { in.Languages[0].Level = "fluent-ish" },
		"dup language":       func(in *ProfileInput) { in.Languages[1] = in.Languages[0] },
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := NewService(repo)
			in := validCreateInput()
			mutate(&in)

			_, err := svc.Create(context.Background(), uuid.New(), in)
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if repo.createdInput != nil {
				t.Error("Create should not reach the repo for invalid input")
			}
		})
	}
}

func TestService_Update_OK_PartialAndChildReplace(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()
	repo := &fakeRepo{
		bySlug: map[string]*Teacher{"nodira-karimova": {
			ID: id, Slug: "nodira-karimova", DisplayName: "Nodira Karimova",
			Headline: "old headline", Kind: KindProfessional,
			CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
			PricePerHour: Money{9_000_000, CurrencyUZS},
			Teaches:      []Language{{Code: "en", Name: "English", Level: LevelC2}},
			Focus:        []string{"IELTS"},
		}},
		ownerBySlug: map[string]uuid.UUID{"nodira-karimova": owner},
	}
	svc := NewService(repo)

	newHeadline := "new headline"
	got, err := svc.Update(context.Background(), "nodira-karimova", owner, ProfilePatch{
		Headline: &newHeadline,
		Focus:    &[]string{"Business English", "Conversation"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Headline != "new headline" {
		t.Errorf("headline = %q", got.Headline)
	}
	if repo.lastUpdate == nil || !repo.lastUpdate.ReplaceFocus || repo.lastUpdate.ReplaceLanguages {
		t.Errorf("replace flags wrong: %+v", repo.lastUpdate)
	}
	if len(got.Focus) != 2 {
		t.Errorf("focus not replaced: %+v", got.Focus)
	}
	// languages were not in the patch — left untouched
	if len(got.Teaches) != 1 {
		t.Errorf("teaches changed unexpectedly: %+v", got.Teaches)
	}
}

func TestService_Update_RejectsNonOwner(t *testing.T) {
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"s": {ID: uuid.New(), Slug: "s"}},
		ownerBySlug: map[string]uuid.UUID{"s": uuid.New()},
	}
	svc := NewService(repo)

	_, err := svc.Update(context.Background(), "s", uuid.New(), ProfilePatch{})
	if !errors.Is(err, ErrNotOwner) {
		t.Fatalf("err = %v, want ErrNotOwner", err)
	}
	if repo.lastUpdate != nil {
		t.Error("Update should not reach the repo for a non-owner")
	}
}

func TestService_Update_RejectsUnclaimedProfile(t *testing.T) {
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"s": {ID: uuid.New(), Slug: "s"}},
		ownerBySlug: map[string]uuid.UUID{}, // no owner => uuid.Nil
	}
	svc := NewService(repo)

	_, err := svc.Update(context.Background(), "s", uuid.New(), ProfilePatch{})
	if !errors.Is(err, ErrNotOwner) {
		t.Fatalf("err = %v, want ErrNotOwner", err)
	}
}

func TestService_Update_NotFound(t *testing.T) {
	svc := NewService(&fakeRepo{bySlug: map[string]*Teacher{}})

	_, err := svc.Update(context.Background(), "ghost", uuid.New(), ProfilePatch{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_Update_RejectsInvalidMerge(t *testing.T) {
	owner := uuid.New()
	repo := &fakeRepo{
		bySlug: map[string]*Teacher{"s": {
			ID: uuid.New(), Slug: "s", DisplayName: "N", Headline: "h",
			Kind: KindProfessional, CountryCode: "UZ", CountryName: "Uzbekistan",
			City: "Tashkent", Timezone: "Asia/Tashkent", PricePerHour: Money{1, CurrencyUZS},
		}},
		ownerBySlug: map[string]uuid.UUID{"s": owner},
	}
	svc := NewService(repo)

	empty := ""
	_, err := svc.Update(context.Background(), "s", owner, ProfilePatch{DisplayName: &empty})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if repo.lastUpdate != nil {
		t.Error("Update should not persist an invalid merge")
	}
}

func TestService_GetOwnProfile(t *testing.T) {
	owner := uuid.New()
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{"mine": {ID: uuid.New(), Slug: "mine"}},
		ownerBySlug: map[string]uuid.UUID{"mine": owner},
	}
	svc := NewService(repo)

	got, err := svc.GetOwnProfile(context.Background(), owner)
	if err != nil || got.Slug != "mine" {
		t.Fatalf("GetOwnProfile = (%+v, %v)", got, err)
	}

	if _, err := svc.GetOwnProfile(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
