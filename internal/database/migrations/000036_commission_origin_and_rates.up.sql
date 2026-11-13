-- Commission origin/accrual for reports plus the global rate fallbacks.
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS commission_origin VARCHAR(20) NOT NULL DEFAULT 'direct';
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS commission_accrual_date DATE;

UPDATE contracts
SET commission_accrual_date = approved_at::date
WHERE commission_accrual_date IS NULL AND approved_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_contracts_commission_origin ON contracts (commission_origin);
CREATE INDEX IF NOT EXISTS idx_contracts_commission_accrual_date ON contracts (commission_accrual_date);

ALTER TABLE general_settings ADD COLUMN IF NOT EXISTS commission_default_rate NUMERIC(5,2) NOT NULL DEFAULT 5;
ALTER TABLE general_settings ADD COLUMN IF NOT EXISTS commission_unassigned_rate NUMERIC(5,2) NOT NULL DEFAULT 3;

CREATE TABLE IF NOT EXISTS seller_commission_rates (
    id BIGSERIAL PRIMARY KEY,
    seller_id BIGINT NOT NULL,
    rate NUMERIC(5,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_seller_commission_rates_seller FOREIGN KEY (seller_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT uq_seller_commission_rates_seller UNIQUE (seller_id)
);
