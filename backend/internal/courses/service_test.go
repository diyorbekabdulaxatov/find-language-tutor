package courses

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// --- fakeRepo: an in-memory Repository ---

type fakeRepo struct {
	teacherByOwner map[uuid.UUID]uuid.UUID
	courses        map[uuid.UUID]Course
	sections       map[uuid.UUID]Section
	items          map[uuid.UUID]Item
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		teacherByOwner: map[uuid.UUID]uuid.UUID{},
		courses:        map[uuid.UUID]Course{},
		sections:       map[uuid.UUID]Section{},
		items:          map[uuid.UUID]Item{},
	}
}

func (r *fakeRepo) TeacherIDByOwner(_ context.Context, userID uuid.UUID) (uuid.UUID, bool, error) {
	tid, ok := r.teacherByOwner[userID]
	return tid, ok, nil
}

func (r *fakeRepo) Create(_ context.Context, p CreateParams) (Course, error) {
	now := time.Now()
	c := Course{
		ID: uuid.New(), TeacherID: p.TeacherID, Title: p.Title, Subtitle: p.Subtitle, Description: p.Description,
		PriceAmountMinor: p.PriceAmountMinor, PriceCurrency: p.PriceCurrency,
		Status: StatusDraft, CreatedAt: now, UpdatedAt: now,
	}
	r.courses[c.ID] = c
	return c, nil
}

func (r *fakeRepo) ByID(_ context.Context, id uuid.UUID) (Course, error) {
	c, ok := r.courses[id]
	if !ok {
		return Course{}, ErrNotFound
	}
	return c, nil
}

