-- Rollback of the empty-value normalization. The original UNIQUE constraints
-- only accept a single empty string, so this down migration fails if the data
-- has more than one NULL (Postgres allows many NULLs in UNIQUE; re-adding is
-- safe) or more than one empty value. Empty values were normalized to NULL by
-- the up migration and must not be reintroduced before rolling back.

DROP INDEX IF EXISTS idx_users_rtn_unique;
DROP INDEX IF EXISTS idx_users_identity_unique;

DROP INDEX IF EXISTS idx_users_rtn;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_identity ON users(identity);
CREATE INDEX IF NOT EXISTS idx_users_rtn ON users(rtn);

ALTER TABLE users ADD CONSTRAINT users_rtn_key UNIQUE (rtn);
ALTER TABLE users ADD CONSTRAINT users_identity_key UNIQUE (identity);
