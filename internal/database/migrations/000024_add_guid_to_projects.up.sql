-- Backfill public GUIDs for projects created before the consolidated schema
-- existed. Legacy/Rails databases can have projects with a missing column or
-- NULL/empty values, which breaks every public portal URL keyed by project GUID.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS guid VARCHAR(255);

UPDATE projects
SET guid = gen_random_uuid()::text
WHERE guid IS NULL OR guid = '';

-- Keep the migration safe on messy legacy data: duplicates would make the
-- unique index creation fail. Keep the first row per GUID and regenerate the rest.
UPDATE projects p
SET guid = gen_random_uuid()::text
FROM (
  SELECT id
  FROM (
    SELECT id, row_number() OVER (PARTITION BY guid ORDER BY id) AS rn
    FROM projects
    WHERE guid IS NOT NULL AND guid <> ''
  ) dupes
  WHERE rn > 1
) d
WHERE p.id = d.id;

ALTER TABLE projects ALTER COLUMN guid SET NOT NULL;

-- New rows always get a GUID even when inserted outside the Go service.
ALTER TABLE projects ALTER COLUMN guid SET DEFAULT gen_random_uuid()::text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_guid ON projects(guid);
