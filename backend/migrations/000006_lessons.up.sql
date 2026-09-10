-- Phase 5 (lessons): meeting links + no-show handling on top of the existing
-- bookings / payments schema. Reminders (asynq) and transactional emails
-- (Resend) live entirely in the app + worker and need no schema.
--
-- Meeting links:
--   teachers.meeting_url          — the teacher's default video room (may be '')
--   bookings.meeting_url_override — an optional per-booking link ('' = use the
--                                   teacher default)
-- The effective link exposed on a booking is the override if set, else the
-- teacher's meeting_url. The API only ever reveals it to a participant of a
-- confirmed / completed booking.
--
-- No-show:
--   bookings.no_show_party — '' normally; 'student' when the student missed a
--   lesson the teacher showed up for (booking is completed + captured like a
--   normal complete), 'teacher' when the teacher self-reports they could not
--   make it (booking is cancelled + the student refunded).

ALTER TABLE teachers
    ADD COLUMN meeting_url text NOT NULL DEFAULT '';

ALTER TABLE bookings
    ADD COLUMN meeting_url_override text NOT NULL DEFAULT '';

ALTER TABLE bookings
    ADD COLUMN no_show_party text NOT NULL DEFAULT ''
        CHECK (no_show_party IN ('', 'student', 'teacher'));
