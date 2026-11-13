DROP TABLE IF EXISTS seller_commission_rates;

ALTER TABLE general_settings
    DROP COLUMN IF EXISTS commission_unassigned_rate,
    DROP COLUMN IF EXISTS commission_default_rate;

DROP INDEX IF EXISTS idx_contracts_commission_accrual_date;
DROP INDEX IF EXISTS idx_contracts_commission_origin;
ALTER TABLE contracts
    DROP COLUMN IF EXISTS commission_accrual_date,
    DROP COLUMN IF EXISTS commission_origin;
