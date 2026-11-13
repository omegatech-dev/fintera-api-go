-- Configurable acceptance window for manual/automatic seller assignments.
ALTER TABLE general_settings
    ADD COLUMN IF NOT EXISTS commission_acceptance_window_hours INTEGER NOT NULL DEFAULT 2;
