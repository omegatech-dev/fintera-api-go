-- Seller profile: public referral slug, score and commission exception.
ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_slug VARCHAR(80);
ALTER TABLE users ADD COLUMN IF NOT EXISTS commission_rate_override NUMERIC(5,2);
ALTER TABLE users ADD COLUMN IF NOT EXISTS score INTEGER NOT NULL DEFAULT 100;
ALTER TABLE users ADD COLUMN IF NOT EXISTS score_level VARCHAR(20) NOT NULL DEFAULT 'high';
ALTER TABLE users ADD COLUMN IF NOT EXISTS score_override INTEGER;
ALTER TABLE users ADD COLUMN IF NOT EXISTS score_override_reason TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS score_override_by BIGINT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS score_override_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_referral_slug
    ON users (referral_slug)
    WHERE referral_slug IS NOT NULL;

-- Backfill a deterministic slug for sellers that already exist.
UPDATE users
SET referral_slug = 'vendedor-' || id
WHERE role = 'seller' AND (referral_slug IS NULL OR referral_slug = '');

-- Existing sellers start with the neutral score.
UPDATE users
SET score = 100, score_level = 'high'
WHERE role = 'seller' AND score IS NULL;
