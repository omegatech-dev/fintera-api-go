-- Legal identity, contact data and editable terms of service.
ALTER TABLE general_settings
    ADD COLUMN IF NOT EXISTS legal_name VARCHAR(160) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legal_tax_id VARCHAR(60) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legal_address TEXT,
    ADD COLUMN IF NOT EXISTS website VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS terms_effective_date DATE,
    ADD COLUMN IF NOT EXISTS terms_content_es TEXT,
    ADD COLUMN IF NOT EXISTS terms_content_en TEXT;
