# Operations Guide

## Runtime configuration

The binary is configured exclusively through environment variables. See
[development.md](development.md#environment-variables) for the full list.

Production requirements:

- `ENVIRONMENT=production`
- `DATABASE_URL`
- `JWT_SECRET` (startup fails without it in production)
- `STORAGE_PATH` pointing at a persistent volume
- `ALLOWED_ORIGINS` restricted to trusted frontend origins
- `RESEND_API_KEY` and `FROM_EMAIL` if email is enabled
- `ROLLBAR_TOKEN` if error tracking is enabled

`ENABLE_EMAIL_NOTIFICATIONS` defaults to `false`; enable it explicitly in
production.

## Docker

The `Dockerfile` is a two-stage build:

1. `golang:1.27.4-alpine` builds `./cmd/api` into `/app/main`.
2. `alpine` runtime installs CA certificates, timezone data, `make`, `curl`,
   `postgresql-client`, and `golang-migrate`.

Container start command:

```sh
make migrate-up && make seed && ./main
```

Implications:

- Migrations run on every container start.
- Seeds run on every container start and must remain idempotent.
- The API runs pending registered backfills asynchronously at startup. Each task
  is recorded in `backfill_runs` and skipped after a successful run for its
  version, so repeated starts are cheap.
- To add a backfill, register a task in `internal/backfills/tasks.go` with a
  unique name and a version. Bump the version when the task must run again.
- The container listens on `8080` by default; override `PORT` if needed.
- The runtime stage has **no Go toolchain**. The Makefile detects this and
  falls back to `/usr/local/bin`, where the Dockerfile preinstalls
  `golang-migrate`, so `make migrate-up` works at runtime; anything that
  would require `go install` (for example `make lint` or `make security`)
  fails there with a clear error.

Build and run:

```bash
make docker-build
make docker-run
# or
docker build -t fintera-api .
docker run -p 8080:8080 --env-file .env fintera-api
```

## Backfills

The API exposes a generic runner for one-off data tasks. Locally:

```bash
make backfill ARGS="-list"
make backfill ARGS="-task=historical_clients"
make backfill ARGS="-task=historical_clients -force"
```

Without `-task`, it runs every pending task. `-force` re-runs a completed task
and records a new run.

The Docker image also contains the `backfill` binary, so a manual run in a
container does not need Go:

```bash
docker exec -it <container> ./backfill -list
docker exec -it <container> ./backfill -task=historical_clients -force
```

The current historical-client alias is still available:

```bash
make backfill-historical-clients
```

## Storage

Files are written under `STORAGE_PATH`:

```
storage/
├── contracts/YYYY/MM/...
├── identity_documents/YYYY/MM/...
├── identity_documents/previews/YYYY/MM/...
├── receipts/YYYY/MM/...
└── uploads/...
```

Uploads are served publicly from `/uploads`. Use a persistent volume; otherwise
files are lost on redeploy. The 10 MB upload limit and allowed MIME types are
enforced in `internal/storage/local.go` and the payment handler.

`identity_documents/` is **not** served statically: DNI and RTN files are
private and are streamed by the authorized `content` and `preview` endpoints.
Back them up together with the rest of `STORAGE_PATH`.

Multipart request bodies are buffered with a small in-memory cap
(`router.MaxMultipartMemory`, 8 MiB; 16 MiB for project drawings) and the rest
spills to temporary files; each bulk upload endpoint also bounds the total
request body with `http.MaxBytesReader`. JSON request bodies are capped
globally by `middleware.BodyLimit` at `MAX_JSON_BODY_BYTES` (default 2 MiB);
oversized bodies are rejected (413 at direct-read binding sites, 400 for Gin
binding). Profile pictures are dimension-checked before decoding (max 4096 px
per side, 16M pixels total) so a compressed pixel bomb cannot expand into
hundreds of MB of bitmap memory.

## Background jobs

Recurring jobs run inside each API process. Consequences:

- Restarting an instance stops scheduled work until it starts again
  (`ScheduleEveryImmediate` runs each job once at startup).
- Running multiple replicas runs each job multiple times. Jobs that send email
  rely on `overdue_reminder_sent_at` / `upcoming_reminder_sent_at` to avoid
  duplicates, but other jobs are not globally coordinated.
- Long-running jobs compete with request workers only via goroutines; there is
  no separate job process.

For higher scale, move scheduling to a dedicated worker deployment or external
scheduler.

## Observability

- **Logs:** structured `slog`. JSON in production, text elsewhere, written to
  stdout. `LOG_FORMAT=text` forces text; `LOG_LEVEL` (default `info`) controls
  verbosity. `LOG_FILE` additionally duplicates every record to that path — a
  local debugging aid only; deployed logging must keep flowing to stdout for the
  platform collector, and the file is not rotated.
- **Request logs** (`Incoming request`) carry `requestId`, `method`, `route`
  (Gin route template, e.g. `/api/v1/projects/:project_id`), `status`,
  `durationMs`, `ip`, `user_agent`, and `user_id` when authenticated. The raw
  path and query string are not logged. The `X-Request-ID` request header is
  kept when it is 1–128 letters, digits, `.`, `_`, or `-`; otherwise a new ID is
  generated. The ID is returned in the `X-Request-ID` response header, so it can
  be matched with proxy access logs. 5xx logs at error, 4xx at warn. A client
  that disconnects mid-request ends as 499 (nginx "client closed request") with
  no body on the public catalog routes, instead of a misleading 404/500.
- **SQL logs:** `Slow SQL` (above 200 ms) and `SQL Error` lines include the
  duration in the message and the parameterized query in `sql` (`$1`, `$2`
  placeholders; bind values are never logged), plus `durationMs` and `rows`.
  `record not found` and `context canceled` are normal control flow (cache misses
  and clients that hung up) and log at `debug` as `SQL record not found` /
  `SQL client canceled`, never as `SQL Error`.
  Routine statements log at `debug`; use `LOG_LEVEL=debug` to see them. When
  `ENVIRONMENT=production`, GORM is fully silent (no SQL lines at all). Any other
  value, including an unset variable, enables SQL logging.
- **Job logs:** scheduled runs log `job` and `durationMs`. Successful runs log at
  `debug`, so a healthy deployment shows only failures (`Scheduled job failed`)
  at info level. Job start messages (`[Job] ...`) are also `debug`.
- **Email logs** mask recipient local parts (`j***@example.com`). Reset codes
  are not logged.
- **Container start:** `make migrate-up` and `make seed` run without echoing
  the command, so `DATABASE_URL` and its password are not written to the logs.
- **Rollbar:** panics and reported errors when `ROLLBAR_TOKEN` is set. Events
  are labeled with `ROLLBAR_ENVIRONMENT`, falling back to `ENVIRONMENT`.
  Credentials and session keys are scrubbed from payloads. HTTP panics are
  reported with request context and re-panicked so Gin recovery still handles
  the request; background job panics are normalized and reported as well.
  Reporting is disabled entirely when no token is configured.
- **Worker stats:** `GET /api/v1/jobs/status` (admin only) returns active,
  completed, failed, queue length, and max concurrency.
- **Health:** `GET /api/v1/health`.
- **Analytics cache:** refreshed every 15 minutes; filtered analytics bypass
  the cache.

There are no metrics or tracing endpoints.

## Health checks and scaling

- Point platform health checks at `/api/v1/health`.
- The process handles `SIGINT`/`SIGTERM` with one 30-second shutdown budget. It
  stops accepting new HTTP connections and drains in-flight requests, stops the
  scheduler and waits for queued and running jobs, closes the database pool,
  then flushes Rollbar. Jobs still running when the budget ends get a cancelled
  context and the log records a forced shutdown.
- Database pool: max open 50, max idle 5, connection lifetime 1 hour (see
  `internal/database/database.go`).

## Security

### Secrets

- Never commit `.env`, `.production.env`, API keys, JWT secrets, or database
  credentials.
- Rotate any key that has ever been committed or committed as a sample.
- Prefer platform secret managers over files.

### Repository hygiene

The repository previously tracked local binaries and a production env file.
They are now ignored and untracked:

- `main`, `server`, `api_bin`, `fintera-api`
- `.production.env`

Untracking does not remove them from git history. If real secrets were ever
committed, rotate them and consider a history purge as a separate, coordinated
operation.

### Application controls

- Passwords: bcrypt.
- JWT: HS256 signed with `JWT_SECRET`; rejected if expired or malformed.
- Authorization: role middleware plus per-resource ownership checks.
- Uploads: MIME allowlist, 10 MB limit, random filenames.
- Downloads: `SafeFullPath` blocks path traversal.
- CORS: configurable; do not leave `*` in production.

### Known hardening gaps

- No rate limiting or login throttling.
- Refresh tokens are stored as plain random strings.
- File contents are validated by MIME header, not magic bytes.
- `/uploads` is publicly served without authentication.
- No secret rotation mechanism.

## Backup and recovery

- Back up PostgreSQL regularly; it is the source of truth for state.
- Back up `STORAGE_PATH` for receipts, contract documents, and profile pictures.
- Migrations are forward-only in practice; test `down` migrations before
  relying on them.
- Seeds target local/dev environments; do not run destructive reset targets
  against production.
