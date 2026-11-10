## 1. Data model

- [x] 1.1 Add `internal/database/migrations/000013_create_branding_settings.up.sql` creating `branding_settings` (`id` PK with `CHECK (id = 1)`, `app_name varchar(60) not null default 'Fintera'`, `logo_path varchar(255)`, `logo_dark_path varchar(255)`, timestamps) and inserting the default row; verified in a rolled-back transaction against the dev database (`make migrate-up` left for deploy).
- [x] 1.2 Add the paired `000013_...down.sql` dropping the table.
- [x] 1.3 Add `internal/models/branding.go` with the `Branding` model, `TableName`, `BrandingResponse`, `ToResponse()` (public URLs as `/uploads/...`), and the `DefaultAppName = "Fintera"` constant.

## 2. Repository and service

- [x] 2.1 Add `internal/repository/branding_repository.go` (`Get`, `Upsert`) and register it in `NewRepositories`.
- [x] 2.2 Add `internal/services/branding_service.go` with `Get` (defaults on missing row), `UpdateName` (trim, 1–60 chars, audit), `UpdateLogo` (variant validation, content sniffing via `http.DetectContentType`, 2 MB cap, `LocalStorage` upload, old-file cleanup, audit), and `RemoveLogo` (clear path, delete file, audit); register it in `NewServices`.

## 3. Handler and routes

- [x] 3.1 Add `internal/handlers/branding_handler.go` with `Show`, `Update`, `UploadLogo`, `DeleteLogo`, Swagger annotations, and `Cache-Control: public, max-age=60` on the public GET; register it in `NewHandlers`.
- [x] 3.2 Register routes in `cmd/api/main.go`: public `v1.GET("/branding", ...)`; admin `PUT /branding`, `POST /branding/logo`, `DELETE /branding/logo`.
- [x] 3.3 Run `make swagger` and commit the regenerated `docs/` output; verify the new endpoints appear in Swagger.

## 4. Tests

- [x] 4.1 Add `internal/services/branding_service_test.go` covering defaults, name validation, variant validation, and cleanup behavior.
- [x] 4.2 Add handler/route tests for public read (`200` unauthenticated), `403` for non-admin writes, `400` for invalid name/file, and successful upload/remove.

## 5. Docs

- [x] 5.1 Update `docs/architecture/api-and-auth.md` (public endpoints list, admin route reference) and `docs/architecture/data-model.md` (`branding_settings` table, persistence note for `STORAGE_PATH`).

## 6. Verification

- [x] 6.1 `go build ./...` and `go test ./...` pass.
- [x] 6.2 `openspec validate --changes` passes.
