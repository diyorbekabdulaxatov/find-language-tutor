-- Teacher moderation (phase B): every teacher profile now carries a moderation
-- status, a verified badge flag, and the admin's note shown to the teacher on a
-- reject / suspend.
--
--   status lifecycle:
--     pending   -> approved | rejected      (admin review of a new profile)
--     approved  -> suspended                (admin pulls a live profile)
--     rejected  -> approved                 (admin reconsiders)
--     suspended -> approved                 (admin reinstates)
--
-- DEFAULT is 'approved' so every existing / seeded teacher stays public through
-- the migration. The API create path (POST /v1/teachers) overrides it to
-- 'pending' — a new profile is not public until an admin approves it.
--
-- Public reads (GET /v1/teachers, GET /v1/teachers/{slug}, the SSG slug list,
-- the slots + reviews sub-resources, and POST /v1/bookings) all hide anything
-- that is not 'approved'. The owner still sees their own profile via
-- GET /v1/teachers/me regardless of status.

ALTER TABLE teachers
    ADD COLUMN status text NOT NULL DEFAULT 'approved'
        CHECK (status IN ('pending', 'approved', 'rejected', 'suspended'));

ALTER TABLE teachers
    ADD COLUMN verified boolean NOT NULL DEFAULT false;

ALTER TABLE teachers
    ADD COLUMN moderation_note text NOT NULL DEFAULT '';

-- Hot path: the admin moderation queue filters by status, and every public read
-- filters status = 'approved'.
CREATE INDEX teachers_status_idx ON teachers (status);
