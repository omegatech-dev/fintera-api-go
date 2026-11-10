-- Track when the "your reservation voucher is missing" reminder was sent so
-- the reminder job only contacts a customer once per reservation.
ALTER TABLE contracts
    ADD COLUMN IF NOT EXISTS reservation_reminder_sent_at TIMESTAMPTZ;
