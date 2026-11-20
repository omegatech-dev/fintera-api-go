## 1. Branch and migration

- [x] 1.1 Create branch `fix/historical-clients-rtn-and-evidence` from `main`
- [x] 1.2 Add migration pair `internal/database/migrations/<next>_users_unique_empty_normalization.up.sql` and `.down.sql`: normalize `UPDATE users SET rtn = NULL WHERE rtn = ''` and same for `identity`; drop `users_rtn_key`/`users_identity_key` constraints; create partial unique indexes `idx_users_rtn_unique` and `idx_users_identity_unique` excluding NULL and `''`; `down` drops indexes and restores original constraints
- [x] 1.3 Update `docs/architecture/data-model.md` with the new partial unique indexes and NULL semantics

## 2. Empty unique-field handling (rtn/identity)

- [x] 2.1 In `internal/repository/user_repository.go`: normalize empty `rtn`/`identity` to NULL before full `Save` in `Update`; keep `CreateHistoricalClient` behavior
- [x] 2.2 In `internal/services/user_service.go` (`Create`) and `internal/handlers/user_handler.go` (`Create`): insert NULL instead of `''` when RTN/identity are empty (handled at the repository layer via `Omit(emptyUniqueColumns...)` so every caller benefits)
- [x] 2.3 Add test: editing two different historical clients without RTN succeeds (service-level, mocked repo plus a repository-level test against the partial-index semantics if a DB test harness exists)
- [x] 2.4 Add test: creating a user without RTN/identity stores NULL, and duplicate non-empty RTN is still rejected

## 3. Evidence upload endpoint

- [x] 3.1 In `internal/handlers/historical_sale_handler.go`: add `UploadEvidence` handler (multipart field `file`, `http.MaxBytesReader` 10 MB, `storage.IsValidContentType`, `LocalStorage.Upload` to `evidence/YYYY/MM/`, persist relative path via `HistoricalSaleService`, audit log `HISTORICAL_EVIDENCE_UPLOAD`) with Swagger annotations
- [x] 3.2 Extend `internal/services/historical_sale_service.go` with `SetEvidence(ctx, id, actorID, isAdmin, path, ip, userAgent)` enforcing creator-or-admin permission
- [x] 3.3 Register `POST /historical_sales/:id/evidence` in `cmd/api/main.go` (sellerAdmin group)

## 4. Evidence view/download endpoint

- [x] 4.1 In `internal/handlers/historical_sale_handler.go`: add `DownloadEvidence` handler resolving `storage.SafeFullPath`, streaming via `c.File` with `Content-Disposition: inline` (or `attachment` when `download=1`), returning stored `http(s)` URLs as JSON for legacy values, `404` when the file is missing, with Swagger annotations
- [x] 4.2 Register `GET /historical_sales/:id/evidence` in `cmd/api/main.go` (sellerAdmin group)

## 5. Tests for evidence endpoints

- [x] 5.1 Handler tests: seller uploads to own record (200), admin uploads to any record (200), seller to another's record (403), unsupported type and oversized file rejected, missing file (400)
- [x] 5.2 Handler tests: admin views inline, admin downloads with `download=1`, seller views own, seller blocked on other's (403), missing file (404), legacy URL returned
- [x] 5.3 Update existing historical-sale tests that now need the storage dependency (no existing tests constructed the handler; new `internal/handlers/historical_sale_evidence_test.go` covers the wiring)

## 6. Docs and verification

- [x] 6.1 Regenerate Swagger with `make swagger` and verify the two new endpoints appear in `docs/`
- [x] 6.2 Update `docs/architecture/api-and-auth.md` route list and `docs/architecture/domain-flows.md` evidence step
- [x] 6.3 Run `go build ./...`, `go test ./...`, `make fmt`, `make lint` (if installed), `make security` (if installed)
- [x] 6.4 Verify `git diff` contains only intended changes and no secrets
