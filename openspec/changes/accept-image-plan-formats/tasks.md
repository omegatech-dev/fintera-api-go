## 1. Model

- [x] 1.1 Add `DrawingTypePNG`, `DrawingTypeJPG` and `DrawingTypeJPEG` constants in `internal/models/media.go`.

## 2. Upload validation

- [x] 2.1 Change `drawingExtensions` to `.svg`, `.png`, `.jpg`, `.jpeg`, `.pdf`.
- [x] 2.2 Extract `validateDrawing` that checks extension, size and content type.
- [x] 2.3 Accept `image/svg+xml`, `image/png`, `image/jpeg`, `application/pdf`, empty and `application/octet-stream` content types.
- [x] 2.4 Use `validateDrawing` in `MediaService.UploadDrawing`.

## 3. Documentation

- [x] 3.1 Update the upload Swagger annotation with the accepted formats.
- [x] 3.2 Run `make swagger` and commit the regenerated docs.

## 4. Tests

- [x] 4.1 Update `TestDrawingType` for PNG/JPG/JPEG/PDF/SVG and rejected DWG/DXF.
- [x] 4.2 Add `TestValidateDrawing` for content types and size.

## 5. Verification

- [x] 5.1 `go build ./...` passes.
- [x] 5.2 `go test ./...` passes.
- [x] 5.3 `openspec validate --changes` passes.
