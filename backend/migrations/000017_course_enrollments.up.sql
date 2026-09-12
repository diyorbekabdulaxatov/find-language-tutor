-- Phase C2: course catalog, one-time purchase, enrollment, the student player,
-- and progress tracking. Builds on phase C1's authoring tables (courses,
-- course_sections, course_items) and phase A1-A3's resources/submissions.
--
-- course_enrollments — one row per (course, student): the student bought the
-- course (source='purchase', amount_paid_minor/currency snapshot the price at
-- purchase time so a later price change never rewrites history) or it was
-- free (source='free', amount_paid_minor stays 0). UNIQUE(course_id,
-- student_id) is the idempotency gate for Purchase — insert first, 23505 means
-- already enrolled, same idiom as booking_resources / payment_events.
--
-- course_item_progress — one row per (enrollment, item): video-item playback
-- position and completion. A `resource`-kind item's completion instead comes
-- from its submission (resources.CourseProgress.ItemCompleted upserts this
-- table to 'completed' on submit/grade), so this table is written from two
-- call sites but always through the courses module's own repository.
--
-- course_payments / course_payment_events mirror internal/payments' payments /
-- payment_events tables exactly (status lifecycle, webhook idempotency), but
-- live in courses' own migration and are owned by internal/payments'
-- CourseService as a sibling to the booking payment flow — a deliberate
-- schema duplication rather than reusing `payments` (which is hardwired to
-- `booking_id NOT NULL UNIQUE`) or introducing a polymorphic owner column.
-- UNIQUE(course_id, student_id): one payment attempt-chain per course
-- purchase, same one-per-thing shape as payments.booking_id.
--
-- submissions.enrollment_id: a course-embedded resource's submission
-- (context='course') attaches here instead of booking_id. The partial unique
-- index (mirroring submissions_lesson_uniq) makes starting the same
-- course-resource submission twice idempotent.

CREATE TABLE course_enrollments (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id          uuid NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    student_id         uuid NOT NULL REFERENCES users (id),
    source             text NOT NULL CHECK (source IN ('purchase', 'free')),
    amount_paid_minor  bigint NOT NULL DEFAULT 0 CHECK (amount_paid_minor >= 0),
    currency           currency_code NOT NULL DEFAULT 'UZS',
    created_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (course_id, student_id)
);
CREATE INDEX course_enrollments_student_idx ON course_enrollments (student_id, created_at DESC);
CREATE INDEX course_enrollments_course_idx ON course_enrollments (course_id);

CREATE TABLE course_item_progress (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    enrollment_id           uuid NOT NULL REFERENCES course_enrollments (id) ON DELETE CASCADE,
    item_id                 uuid NOT NULL REFERENCES course_items (id) ON DELETE CASCADE,
    status                  text NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed')),
    video_position_seconds  integer NOT NULL DEFAULT 0,
    completed_at            timestamptz,
    updated_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (enrollment_id, item_id)
);
CREATE INDEX course_item_progress_enrollment_idx ON course_item_progress (enrollment_id);

CREATE TABLE course_payments (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id      uuid NOT NULL REFERENCES courses (id),
    student_id     uuid NOT NULL REFERENCES users (id),
    provider       text NOT NULL,
    provider_ref   text,
    status         text NOT NULL CHECK (status IN ('requires_payment', 'authorized', 'captured', 'refunded', 'failed')),
    amount_minor   bigint NOT NULL CHECK (amount_minor >= 0),
    currency       currency_code NOT NULL,
    last_error     text NOT NULL DEFAULT '',
    authorized_at  timestamptz,
    captured_at    timestamptz,
    refunded_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (course_id, student_id)
);
CREATE INDEX course_payments_status_idx ON course_payments (status);

CREATE TABLE course_payment_events (
    event_id     text PRIMARY KEY,
    payment_id   uuid NOT NULL REFERENCES course_payments (id),
    type         text NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX course_payment_events_payment_idx ON course_payment_events (payment_id);

ALTER TABLE submissions ADD COLUMN enrollment_id uuid REFERENCES course_enrollments (id) ON DELETE CASCADE;

CREATE UNIQUE INDEX submissions_course_uniq
    ON submissions (resource_id, student_id, enrollment_id) WHERE enrollment_id IS NOT NULL;
CREATE INDEX submissions_enrollment_idx ON submissions (enrollment_id);
