-- Timeline timestamps for customer reservation tracking.
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS rejected_at TIMESTAMPTZ;
