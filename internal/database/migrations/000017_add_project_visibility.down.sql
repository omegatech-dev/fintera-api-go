DROP INDEX IF EXISTS idx_projects_public;
ALTER TABLE projects DROP COLUMN IF EXISTS is_active;
ALTER TABLE projects DROP COLUMN IF EXISTS is_published;
