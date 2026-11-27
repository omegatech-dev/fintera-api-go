ALTER TABLE general_settings
    DROP COLUMN IF EXISTS reservation_min_amount,
    DROP COLUMN IF EXISTS reservation_due_days,
    DROP COLUMN IF EXISTS down_payment_due_days,
    DROP COLUMN IF EXISTS down_payment_due_days_with_reservation,
    DROP COLUMN IF EXISTS email_notifications_enabled,
    DROP COLUMN IF EXISTS overdue_reminders_enabled,
    DROP COLUMN IF EXISTS upcoming_reminder_days,
    DROP COLUMN IF EXISTS default_currency,
    DROP COLUMN IF EXISTS default_locale,
    DROP COLUMN IF EXISTS timezone;
