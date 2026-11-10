## Context

`drawingExtensions` in `MediaService` maps file extensions to `file_type` and is
the gate used by `UploadDrawing`. It currently contains DWG, DXF, PDF and SVG.
The storage layer accepts any content once the extension passes, so adding the
image extensions plus a content-type check keeps the upload safe.

The customer portal renders SVG/PNG/JPG with the plan viewer and PDF with the
browser viewer (FinteraWeb `replace-smplrspace-with-plan-viewer`).

See `proposal.md` and
`openspec/changes/accept-image-plan-formats/specs/project-plan-formats/spec.md`.

## Goals / Non-Goals

**Goals:**

- Make the API accept exactly the plan formats the portal can render.
- Reject DWG/DXF and unrelated files with a clear `422`.
- Keep legacy drawings readable and downloadable.

**Non-Goals:**

- File conversion (CAD to SVG/PDF).
- Restoring in-browser CAD support.
- Changing the 50 MB drawing limit.

## Decisions

### Extension map is the source of truth

`drawingType` already returns `ErrUnsupportedMedia` for unknown extensions, and
the handler turns an all-failed upload into `422`. Extending the map is the
smallest change that aligns the API with the UI.

### Content-type check

The browser sends a `Content-Type` for uploaded files. Accepting only
`image/svg+xml`, `image/png`, `image/jpeg`, `application/pdf` (plus empty or
`application/octet-stream` for non-browser clients) prevents a renamed
executable from passing on extension alone. The existing photo validation uses
the same pattern.

### Keep legacy constants

`DrawingTypeDWG`/`DrawingTypeDXF` remain as constants so existing rows and tests
that reason about stored values keep compiling; they are simply no longer
accepted for new uploads.

## Risks / Trade-offs

- **Existing DWG/DXF plans can no longer be re-uploaded.** Expected; admins
  export to SVG/PDF.
- **Some clients send no content type.** Covered by allowing empty and
  `application/octet-stream`.

## Migration Plan

No schema change. Deploy the API before or with the admin UI change.
