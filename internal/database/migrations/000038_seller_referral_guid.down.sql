-- Restore the slug columns. The rollback keeps the GUID values as slugs so
-- existing public links continue to resolve without a lossy mapping.
ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_slug VARCHAR(80);
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS referral_slug VARCHAR(80);

UPDATE users
SET referral_slug = referral_guid::text
WHERE referral_guid IS NOT NULL;

UPDATE contracts
SET referral_slug = referral_guid
WHERE referral_guid IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_referral_slug
    ON users (referral_slug)
    WHERE referral_slug IS NOT NULL;

DROP INDEX IF EXISTS idx_users_referral_guid;
ALTER TABLE users DROP COLUMN IF EXISTS referral_guid;
ALTER TABLE contracts DROP COLUMN IF EXISTS referral_guid;
