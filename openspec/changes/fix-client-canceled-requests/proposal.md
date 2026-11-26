# Treat client cancellations as normal flow, and capture dev logs to a file

## Why

Local logs from 2026-11-25 show a burst of `ERROR SQL Error ...` lines while the
customer portal loads, all of them with `error="context canceled"`:

```
ERROR SQL Error 0.3ms: SELECT * FROM "projects" WHERE is_published = $1 ... error="context canceled"
ERROR Incoming request ... GET /api/v1/public/projects status:500
ERROR SQL Error 0.0ms: SELECT * FROM "projects" WHERE guid = $1 ...       error="context canceled"
WARN  Incoming request ... GET /api/v1/public/projects/:guid status:404
INFO  Incoming request ... GET /api/v1/public/projects/:guid status:200   (20 ms later)
```

The browser aborts a request microseconds after firing it and immediately
refires the same call (a dev-mode double fetch in the portal). Nothing is wrong
with the database: the query simply lost its caller. Three defects turn that
benign event into noise and wrong answers:

1. **Every abort is an `SQL Error`.** `pkg/logger/gorm.go` logs any non-`record
   not found` error at ERROR, so aborted clients look like outages. This is the
   same category the analytics change already demoted for cache misses.
2. **Aborts are answered as failures.** The public catalog handlers map any
   error to `500` (`ListProjects`) or `404` (`ShowProject`, `ListLots`,
   `ShowLot`, `PaymentInstructions`, drawings and photos) — the 404s are actively
   misleading, because the project exists and the retry succeeds. The false 404
   is worse than a bad status code: `PublicCatalogService` and the public
   `MediaService` methods collapse every repository error, cancellation included,
   into `ErrNotFound`, so the information is lost before the handler can act on
   it.
3. **Dev logs cannot be read.** Logs go only to stdout, so the lines that matter
   are scroll-truncated in the terminal and cannot be grepped after the fact.
   `.env.example` already advertised a `LOG_FILE` variable that the code never
   implemented.

## What Changes

- Classify `context.Canceled` in the GORM logger as normal flow: a `debug`
  `SQL client canceled` line instead of an `SQL Error`. `context.DeadlineExceeded`
  stays an error, because a timeout is a real problem.
- Preserve cancellation through the service layer with a shared lookup
  classifier, so an aborted caller is never reported as a missing resource.
- Answer aborted requests with **499** (nginx "Client Closed Request") and no
  body in the public catalog handlers, instead of 500/404. Other error branches
  are unchanged.
- Implement `LOG_FILE`: when set, every record is duplicated from stdout into
  that path, so `make dev` runs can be grepped after the terminal scrolls past.
  `make dev` creates the parent directory and announces the file; `logger.Close`
  releases the handle at shutdown.
- Document both in `docs/guides/operations.md` and `docs/guides/development.md`,
  and ignore local log files in git.

No route, request shape, or success response changes. Only requests the client
already abandoned get a new status, and nobody is left to read it.

## Capabilities

### New Capabilities

- `client-cancellation-handling`: aborted client requests end as 499, are never
  reported as missing resources, and never log as SQL errors.
- `local-log-capture`: `LOG_FILE` keeps a greppable copy of structured logs
  during local runs without changing what deployed environments emit.

### Modified Capabilities

None. `openspec/specs/` holds no synced base specs yet; the `diagnostic-logging`
capability proposed in `optimize-analytics-refresh` is extended in spirit (the
same cache-miss reasoning now covers client cancellations) but has no base spec
to modify.

## Impact

- `pkg/logger/gorm.go` (`Trace` cancellation branch) and `gorm_test.go`
- `pkg/logger/logger.go` (`logOutput`, `Close`) and `logger_test.go`
- `cmd/api/main.go` (close the log file on shutdown)
- `internal/services/errors.go` (`lookupError`)
- `internal/services/public_catalog_service.go` (`GetProject`, `ListLots`,
  `GetLot`, `ListPaymentInstructions`)
- `internal/services/media_service.go` (`ListPublicDrawings`,
  `ListPublicProjectPhotos`, `ListPublicLotPhotos`)
- `internal/handlers/respond.go` (new `AbortIfCanceled`) and `respond_test.go`
- `internal/handlers/public_handler.go` (all eight handlers, `mapMediaError`)
- `Makefile` (`export LOG_FILE`, `make dev` creates the log directory),
  `.env.example`, `.gitignore`
- `docs/guides/operations.md`, `docs/guides/development.md`, `AGENTS.md`
- No migration, no new dependency, no change to any other handler group
