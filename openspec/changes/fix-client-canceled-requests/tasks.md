# Tasks

## 1. Log classification

- [x] 1.1 In `pkg/logger/gorm.go`, add `isClientCanceled(err)` and log
      `context.Canceled` in `Trace` at debug as `SQL client canceled`, without the
      `SQL Error` label; keep `context.DeadlineExceeded` as an error
- [x] 1.2 Add `TestGormLoggerClientCanceledIsNotAnError` in `pkg/logger/gorm_test.go`
      asserting a wrapped cancellation yields one DEBUG entry

## 2. Preserve cancellation through services

- [x] 2.1 Add `lookupError(err)` in `internal/services/errors.go`: return the
      original error for `context.Canceled`, `ErrNotFound` otherwise
- [x] 2.2 Use it in `PublicCatalogService.GetProject`, `ListLots`, `GetLot`
      (splitting `err != nil || status == active` so the status check still yields
      `ErrNotFound`) and `ListPaymentInstructions`
- [x] 2.3 Use it in `MediaService.ListPublicDrawings`, `ListPublicProjectPhotos`
      and `ListPublicLotPhotos`
- [x] 2.4 Add a service test asserting a canceled lookup surfaces
      `context.Canceled`, not `ErrNotFound`

## 3. Correct HTTP outcome for aborted requests

- [x] 3.1 Add `internal/handlers/respond.go` with `AbortIfCanceled(c, err)`
      writing 499 with no body, plus the local `statusClientClosedRequest` constant
- [x] 3.2 Call it first in every error branch of `internal/handlers/public_handler.go`
      and add the cancellation case to `mapMediaError`
- [x] 3.3 Add `internal/handlers/respond_test.go` covering 499 for a wrapped
      cancellation and unchanged handling for `nil`, database errors, and
      `DeadlineExceeded`

## 4. Local log capture

- [x] 4.1 In `pkg/logger/logger.go`, add `logOutput()` returning
      `io.MultiWriter(os.Stdout, file)` when `LOG_FILE` is set, appending and
      creating the file, with a stderr warning and stdout-only fallback when the
      path is unusable
- [x] 4.2 Track the opened handle, replace it on re-`Setup`, and add `logger.Close()`;
      call `defer logger.Close()` in `cmd/api/main.go`
- [x] 4.3 Add tests for duplication, append-across-restart, and the unusable-path
      fallback in `pkg/logger/logger_test.go`
- [x] 4.4 `Makefile`: `export LOG_FILE`, and have `make dev` create the file's
      parent directory and print the path
- [x] 4.5 Document the variable in `.env.example` (previously advertised but
      unimplemented) and ignore `trace.log` / `logs/` in `.gitignore`

## 5. Verification and docs

- [x] 5.1 `gofmt -l` clean, `go build ./...`, `go test ./...` and `make lint` pass
- [x] 5.2 Update `docs/guides/operations.md`: 499 for disconnected clients in the
      request-log entry, and the two debug-level benign SQL cases in the SQL-log entry
- [x] 5.3 Update `docs/guides/development.md` with a "Capturing logs to a file"
      section and the `LOG_FILE` row, plus the `make dev` line in `AGENTS.md`
- [x] 5.4 Manual check against the portal: with `LOG_FILE=trace.log make dev`, the
      14:37 traffic reproduced 7 `SQL Error` lines; after the fix the same
      navigation logs zero `SQL Error` lines, no false 404s, and the aborted
      requests as `status: 499`
- [ ] 5.5 Confirm in staging that no `SQL Error ... context canceled` line appears
      under normal portal traffic

## 6. Follow-ups (other repo)

- [ ] 6.1 Find why FinteraUI aborts and refires the same portal requests (likely a
      dev-mode double mount or an effect cleanup racing the fetch). Harmless now,
      but it doubles public catalog traffic
