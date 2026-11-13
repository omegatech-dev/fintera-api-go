ALTER TABLE project_payment_instructions
    ADD COLUMN currency VARCHAR(3) NOT NULL DEFAULT 'HNL',
    ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;

WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY position ASC, id ASC) AS rn
    FROM project_payment_instructions
)
UPDATE project_payment_instructions AS p
SET is_default = TRUE
FROM ranked AS r
WHERE p.id = r.id AND r.rn = 1;
