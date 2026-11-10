## 1. Service and model

- [x] 1.1 Add `CoverPhotoURL *string json:"cover_photo_url"` to `ProjectResponse` in `internal/models/project.go` and verify the struct builds.
- [x] 1.2 Add a project service method that sets `is_published`, preserves `is_active`, writes an audit entry, and returns the project; verify with a service test.
- [x] 1.3 Populate `cover_photo_url` from the first project photo by position when building project responses; verify the no-photo case returns `null`.

## 2. HTTP layer

- [x] 2.1 Add `PublishProject` and `UnpublishProject` handlers with Swagger annotations in `internal/handlers/project_handler.go`.
- [x] 2.2 Register `POST /projects/:id/publish` and `POST /projects/:id/unpublish` in the admin group in `cmd/api/main.go`.
- [x] 2.3 Add handler tests for both endpoints, including the non-admin rejection path.

## 3. Documentation and verification

- [x] 3.1 Run `make swagger` and commit the regenerated docs.
- [x] 3.2 Update `docs/architecture/api-and-auth.md` with the new routes.
- [x] 3.3 Run `go build ./...`, `go test ./...`, and confirm the change validates with `openspec validate add-project-publish-endpoints --strict`.
