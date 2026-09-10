-- Reverse of 000006_lessons.up.sql.
ALTER TABLE bookings DROP COLUMN IF EXISTS no_show_party;
ALTER TABLE bookings DROP COLUMN IF EXISTS meeting_url_override;
ALTER TABLE teachers DROP COLUMN IF EXISTS meeting_url;
