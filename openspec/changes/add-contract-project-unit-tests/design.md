## Context

See `proposal.md` for motivation. The repository already has three testing
styles to reuse, so this change introduces no new test infrastructure:

- **Services**: testify mocks that embed the repository interfaces, plus
  `go-sqlmock` only for the `AuditService` insert
  (`internal/services/contract_service_test.go`, `mock_db_test.go`).
- **Handlers**: `gin.CreateTestContext` + `httptest.ResponseRecorder` with small
  stub repositories, as in `internal/handlers/project_handler_test.go`.
- **Repositories**: `go-sqlmock` with the GORM Postgres dialector and
  `SkipDefaultTransaction`, as in `internal/repository/user_repository_test.go`.

The contract FSM is pure (it mutates `Contract.Status` through `looplab/fsm`)
and needs no mocks at all.

## Goals / Non-Goals

**Goals:**

- Lock down every contract state transition (allowed and rejected) with a fast,
  dependency-free test.
- Cover the contract and project HTTP entry points that currently have no tests,
  including ownership checks and validation errors.
- Cover the contract service branches that can corrupt the ledger/schedule.
- Cover the repository queries where regressions are plausible (availability,
  stats, filtering) rather than trivial CRUD.
- Keep the suite fast and hermetic (`go test ./...` needs no database).

**Non-Goals:**

- Integration/e2e tests against a real Postgres instance.
- Changing production code except for small, behavior-preserving seams needed to
  inject fakes.
- Full line coverage; the goal is meaningful branch coverage of invariants.
- Testing payment endpoints beyond the payment FSM (owned by a separate effort).

## Decisions

### Test the FSM directly instead of only through the service

`ContractFSM` encodes the allowed state graph, and the service calls it on every
approve/reject/cancel/close. Testing through the service only exercises the
happy path plus whatever state the mock returns. A table-driven test over
`NewContractFSM` covers all source/target combinations cheaply.

Alternative rejected: rely on service tests only — they would require heavy
mocking for each transition and would not prove the graph itself.

### Table-driven transition matrix

For each event (`submit`, `approve`, `reject`, `cancel`, `close`, `reopen`) assert
the transition succeeds from every documented source state and returns an error
from every other state. This directly mirrors the invariant list in `AGENTS.md`
and `docs/architecture/domain-flows.md`.

### Reuse existing mocks in the `services` package

`contract_service_test.go` already defines `mockContractRepo`, `mockLotRepo`,
`mockPaymentRepo`, `mockLedgerRepo`, and `mockUserRepoForContract`. New service
tests extend that file/package rather than adding a second mock layer, so the
fakes stay in sync with the interfaces.

For the async paths (`Approve`, `CapitalRepayment`), the `jobs.Worker` runs
callbacks in the background; tests assert on the synchronous repository calls
and, where required, use a stopped/zero-worker setup so assertions are
deterministic.

### Handler tests use stubs, not the full service graph

Handler tests build a `ContractHandler`/`ProjectHandler` with stub services or
stub repositories so they can assert status codes, wrapped response shapes, and
ownership rules without touching GORM. `contract_handler.go` depends on the
concrete `*services.ContractService`, so the design is to construct it with
stub repositories (as `project_handler_test.go` does for `ProjectService`)
rather than introducing an interface solely for testing.

### Repository tests target the non-trivial queries

The repository concrete types wrap `*gorm.DB`; tests construct them with the
sqlmock DB. Priorities:

- `ContractRepository.FindActiveByLot` / `FindPendingReservations` /
  `FindReservationsAwaitingVoucher` and `GetStats` (status filters).
- `LotRepository.ReserveIfAvailable` (the atomic double-booking guard).
- `ProjectRepository.ListPublished` / `FindPublishedByGUID`.

Regex expectations should match the generated SQL loosely enough to survive
formatting changes but tightly enough to assert the important `WHERE` clauses.

## Risks / Trade-offs

- **Brittle sqlmock expectations** (SQL string coupling) → keep repository tests
  to a small, high-value set and match on distinctive fragments, not full
  statements.
- **Background worker races in service tests** → assert only synchronous effects
  or drain/cancel the worker before assertions; never depend on email/notification
  side effects.
- **Concrete `*ContractService` in handlers limits stubbing** → build the service
  with interface-based stub repositories (already the pattern) instead of
  refactoring the handler signature.
- **Test-only change can drift from production** → no specs are added
  (`skip_specs: true`); tests are the contract, and the tasks reference the exact
  invariants they protect.

## Migration Plan

No data or deployment changes. Land as a single test-only pull request and roll
back by reverting it; production behavior is unaffected either way.

## Open Questions

None.
