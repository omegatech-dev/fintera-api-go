## Context

See `proposal.md`. FinteraAPI already has local file storage
(`internal/storage/local.go`) whose `uploads/` subtree is served statically at
`/uploads`, a working RBAC middleware (`RequireAdmin`), an `AuditService`, and
paired golang-migrate migrations. Branding is a small, low-risk addition; the
design reuses those mechanisms instead of introducing new infrastructure.

## Goals / Non-Goals

**Goals:**

- One global branding record editable by admins without a redeploy.
- Public, unauthenticated read for both frontends.
- Safe file upload with validation and cleanup of replaced files.
- Defaults that let fresh databases and unavailable APIs still render.

**Non-Goals:**

- Multi-tenant branding (one branding record per deployment).
- Rebranding email templates, PDF reports, or build-time PWA manifests.
- SVG uploads (script-bearing SVG served from the API origin is an XSS risk).
- Remote/cloud object storage.

## Decisions

### Singleton table `branding_settings`

A single row (`CHECK (id = 1)`) keeps queries trivial (`First(&b, 1)`) and
matches the domain: branding is per deployment, not per user or project. The
migration inserts the default row so `up` + seeds leave a usable state.

Alternative: a generic key/value `app_settings` table. Rejected for now —
typed columns give validation and Swagger clarity; generalizing later is a
simple migration.

### Public read, admin write

`GET /api/v1/branding` is registered directly on the `v1` group (like health
and auth) because both frontends need branding before login. Writes live in the
existing `admin` group and inherit `RequireAdmin`. No new middleware.

### Storage via `LocalStorage`, URLs relative to `/uploads`

Files are saved with `storage.Upload(file, header, "uploads/branding")`, which
lands under `StoragePath/uploads/branding/YYYY/MM/<id>.<ext>` and is served at
`/uploads/branding/...`. The DB stores the relative path; responses return
`"/" + path`. This matches how profile pictures are addressed and keeps the
frontends free to prefix their configured API origin.

### Validation in the service, not the handler

The service sniffs the first 512 bytes with `http.DetectContentType` (client
headers are untrusted), rejects anything but `image/png`, `image/jpeg`,
`image/webp`, enforces a 2 MB cap, and trims/length-checks `app_name`
(1–60 characters). Handlers stay thin like the rest of the codebase.

### File cleanup on replace/remove

When a variant is replaced or removed, the previous file is deleted after the
row is updated successfully. Deletion failure is logged, not fatal — an orphan
file is preferable to a broken branding record.

### Cache headers

The public GET sets `Cache-Control: public, max-age=60`: cheap to serve, fresh
enough for an admin edit to appear within a minute, and compatible with the
frontends' own localStorage caching.

### Audit

Reuse `AuditService.Log` with action `UPDATE_BRANDING`, entity `Branding`,
entity ID `1`, mirroring how profile picture updates are audited.

## Risks / Trade-offs

- **Orphaned files on edge failures** → deletion is best-effort; storage growth
  is bounded by admin edit frequency.
- **Uploaded logo invisible on dark backgrounds** → solved in the frontends by
  an optional dark variant; the API just stores both.
- **`STORAGE_PATH` not persistent in production** → same requirement as
  receipts/profile pictures; must be a mounted volume (Railway) or logos
  disappear on redeploy. Documented in the data-model doc.
- **Rollback** → `make migrate-down` drops the table; the routes and wiring are
  additive and frontends keep bundled fallbacks if the endpoint disappears.

## Migration Plan

1. Ship the API (migration is backward compatible; nothing reads it yet).
2. Ship FinteraUI and FinteraWeb, which fetch branding with bundled fallbacks.

## Open Questions

None.
