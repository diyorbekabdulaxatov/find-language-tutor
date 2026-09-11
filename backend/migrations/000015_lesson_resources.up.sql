-- Phase A2/A3: attaching a resource to a scheduled lesson (as material or
-- homework) and the student's work against it.
--
-- booking_resources — a resource attached to one booking. `kind` is
-- 'material' (just shown to the student) or 'homework' (also submittable, so
-- only quiz/listening/reading/writing resources may be attached that way —
-- enforced in the service, not here). `position` orders a booking's
-- attachments in the lesson view; the repository fills it from the current
-- count on insert. Detaching an attachment never deletes the submissions
-- filed against it — the homework's history survives independently of
-- whether it is still attached.
--
-- The UNIQUE(booking_id, resource_id) makes a duplicate attach a no-op
-- error (23505 -> ErrAlreadyAttached) rather than a second row, same
-- idempotency idiom as payment_events / reviews_booking_uniq /
-- disputes_one_open_per_booking: INSERT first, never check-then-insert.
--
-- submissions — one student's work against a submittable resource.
-- `context` future-proofs the table for course-embedded and standalone
-- practice (phase A3 only ever writes 'lesson', with booking_id set); a
-- lesson submission is unique per (resource, student, booking) via the
-- partial index, so starting homework twice returns the same row (an
-- idempotent get-or-create, not a second attempt).
--
-- Grading: quiz / listening / reading auto-grade on submit (auto_score /
-- auto_max set, status jumps straight to 'graded' — nothing for a teacher to
-- do). writing goes to 'submitted' and waits for a teacher's teacher_score
-- (nullable — feedback-only grading is allowed) / teacher_feedback.

CREATE TABLE booking_resources (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id   uuid NOT NULL REFERENCES bookings (id) ON DELETE CASCADE,
    resource_id  uuid NOT NULL REFERENCES resources (id),

    kind         text NOT NULL CHECK (kind IN ('material', 'homework')),
    position     integer NOT NULL DEFAULT 0,

    assigned_by  uuid NOT NULL REFERENCES users (id),
    due_at       timestamptz,

    created_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (booking_id, resource_id)
);

-- The lesson-detail attachment list, ordered.
CREATE INDEX booking_resources_booking_idx ON booking_resources (booking_id, position);
-- IsAssigned (can a resource be deleted?) and the file-access widening both
-- look up by resource.
CREATE INDEX booking_resources_resource_idx ON booking_resources (resource_id);

CREATE TABLE submissions (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    resource_id      uuid NOT NULL REFERENCES resources (id),
    student_id       uuid NOT NULL REFERENCES users (id),

    context          text NOT NULL DEFAULT 'lesson' CHECK (context IN ('lesson', 'course', 'standalone')),
    -- Set when context = 'lesson'; null for the future contexts. Cascades
    -- with the booking like disputes does — a submission is meaningless once
    -- the lesson it was assigned for is gone.
    booking_id       uuid REFERENCES bookings (id) ON DELETE CASCADE,

    status           text NOT NULL DEFAULT 'in_progress'
        CHECK (status IN ('in_progress', 'submitted', 'graded')),
    answers          jsonb NOT NULL DEFAULT '{}'::jsonb,

    auto_score       integer,
    auto_max         integer,

    teacher_score    integer,
    teacher_feedback text NOT NULL DEFAULT '',
    graded_by        uuid REFERENCES users (id),
    graded_at        timestamptz,

    submitted_at     timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- One submission per student per lesson-homework — StartSubmission's
-- idempotent get-or-create upserts against this (ON CONFLICT DO UPDATE, so a
-- second "start" returns the existing row rather than erroring).
CREATE UNIQUE INDEX submissions_lesson_uniq
    ON submissions (resource_id, student_id, booking_id) WHERE booking_id IS NOT NULL;

-- The lesson-detail submission list.
CREATE INDEX submissions_booking_idx ON submissions (booking_id);
-- The teacher grading inbox scans by status, joined through resources.teacher_id.
CREATE INDEX submissions_status_idx ON submissions (status);