func (r *fakeRepo) List(_ context.Context, teacherID uuid.UUID, q ListQuery) ([]Course, int, error) {
	var out []Course
	for _, c := range r.courses {
		if c.TeacherID != teacherID {
			continue
		}
		if q.Status != "" && c.Status != q.Status {
			continue
		}
		if !q.IncludeArchived && c.ArchivedAt != nil {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	total := len(out)
	start := (q.Page - 1) * q.PageSize
	if start > len(out) {
		start = len(out)
	}
	end := start + q.PageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}

func (r *fakeRepo) Update(_ context.Context, id uuid.UUID, p UpdateParams) (Course, error) {
	c, ok := r.courses[id]
	if !ok {
		return Course{}, ErrNotFound
	}
	c.Title, c.Subtitle, c.Description = p.Title, p.Subtitle, p.Description
	c.CoverAssetID = p.CoverAssetID
	c.PriceAmountMinor, c.PriceCurrency = p.PriceAmountMinor, p.PriceCurrency
	c.UpdatedAt = time.Now()
	r.courses[id] = c
	return c, nil
}

func (r *fakeRepo) SetStatus(_ context.Context, id uuid.UUID, status Status) (Course, error) {
	c, ok := r.courses[id]
	if !ok {
		return Course{}, ErrNotFound
	}
	c.Status = status
	if status == StatusPublished {
		c.EverPublished = true
	}
	c.UpdatedAt = time.Now()
	r.courses[id] = c
	return c, nil
}

func (r *fakeRepo) SetArchived(_ context.Context, id uuid.UUID, archived bool) (Course, error) {
	c, ok := r.courses[id]
	if !ok {
		return Course{}, ErrNotFound
	}
	if archived {
		now := time.Now()
		c.ArchivedAt = &now
	} else {
		c.ArchivedAt = nil
	}
	c.UpdatedAt = time.Now()
	r.courses[id] = c
	return c, nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.courses, id)
	return nil
}

// --- sections ---

func (r *fakeRepo) AddSection(_ context.Context, courseID uuid.UUID, title string) (Section, error) {
	pos := 0
	for _, s := range r.sections {
		if s.CourseID == courseID {
			pos++
		}
	}
	now := time.Now()
	s := Section{ID: uuid.New(), CourseID: courseID, Title: title, Position: pos, CreatedAt: now, UpdatedAt: now}
	r.sections[s.ID] = s
	return s, nil
}

func (r *fakeRepo) SectionByID(_ context.Context, id uuid.UUID) (Section, error) {
	s, ok := r.sections[id]
	if !ok {
		return Section{}, ErrSectionNotFound
	}
	return s, nil
}

func (r *fakeRepo) ListSections(_ context.Context, courseID uuid.UUID) ([]Section, error) {
	var out []Section
	for _, s := range r.sections {
		if s.CourseID == courseID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

func (r *fakeRepo) RenameSection(_ context.Context, id uuid.UUID, title string) (Section, error) {
	s, ok := r.sections[id]
	if !ok {
		return Section{}, ErrSectionNotFound
	}
	s.Title = title
	s.UpdatedAt = time.Now()
	r.sections[id] = s
	return s, nil
}

func (r *fakeRepo) DeleteSection(_ context.Context, id uuid.UUID) error {
	delete(r.sections, id)
	for iid, it := range r.items {
		if it.SectionID == id {
			delete(r.items, iid)
		}
	}
	return nil
}

func (r *fakeRepo) ReorderSections(_ context.Context, courseID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		s, ok := r.sections[id]
		if !ok || s.CourseID != courseID {
			return errors.New("fakeRepo: reorder id not in course")
		}
		s.Position = i
		r.sections[id] = s
	}
	return nil
}

// --- items ---

func (r *fakeRepo) AddItem(_ context.Context, p AddItemParams) (Item, error) {
	pos := 0
	for _, it := range r.items {
		if it.SectionID == p.SectionID {
			pos++
		}
	}
	it := Item{
		ID: uuid.New(), SectionID: p.SectionID, Kind: p.Kind, Title: p.Title,
		VideoAssetID: p.VideoAssetID, ResourceID: p.ResourceID, Position: pos, CreatedAt: time.Now(),
	}
	r.items[it.ID] = it
	return it, nil
}

func (r *fakeRepo) ItemByID(_ context.Context, id uuid.UUID) (Item, error) {
	it, ok := r.items[id]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	return it, nil
}

func (r *fakeRepo) ListItems(_ context.Context, sectionID uuid.UUID) ([]Item, error) {
	var out []Item
	for _, it := range r.items {
		if it.SectionID == sectionID {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

func (r *fakeRepo) ListItemsByCourse(_ context.Context, courseID uuid.UUID) ([]Item, error) {
	var sections []Section
	for _, s := range r.sections {
		if s.CourseID == courseID {
			sections = append(sections, s)
		}
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].Position < sections[j].Position })

	var out []Item
	for _, s := range sections {
		var items []Item
		for _, it := range r.items {
			if it.SectionID == s.ID {
				items = append(items, it)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
		out = append(out, items...)
	}
	return out, nil
}

func (r *fakeRepo) RenameItem(_ context.Context, id uuid.UUID, title string) (Item, error) {
	it, ok := r.items[id]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	it.Title = title
	r.items[id] = it
	return it, nil
}

func (r *fakeRepo) DeleteItem(_ context.Context, id uuid.UUID) error {
	delete(r.items, id)
	return nil
}

func (r *fakeRepo) ReorderItems(_ context.Context, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		it, ok := r.items[id]
		if !ok || it.SectionID != sectionID {
			return errors.New("fakeRepo: reorder id not in section")
		}
		it.Position = i
		r.items[id] = it
	}
	return nil
}

// --- fakeResourceReader: an in-memory ResourceReader ---

type fakeResourceReader struct {
	// owned[resourceID|teacherID] = true means "belongs to teacherID and is
	// published, not archived" — exactly what the real adapter
	// (resources.CourseGateway) collapses into one bool.
	owned map[string]bool
	err   error
}

func newFakeResourceReader() *fakeResourceReader {
	return &fakeResourceReader{owned: map[string]bool{}}
}

func (f *fakeResourceReader) allow(resourceID, teacherID uuid.UUID) {
	f.owned[resourceID.String()+"|"+teacherID.String()] = true
}

func (f *fakeResourceReader) ResourceOwnedAndPublished(_ context.Context, resourceID, teacherID uuid.UUID) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.owned[resourceID.String()+"|"+teacherID.String()], nil
}

var _ ResourceReader = (*fakeResourceReader)(nil)

// --- fakeFileReader: an in-memory FileReader ---

type fileRecord struct {
	owner       uuid.UUID
	contentType string
}

type fakeFileReader struct {
	byID map[uuid.UUID]fileRecord
	err  error
}

func newFakeFileReader() *fakeFileReader { return &fakeFileReader{byID: map[uuid.UUID]fileRecord{}} }

func (f *fakeFileReader) put(id, owner uuid.UUID, contentType string) {
	f.byID[id] = fileRecord{owner: owner, contentType: contentType}
}

func (f *fakeFileReader) FileOwnedBy(_ context.Context, fileAssetID, callerID uuid.UUID) (bool, string, error) {
	if f.err != nil {
		return false, "", f.err
	}
	rec, ok := f.byID[fileAssetID]
	if !ok || rec.owner != callerID {
		return false, "", nil
	}
	return true, rec.contentType, nil
}

var _ FileReader = (*fakeFileReader)(nil)

// --- test scaffolding ---

type testEnv struct {
	svc  *Service
	repo *fakeRepo
	res  *fakeResourceReader
	file *fakeFileReader
}

func newTestEnv() *testEnv {
	repo := newFakeRepo()
	svc := NewService(repo, discardLogger())
	res := newFakeResourceReader()
	file := newFakeFileReader()
	svc.SetResourceReader(res)
	svc.SetFileReader(file)
	return &testEnv{svc: svc, repo: repo, res: res, file: file}
}

// seedTeacher registers a fresh account+teacher pair in the repo.
func (e *testEnv) seedTeacher() (ownerID, teacherID uuid.UUID) {
	ownerID, teacherID = uuid.New(), uuid.New()
	e.repo.teacherByOwner[ownerID] = teacherID
	return
}

func mustCreate(t *testing.T, e *testEnv, ownerID uuid.UUID, title string) CourseDetail {
	t.Helper()
	d, err := e.svc.Create(context.Background(), ownerID, title, "sub", "desc", 0, "")
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	return d
}

func asValidationError(t *testing.T, err error) {
	t.Helper()
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v (%T)", err, err)
	}
}

// --- Create ---

func TestService_Create(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()

	if _, err := e.svc.Create(ctx, uuid.New(), "x", "", "", 0, ""); !errors.Is(err, ErrNoTeacher) {
		t.Errorf("no teacher profile: %v", err)
	}
	if _, err := e.svc.Create(ctx, owner, "  ", "", "", 0, ""); err == nil {
		t.Error("empty title should fail")
	} else {
		asValidationError(t, err)
	}
	if _, err := e.svc.Create(ctx, owner, "Course", "", "", -1, ""); err == nil {
		t.Error("negative price should fail")
	} else {
		asValidationError(t, err)
	}
	if _, err := e.svc.Create(ctx, owner, "Course", "", "", 0, "EUR"); err == nil {
		t.Error("bad currency should fail")
	} else {
		asValidationError(t, err)
	}

	d, err := e.svc.Create(ctx, owner, "  Spoken Uzbek  ", "", "", 0, "")
	if err != nil {
		t.Fatalf("valid create: %v", err)
	}
	if d.Course.Title != "Spoken Uzbek" || d.Course.Status != StatusDraft || d.Course.EverPublished {
		t.Errorf("unexpected course: %+v", d.Course)
	}
	if d.Course.PriceCurrency != "UZS" {
		t.Errorf("currency default: %q", d.Course.PriceCurrency)
	}
	if len(d.Sections) != 0 {
		t.Errorf("new course should have no sections: %+v", d.Sections)
	}
}

// --- Get / ownership ---

func TestService_Get_OwnershipEnforced(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	ownerA, _ := e.seedTeacher()
	ownerB, _ := e.seedTeacher()
	d := mustCreate(t, e, ownerA, "Course A")

	if _, err := e.svc.Get(ctx, ownerA, d.Course.ID); err != nil {
		t.Errorf("owner get: %v", err)
	}
	if _, err := e.svc.Get(ctx, ownerB, d.Course.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("other teacher get: %v, want ErrForbidden", err)
	}
	if _, err := e.svc.Get(ctx, ownerA, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Get(ctx, uuid.New(), d.Course.ID); !errors.Is(err, ErrNoTeacher) {
		t.Errorf("no teacher profile: %v, want ErrNoTeacher", err)
	}
}

// --- Update ---

func TestService_Update(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	ownerA, _ := e.seedTeacher()
	ownerB, _ := e.seedTeacher()
	d := mustCreate(t, e, ownerA, "Course")

	if _, err := e.svc.Update(ctx, ownerB, d.Course.ID, "New", "", "", nil, 0, ""); !errors.Is(err, ErrForbidden) {
		t.Errorf("other teacher update: %v, want ErrForbidden", err)
	}
	if _, err := e.svc.Update(ctx, ownerA, d.Course.ID, "  ", "", "", nil, 0, ""); err == nil {
		t.Error("empty title should fail")
	} else {
		asValidationError(t, err)
	}
	if _, err := e.svc.Update(ctx, ownerA, d.Course.ID, "New", "", "", nil, -5, ""); err == nil {
		t.Error("negative price should fail")
	} else {
		asValidationError(t, err)
	}

	// cover asset not owned by caller.
	coverID := uuid.New()
	if _, err := e.svc.Update(ctx, ownerA, d.Course.ID, "New", "", "", &coverID, 0, ""); err == nil {
		t.Error("un-owned cover asset should fail")
	} else {
		asValidationError(t, err)
	}

	// cover asset owned but not an image.
	e.file.put(coverID, ownerA, "video/mp4")
	if _, err := e.svc.Update(ctx, ownerA, d.Course.ID, "New", "", "", &coverID, 0, ""); err == nil {
		t.Error("non-image cover should fail")
	} else {
		asValidationError(t, err)
	}

	// cover asset owned and an image: succeeds.
	e.file.put(coverID, ownerA, "image/png")
	updated, err := e.svc.Update(ctx, ownerA, d.Course.ID, "New Title", "sub2", "desc2", &coverID, 150000, "USD")
	if err != nil {
		t.Fatalf("valid update: %v", err)
	}
	if updated.Course.Title != "New Title" || updated.Course.PriceAmountMinor != 150000 || updated.Course.PriceCurrency != "USD" {
		t.Errorf("unexpected course: %+v", updated.Course)
	}
	if updated.Course.CoverAssetID == nil || *updated.Course.CoverAssetID != coverID {
		t.Errorf("cover asset id not set: %+v", updated.Course.CoverAssetID)
	}
}

// --- Publish validation ---

func TestService_SetPublished_ValidatesCurriculum(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, teacherID := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")

	// No sections yet.
	if _, err := e.svc.SetPublished(ctx, owner, d.Course.ID, true); err == nil {
		t.Error("publish with no sections should fail")
	} else {
		asValidationError(t, err)
	}

	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	sectionID := d.Sections[0].Section.ID

	// Section with no items.
	if _, err := e.svc.SetPublished(ctx, owner, d.Course.ID, true); err == nil {
		t.Error("publish with an empty section should fail")
	} else {
		asValidationError(t, err)
	}

	// Add a resource item so the section is non-empty.
	resourceID := uuid.New()
	e.res.allow(resourceID, teacherID)
	d, err = e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "", nil, &resourceID)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}

	published, err := e.svc.SetPublished(ctx, owner, d.Course.ID, true)
	if err != nil {
		t.Fatalf("publish should succeed: %v", err)
	}
	if published.Course.Status != StatusPublished || !published.Course.EverPublished {
		t.Errorf("unexpected course after publish: %+v", published.Course)
	}

	// Unpublish keeps EverPublished latched.
	unpublished, err := e.svc.SetPublished(ctx, owner, d.Course.ID, false)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if unpublished.Course.Status != StatusDraft || !unpublished.Course.EverPublished {
		t.Errorf("unpublish should keep ever_published latched: %+v", unpublished.Course)
	}
}

// --- Delete ---

func TestService_Delete_BlockedOncePublished(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, teacherID := e.seedTeacher()

	// Never published: delete succeeds.
	d := mustCreate(t, e, owner, "Draft course")
	if err := e.svc.Delete(ctx, owner, d.Course.ID); err != nil {
		t.Errorf("delete never-published course: %v", err)
	}

	// Published then unpublished: delete blocked.
	d = mustCreate(t, e, owner, "Once-live course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	resourceID := uuid.New()
	e.res.allow(resourceID, teacherID)
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, d.Sections[0].Section.ID, ItemKindResource, "", nil, &resourceID); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if _, err := e.svc.SetPublished(ctx, owner, d.Course.ID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := e.svc.SetPublished(ctx, owner, d.Course.ID, false); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if err := e.svc.Delete(ctx, owner, d.Course.ID); !errors.Is(err, ErrInUse) {
		t.Errorf("delete once-published course: %v, want ErrInUse", err)
	}

	// Ownership still enforced on delete.
	ownerB, _ := e.seedTeacher()
	d2 := mustCreate(t, e, owner, "Another draft")
	if err := e.svc.Delete(ctx, ownerB, d2.Course.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("other teacher delete: %v, want ErrForbidden", err)
	}
}

// --- Sections ---

func TestService_Sections_OwnershipAndValidation(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	ownerA, _ := e.seedTeacher()
	ownerB, _ := e.seedTeacher()
	dA := mustCreate(t, e, ownerA, "Course A")
	dB := mustCreate(t, e, ownerB, "Course B")

	if _, err := e.svc.AddSection(ctx, ownerA, dA.Course.ID, "  "); err == nil {
		t.Error("empty section title should fail")
	} else {
		asValidationError(t, err)
	}
	if _, err := e.svc.AddSection(ctx, ownerB, dA.Course.ID, "Intro"); !errors.Is(err, ErrForbidden) {
		t.Errorf("other teacher add section: %v, want ErrForbidden", err)
	}

	dA, err := e.svc.AddSection(ctx, ownerA, dA.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	sectionID := dA.Sections[0].Section.ID

	// A section from another course is not found under this course.
	if _, err := e.svc.RenameSection(ctx, ownerB, dB.Course.ID, sectionID, "New"); !errors.Is(err, ErrSectionNotFound) {
		t.Errorf("rename section from foreign course: %v, want ErrSectionNotFound", err)
	}

	dA, err = e.svc.RenameSection(ctx, ownerA, dA.Course.ID, sectionID, "Introduction")
	if err != nil {
		t.Fatalf("rename section: %v", err)
	}
	if dA.Sections[0].Section.Title != "Introduction" {
		t.Errorf("rename didn't take: %+v", dA.Sections[0])
	}

	dA, err = e.svc.DeleteSection(ctx, ownerA, dA.Course.ID, sectionID)
	if err != nil {
		t.Fatalf("delete section: %v", err)
	}
	if len(dA.Sections) != 0 {
		t.Errorf("section not removed: %+v", dA.Sections)
	}
}

func TestService_ReorderSections_ValidatesIDSet(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		var err error
		d, err = e.svc.AddSection(ctx, owner, d.Course.ID, "Section")
		if err != nil {
			t.Fatalf("add section: %v", err)
		}
	}
	for _, sd := range d.Sections {
		ids = append(ids, sd.Section.ID)
	}

	// Missing one id.
	if _, err := e.svc.ReorderSections(ctx, owner, d.Course.ID, ids[:2]); err == nil {
		t.Error("partial id set should fail")
	} else {
		asValidationError(t, err)
	}
	// Foreign id.
	foreign := append(append([]uuid.UUID{}, ids...), uuid.New())
	if _, err := e.svc.ReorderSections(ctx, owner, d.Course.ID, foreign); err == nil {
		t.Error("id set with a foreign extra id should fail")
	} else {
		asValidationError(t, err)
	}
	// Duplicate id.
	dup := []uuid.UUID{ids[0], ids[0], ids[1]}
	if _, err := e.svc.ReorderSections(ctx, owner, d.Course.ID, dup); err == nil {
		t.Error("duplicate id should fail")
	} else {
		asValidationError(t, err)
	}

	// Valid reverse order.
	reversed := []uuid.UUID{ids[2], ids[1], ids[0]}
	d, err := e.svc.ReorderSections(ctx, owner, d.Course.ID, reversed)
	if err != nil {
		t.Fatalf("valid reorder: %v", err)
	}
	for i, sd := range d.Sections {
		if sd.Section.ID != reversed[i] {
			t.Errorf("position %d: got %s, want %s", i, sd.Section.ID, reversed[i])
		}
	}
}

// --- Items ---

func TestService_AddItem_ResourceKind(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, teacherID := e.seedTeacher()
	_, otherTeacherID := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	sectionID := d.Sections[0].Section.ID

	// Missing resource_id.
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "", nil, nil); err == nil {
		t.Error("resource item without resource_id should fail")
	} else {
		asValidationError(t, err)
	}

	// Both video_asset_id and resource_id set.
	videoID, resourceID := uuid.New(), uuid.New()
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "", &videoID, &resourceID); err == nil {
		t.Error("resource item with a video id too should fail")
	} else {
		asValidationError(t, err)
	}

	// Another teacher's resource.
	e.res.allow(resourceID, otherTeacherID)
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "", nil, &resourceID); err == nil {
		t.Error("another teacher's resource should be rejected")
	} else {
		asValidationError(t, err)
	}

	// Unpublished / not-owned resource (never allowed for this teacher).
	unpublished := uuid.New()
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "", nil, &unpublished); err == nil {
		t.Error("unpublished resource should be rejected")
	} else {
		asValidationError(t, err)
	}

	// Owned + published: succeeds.
	e.res.allow(resourceID, teacherID)
	d, err = e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "My reading", nil, &resourceID)
	if err != nil {
		t.Fatalf("valid resource item: %v", err)
	}
	items := d.Sections[0].Items
	if len(items) != 1 || items[0].Kind != ItemKindResource || items[0].ResourceID == nil || *items[0].ResourceID != resourceID {
		t.Errorf("unexpected items: %+v", items)
	}

	// No ResourceReader wired: fails closed.
	e2 := newTestEnv()
	e2.svc.resources = nil
	owner2, _ := e2.seedTeacher()
	d2 := mustCreate(t, e2, owner2, "Course")
	d2, err = e2.svc.AddSection(ctx, owner2, d2.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	if _, err := e2.svc.AddItem(ctx, owner2, d2.Course.ID, d2.Sections[0].Section.ID, ItemKindResource, "", nil, &resourceID); err == nil {
		t.Error("nil ResourceReader should fail closed")
	} else {
		asValidationError(t, err)
	}
}

