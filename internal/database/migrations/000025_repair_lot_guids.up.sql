-- Lot updates used to overwrite the public GUID with an empty string because the
-- repository saves every field. That leaves one row holding '' and makes the
-- unique idx_lots_guid index reject updates for every other lot.
UPDATE lots
SET guid = gen_random_uuid()::text
WHERE guid IS NULL OR btrim(guid) = '';
