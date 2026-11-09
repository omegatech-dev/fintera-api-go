---
description: Create a paired golang-migrate up/down migration following repo conventions.
agent: build
---

Create a new database migration for: $ARGUMENTS

Steps:

1. Inspect `internal/database/migrations/` to find the highest existing
   version.
2. Create the next zero-padded version pair:
   `NNNNNN_name.up.sql` and `NNNNNN_name.down.sql`.
3. Use a lowercase snake_case name based on the requested change.
4. Write reversible SQL. The `down` file must undo the `up` file.
5. Do not modify existing migrations; they are immutable.
6. Do not run `make migrate-up` unless the user explicitly asks.
7. Update `docs/architecture/data-model.md` if the schema or relationships
   change.

Report the created file paths and a one-line summary of each.
