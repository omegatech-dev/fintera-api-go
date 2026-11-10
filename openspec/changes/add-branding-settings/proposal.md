## Why

The application name and logo are hardcoded across the frontends (bundled logo
assets, "Fintera" strings, build-time PWA manifest). Rebranding currently
requires a code change and redeploy in two repositories. Storing branding in
the API lets an administrator change it once and have FinteraUI and FinteraWeb
pick it up at runtime.

## What Changes

- Add a singleton `branding_settings` table (migration `000013`) storing
  `app_name`, `logo_path`, and `logo_dark_path`.
- Add a public `GET /api/v1/branding` endpoint returning the current branding,
  or defaults (`Fintera`, null logos) when nothing is configured.
- Add admin-only endpoints:
  - `PUT /api/v1/branding` — update the application name.
  - `POST /api/v1/branding/logo` — multipart logo upload (`light` or `dark`
    variant).
  - `DELETE /api/v1/branding/logo?variant=light|dark` — remove a logo and fall
    back to defaults.
- Store logo files through `LocalStorage` under `uploads/branding/` so they are
  served by the existing `/uploads` static route. Accept PNG/JPEG/WebP, max
  2 MB; SVG is rejected.
- Delete the previous file when a logo is replaced or removed.
- Write an audit log entry for every successful branding change.

## Capabilities

### New Capabilities

- `branding`: global, admin-managed application name and logo exposed publicly
  and consumed by FinteraUI and FinteraWeb.

### Modified Capabilities

None.

## Impact

- `internal/models`, `internal/repository`, `internal/services`,
  `internal/handlers`, `cmd/api/main.go`
- `docs/` (regenerated Swagger), `docs/architecture/api-and-auth.md`,
  `docs/architecture/data-model.md`
- Downstream consumers: FinteraUI (admin branding page, app-wide branding) and
  FinteraWeb (public site branding). Both must render defaults while this
  endpoint is unavailable, so deployment order is non-breaking.
