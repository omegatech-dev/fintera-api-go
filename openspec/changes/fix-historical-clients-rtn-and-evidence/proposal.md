## Why

Editing a historical client (or uploading evidence on a historical sale) fails with
`duplicate key value violates unique constraint "users_rtn_key" (SQLSTATE 23505)`.
Historical clients are created with `rtn = NULL` (via `Omit("RTN")`), but any later
full-row save (GORM `Save`) writes `rtn = ''`, and Postgres UNIQUE allows only one
empty string. Additionally, when a seller uploads evidence (the "respaldo" image)
for a historical sale, the admin cannot view or download it to verify the sale,
because `evidence_file` stores only a text value and no upload/download endpoints
exist.

## What Changes

- Add a migration that normalizes empty `rtn`/`identity` values to `NULL` and
  replaces the UNIQUE constraints with partial unique indexes that ignore empty
  values (`users_rtn_key`, `users_identity_key`).
- Stop writing empty strings into unique user columns on create and update
  (`userRepository.Update` full save, `UserService.Create`, user create handler).
- Add `POST /historical_sales/:id/evidence` (multipart upload): PDF/JPEG/PNG,
  max 10 MB, stored under `STORAGE_PATH/evidence/YYYY/MM/`, relative path saved in
  `evidence_file`. Allowed for the creating seller and admins.
- Add `GET /historical_sales/:id/evidence` to view (`inline`) or download
  (`?download=1`) the evidence file. Admins always; sellers only if they created
  the sale. Legacy `evidence_file` values that are external URLs are returned as-is.
- Register the new routes in `cmd/api/main.go` (seller/admin group), regenerate
  Swagger, and update `docs/architecture/api-and-auth.md`.

## Capabilities

### New Capabilities

- `historical-sale-evidence`: uploading, viewing and downloading the evidence
  ("respaldo") file of a historical sale, with role-based access rules.

### Modified Capabilities

- `historical-client-accounts`: historical/regular clients without RTN or identity
  can be created and edited repeatedly without hitting the unique constraints
  (empty values are stored as `NULL` and excluded from uniqueness).

## Impact

- `internal/database/migrations/` — new numbered up/down pair; data normalization
  required before deploying (existing `''` rows must become `NULL`).
- `internal/repository/user_repository.go` — `Update`, `Create`, `CreateHistoricalClient`.
- `internal/services/user_service.go`, `internal/handlers/user_handler.go` — empty
  unique-field handling.
- `internal/handlers/historical_sale_handler.go`,
  `internal/services/historical_sale_service.go` — new evidence endpoints.
- `cmd/api/main.go` — new routes; frontend must point the drag-and-drop upload at
  `POST /historical_sales/:id/evidence` (separate frontend repo change).
- Swagger docs in `docs/` regenerated via `make swagger`.
