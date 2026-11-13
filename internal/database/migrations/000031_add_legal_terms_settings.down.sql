ALTER TABLE general_settings
    DROP COLUMN IF EXISTS legal_name,
    DROP COLUMN IF EXISTS legal_tax_id,
    DROP COLUMN IF EXISTS legal_address,
    DROP COLUMN IF EXISTS website,
    DROP COLUMN IF EXISTS terms_effective_date,
    DROP COLUMN IF EXISTS terms_content_es,
    DROP COLUMN IF EXISTS terms_content_en;
