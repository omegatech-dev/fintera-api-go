# Testing Guide

## Commands

```bash
make test                 # go test -v ./...
make test-cover           # coverage.out + coverage.html
go test ./internal/services -run TestName -v
```

The Makefile uses the full module path; run commands from the repository root.

## Current test layout

| File | Focus |
| --- | --- |
| `internal/services/auth_service_test.go` | Login/refresh rules using mocked repositories |
| `internal/services/payment_service_interest_test.go` | Overdue interest calculation and ledger entry behavior |
| `internal/services/report_service_test.go` | CSV/report generation with mocked repositories |
| `internal/handlers/user_handler_test.go` | Request binding for user creation |

Tests use `stretchr/testify` and handwritten mocks that embed repository
interfaces. They are predominantly unit tests and do not require a live
database.

## CI

`.github/workflows/go-ci.yml` runs on pushes and pull requests to `main`:

1. Starts PostgreSQL 15 (`fintera_test`).
2. Sets up Go from `go.mod` (Go 1.27.3) with module caching.
3. Downloads and verifies modules.
4. Runs `make migrate-up` against the test DB; the Makefile installs
   `golang-migrate` on demand.
5. Runs `go test -v ./...` with `ENVIRONMENT=test`.
6. Runs `go build -v ./...`.

The workflow does not currently run `golangci-lint`.

## Writing tests

### Services

Prefer testing services directly. Create mocks by embedding the repository
interface and overriding only the methods under test:

```go
type mockUserRepo struct {
    repository.UserRepository
    mockFindByEmail func(ctx context.Context, email string) (*models.User, error)
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
    return m.mockFindByEmail(ctx, email)
}
```

This keeps mocks small and compile-safe when interfaces grow.

### Handlers

Use `gin.SetMode(gin.TestMode)`, `httptest.NewRecorder()`, and
`httptest.NewRequest`. Call `router.ServeHTTP` and assert status/body. Set
context values (`userID`, `userRole`) when the handler reads them from Gin
context.

### FSM and ledger

When changing transitions or ledger signs, add tests asserting:

- invalid transitions are rejected,
- balance moves toward zero for credits and away for debits,
- interest updates replace the prior accumulated entry rather than duplicating.

## Gaps worth filling

- Contract approval/rejection/close and payment approval/undo integration
  behavior.
- Receipt upload authorization and path traversal protection.
- Pagination/filter/sort behavior in repositories.
- Background job idempotency.
- Report PDF rendering (requires `wkhtmltopdf`).

## Definition of done for test changes

- `go test ./...` passes locally.
- New behavior has at least one focused test.
- Tests do not depend on wall-clock flakiness; inject or freeze time where
  practical.
