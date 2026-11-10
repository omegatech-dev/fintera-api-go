## Why

Project cards can only show one image (`cover_photo_url`), so the UI cannot
present the gallery as a carousel. The gallery already exists in the API and is
managed from the project edit page, but the admin project list response does not
expose it.

## What Changes

- Add `photos` to admin project responses, using the existing public photo
  payload (`id`, `position`, `url`, `thumb_url`, `alt`).
- Limit the card payload to the first photos by position so the list response
  stays small.
- Keep `cover_photo_url` and derive it from the same photo lookup.
- Cover the new field with handler/service tests and Swagger.

## Capabilities

### New Capabilities

- `project-card-gallery`: admin project responses expose a small, ordered
  gallery preview for card carousels.

### Modified Capabilities

None.

## Impact

- `internal/models/project.go`
- `internal/services/media_service.go`
- `internal/handlers/resource_handlers.go`
- `internal/services/media_service_test.go`
- `internal/handlers/project_handler_test.go`
- `docs/` Swagger output
- `docs/architecture/api-and-auth.md`
- No database migration.
