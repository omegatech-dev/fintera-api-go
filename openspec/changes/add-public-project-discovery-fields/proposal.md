## Why

The customer portal's project directory (`FinteraWeb`,
`docs/design/customer-portal/directory.html`) needs a richer discovery payload
than the current public catalog exposes. Today the public list returns the
project name, address, type, delivery date, unit price, aggregate lot counts and
photos, but it cannot power the approved design's city filter, "Publicado"
chip, project code, budget range, plans count, or separate financed/paid
breakdown.

The data already exists in FinteraAPI (`created_at`, `description`, lots with
`effective_price`, `ProjectDrawing` rows), except `city` and `code`, which need
to be persisted and managed by admins.

## What Changes

- Add nullable `city` and `code` columns to `projects` (migration `000020`),
  exposed in the admin `ProjectResponse` and persisted by the existing
  create/update handlers.
- Extend `PublicProjectResponse` with:
  - `city`, `code`, `description`, `created_at`
  - `price_from`, `price_to` (min/max `effective_price` of non-inactive lots)
  - `drawing_count` (published drawings of the project)
- Split the public lot statistics into `available_lots`, `reserved_lots`,
  `financed_lots` and `paid_lots`, keeping `sold_lots` (`financed + paid`) for
  backwards compatibility.
- Preload project drawings (metadata only) in the published project list/detail
  queries so `drawing_count` does not introduce per-project queries.
- Update seeds, service tests, Swagger and the data-model documentation.

## Capabilities

### New Capabilities

- `public-project-catalog`: the unauthenticated project payload consumed by the
  customer portal, including discovery fields and privacy rules.

### Modified Capabilities

None.

## Impact

- `internal/database/migrations/000020_add_project_discovery_fields.*.sql`
- `internal/models/project.go`, `internal/models/public_catalog.go`
- `internal/services/public_catalog_service.go`
- `internal/repository/contract_repository.go`
- `internal/database/seeds/seeds.sql`
- `docs/` (Swagger and architecture/data-model notes)
- Consumers: FinteraWeb directory (this release) and FinteraUI admin forms
  (`add-project-admin-discovery-fields`). The new public fields are additive;
  old clients keep working.
