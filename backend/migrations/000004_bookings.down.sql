DROP TABLE IF EXISTS bookings;

-- btree_gist is left in place: migration 000002 (teacher_availability) still
-- depends on it and owns its lifecycle.
