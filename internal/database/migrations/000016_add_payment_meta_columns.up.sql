ALTER TABLE payments ADD COLUMN IF NOT EXISTS authorization_number VARCHAR(64);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS payer_account_name VARCHAR(64);