func TestService_AddItem_VideoKind(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	sectionID := d.Sections[0].Section.ID

	// Missing video_asset_id.
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindVideo, "", nil, nil); err == nil {
		t.Error("video item without video_asset_id should fail")
	} else {
		asValidationError(t, err)
	}

	// Both set.
	videoID, resourceID := uuid.New(), uuid.New()
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindVideo, "", &videoID, &resourceID); err == nil {
		t.Error("video item with a resource id too should fail")
	} else {
		asValidationError(t, err)
	}

	// Not owned by caller.
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindVideo, "", &videoID, nil); err == nil {
		t.Error("non-owned file should be rejected")
	} else {
		asValidationError(t, err)
	}

	// Owned, but not a video content type.
	e.file.put(videoID, owner, "application/pdf")
	if _, err := e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindVideo, "", &videoID, nil); err == nil {
		t.Error("non-video content type should be rejected")
	} else {
		asValidationError(t, err)
	}

	// Owned and a video: succeeds.
	e.file.put(videoID, owner, "video/mp4")
	d, err = e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindVideo, "Lesson 1", &videoID, nil)
	if err != nil {
		t.Fatalf("valid video item: %v", err)
	}
	items := d.Sections[0].Items
	if len(items) != 1 || items[0].Kind != ItemKindVideo || items[0].VideoAssetID == nil || *items[0].VideoAssetID != videoID {
		t.Errorf("unexpected items: %+v", items)
	}

	// AddItem addressed at a second course the same teacher owns, using the
	// first course's section id: the section doesn't belong to that course.
	otherD := mustCreate(t, e, owner, "Other course")
	if _, err := e.svc.AddItem(ctx, owner, otherD.Course.ID, sectionID, ItemKindVideo, "", &videoID, nil); !errors.Is(err, ErrSectionNotFound) {
		t.Errorf("section from a different course: %v, want ErrSectionNotFound", err)
	}
}

