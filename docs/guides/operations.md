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

1. `golang:1.27.3-alpine` builds `./cmd/api` into `/app/main`.
2. `alpine` runtime installs CA certificates, timezone data, `make`, `curl`,
   `postgresql-client`, and `golang-migrate`.

Container start command:

```sh
make migrate-up && make seed && ./main
```

Implications:

- Migrations run on every container start.
- Seeds run on every container start and must remain idempotent.
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

## Storage

Files are written under `STORAGE_PATH`:

```
storage/
├── contracts/YYYY/MM/...
├── receipts/YYYY/MM/...
└── uploads/...
```

Uploads are served publicly from `/uploads`. Use a persistent volume; otherwise
files are lost on redeploy. The 10 MB upload limit and allowed MIME types are
enforced in `internal/storage/local.go` and the payment handler.

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
  stdout.
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
- The process handles `SIGINT`/`SIGTERM` with a 30-second HTTP shutdown window,
  then stops the worker and flushes Rollbar.
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
