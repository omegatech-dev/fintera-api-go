## Context

`ContractService.Reassign` currently reads the contract, chooses/validates a
seller, reverses credit applications in a repository-owned transaction, then
supersedes offers, creates an accepted/pending assignment, and saves the
contract as separate writes. A later failure leaves compensation committed
without reassignment. `AssignmentService.Accept` conditionally changes an
assignment row and only then updates the contract mirror, so a concurrent
reassignment can leave history and the contract pointing at different sellers.

Repositories currently own their GORM handles, but the `Repositories` bundle
does not retain its database handle. Audit and notification services also have
their own DB-backed dependencies. Both flows need to use transaction-scoped
instances of every repository that writes state.

## Goals / Non-Goals

**Goals:**

- Reassignment and commission compensation commit or roll back together.
- Assignment acceptance and reassignment serialize on the contract row.
- All operations acquire locks in one order: contract first, then credits, then
  assignments.
- Seller choice, current seller exclusion, paid-state checks, and assignment
  state are derived from the contract/assignment state re-read under lock.
- Assignment audit and notification rows use the same transaction.
- Test rollback against PostgreSQL and cover `Accept`/`Reassign` concurrency.

**Non-Goals:**

- No API contract or response shape changes.
- Do not convert cancellation, rejection, expiration, or automated backfill in
  this change. Cancellation currently has separate ledger side effects and must
  be addressed as a follow-up.
- No external notification transport is added; assignment notifications are
  database records today.

## Decisions

**Transaction runner is owned by the repository bundle.**
`Repositories` retains its `*gorm.DB`, can create a tx-scoped copy with
`WithTx(tx)`, and exposes `RunInTx(ctx, fn)`. Existing constructors remain
unchanged. `NewServices` injects the runner into `ContractService` through a
setter to avoid breaking focused constructors and mocks.

**Service clones keep business dependencies but replace persistence.**
`ContractService` and `AssignmentService` get tx-scoped clones that replace
contract, assignment, commission, user, notification, and audit dependencies
with repository instances backed by the same transaction. `CommissionService`
also uses a tx-scoped repository; its compensation method may use a nested
GORM transaction/savepoint, which remains part of the outer transaction and
rolls back with it.

**Reassign locks before all decisions.**
The transaction starts by selecting the contract by GUID `FOR UPDATE` and
preloading the data required by commission calculation. Manual seller
validation and automatic seller selection happen after the lock. Automatic
selection excludes the seller on the locked contract, not a stale snapshot.
Then compensation, superseding prior pending assignments, creating the new
offer/accepted assignment, and saving the contract all use the tx-scoped clone.
The existing reassignment audit and history lookup happen after commit.

**Accept uses contract-first locking and re-reads both rows.**
It may do an initial unlocked assignment read only to discover the contract ID.
The transaction locks that contract first, then re-reads/locks the assignment,
revalidates ownership and pending status, and updates both rows atomically.
Already-accepted remains idempotent; other terminal assignment states retain
the existing `ErrAssignmentNotPending` behavior. An overdue offer continues
through the existing expiration flow, which is explicitly out of scope here.

**Assignment audit and notification writes stay inside the transaction.**
`AuditService.WithTx` and a transaction-scoped `NotificationService` use the
same tx-backed GORM/repositories. Their inserts roll back if assignment writes
fail. The contract-level reassignment audit remains after commit, as it is
today, to avoid recording an operation that did not commit.

**Reassignment no longer performs its compensation before assignment writes.**
The existing `RestoreAndCreateCredit` remains available to cancellation and
other callers. In a tx-scoped service it executes against the outer tx (GORM
nested transaction/savepoint), preserving existing behavior while allowing the
outer reassignment to roll everything back.

## Risks / Trade-offs

- **A transaction holds locks while automatic seller selection reads ranking
  inputs.** → Keep the locked section limited to reassignment reads/writes;
  do not enqueue external work or perform network calls there.
- **Assignment notifications and audits roll back if reassignment fails.**
  → This is intentional because they are database records, not external sends.
- **Nested GORM transactions use savepoints.** → Verify with the live PostgreSQL
  rollback test that a later assignment failure also rolls back the credit
  compensation.
- **Other flows can still race or partially commit.** → Document `Cancel`,
  `Reject`, `ExpireOverdue`, and backfill as follow-ups; do not claim they use
  this protocol.
- **Concurrent assignment requests may serialize and observe terminal state.**
  → Return the existing idempotent/already-responded behavior based on the
  locked assignment row.

## Migration Plan

No schema migration is required. Deploy the code with the existing commission
credit schema. Rollback is a binary rollback; no persistent schema changes or
new API fields are introduced. The transaction runner and `WithTx` clones are
internal wiring only.

## Open Questions

None. `Cancel`, rejection, and expiration remain explicit follow-ups.
