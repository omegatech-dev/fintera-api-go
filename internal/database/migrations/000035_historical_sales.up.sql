-- Migrated sales that never existed as a reservation in this system.
CREATE TABLE IF NOT EXISTS historical_sales (
    id BIGSERIAL PRIMARY KEY,
    client_name VARCHAR(255) NOT NULL,
    client_identity VARCHAR(255),
    client_phone VARCHAR(50),
    client_email VARCHAR(255),
    project_name VARCHAR(255) NOT NULL,
    lot_label VARCHAR(255),
    area_m2 NUMERIC(10,2),
    sale_price NUMERIC(15,2) NOT NULL,
    sale_date DATE NOT NULL,
    payment_mode VARCHAR(50),
    sale_status VARCHAR(50),
    installments_paid INTEGER NOT NULL DEFAULT 0,
    rate NUMERIC(5,2),
    commission_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    commission_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    evidence_file VARCHAR(500),
    validation_status VARCHAR(30) NOT NULL DEFAULT 'pending',
    rejection_reason TEXT,
    created_by BIGINT NOT NULL,
    validated_by BIGINT,
    validated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_historical_sales_creator FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT fk_historical_sales_validator FOREIGN KEY (validated_by) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_historical_sales_created_by ON historical_sales (created_by);
CREATE INDEX IF NOT EXISTS idx_historical_sales_validation_status ON historical_sales (validation_status);
CREATE INDEX IF NOT EXISTS idx_historical_sales_sale_date ON historical_sales (sale_date);

-- Duplicate detection for CSV imports. Identity is optional, so the index only
-- covers rows that provide one.
CREATE UNIQUE INDEX IF NOT EXISTS idx_historical_sales_unique_identity
    ON historical_sales (created_by, client_identity, sale_date)
    WHERE client_identity IS NOT NULL AND client_identity <> '';
