-- One row per assignment attempt. The history drives the seller score while
-- contracts keep the current seller/status for fast reads.
CREATE TABLE IF NOT EXISTS assignments (
    id BIGSERIAL PRIMARY KEY,
    contract_id BIGINT NOT NULL,
    seller_id BIGINT NOT NULL,
    origin VARCHAR(20) NOT NULL DEFAULT 'automatic',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accept_deadline_at TIMESTAMPTZ,
    responded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_assignments_contract FOREIGN KEY (contract_id) REFERENCES contracts(id) ON DELETE CASCADE,
    CONSTRAINT fk_assignments_seller FOREIGN KEY (seller_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_assignments_contract_id ON assignments (contract_id);
CREATE INDEX IF NOT EXISTS idx_assignments_seller_status ON assignments (seller_id, status);
CREATE INDEX IF NOT EXISTS idx_assignments_status_deadline ON assignments (status, accept_deadline_at);
CREATE INDEX IF NOT EXISTS idx_assignments_assigned_at ON assignments (assigned_at);

-- At most one active offer per contract.
CREATE UNIQUE INDEX IF NOT EXISTS idx_assignments_pending_contract
    ON assignments (contract_id)
    WHERE status = 'pending';

ALTER TABLE contracts ADD COLUMN IF NOT EXISTS seller_id BIGINT;
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS assignment_origin VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS assignment_status VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMPTZ;
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS accept_deadline_at TIMESTAMPTZ;
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS responded_at TIMESTAMPTZ;
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS referral_slug VARCHAR(80);

CREATE INDEX IF NOT EXISTS idx_contracts_seller_id ON contracts (seller_id);
CREATE INDEX IF NOT EXISTS idx_contracts_assignment_status ON contracts (assignment_status);
CREATE INDEX IF NOT EXISTS idx_contracts_accept_deadline_at ON contracts (accept_deadline_at);
