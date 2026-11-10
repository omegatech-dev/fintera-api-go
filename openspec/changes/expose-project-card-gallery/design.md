## Context

See `proposal.md` — Why. `ProjectHandler.response` already calls
`MediaService.ProjectCoverPhotoURL`, which loads the project gallery once per
project. The gallery repository returns photos ordered by `position, id`, and
`ProjectPhoto.ToResponse()` already builds the public payload used by the media
endpoints.

## Goals / Non-Goals

**Goals:**

- Provide one gallery lookup that feeds both `photos` and `cover_photo_url`.
- Keep list responses bounded with a ten-photo preview.
- Reuse `PhotoResponse` so card thumbnails and alt text match the gallery.

**Non-Goals:**

- Changing the public portal response.
- Pagination for the gallery preview.
- New media routes or uploads.

## Decisions

### Card preview limit

Ten photos is enough for a card carousel while keeping the admin list response
predictable. The complete gallery remains available at
`GET /projects/:id/photos`.

### Cover derived from the same lookup

`ProjectCoverPhotoURL` is replaced by a preview lookup in the handler so the
list performs one gallery query per project instead of two, and the cover can
never disagree with `photos[0]`.

## Risks / Trade-offs

- **Larger list payload** → bounded to ten entries; thumbnails can be used by the
  UI when available.
- **N+1 gallery queries** → unchanged from the current cover lookup. If project
  lists grow, preload photo paths in the project query.
- **Rollback** → remove the `photos` field and preview helper; no migration.
