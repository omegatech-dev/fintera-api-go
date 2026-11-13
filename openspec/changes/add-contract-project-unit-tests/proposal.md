## Why

Contracts and projects are the core of the product, and both already have
service-layer tests. However, the parts that protect the non-negotiable
invariants are still untested: the contract state machine has **zero** tests,
most contract HTTP endpoints have no handler tests, and several service paths
(`Approve`, `Close`, `DeleteRejected`, `ResendAccountSetup`, the successful
capital-repayment path) are only exercised indirectly. A regression in these
areas — illegal state transitions, wrong ledger/balance effects, or broken
endpoint authorization/validation — would ship silently. Closing the gaps now is
cheap because the tests are pure unit tests that do not need new infrastructure.

## What Changes

- Add table-driven tests for `internal/statemachine/contract_fsm.go` covering
  every allowed transition and every rejected transition. (Payment FSM tests are
  included opportunistically because it is the same package and pattern.)
- Add handler tests for the untested contract endpoints: list/stats, show,
  show-by-GUID (owner vs. stranger), prepayment request, update (schedule and
  bank fields), approve, reject, cancel, delete-rejected, and ledger.
- Add service tests for the untested contract paths: `Approve`, `Close`,
  `DeleteRejected`, `ResendAccountSetup`, and the success path of
  `CapitalRepayment` (payment + ledger + schedule effects and auto-close).
- Add handler tests for the untested project endpoints: list, create, update,
  and delete.
- Add repository tests for `ContractRepository`, `ProjectRepository`, and
  `LotRepository` using `go-sqlmock`, following the existing
  `user_repository_test.go` pattern, focused on the non-trivial queries
  (availability checks, stats, filters) rather than trivial CRUD.
- No production behavior changes; test-only additions plus any minimal,
  behavior-preserving refactors needed to make code testable.

## Capabilities

### New Capabilities

None — this is a test-only change with no observable behavior change, so
`.openspec.yaml` sets `skip_specs: true`.

### Modified Capabilities

None.

## Impact

- `internal/statemachine/contract_fsm_test.go` (new)
- `internal/statemachine/payment_fsm_test.go` (new, opportunistic)
- `internal/handlers/contract_handler_test.go` (new)
- `internal/handlers/project_handler_test.go` (extended)
- `internal/services/contract_service_test.go` (extended)
- `internal/repository/contract_repository_test.go` (new)
- No migrations, routes, config, or API contract changes.
- Verification: `go build ./...`, `go test ./...`, `make lint`.
