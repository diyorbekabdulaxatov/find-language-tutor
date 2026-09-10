-- Reverse of 000013_auth_recovery.up.sql.
DROP TABLE IF EXISTS auth_tokens;

ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified_at;
