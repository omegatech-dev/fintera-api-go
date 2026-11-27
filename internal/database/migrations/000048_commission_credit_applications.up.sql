-- Track how much of each seller commission credit has been consumed, where it
-- came from, and which contract consumed it. This enables netting credits at
-- payout time and restoring them when that payout is reversed.

ALTER TABLE seller_commission_credits
    ADD COLUMN IF NOT EXISTS applied_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'legacy';

-- Credits settled by hand before this change have no applications. Treat them
-- as fully applied legacy credits and never reverse them automatically.
UPDATE seller_commission_credits
   SET applied_amount = amount
 WHERE status = 'settled'
   AND applied_amount = 0;

UPDATE seller_commission_credits
   SET source = 'legacy'
 WHERE source = '';

-- Keep old API instances compatible during a rolling deployment. Before the
-- new repository version is live, an older manual-apply request changes only
-- status; promote that legacy full settlement to applied_amount = amount.
CREATE OR REPLACE FUNCTION set_legacy_commission_credit_applied_amount()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'settled'
       AND OLD.status <> 'settled'
       AND NEW.applied_amount = OLD.applied_amount THEN
        NEW.applied_amount := NEW.amount;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_legacy_commission_credit_applied_amount
    ON seller_commission_credits;

CREATE TRIGGER trg_legacy_commission_credit_applied_amount
    BEFORE UPDATE OF status ON seller_commission_credits
    FOR EACH ROW
    EXECUTE FUNCTION set_legacy_commission_credit_applied_amount();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_seller_commission_credits_applied_range'
    ) THEN
        ALTER TABLE seller_commission_credits
            ADD CONSTRAINT chk_seller_commission_credits_applied_range
            CHECK (applied_amount >= 0 AND applied_amount <= amount);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_seller_commission_credits_settled_full'
    ) THEN
        ALTER TABLE seller_commission_credits
            ADD CONSTRAINT chk_seller_commission_credits_settled_full
            CHECK (status <> 'settled' OR applied_amount = amount);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS credit_applications (
    id BIGSERIAL PRIMARY KEY,
    credit_id BIGINT NOT NULL,
    contract_id BIGINT NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    actor_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reversed_at TIMESTAMPTZ,
    reversed_by BIGINT,
    CONSTRAINT fk_credit_applications_credit FOREIGN KEY (credit_id) REFERENCES seller_commission_credits(id),
    CONSTRAINT fk_credit_applications_contract FOREIGN KEY (contract_id) REFERENCES contracts(id),
    CONSTRAINT fk_credit_applications_actor FOREIGN KEY (actor_id) REFERENCES users(id),
    CONSTRAINT fk_credit_applications_reversed_by FOREIGN KEY (reversed_by) REFERENCES users(id),
    CONSTRAINT chk_credit_applications_amount CHECK (amount > 0)
);

-- Only active applications are unique per credit and contract. A reversed
-- application must not block applying the restored credit to the same contract
-- again after a reassignment.
CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_applications_active
    ON credit_applications (credit_id, contract_id)
    WHERE reversed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_credit_applications_contract_id
    ON credit_applications (contract_id);

CREATE INDEX IF NOT EXISTS idx_credit_applications_credit_id
    ON credit_applications (credit_id);
