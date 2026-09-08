-- Reverse of 000005_payments.up.sql. Drop in FK order: payout_ledger and
-- payment_events reference payments (and bookings / teachers), payments
-- references bookings.
DROP TABLE IF EXISTS payout_ledger;
DROP TABLE IF EXISTS payment_events;
DROP TABLE IF EXISTS payments;
