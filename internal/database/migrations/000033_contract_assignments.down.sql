DROP INDEX IF EXISTS idx_contracts_accept_deadline_at;
DROP INDEX IF EXISTS idx_contracts_assignment_status;
DROP INDEX IF EXISTS idx_contracts_seller_id;
ALTER TABLE contracts
    DROP COLUMN IF EXISTS referral_slug,
    DROP COLUMN IF EXISTS responded_at,
    DROP COLUMN IF EXISTS accept_deadline_at,
    DROP COLUMN IF EXISTS assigned_at,
    DROP COLUMN IF EXISTS assignment_status,
    DROP COLUMN IF EXISTS assignment_origin,
    DROP COLUMN IF EXISTS seller_id;

DROP INDEX IF EXISTS idx_assignments_pending_contract;
DROP INDEX IF EXISTS idx_assignments_assigned_at;
DROP INDEX IF EXISTS idx_assignments_status_deadline;
DROP INDEX IF EXISTS idx_assignments_seller_status;
DROP INDEX IF EXISTS idx_assignments_contract_id;
DROP TABLE IF EXISTS assignments;
