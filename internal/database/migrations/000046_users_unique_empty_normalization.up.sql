-- Normalize empty unique values so multiple users without RTN/identity can
-- coexist. Postgres UNIQUE allows many NULLs but only one empty string, which
-- made editing historical clients fail with
-- `duplicate key value violates unique constraint "users_rtn_key" (23505)`.

UPDATE users SET rtn = NULL WHERE rtn = '';
UPDATE users SET identity = NULL WHERE identity = '';

-- The inline UNIQUE constraints are replaced by partial unique indexes that
-- exclude empty values, so clients without a tax id never collide again.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_rtn_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_identity_key;

-- idx_users_identity duplicates the dropped constraint; replace it with a
-- partial unique index with the same semantics as the RTN one.
DROP INDEX IF EXISTS idx_users_identity;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_rtn_unique
    ON users(rtn) WHERE rtn IS NOT NULL AND rtn <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_identity_unique
    ON users(identity) WHERE identity IS NOT NULL AND identity <> '';

-- The non-unique helper index stays for the plain rtn lookups/sorting.
CREATE INDEX IF NOT EXISTS idx_users_rtn ON users(rtn);
