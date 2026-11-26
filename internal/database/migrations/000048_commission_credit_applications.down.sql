DROP INDEX IF EXISTS idx_credit_applications_credit_id;
DROP INDEX IF EXISTS idx_credit_applications_contract_id;
DROP INDEX IF EXISTS idx_credit_applications_active;
DROP TABLE IF EXISTS credit_applications;

DROP TRIGGER IF EXISTS trg_legacy_commission_credit_applied_amount
    ON seller_commission_credits;
DROP FUNCTION IF EXISTS set_legacy_commission_credit_applied_amount();

ALTER TABLE seller_commission_credits
    DROP CONSTRAINT IF EXISTS chk_seller_commission_credits_settled_full;

ALTER TABLE seller_commission_credits
    DROP CONSTRAINT IF EXISTS chk_seller_commission_credits_applied_range;

ALTER TABLE seller_commission_credits
    DROP COLUMN IF EXISTS source;

ALTER TABLE seller_commission_credits
    DROP COLUMN IF EXISTS applied_amount;
