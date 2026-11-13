-- Commission payout tracking. NULL paid_at means the commission is still owed.
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS commission_paid_at TIMESTAMPTZ;
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS commission_paid_by BIGINT;
CREATE INDEX IF NOT EXISTS idx_contracts_commission_paid_at ON contracts (commission_paid_at);
