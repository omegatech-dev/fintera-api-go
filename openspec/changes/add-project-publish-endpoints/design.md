## Context

See `proposal.md` — Why. Project visibility is stored on `projects.is_published`
and `is_active`; the public portal already requires both to be true. The normal
project update preserves those fields, so visibility needs a separate write
path. Admin project responses are assembled by `Project.ToResponse()` and do not
currently load gallery photos.

## Goals / Non-Goals

**Goals:**

- Add explicit publish and unpublish admin endpoints.
- Keep `is_published` and `is_active` independent.
- Expose the first gallery photo as `cover_photo_url`.
- Keep the existing project CRUD payload unchanged.

**Non-Goals:**

- Changing the public portal queries or response shapes.
- Adding a visibility field to the normal update payload.
- Generating thumbnails or resizing cover photos.

## Decisions

### Separate publish/unpublish routes

`POST /projects/:id/publish` and `POST /projects/:id/unpublish` make the intent
explicit and avoid the edit form silently changing visibility. Both delegate to
one service method that sets only `is_published`, keeping `is_active` intact.

### Cover photo loaded by the service

The project service already has access to the project photo repository. A small
helper reads the first photo by position and returns its public URL. This avoids
per-card requests from the UI. `PhotoResponse.PublicFileURL` is reused so the URL
matches the gallery.

### Nullable cover field

`cover_photo_url` is `*string`/`null` when no photos exist. Clients fall back to
their existing placeholder.

## Risks / Trade-offs

- **Extra query per project list** → one photo lookup per listed project. The
  admin list is paginated and small; if it becomes hot, preload cover paths in
  the project query.
- **Visibility semantics** → a project can be published but inactive (or the
  reverse). This is intentional; the public portal requires both.
- **Rollback** → remove the routes and the response field; no schema migration
  is involved.
