package courses

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
)

// seedVideoCourse builds a one-section course owned by a fresh teacher and
// returns the pieces the phase-D1 tests keep needing.
func seedVideoCourse(t *testing.T, e *testEnv) (owner, courseID, sectionID, videoID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	owner, _ = e.seedTeacher()
	d := mustCreate(t, e, owner, "Course")
	d, err := e.svc.AddSection(ctx, owner, d.Course.ID, "Intro")
	if err != nil {
		t.Fatalf("add section: %v", err)
	}
	videoID = uuid.New()
	e.file.put(videoID, owner, "video/mp4")
	return owner, d.Course.ID, d.Sections[0].Section.ID, videoID
}

func TestService_AddItem_PreviewAndDuration(t *testing.T) {
	ctx := context.Background()

	t.Run("video item accepts both", func(t *testing.T) {
		e := newTestEnv()
		owner, courseID, sectionID, videoID := seedVideoCourse(t, e)

		d, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "Lesson 1", &videoID, nil, true, 252)
		if err != nil {
			t.Fatalf("add preview video item: %v", err)
		}
		it := d.Sections[0].Items[0]
		if !it.IsPreview || it.DurationSeconds != 252 {
			t.Errorf("want preview/252, got preview=%v duration=%d", it.IsPreview, it.DurationSeconds)
		}
	})

	t.Run("resource item refuses both", func(t *testing.T) {
		e := newTestEnv()
		owner, courseID, sectionID, _ := seedVideoCourse(t, e)
		resourceID := uuid.New()
		tid := e.repo.teacherByOwner[owner]
		e.res.allow(resourceID, tid)

		if _, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindResource, "", nil, &resourceID, true, 0); err == nil {
			t.Error("a resource item must not be markable as a free preview")
		} else {
			asValidationError(t, err)
		}
		if _, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindResource, "", nil, &resourceID, false, 60); err == nil {
			t.Error("a resource item must not carry a duration")
		} else {
			asValidationError(t, err)
		}
	})

	t.Run("duration bounds", func(t *testing.T) {
		e := newTestEnv()
		owner, courseID, sectionID, videoID := seedVideoCourse(t, e)

		if _, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "", &videoID, nil, false, -1); err == nil {
			t.Error("negative duration should be rejected")
		} else {
			asValidationError(t, err)
		}
		if _, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "", &videoID, nil, false, MaxItemDurationSeconds+1); err == nil {
			t.Error("absurd duration should be rejected")
		} else {
			asValidationError(t, err)
		}
		// 0 is legal: "the browser couldn't read a duration".
		if _, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "", &videoID, nil, false, 0); err != nil {
			t.Errorf("0 duration means unknown and must be allowed: %v", err)
		}
	})
}

// UpdateItem's pointers exist so that un-checking "free preview" actually
// sticks — a plain bool would be indistinguishable from an omitted field.
func TestService_UpdateItem_PreviewToggleAndMerge(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner, courseID, sectionID, videoID := seedVideoCourse(t, e)

	d, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "Lesson", &videoID, nil, true, 300)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	itemID := d.Sections[0].Items[0].ID

	// Omitting both leaves them alone; the title still replaces.
	d, err = e.svc.UpdateItem(ctx, owner, courseID, sectionID, itemID, "Renamed", nil, nil)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	it := d.Sections[0].Items[0]
	if it.Title != "Renamed" || !it.IsPreview || it.DurationSeconds != 300 {
		t.Errorf("a plain rename must not disturb preview/duration: %+v", it)
	}

	// Explicit false turns the preview off.
	off := false
	d, err = e.svc.UpdateItem(ctx, owner, courseID, sectionID, itemID, "Renamed", &off, nil)
	if err != nil {
		t.Fatalf("un-preview: %v", err)
	}
	if d.Sections[0].Items[0].IsPreview {
		t.Error("is_preview:false must stick")
	}

	// A resource item can't be promoted to a preview after the fact either.
	resourceID := uuid.New()
	e.res.allow(resourceID, e.repo.teacherByOwner[owner])
	d, err = e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindResource, "Quiz", nil, &resourceID, false, 0)
	if err != nil {
		t.Fatalf("add resource item: %v", err)
	}
	resItemID := d.Sections[0].Items[1].ID
	on := true
	if _, err := e.svc.UpdateItem(ctx, owner, courseID, sectionID, resItemID, "Quiz", &on, nil); err == nil {
		t.Error("promoting a resource item to a preview must be refused")
	} else {
		asValidationError(t, err)
	}
}

