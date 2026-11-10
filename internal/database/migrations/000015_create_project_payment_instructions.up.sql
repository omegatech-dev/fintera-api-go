CREATE TABLE IF NOT EXISTS project_payment_instructions (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    bank_name VARCHAR(120) NOT NULL,
    beneficiary_name VARCHAR(160) NOT NULL,
    account_number VARCHAR(64) NOT NULL,
    account_type VARCHAR(20) NOT NULL DEFAULT 'savings',
    notes TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_payment_instructions_project
    ON project_payment_instructions(project_id, position);
