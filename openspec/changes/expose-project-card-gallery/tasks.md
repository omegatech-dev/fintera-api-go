## 1. Model and service

- [x] 1.1 Add `Photos []PhotoResponse json:"photos"` to `ProjectResponse` in `internal/models/project.go`.
- [x] 1.2 Add a bounded card-gallery helper to `internal/services/media_service.go` and cover it with tests.

## 2. Handler and docs

- [x] 2.1 Populate `photos` and `cover_photo_url` from one gallery lookup in `internal/handlers/resource_handlers.go`.
- [x] 2.2 Extend `internal/handlers/project_handler_test.go` for ordered photos and the no-photo case.
- [x] 2.3 Run `make swagger` and update `docs/architecture/api-and-auth.md`.
- [x] 2.4 Run `go build ./...`, `go test ./...`, and `openspec validate expose-project-card-gallery --strict`.