func TestService_RenameItem_DeleteItem_Ownership(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, teacherID := e.seedTeacher()
	ownerB, _ := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	sectionID := d.Sections[0].Section.ID
	resourceID := uuid.New()
	e.res.allow(resourceID, teacherID)
	d, err = e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "", nil, &resourceID)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	itemID := d.Sections[0].Items[0].ID

	if _, err := e.svc.RenameItem(ctx, ownerB, d.Course.ID, sectionID, itemID, "New"); !errors.Is(err, ErrForbidden) {
		t.Errorf("other teacher rename item: %v, want ErrForbidden", err)
	}
	d, err = e.svc.RenameItem(ctx, owner, d.Course.ID, sectionID, itemID, "Renamed")
	if err != nil {
		t.Fatalf("rename item: %v", err)
	}
	if d.Sections[0].Items[0].Title != "Renamed" {
		t.Errorf("rename didn't take: %+v", d.Sections[0].Items[0])
	}

	// Item id that exists but belongs to a different section.
	d, err = e.svc.AddSection(ctx, owner, d.Course.ID, "Second")
	if err != nil {
		t.Fatalf("add second section: %v", err)
	}
	secondSectionID := d.Sections[1].Section.ID
	if _, err := e.svc.RenameItem(ctx, owner, d.Course.ID, secondSectionID, itemID, "X"); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("item from a different section: %v, want ErrItemNotFound", err)
	}

	d, err = e.svc.DeleteItem(ctx, owner, d.Course.ID, sectionID, itemID)
	if err != nil {
		t.Fatalf("delete item: %v", err)
	}
	if len(d.Sections[0].Items) != 0 {
		t.Errorf("item not removed: %+v", d.Sections[0].Items)
	}
}

