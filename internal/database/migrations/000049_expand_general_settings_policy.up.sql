-- Deployment-wide reservation, notification and regional policy.
ALTER TABLE general_settings
    ADD COLUMN IF NOT EXISTS reservation_min_amount NUMERIC(12,2) NOT NULL DEFAULT 5000,
    ADD COLUMN IF NOT EXISTS reservation_due_days INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS down_payment_due_days INTEGER NOT NULL DEFAULT 14,
    ADD COLUMN IF NOT EXISTS down_payment_due_days_with_reservation INTEGER NOT NULL DEFAULT 21,
    ADD COLUMN IF NOT EXISTS email_notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS overdue_reminders_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS upcoming_reminder_days INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS default_currency VARCHAR(3) NOT NULL DEFAULT 'HNL',
    ADD COLUMN IF NOT EXISTS default_locale VARCHAR(8) NOT NULL DEFAULT 'es',
    ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT 'America/Tegucigalpa';
