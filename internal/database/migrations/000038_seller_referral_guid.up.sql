-- Replace the predictable seller referral slug with an opaque GUID.
ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_guid UUID;
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS referral_guid TEXT;

-- Every existing seller gets a GUID; contracts keep traceability of the link
-- that attributed them before the slug columns disappear.
UPDATE users
SET referral_guid = gen_random_uuid()
WHERE role = 'seller' AND referral_guid IS NULL;

UPDATE contracts
SET referral_guid = referral_slug
WHERE referral_guid IS NULL AND referral_slug IS NOT NULL AND referral_slug <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_referral_guid
    ON users (referral_guid)
    WHERE referral_guid IS NOT NULL;

DROP INDEX IF EXISTS idx_users_referral_slug;
ALTER TABLE users DROP COLUMN IF EXISTS referral_slug;
ALTER TABLE contracts DROP COLUMN IF EXISTS referral_slug;
