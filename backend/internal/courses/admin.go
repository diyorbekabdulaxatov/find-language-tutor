// Phase C3: the admin course-moderation queue. Mirrors internal/reviews'
// phase-F moderation shape (AdminReview / ModerationQuery / AdminPage) as
// closely as makes sense for courses: a list with filters, and an idempotent
// binary toggle (SetSuspended) instead of reviews' hide/unhide.
package courses

// AdminCourse is one row of the operator moderation queue: the course plus
// the teacher it belongs to. Unlike the teacher's own authoring views (which
// return a Course/CourseDetail), this never carries curriculum — moderation
// acts on the course record, not its content.
type AdminCourse struct {
	Course  Course
	Teacher TeacherSummary
}

// AdminCourseQuery is the validated input to the moderation list.
//
// Status is "" (any) | "draft" | "published" | "archived" — "archived" means
// archived_at IS NOT NULL regardless of the underlying status column, since
// archiving is a separate axis from draft/published in this module (unlike
// reviews' single Visibility enum). Suspended is "" (any) | "true" | "false".
type AdminCourseQuery struct {
	Status      string
	Suspended   string
	TeacherSlug string
	Q           string
	Page        int
	PageSize    int
}

// AdminCoursePage is a page of the moderation queue plus the total match count.
type AdminCoursePage struct {
	Courses []AdminCourse
	Total   int
}
