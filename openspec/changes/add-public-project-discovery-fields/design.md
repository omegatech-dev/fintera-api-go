## Context

The public catalog already serves `GET /api/v1/public/projects` and
`GET /api/v1/public/projects/:guid` from `PublicCatalogService`, restricted to
published and active projects. The customer portal now needs the discovery
fields described in `proposal.md`. Most values are derived from existing rows;
only `city` and `code` require persistence.

See `proposal.md` and
`openspec/changes/add-public-project-discovery-fields/specs/public-project-catalog/spec.md`.

## Goals / Non-Goals

**Goals:**

- One additive payload extension that powers the approved directory design.
- Keep the public surface privacy-safe (no rates, commissions or internal
  notes).
- Avoid N+1 queries for drawings and price ranges.
- Backwards compatibility for older portal builds and for projects without the
  new optional fields.

**Non-Goals:**

- Server-side filtering/sorting of the public list; the portal filters the
  small published set client-side.
- A `visa_approved` flag or other marketing-only labels that have no business
  rule in FinteraAPI/FinteraUI.
- Public exposure of lot-level prices per project (only the aggregate range is
  added).
- A new endpoint; the existing public catalog contract is extended.

## Decisions

### Nullable `city` and `code` columns

`projects.city VARCHAR(120)` and `projects.code VARCHAR(40)` are nullable so the
migration is additive and existing records keep working. The admin create/update
handlers already bind the `Project` model, so persisting the fields requires no
handler change. A plain index on `city` supports future server-side filtering
without adding one now.

Alternative: derive `city` from `address` in the portal. Rejected for parity:
free-text addresses are unreliable and the design filters by city.

### Price range from effective lot prices

`price_from` and `price_to` are pointers computed from the non-inactive lots:

- `effective_price = override_price` when set, otherwise the stored lot price.
- Lots with a non-positive price are ignored.
- No eligible lots → both fields are `null`.

This keeps the range consistent with the lot explorer, which already exposes
`effective_price`.

### Drawing count via preload

`ListPublished` and `FindPublishedByGUID` preload `Drawings`. Drawings are
metadata rows (path, name, type, size), not file contents, so the added query is
bounded and avoids a count query per project. If the media volume grows, a
`count(project_drawings)` aggregate can replace the preload later without
changing the response shape.

### Separate financed and paid counts

`PublicProjectStats` keeps `sold_lots` (`financed + paid`) for existing
consumers and adds `financed_lots` and `paid_lots`. The directory can render
accurate status dots and still support older clients.

### Privacy

The public response adds only descriptive/aggregate fields. It does not expose
`interest_rate`, commission rates, internal notes, `uploaded_by`, or lot-level
owner data.

## Risks / Trade-offs

- **Extra preload on lot endpoints** → `FindPublishedByGUID` is also used by
  lot lookups, so drawings are loaded there too. Metadata only; acceptable at
  current volumes. If it becomes hot, split a lite query.
- **Legacy projects without city/code** → fields are `null`; the portal falls
  back to the first address segment and hides the code chip.
- **Budget semantics** → `price_from` is a total lot price, while the card's
  "Desde L X / m²" remains `price_per_square_unit`. The portal labels each
  separately.
- **Rollback** → drop the two columns and remove the new response fields; no
  data migration is needed because all other values are computed.
