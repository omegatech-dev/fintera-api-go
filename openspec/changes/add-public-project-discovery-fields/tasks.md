## 1. Data model

- [ ] 1.1 Add `internal/database/migrations/000020_add_project_discovery_fields.up.sql` adding nullable `city VARCHAR(120)` and `code VARCHAR(40)` to `projects`, plus an index on `city`.
- [ ] 1.2 Add the paired `000020_add_project_discovery_fields.down.sql` dropping the index and both columns.
- [ ] 1.3 Add `City *string` and `Code *string` to `models.Project` and `models.ProjectResponse` (and `ToResponse`).

## 2. Public payload

- [ ] 2.1 Extend `models.PublicProjectStats` with `FinancedLots` and `PaidLots` while keeping `SoldLots`.
- [ ] 2.2 Extend `models.PublicProjectResponse` with `City`, `Code`, `Description`, `CreatedAt`, `PriceFrom`, `PriceTo` and `DrawingCount`.
- [ ] 2.3 In `services.PublicCatalogService`, compute the price range from `Lot.EffectivePrice()` for non-inactive lots, count `project.Drawings`, and split financed/paid stats.
- [ ] 2.4 Copy the new project fields in `publicProjectResponse`.

## 3. Repository

- [ ] 3.1 Preload `Drawings` in `projectRepository.ListPublished` and `projectRepository.FindPublishedByGUID`.
- [ ] 3.2 Confirm no admin-only fields leak into the public response.

## 4. Seeds and documentation

- [ ] 4.1 Update `internal/database/seeds/seeds.sql` with realistic `city` and `code` values for the demo projects.
- [ ] 4.2 Update `docs/architecture/data-model.md` with the new `projects` columns and the public payload additions.
- [ ] 4.3 Run `make swagger` if handler annotations change and commit the regenerated `docs/` output.

## 5. Tests

- [ ] 5.1 Extend `internal/services/public_catalog_service_test.go` for `city`, `code`, `description`, `created_at`, `price_from`, `price_to`, `drawing_count`, and the financed/paid split.
- [ ] 5.2 Add a test for a project without lots (null range) and without city/code (null fields).
- [ ] 5.3 Add a test asserting the public payload does not contain admin-only fields.

## 6. Verification

- [ ] 6.1 `go build ./...` passes.
- [ ] 6.2 `go test ./...` passes.
- [ ] 6.3 `openspec validate --changes` passes.
