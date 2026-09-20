-- One trial per student per teacher (italki's rule). The 000025 comment said
-- nothing limited trial bookings; this is where that changes. As with the
-- double-booking EXCLUDE constraints, the DB is the authority: the service
-- pre-checks for a friendly error, and a lost race surfaces here as 23505.
--
-- Cancelled trials do not count — a student whose trial was cancelled (by
-- either side) may book another one. Every other status does, including a
-- pending_payment reservation, so a student cannot hold several open trials.
CREATE UNIQUE INDEX bookings_one_trial_per_student_idx
    ON bookings (teacher_id, student_id)
    WHERE is_trial AND status <> 'cancelled';
