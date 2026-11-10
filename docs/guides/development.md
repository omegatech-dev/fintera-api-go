# Development Guide

## Prerequisites

| Tool | Purpose | Required |
| --- | --- | --- |
| Go 1.27.3+ | Build and test | yes |
| PostgreSQL | Database | yes |
| `golang-migrate` | Apply migrations | yes for local DB work |
| `psql` | Seeds and DB inspection | yes for reset/seed targets |
| `air` | Hot reload (`make dev`) | optional |
| `swag` | Regenerate Swagger | when routes/annotations change |
| `golangci-lint` | Lint | recommended |
| `wkhtmltopdf` | PDF report generation | for PDF endpoints |
| Docker | Container builds | optional |

The Makefile installs Go tools into `$(go env GOPATH)/bin` and adds that
directory to `PATH` for its recipes. `make dev`, `make swagger`, `make
migrate-*`, `make lint`, and `make security` install their tool on demand when
it is missing. To install all tools up front:

```bash
make tools
```

If multiple Go versions are installed, select one explicitly:

```bash
make GO=/usr/local/go/bin/go build
```

## First-time setup

```bash
cp .env.example .env
# Edit DATABASE_URL and JWT_SECRET for your machine
make deps
make migrate-up
make seed            # optional sample data
make dev
```

The API reads `.env` automatically through `godotenv/autoload`. `.production.env`
is **not** loaded by the application; production values must come from the
platform environment or `.env`.

Default local port is `8080` from `internal/config/config.go`, but the sample
`.env.example` sets `PORT=8081`. Use whichever your `.env` specifies.

Useful URLs:

- Health: `http://localhost:8081/api/v1/health`
- Swagger UI: `http://localhost:8081/swagger/index.html`
- Uploaded files: `http://localhost:8081/uploads/...`

## Environment variables

Source: `internal/config/config.go`.

| Variable | Default | Notes |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port |
| `ENVIRONMENT` | `development` | `production` enables Gin release mode and JSON logs |
| `DATABASE_URL` | — | **Required** PostgreSQL URL |
| `JWT_SECRET` | dev fallback | Required in production |
| `JWT_EXPIRATION_HOURS` | `24` | JWT lifetime |
| `STORAGE_PATH` | `./storage` | Local upload root |
| `WORKER_COUNT` | `5` | Background worker consumers |
| `ALLOWED_ORIGINS` | `*` | Comma-separated CORS origins |
| `APP_URL` | `https://fintera.securexapp.com` | Used in internal/admin email links |
| `PORTAL_URL` | `https://portal.fintera.app` | Customer portal origin for email deep links (`/reserva/:guid/seguimiento`, `/mis-reservas`) |
| `RESEND_API_KEY` | empty | Email is disabled without it |
| `FROM_EMAIL` | `noreply@fintera.app` | Sender address (must be verified in Resend; use `onboarding@resend.dev` for testing) |
| `ENABLE_EMAIL_NOTIFICATIONS` | `false` | Global email switch |
| `EMAIL_OVERRIDE_TO` | empty | Redirects every outbound email to this test inbox. Use `delivered@resend.dev` (or `delivered+staging@resend.dev`) locally/staging. **Leave empty in production.** |
| `ROLLBAR_TOKEN` | empty | Error tracking disabled when empty |
| `ROLLBAR_CODE_VERSION` | empty | Rollbar metadata |
| `ROLLBAR_SERVER_ROOT` | `github.com/sjperalta/fintera-api` | Rollbar metadata |
| `DEFAULT_EMAIL` | empty | Development Swagger default login |
| `DEFAULT_PASSWORD` | empty | Development Swagger default password |

Never commit real values for `JWT_SECRET`, `RESEND_API_KEY`, `ROLLBAR_TOKEN`, or
database credentials.

## Make targets

| Command | Description |
| --- | --- |
| `make deps` | `go mod download` + `go mod tidy` |
| `make tools` | Install `air`, `swag`, `migrate`, `golangci-lint`, `gosec` |
| `make build` | Build `bin/api` |
| `make run` | Build and run `bin/api` |
| `make dev` | Hot reload with `air` |
| `make test` | `go test -v ./...` |
| `make test-cover` | Coverage profile + HTML report |
| `make fmt` | `go fmt ./...` |
| `make lint` | `golangci-lint run` |
| `make security` | `gosec ./...` |
| `make migrate-up` | Apply all migrations |
| `make migrate-down` | Roll back one migration |
| `make migrate-create` | Prompt for a name and create a migration pair |
| `make seed` | Run all SQL files in `internal/database/seeds/` |
| `make drop-db-seed-migrate` | Recreate `fintera_api_development`, migrate, seed |
| `make drop-db-seed-migrate-force` | Same, terminating active connections first |
| `make swagger` | Regenerate Swagger into `docs/` |
| `make docker-build` | Build the Docker image |
| `make docker-run` | Run the Docker image with `.env` |

The DB reset targets assume the database name `fintera_api_development`.

## Database workflow

### Add a migration

```bash
make migrate-create
# enter a descriptive name, then edit the generated up/down SQL files
make migrate-up
```

Migration files follow `NNNNNN_name.up.sql` / `.down.sql`. Applied migrations
are immutable; always add a new numbered pair.

### Seeds

`make seed` runs every `*.sql` file in `internal/database/seeds/` in sorted
order through `psql`. The Docker image also runs seeds on container start.

Local migration state is tracked by golang-migrate in the database.

## Background jobs

Jobs are registered in `cmd/api/main.go` (`scheduleJobs`). They run in-process
on every API instance. Do not add long-running jobs without considering:

- restarts (jobs are not persisted),
- multiple replicas (each instance runs its own schedule),
- idempotency (jobs such as reminders track `*_reminder_sent_at`).

## Swagger

Annotations live on handler methods. After changing routes or request/response
shapes:

```bash
make swagger
```

Commit the regenerated `docs/docs.go`, `docs/swagger.json`, and
`docs/swagger.yaml`. `docs/` is the only canonical Swagger location.

## Common issues

- **`DATABASE_URL is required`** — `.env` is missing or not in the working
  directory.
- **`air: No such file or directory` or tools installed but not found** —
  `$(go env GOPATH)/bin` is not on `PATH`. The Makefile adds it automatically
  for make recipes; for direct shell use, add
  `export PATH="$(go env GOPATH)/bin:$PATH"` to your shell profile.
- **Wrong Go version / `go version` shows an older release** — another Go
  install may be earlier on `PATH` (for example Homebrew vs `/usr/local/go`).
  Put the intended Go first, e.g. `export PATH=/usr/local/go/bin:$PATH`, or
  run `make GO=/usr/local/go/bin/go <target>`.
- **Migration fails** — check PostgreSQL is running and the user can create
  tables; `make drop-db-seed-migrate-force` helps with stale connections.
- **PDF generation fails** — install `wkhtmltopdf`; it is not bundled locally.
- **Email silently skipped** — `ENABLE_EMAIL_NOTIFICATIONS=false` or
  `RESEND_API_KEY` / `FROM_EMAIL` missing.
- **Port already in use** — change `PORT` in `.env`.
- **Uploads disappear** — local storage is not persistent by default.
