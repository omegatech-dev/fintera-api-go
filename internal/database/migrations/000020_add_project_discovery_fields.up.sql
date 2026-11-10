-- Discovery fields for the customer portal project directory.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS city VARCHAR(120);
ALTER TABLE projects ADD COLUMN IF NOT EXISTS code VARCHAR(40);

CREATE INDEX IF NOT EXISTS idx_projects_city ON projects(city);
