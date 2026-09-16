-- One teacher profile per account was only a service-side check-then-insert;
-- two concurrent POST /v1/teachers could create two, after which every
-- owner lookup (`SELECT id FROM teachers WHERE user_id = $1` as :one) errors.
-- The database now holds the rule. Multiple NULLs (unclaimed seed profiles)
-- stay allowed — UNIQUE ignores NULLs.
ALTER TABLE teachers
    ADD CONSTRAINT teachers_user_id_uniq UNIQUE (user_id);
