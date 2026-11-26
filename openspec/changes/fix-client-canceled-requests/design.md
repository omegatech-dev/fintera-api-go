# Design: client cancellations and local log capture

## Layers affected

`handlers → services → repository → models` stays intact. Cancellation is
detected where each layer can still tell the difference:

| Layer | Concern | Mechanism |
| --- | --- | --- |
| `pkg/logger/gorm.go` | log level only | `errors.Is(err, context.Canceled)` branch in `Trace`, beside the existing `record not found` branch |
| `internal/services` | an abort is not a missing row | unexported `lookupError(err)` in `errors.go`, used by the public lookups that previously returned `ErrNotFound` for every error |
| `internal/handlers` | HTTP outcome | `AbortIfCanceled(c, err)` in `respond.go`, called first in each public catalog error branch |
| `internal/middleware` | request log | unchanged: it already derives the level from the status, so 499 logs at `warn` |

## Why 499

The client is gone, so the response body is unreadable by definition. The status
exists only to describe the outcome in the request log and in proxy access logs.
499 is nginx's convention for "client closed request"; `net/http` has no constant
for it, so it is declared locally. Gin reports the status through
`c.Writer.Status()` before the flush, so `RequestLogger` sees it without any
middleware change.

Alternatives rejected:

- **204/200.** Reports a success the client never received and hides the abort.
- **500.** What happens today, and it is what put fake incidents in Rollbar.
- **Silent return with no status.** Gin defaults to 200, which is worse.

## Why not decouple request contexts

Go 1.21+ offers `context.WithoutCancel`, and wrapping `c.Request.Context()` in a
middleware would erase every cancellation in the codebase in one line. Rejected:
cancellation is what stops expensive report exports and analytics aggregates from
running on behalf of clients who already left, and it makes the DB release the
connection. The goal is to classify aborts correctly, not to prevent them.

## Why the service layer had to change

`PublicCatalogService.GetProject` and friends do:

```go
project, err := s.projects.FindPublishedByGUID(ctx, guid)
if err != nil {
    return nil, ErrNotFound
}
```

so a canceled lookup arrives at the handler as `ErrNotFound` and the handler has
no way to know the client hung up. Any fix that only touched the handlers would
still return false 404s. `lookupError` keeps the repository error when it is a
cancellation and collapses everything else to `ErrNotFound`, preserving the
existing contract for genuine misses. `GetLot`'s combined
`if err != nil || lot.Status == active` condition was split so the status check
still yields `ErrNotFound` exactly as before.

Scope is deliberately the public catalog paths that produce this traffic in the
logs. Other handler groups inherit the log-level fix (their aborts are no longer
`SQL Error` lines) and can adopt `AbortIfCanceled` when a route shows the same
pattern, rather than sweeping ~30 files speculatively.

## Local log capture

`logOutput()` returns `io.MultiWriter(os.Stdout, file)` when `LOG_FILE` names a
usable path:

- stdout stays first in the writer list, so records keep reaching it even if the
  file is closed later, and the deployed collector is unaffected.
- the file opens `O_CREATE|O_APPEND`: `air` restarts on every save, and a
  truncated log would lose the run that produced the failure.
- an unusable path (missing parent directory, permissions) warns once on stderr
  and degrades to stdout only. Logging must never take the process down.
- the handle is tracked so a repeated `Setup` replaces it, and `Close` releases it
  at shutdown. Writes are unbuffered, so there is nothing to flush: the file is
  current the instant a line is logged.

`make dev` exports `LOG_FILE`, creates the parent directory, and prints the path
being written, which is what makes `LOG_FILE=trace.log make dev` the answer to
"where are the logs?".

## Rollback

Revert the two commits. No migration, no persisted shape, no config required:
with `LOG_FILE` unset the logger behaves exactly as before, and removing the
`AbortIfCanceled`/`lookupError` calls restores the previous 404/500 responses.