func TestService_ReorderItems_ValidatesIDSet(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, teacherID := e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	sectionID := d.Sections[0].Section.ID

	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		resourceID := uuid.New()
		e.res.allow(resourceID, teacherID)
		var err error
		d, err = e.svc.AddItem(ctx, owner, d.Course.ID, sectionID, ItemKindResource, "", nil, &resourceID)
		if err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	for _, it := range d.Sections[0].Items {
		ids = append(ids, it.ID)
	}

	if _, err := e.svc.ReorderItems(ctx, owner, d.Course.ID, sectionID, ids[:2]); err == nil {
		t.Error("partial id set should fail")
	} else {
		asValidationError(t, err)
	}
	foreign := append(append([]uuid.UUID{}, ids...), uuid.New())
	if _, err := e.svc.ReorderItems(ctx, owner, d.Course.ID, sectionID, foreign); err == nil {
		t.Error("foreign id should fail")
	} else {
		asValidationError(t, err)
	}

	reversed := []uuid.UUID{ids[2], ids[1], ids[0]}
	d, err = e.svc.ReorderItems(ctx, owner, d.Course.ID, sectionID, reversed)
	if err != nil {
		t.Fatalf("valid reorder: %v", err)
	}
	for i, it := range d.Sections[0].Items {
		if it.ID != reversed[i] {
			t.Errorf("position %d: got %s, want %s", i, it.ID, reversed[i])
		}
	}
}

// --- Library ---

func TestService_Library(t *testing.T) {
	e := newTestEnv()
	ctx := context.Background()
	owner, _ := e.seedTeacher()

	if _, err := e.svc.Library(ctx, owner, ListQuery{Status: "bogus"}); err == nil {
		t.Error("bad status filter should fail")
	} else {
		asValidationError(t, err)
	}
	if _, err := e.svc.Library(ctx, uuid.New(), ListQuery{}); !errors.Is(err, ErrNoTeacher) {
		t.Errorf("no teacher profile: %v, want ErrNoTeacher", err)
	}

	mustCreate(t, e, owner, "Course 1")
	mustCreate(t, e, owner, "Course 2")
	page, err := e.svc.Library(ctx, owner, ListQuery{})
	if err != nil {
		t.Fatalf("library: %v", err)
	}
	if page.Total != 2 || len(page.Courses) != 2 {
		t.Errorf("unexpected page: %+v", page)
	}
}
