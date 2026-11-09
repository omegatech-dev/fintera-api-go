# AGENTS.md

Guidance for AI agents and contributors working in this repository.

## What this repo is

Fintera API: a Go 1.27.3 / Gin / GORM REST backend for real-estate contract
management, migrated from Ruby on Rails. It manages users, projects, lots,
contracts, payment schedules, a contract ledger, notifications, reports, and
scheduled background work.

Start with:
- [docs/architecture/overview.md](docs/architecture/overview.md)
- [docs/architecture/data-model.md](docs/architecture/data-model.md)
- [docs/architecture/domain-flows.md](docs/architecture/domain-flows.md)
- [docs/architecture/api-and-auth.md](docs/architecture/api-and-auth.md)
- [docs/guides/development.md](docs/guides/development.md)

## Golden commands

Run from the repository root.

```bash
make deps          # go mod download + tidy
make tools         # install air, swag, migrate, golangci-lint, gosec
make build         # build bin/api
make run           # build and run
make dev           # hot reload with air
make test          # go test -v ./...
make test-cover    # coverage report
make fmt           # go fmt ./...
make lint          # golangci-lint run
make security      # gosec ./...
make migrate-up    # apply migrations (needs DATABASE_URL)
make migrate-down  # roll back one migration
make seed          # run SQL seeds
make swagger       # regenerate docs/ Swagger
```

Minimal verification after a code change:

```bash
go build ./...
go test ./...
```

The Makefile resolves every Go tool from `$(go env GOBIN)` and installs it on
demand, so `make dev`, `make swagger`, `make migrate-*`, `make lint`, and
`make security` work without a separate tool setup. `psql`, Docker, and
`wkhtmltopdf` remain external prerequisites for their respective targets. Use
`make GO=/path/to/go <target>` when multiple Go versions are installed.

Repo-local opencode commands (see `.opencode/command/`): `/verify`,
`/new-migration`, `/new-endpoint`, `/docs-sync`.

## Architecture map

```
cmd/api/main.go          wiring, router, scheduled jobs, graceful shutdown
internal/config          env config and defaults
internal/database        GORM connection, migrations, seeds
internal/models          GORM models, status constants, ToResponse DTOs
internal/repository      data access interfaces + implementations
internal/services        business logic, reports, email, analytics
internal/handlers        Gin HTTP handlers
internal/middleware      JWT auth, RBAC, CORS, request logging
internal/jobs            in-process worker pool and scheduler
internal/statemachine    contract and payment FSMs
internal/storage         local file storage and path safety
pkg/logger               slog setup and GORM logger adapter
docs                     canonical generated Swagger + documentation
```

Dependency direction: `handlers → services → repository → models`. Do not
import handlers from services or repositories from handlers. Register new
components in `NewRepositories`, `NewServices`, and `NewHandlers`.

## Non-negotiable invariants

### Contract state machine

States: `pending`, `submitted`, `approved`, `signed`, `rejected`, `cancelled`,
`closed`.

Transitions: `pending/rejected → submitted`, `pending/submitted/rejected →
approved`, `pending/submitted → rejected`, `pending/submitted/rejected →
cancelled`, `approved → closed`, `closed → approved`.

Use `internal/statemachine/contract_fsm.go`, not direct status assignment.

### Payment state machine

States: `pending`, `submitted`, `paid`, `rejected`, `readjustment`.

Transitions: `pending → submitted` (receipt required), `pending/submitted →
paid`, `submitted → rejected`, `paid → submitted` (undo), `pending →
readjustment`.

Use `internal/statemachine/payment_fsm.go`.

### Ledger signs

The ledger is authoritative; `contracts.balance` is a cache.

- Negative entry increases debt: `initial`, `interest`.
- Positive entry reduces debt: `payment`, `prepayment`.
- `adjustment` can be either sign (used for reversals).
- Balance is the sum of entries; the contract closes at `balance >= 0`.

The comment in `internal/models/contract_ledger_entry.go` is stale. Follow the
implementation in `internal/services/payment_service.go` and
`payment_business_logic.go`.

### Authorization

Roles: `admin`, `seller`, `user`. Public routes are limited to health, auth,
password recovery, Swagger, and `/uploads`. Protected routes use
`middleware.Auth`, then one of `RequireAdmin`, `RequireRole`,
`RequireAdminSellerOrOwner`, or `RequireAdminOrOwner`. Add routes in the
correct group in `cmd/api/main.go`.