// PreviewVideo is the only unauthenticated read of course video bytes, so
// every gate gets its own case. All of them must be indistinguishable from
// "no such item" to a prober.
func TestService_PreviewVideo_AccessRules(t *testing.T) {
	ctx := context.Background()

	// setup returns an env with one published course whose single video item
	// is flagged as a preview, plus the ids needed to poke at it.
	setup := func(t *testing.T) (*testEnv, uuid.UUID, uuid.UUID, uuid.UUID) {
		t.Helper()
		e := newTestEnv()
		owner, courseID, sectionID, videoID := seedVideoCourse(t, e)
		d, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "Lesson", &videoID, nil, true, 120)
		if err != nil {
			t.Fatalf("add item: %v", err)
		}
		if _, err := e.svc.SetPublished(ctx, owner, courseID, true); err != nil {
			t.Fatalf("publish: %v", err)
		}
		return e, owner, courseID, d.Sections[0].Items[0].ID
	}

	t.Run("published preview streams", func(t *testing.T) {
		e, _, courseID, itemID := setup(t)
		_, body, contentType, err := e.svc.PreviewVideo(ctx, courseID, itemID)
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		defer body.Close()
		if contentType != "video/mp4" {
			t.Errorf("content type = %q", contentType)
		}
		if b, _ := io.ReadAll(body); len(b) == 0 {
			t.Error("expected bytes")
		}
	})

	t.Run("non-preview item is 404", func(t *testing.T) {
		e, owner, courseID, itemID := setup(t)
		off := false
		secID := e.repo.items[itemID].SectionID
		if _, err := e.svc.UpdateItem(ctx, owner, courseID, secID, itemID, "Lesson", &off, nil); err != nil {
			t.Fatalf("un-preview: %v", err)
		}
		if _, _, _, err := e.svc.PreviewVideo(ctx, courseID, itemID); !errors.Is(err, ErrItemNotFound) {
			t.Errorf("want ErrItemNotFound, got %v", err)
		}
	})

	t.Run("unpublished course is 404", func(t *testing.T) {
		e, owner, courseID, itemID := setup(t)
		if _, err := e.svc.SetPublished(ctx, owner, courseID, false); err != nil {
			t.Fatalf("unpublish: %v", err)
		}
		if _, _, _, err := e.svc.PreviewVideo(ctx, courseID, itemID); !errors.Is(err, ErrItemNotFound) {
			t.Errorf("want ErrItemNotFound, got %v", err)
		}
	})

	t.Run("suspended course is 404", func(t *testing.T) {
		e, _, courseID, itemID := setup(t)
		c := e.repo.courses[courseID]
		now := c.CreatedAt
		c.SuspendedAt = &now
		e.repo.courses[courseID] = c
		if _, _, _, err := e.svc.PreviewVideo(ctx, courseID, itemID); !errors.Is(err, ErrItemNotFound) {
			t.Errorf("want ErrItemNotFound, got %v", err)
		}
	})

	t.Run("unapproved teacher is 404", func(t *testing.T) {
		e, _, courseID, itemID := setup(t)
		e.repo.unapprovedTeachers[e.repo.courses[courseID].TeacherID] = true
		if _, _, _, err := e.svc.PreviewVideo(ctx, courseID, itemID); !errors.Is(err, ErrItemNotFound) {
			t.Errorf("want ErrItemNotFound, got %v", err)
		}
	})

	t.Run("item paired with the wrong course id is 404", func(t *testing.T) {
		e, owner, _, itemID := setup(t)
		other := mustCreate(t, e, owner, "Other course")
		if _, _, _, err := e.svc.PreviewVideo(ctx, other.Course.ID, itemID); !errors.Is(err, ErrItemNotFound) {
			t.Errorf("want ErrItemNotFound, got %v", err)
		}
	})

	t.Run("nil FileReader fails closed", func(t *testing.T) {
		e, _, courseID, itemID := setup(t)
		e.svc.files = nil
		if _, _, _, err := e.svc.PreviewVideo(ctx, courseID, itemID); !errors.Is(err, ErrItemNotFound) {
			t.Errorf("want ErrItemNotFound, got %v", err)
		}
	})
}

// The landing page's outline carries the preview flag and per-item duration,
// and the headline totals are summed from the items actually returned.
func TestService_CatalogDetail_PreviewAndTotals(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner, courseID, sectionID, videoID := seedVideoCourse(t, e)

	if _, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "Free sample", &videoID, nil, true, 90); err != nil {
		t.Fatalf("add preview item: %v", err)
	}
	second := uuid.New()
	e.file.put(second, owner, "video/mp4")
	if _, err := e.svc.AddItem(ctx, owner, courseID, sectionID, ItemKindVideo, "Paid lesson", &second, nil, false, 210); err != nil {
		t.Fatalf("add second item: %v", err)
	}
	if _, err := e.svc.SetPublished(ctx, owner, courseID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Anonymous viewer.
	d, err := e.svc.CatalogDetail(ctx, uuid.Nil, courseID)
	if err != nil {
		t.Fatalf("catalog detail: %v", err)
	}
	if d.ItemCount != 2 || d.TotalDurationSeconds != 300 {
		t.Errorf("want 2 items / 300s, got %d / %d", d.ItemCount, d.TotalDurationSeconds)
	}
	items := d.Outline[0].Items
	if len(items) != 2 {
		t.Fatalf("outline items = %d", len(items))
	}
	if !items[0].IsPreview || items[0].DurationSeconds != 90 {
		t.Errorf("first item should be the 90s preview: %+v", items[0])
	}
	if items[1].IsPreview || items[1].DurationSeconds != 210 {
		t.Errorf("second item should be the 210s paid lesson: %+v", items[1])
	}
}
