DROP INDEX IF EXISTS idx_contracts_commission_paid_at;
ALTER TABLE contracts DROP COLUMN IF EXISTS commission_paid_by;
ALTER TABLE contracts DROP COLUMN IF EXISTS commission_paid_at;
