ALTER TABLE project_payment_instructions
    DROP COLUMN IF EXISTS is_default,
    DROP COLUMN IF EXISTS currency;
