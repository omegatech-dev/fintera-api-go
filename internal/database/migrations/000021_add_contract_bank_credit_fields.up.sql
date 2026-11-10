-- Bank financing details reported by the customer at reservation time.
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS bank_name VARCHAR(120);
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS bank_credit_status VARCHAR(30);
