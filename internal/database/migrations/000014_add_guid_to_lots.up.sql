-- Add a public GUID to lots so the customer portal can address lots without
-- exposing internal numeric ids.
ALTER TABLE lots ADD COLUMN IF NOT EXISTS guid VARCHAR(255);

UPDATE lots SET guid = gen_random_uuid()::text WHERE guid IS NULL;

ALTER TABLE lots ALTER COLUMN guid SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_lots_guid ON lots(guid);
