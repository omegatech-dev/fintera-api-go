-- Reverting fails when a row holds a value above the old NUMERIC(10,2) limit.
ALTER TABLE payments ALTER COLUMN amount TYPE NUMERIC(10,2);
ALTER TABLE payments ALTER COLUMN interest_amount TYPE NUMERIC(10,2);
