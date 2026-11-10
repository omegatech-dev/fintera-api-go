## Why

Admins can currently edit a project but cannot control whether it is published
to the customer portal. The project list also has no cover image, so cards fall
back to a gradient even when the project gallery has photos.

## What Changes

- Add `POST /api/v1/projects/:id/publish` and `POST /api/v1/projects/:id/unpublish`.
  Both are admin-only, change only `is_published`, and leave `is_active` alone.
- Write an audit log entry for each visibility change.
- Add `cover_photo_url` to the admin project response, populated from the first
  gallery photo by position.
- Cover the new routes and response field with service/handler tests and Swagger.

## Capabilities

### New Capabilities

- `project-publishing`: publish/unpublish endpoints and the cover-photo field
  exposed on admin project responses.

### Modified Capabilities

None.

## Impact

- `cmd/api/main.go` (admin project routes)
- `internal/handlers/project_handler.go`
- `internal/services/project_service.go`
- `internal/models/project.go`
- `internal/services/project_service_test.go`
- `internal/handlers/*_test.go`
- `docs/` Swagger and `docs/architecture/api-and-auth.md`
- No database migration: `is_published` already exists.
