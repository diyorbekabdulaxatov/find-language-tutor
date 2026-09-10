DROP INDEX IF EXISTS teachers_user_id_idx;
ALTER TABLE teachers DROP COLUMN IF EXISTS user_id;

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

-- citext was added by the up migration; drop it if nothing else needs it.
DROP EXTENSION IF EXISTS citext;