### Storage

Local disk under `STORAGE_PATH`. Allowed MIME types: PDF, JPEG, PNG. Max 10 MB.
Use `storage.SafeFullPath` for downloads; never join user-controlled paths
without validation. Receipts live under `receipts/YYYY/MM/`.

### Migrations

Files live in `internal/database/migrations/` as paired `up`/`down` SQL.
Applied migrations are immutable. Add the next numbered pair. Seeds live in
`internal/database/seeds/` and run through `psql`.

## Coding conventions

- Manual dependency injection through constructors; no DI framework.
- Handlers stay thin: parse request, call one service method, write response.
- Services own validation, state transitions, transactions, audit logs, and
  async side effects.
- Repositories own GORM queries and expose interfaces. Services depend on
  interfaces.
- Add constants for statuses/types in `internal/models`; do not scatter string
  literals.
- JSON responses use `snake_case`. Wrap resources: `{"contract": {...}}`,
  `{"projects": [...], "pagination": {...}}`. Errors: `{"error": "..."}`.
- Many user-facing messages are Spanish; match the surrounding handler.
- Use `ToResponse()` DTOs for API output instead of raw models.
- Run notifications/emails through `worker.EnqueueAsync`.
- Write audit logs with `AuditService.Log` for admin state changes.
- Keep comments purposeful. Add Swagger annotations on new/changed handlers.
- Tests use `testify` and small mocks that embed repository interfaces.

## Change recipes

### Add an endpoint

1. Handler method with Swagger annotations.
2. Service method with business logic.
3. Repository method + implementation if new persistence is needed.
4. Route registration in `cmd/api/main.go` under the correct role group.
5. Tests.
6. `make swagger` if annotations changed.
7. Update `docs/architecture/api-and-auth.md`.

### Add a migration

1. `make migrate-create` or add the next `NNNNNN_name.{up,down}.sql` pair.
2. Keep `down` reversible.
3. Update `docs/architecture/data-model.md`.

### Add a background job

1. Implement it as a context-aware service method.
2. Register in `scheduleJobs` (`cmd/api/main.go`) using
   `ScheduleEveryImmediate` when it must run after restarts.
3. Make it idempotent and safe under multiple replicas.
4. Update `docs/architecture/domain-flows.md`.

### Add a report

1. Add an embedded HTML template under
   `internal/services/templates/reports/`.
2. Add a `ReportService` generator.
3. Add a handler and route in the admin/seller group.
4. Add a mocked unit test.
5. Update the route list in `docs/architecture/api-and-auth.md`.

## Guardrails

- Never commit `.env`, `.production.env`, secrets, API keys, credentials, or
  generated binaries (`main`, `server`, `api_bin`, `fintera-api`).
- Never print or copy secret values. Reference variable names only.
- Do not edit already-applied migrations.
- Do not run destructive database targets (`make drop-db-seed-migrate*`)
  against anything but a disposable local database.
- Do not delete or rewrite files outside the task scope.
- Only commit when the user explicitly asks. Inspect `git status` and
  `git diff` first.
- Update documentation in the same change that alters routes, statuses,
  config, schema, scheduled jobs, or operational behavior.
- `docs/` is the only canonical Swagger location. Regenerate with
  `make swagger`; do not recreate `api/openapi/` or `openapi/`.

## Known gotchas

- Background jobs are in-process and are not persisted across restarts;
  multiple replicas duplicate scheduled work.
- Local storage is not shared across replicas and may not survive redeploys
  without a persistent volume.
- `POST /projects/:project_id/approve` is a placeholder.
- `GET /reports/user_information_pdf` reuses the user balance PDF.
- `GET /health` is under `/api/v1`, not `/health`.
- The Docker image runs migrations and seeds on every container start.
- README and older docs may lag behind code; prefer this file and `docs/`.

## Definition of done

- `make fmt` (or `gofmt`) applied.
- `go build ./...` succeeds.
- `go test ./...` passes.
- `make lint` passes if `golangci-lint` is installed.
- New behavior is documented and Swagger regenerated when applicable.
- No secrets or artifacts added; `git diff` contains only intended changes.
