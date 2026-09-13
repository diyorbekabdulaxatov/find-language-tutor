-- Phase C3 (part 1): revenue-share payouts for course sales.
--
-- payout_ledger was hardwired to one booking-shaped earning per row
-- (booking_id NOT NULL UNIQUE). A course purchase (phase C2's
-- course_enrollments) needs the exact same clearing-window machinery — held ->
-- available (derived) -> paid/reversed, settled by the same payout run — so
-- rather than a parallel ledger table, this widens the existing one: booking_id
-- becomes nullable, course_enrollment_id is added (also nullable, also
-- UNIQUE — Postgres does not treat multiple NULLs as duplicates, so this
-- allows any number of booking-sourced rows while still keeping at most one
-- ledger row per course enrollment), and a CHECK enforces exactly one of the
-- two is set on every row. The idiom mirrors course_items' kind-discriminated
-- CHECK from migration 000016 (video_asset_id / resource_id), just without a
-- discriminator column since there are only ever the two sources here.
--
-- internal/payouts' own queries (AdminListOwedPayouts, LockPayablePayoutLedger,
-- MarkLedgerRowsPaid, ...) never join bookings — they group by teacher_id/
-- state/payout_batch_id only — so the admin payout dashboard and the payout
-- run itself need no change to pick up course-sourced rows. Only
-- internal/db/queries/payments.sql's ListTeacherEarnings (the teacher-facing
-- GET /v1/payments/me) joins bookings, and is widened separately in Go/SQL
-- alongside this migration.

ALTER TABLE payout_ledger
    ALTER COLUMN booking_id DROP NOT NULL,
    ADD COLUMN course_enrollment_id uuid REFERENCES course_enrollments (id);

ALTER TABLE payout_ledger
    ADD CONSTRAINT payout_ledger_source_check CHECK (
        (booking_id IS NOT NULL AND course_enrollment_id IS NULL)
        OR
        (booking_id IS NULL AND course_enrollment_id IS NOT NULL)
    );

ALTER TABLE payout_ledger
    ADD CONSTRAINT payout_ledger_course_enrollment_uniq UNIQUE (course_enrollment_id);
