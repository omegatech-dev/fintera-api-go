## Why

The admin UI now accepts SVG, PNG, JPG/JPEG and PDF plan uploads, but the API
still only maps `.dwg`, `.dxf`, `.pdf` and `.svg`. Uploading a PNG or JPG plan
returns `422 Unprocessable Entity` with "tipo de archivo no permitido", so the
new supported formats cannot actually be uploaded.

DWG/DXF are no longer accepted by the UI (the portal cannot render them), so the
API should enforce the same set.

## What Changes

- Accept `.svg`, `.png`, `.jpg`, `.jpeg` and `.pdf` for project drawings.
- Remove `.dwg` and `.dxf` from the accepted drawing extensions.
- Add a drawing content-type check (SVG, PNG, JPEG, PDF, octet-stream/empty for
  clients that do not send one), mirroring the photo upload validation.
- Add `drawing_type` constants for PNG/JPG/JPEG; keep DWG/DXF constants for
  legacy rows.
- Update Swagger documentation and tests.

## Capabilities

### New Capabilities

- `project-plan-formats`: the plan file formats the API accepts and exposes.

### Modified Capabilities

None.

## Impact

- `internal/models/media.go`
- `internal/services/media_service.go`
- `internal/services/media_service_test.go`
- `internal/handlers/media_handler.go` (Swagger annotation)
- `docs/` (regenerated Swagger)
- Consumers: FinteraUI plan uploads and the FinteraWeb plan viewer. Existing
  DWG/DXF rows keep their stored `file_type`.
