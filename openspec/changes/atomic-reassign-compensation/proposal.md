## Why

Commission reassignment currently commits credit restoration and clears payment markers before assignment updates finish. A later error can leave the seller, assignment history, and credits inconsistent; concurrent payout or assignment acceptance can also race the final contract write. Reassignment and acceptance need one transaction boundary with a shared contract-lock protocol.

## What Changes

- Make reassignment atomic across the locked contract, commission-credit applications and balances, assignment history, contract assignment fields, and the new assignment.
- Make assignment acceptance use the same contract-first lock protocol so an acceptance cannot overwrite a newer reassignment.
- Re-read and recalculate reassignment decisions under the contract lock, including current seller, paid state, and automatic seller selection.
- Bind audit and notification writes created by assignment operations to the transaction so they roll back with it; emit reassignment-level audit only after commit.
- Add rollback and concurrency tests, including live PostgreSQL tests gated by `DATABASE_URL`.
- Keep cancellation, rejection, and expiration transaction refactors out of scope; document these as follow-ups.

## Capabilities

### New Capabilities

- `atomic-contract-assignments`: transactional and concurrency-safe reassignment and assignment acceptance behavior.

### Modified Capabilities

<!-- No existing canonical specs are present in openspec/specs. -->

## Impact

- `internal/repository/repositories.go`: transaction runner and transaction-scoped repository bundles.
- `internal/repository/assignment_repository.go`, `contract_repository.go`, and `commission_repository.go`: transaction-safe methods and row locking.
- `internal/services/contract_service.go` and `assignment_service.go`: reassignment/acceptance orchestration and post-commit side effects.
- `internal/services/audit_service.go` and `notification_service.go`: transaction-scoped service clones.
- Tests for assignment/reassignment and gated PostgreSQL rollback/concurrency coverage.
- `docs/architecture/domain-flows.md` and the OpenSpec follow-up notes; no API shape changes.
