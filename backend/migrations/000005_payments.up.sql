-- Payments module: one payment intent per booking, its webhook-event log (the
-- idempotency guard), and a simplified teacher-earnings ledger.
--
-- The app talks to a payments.Provider port; the MVP ships a deterministic fake
-- (Stripe does not operate in Uzbekistan). A real Payme / Click / Uzum adapter
-- drops in behind the same port later without touching this schema.
--
-- payment status lifecycle:
--   requires_payment -> authorized -> captured
--   requires_payment | authorized -> failed        (decline / voided intent)
--   authorized | captured -> refunded              (cancel / refund)
--
-- Money is integer minor units everywhere (never float), matching bookings and
-- openapi.yaml's Money schema.

CREATE TABLE payments (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Exactly one payment per booking.
    booking_id     uuid NOT NULL UNIQUE REFERENCES bookings (id),

    provider       text NOT NULL,
    -- The provider's own reference for the intent/charge. NULL until the first
    -- successful Authorize.
    provider_ref   text,

    status         text NOT NULL
        CHECK (status IN ('requires_payment', 'authorized', 'captured', 'refunded', 'failed')),

    amount_minor   bigint NOT NULL CHECK (amount_minor >= 0),
    currency       text NOT NULL,

    -- Last provider-reported failure message, for surfacing to the payer. Empty
    -- when there is nothing to report.
    last_error     text NOT NULL DEFAULT '',

    authorized_at  timestamptz,
    captured_at    timestamptz,
    refunded_at    timestamptz,

    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX payments_status_idx ON payments (status);

-- Webhook idempotency. Every provider state change carries a stable event_id;
-- we insert-on-first-see and treat a duplicate-key violation (SQLSTATE 23505)
-- as "already processed, no-op". The PRIMARY KEY is the race-safe dedupe gate —
-- never a check-then-insert.
CREATE TABLE payment_events (
    event_id     text PRIMARY KEY,
    payment_id   uuid NOT NULL REFERENCES payments (id),
    type         text NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX payment_events_payment_idx ON payment_events (payment_id);

-- Simplified teacher-earnings ledger: one row per captured booking.
--   held      -> money captured from the student, not yet payable
--   available -> payable to the teacher
--   reversed  -> refunded to the student, no longer owed
--
-- For the MVP a row is created 'held' on capture and moved to 'available'
-- immediately (no clearing window).
-- TODO(payouts): a real clearing/hold period before held -> available, plus a
-- payout run that pays out 'available' rows and marks them paid.
CREATE TABLE payout_ledger (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    teacher_id    uuid NOT NULL REFERENCES teachers (id),
    booking_id    uuid NOT NULL REFERENCES bookings (id),

    amount_minor  bigint NOT NULL CHECK (amount_minor >= 0),
    currency      text NOT NULL,

    state         text NOT NULL
        CHECK (state IN ('held', 'available', 'reversed')),

    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    -- At most one ledger row per booking.
    CONSTRAINT payout_ledger_booking_uniq UNIQUE (booking_id)
);

CREATE INDEX payout_ledger_teacher_idx ON payout_ledger (teacher_id, state);
