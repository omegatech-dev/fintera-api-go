# Contributing

## Architecture rules

- Preserve dependency direction: `handlers → services → repository → models`.
- Handlers parse HTTP and delegate. Business rules belong in services.
- Repositories expose interfaces; add methods to the interface in the same
  package as the implementation.
- Use constructor injection and register new components in the existing
  factories:
  - `repository.NewRepositories`
  - `services.NewServices`
  - `handlers.NewHandlers`
- Add routes in `cmd/api/main.go` under the correct middleware group.
- Use the FSM package for contract and payment state changes. Do not assign
  statuses directly unless the service intentionally bypasses the FSM (for
  example, auto-close after balance recalculation).

## Code conventions

### Naming and layout

- One primary type per file, named `snake_case` files (e.g.
  `contract_service.go`).
- Constructors are `NewXxx(...)`.
- Status and type values live as constants in `internal/models`.
- JSON and GORM tags use `snake_case`.
- Prefer small, focused methods; handlers should stay readable.

### Responses

- Wrap single resources: `{"contract": ...}`, `{"project": ...}`.
- Wrap lists with a resource key and `pagination` when paginated.
- Errors: `{"error": "..."}`. User-facing messages are Spanish in most flows.
- Use `model.ToResponse()` for API output rather than returning GORM models
  directly.

### Side effects

- Notifications and emails that can fail should run through
  `worker.EnqueueAsync` so the HTTP response is not blocked.
- Write audit entries through `AuditService.Log` for state-changing admin
  actions.
- Keep email/report templates embedded and under
  `internal/services/templates/`.

### Comments and docs

- Keep comments purposeful. Explain non-obvious business rules, ledger signs,
  or workarounds.
- Add Swagger annotations to new or changed handler methods.
- Update the relevant file under `docs/` in the same change. See
  [../README.md](../README.md).

## Change recipes

### Add an endpoint

1. Add a handler method with Swagger annotations.
2. Add or extend a service method with business logic.
3. Add a repository method and implementation if new persistence is needed.
4. Register the route in `cmd/api/main.go` under the correct role group.
5. Add tests.
6. Run `make swagger`.
7. Update [../architecture/api-and-auth.md](../architecture/api-and-auth.md).

### Add a migration

1. `make migrate-create` and enter a name.
2. Write reversible `up`/`down` SQL.
3. Apply locally and verify.
4. Update [../architecture/data-model.md](../architecture/data-model.md).
5. Never edit an already-applied migration.

### Add a background job

1. Implement the logic as a service method that accepts a context.
2. Register it in `scheduleJobs` with `ScheduleEveryImmediate` if it must run
   after restarts.
3. Make it idempotent and safe under multiple replicas.
4. Update the job table in
   [../architecture/domain-flows.md](../architecture/domain-flows.md).

### Add a report

1. Add a template under `internal/services/templates/reports/`.
2. Implement the generator in `ReportService` and embed the template.
3. Add a handler and route in the admin/seller group.
4. Add a test with mocked repositories.
5. Update the route list in
   [../architecture/api-and-auth.md](../architecture/api-and-auth.md).

## Definition of done

- Code is formatted (`make fmt`).
- `go build ./...` succeeds.
- `go test ./...` passes.
- `make lint` passes when `golangci-lint` is installed.
- New routes/statuses/config keys are documented.
- Swagger is regenerated when handler annotations change.
- No secrets, `.env` files, binaries, or uploads are committed.
- `git diff` contains only intended changes.

## Before opening a pull request

- Rebase on the target branch and review the full diff.
- Summarize behavior changes, migration steps, and rollback notes.
- Link related issues.
- Call out any operational impact (new env vars, job schedules, storage needs).
- Do not commit generated binaries (`main`, `server`, `api_bin`,
  `fintera-api`).
