DROP INDEX IF EXISTS idx_users_referral_slug;
ALTER TABLE users
    DROP COLUMN IF EXISTS score_override_at,
    DROP COLUMN IF EXISTS score_override_by,
    DROP COLUMN IF EXISTS score_override_reason,
    DROP COLUMN IF EXISTS score_override,
    DROP COLUMN IF EXISTS score_level,
    DROP COLUMN IF EXISTS score,
    DROP COLUMN IF EXISTS commission_rate_override,
    DROP COLUMN IF EXISTS referral_slug;
