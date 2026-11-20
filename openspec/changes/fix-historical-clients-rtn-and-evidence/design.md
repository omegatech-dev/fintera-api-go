## Context

Two related defects in the historical-clients flow:

1. **`users_rtn_key` duplicate key (SQLSTATE 23505).** The `users` table declares
   `rtn VARCHAR(255) UNIQUE` (and `identity` is likewise unique). In Postgres a
   UNIQUE column accepts many `NULL`s but only one empty string. Historical
   clients are created with `Omit("RTN")` (NULL), but every later full-row save
   (GORM `Save` in `userRepository.Update`) writes `rtn = ''` because `NULL`
   scans into the Go `string` as `""`. Once any row holds `''` (staff account or
   a client created via the UI without RTN), every other edit of a client
   without RTN collides — which is exactly what happens when the seller edits a
   historical sale to attach evidence and `SyncUser` saves the linked client.

2. **Evidence ("respaldo") is not viewable.** `historical_sales.evidence_file`
   is a plain `*string` column. There is no upload endpoint and no download/view
   endpoint, so the image the seller drags-and-drops never becomes a real file
   the admin can inspect. The receipts flow (`payment_handler.go`) already
   implements the correct pattern: `LocalStorage.Upload` → relative path in DB →
   download endpoint with `storage.SafeFullPath` + `c.File` and role checks.

Constraints from AGENTS.md: local storage under `STORAGE_PATH`, allowed MIME
types PDF/JPEG/PNG, max 10 MB, `storage.SafeFullPath` for downloads, immutable
numbered migration pairs, `docs/` is the canonical Swagger location.

## Goals / Non-Goals

**Goals:**

- Editing any user without RTN/identity (historical client or not) never fails
  with a unique-constraint violation caused by empty-string values.
- The seller can upload a real evidence file for a historical sale.
- The admin can view the evidence inline in the browser and download it.
- Backward compatibility for existing `evidence_file` values.

**Non-Goals:**

- Changing the historical-sale state machine or validation workflow.
- Migrating storage to S3/cloud or sharing files across replicas.
- Rebuilding the frontend drag-and-drop (the frontend repo change to point at
  the new upload endpoint is tracked separately).

## Decisions

### D1. Partial unique indexes instead of keeping raw UNIQUE constraints

Replace `users_rtn_key` / `users_identity_key` with partial unique indexes:

```sql
CREATE UNIQUE INDEX idx_users_rtn_unique ON users(rtn) WHERE rtn IS NOT NULL AND rtn <> '';
```

- Why: keeps duplicate protection for real values while allowing unlimited
  clients without RTN/identity. Multiple NULLs were already legal; the empty
  string is the only value that behaves differently.
- Alternative considered: change the Go model to `*string` pointers everywhere.
  Rejected for now: touches many handlers/DTOs/tests and still leaves the DB
  fragile (any raw SQL insert with `''` would break).

### D2. Data normalization before the index swap

`UPDATE users SET rtn = NULL WHERE rtn = ''` (and the same for `identity`) must
run before creating the partial index, otherwise the `''` row plus duplicates
would block index creation. The `down` migration restores the original
constraints only after coercing `NULL` back is NOT attempted — down simply
re-adds the original UNIQUE constraints, failing if duplicates exist (rollback
requires manual cleanup, documented in the migration comment).

### D3. Empty-string defense in the repository layer

`userRepository.Update` (full `Save`) and `UserService.Create` normalize empty
`rtn`/`identity` to `NULL` before writing. Implementation choice: convert the
model fields to empty-string-omit by using `Select`/`Omit` guards, or set the
columns explicitly with `gorm.Expr("NULL")` when empty. Keeps the fix local to
the data layer instead of scattering checks across handlers and services.

### D4. Dedicated evidence upload endpoint (multipart)

`POST /historical_sales/:id/evidence` with field `file`, multipart/form-data:

- Allowed MIME: PDF/JPEG/PNG via `storage.IsValidContentType`; max 10 MB via
  `http.MaxBytesReader` (same as receipts).
- Stored at `evidence/YYYY/MM/<random-id>.<ext>` under `STORAGE_PATH` using
  `LocalStorage.Upload`; the returned relative path overwrites
  `historical_sales.evidence_file`.
- Permission: the creating seller or an admin. Upload records an audit log
  (`AuditService.Log`, action `HISTORICAL_EVIDENCE_UPLOAD`).
- Why a dedicated endpoint instead of making `PUT /historical_sales/:id`
  multipart: keeps JSON update semantics untouched, matches the receipts
  pattern, and gives the frontend one obvious endpoint for the drag-and-drop.

### D5. Download/view endpoint with inline vs attachment

`GET /historical_sales/:id/evidence`:

- Resolves `storage.SafeFullPath(evidence_file)` and streams with `c.File`.
- `Content-Disposition: inline` by default (image renders in the browser);
  `?download=1` switches to `attachment` with the original filename hint.
- Access: admin always; seller only if `sale.CreatedBy == actorID`.
- Legacy values: if `evidence_file` looks like an external URL
  (`http://`/`https://`), the endpoint responds `200` with
  `{"evidence_url": "..."}` (or a redirect) so old records still open; if the
  stored path does not exist on disk, respond `404`.

### D6. Route placement

Register both routes in the `sellerAdmin` group of `cmd/api/main.go`
(authenticated admin-or-seller), next to the existing
`POST /historical_sales` / `PUT /historical_sales/:id` routes. Per-sale
ownership checks happen in the handler/service, mirroring the existing
historical-sale permission model.

## Risks / Trade-offs

- [Partial index differs from original schema semantics] → `down` migration
  documents the limitation; rollback to strict UNIQUE requires the data to have
  at most one empty value, which the normalization guarantees going forward
  only if the code fix is deployed together with the migration.
- [LocalStorage is per-replica and may not survive redeploys] → evidence files
  are as durable as receipts already are; acknowledged known gotcha in
  AGENTS.md, no change in this design.
- [Frontend must switch to the new upload endpoint or admins still see nothing]
  → the download endpoint also handles legacy URL values, and this repo's
  Swagger/docs make the contract explicit for the frontend team.
- [Deploy ordering] → migration must run before/with the new binary; the Docker
  image runs migrations on start, so the code fix and migration land in the same
  release.

## Migration Plan

1. Add next numbered pair `NNNNNN_users_unique_empty_normalization.{up,down}.sql`.
2. `up`: normalize `''` → `NULL` for `rtn` and `identity`; drop the two UNIQUE
   constraints; create the two partial unique indexes.
3. `down`: drop the indexes; re-add the original UNIQUE constraints.
4. Deploy with the code changes (Docker image applies migrations on start).
5. Rollback: run `make migrate-down`, then redeploy the previous image.

## Open Questions

- None blocking. During implementation the `HistoricalSaleService.Update`
  behavior was refined per frontend feedback: an update payload without a
  non-empty `evidence_file` preserves the previously uploaded evidence instead
  of clearing it (regression tests included). Two annotated follow-ups, out of
  scope: an explicit endpoint to remove evidence, and persisting the original
  upload filename.
