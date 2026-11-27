## 1. Transaction-scoped repository wiring

- [x] 1.1 Retain `*gorm.DB` in `internal/repository/Repositories` and implement `WithTx` and `RunInTx`
- [x] 1.2 Add transaction-scoped clones for `AuditService` and `NotificationService`
- [x] 1.3 Add `WithRepositories`/`WithTx` clones for `AssignmentService`, `CommissionService`, and `ContractService` without changing existing constructors
- [x] 1.4 Inject the transaction runner into `ContractService` from `internal/services/services.go`

## 2. Atomic contract reassignment

- [x] 2.1 Add a contract repository method to lock/read by GUID with required details using `FOR UPDATE`
- [x] 2.2 Refactor `ContractService.Reassign` so current seller, status, paid markers, manual seller validation, and automatic seller selection are read/recomputed inside the transaction
- [x] 2.3 Run credit application reversals, credit restoration/cash credit creation, `SupersedePending`, `AssignNow`/`Offer`, and final contract persistence against the same transaction
- [x] 2.4 Keep transaction-owned audit/notification inserts inside the transaction and keep reassignment audit/history response work after commit

## 3. Atomic assignment acceptance

- [x] 3.1 Add an assignment repository method to re-read and lock an assignment after the contract lock
- [x] 3.2 Refactor `AssignmentService.Accept` to lock the contract first, then lock/revalidate the assignment and update both rows atomically
- [x] 3.3 Preserve already-accepted idempotency, ownership checks, terminal-state errors, and the current overdue-expiration behavior

## 4. Tests

- [x] 4.1 Update service/repository fakes and add rollback-capable unit transaction runner tests
- [x] 4.2 Test reassignment failure at supersede, accepted assignment, offer creation, and final contract persistence; assert no compensation or assignment changes remain
- [x] 4.3 Test netted-total, netted-partial, and unpaid reassignment paths
- [x] 4.4 Add a `DATABASE_URL`-gated PostgreSQL test that forces a failure after credit restoration and verifies real transaction rollback
- [x] 4.5 Add a `DATABASE_URL`-gated concurrent `PayCommission`/`Reassign` test
- [x] 4.6 Add a `DATABASE_URL`-gated concurrent `Accept`/`Reassign` test that verifies contract and assignment history agree

## 5. Documentation and verification

- [x] 5.1 Document the shared contract-first lock order and the remaining cancellation/rejection/expiration follow-ups in `docs/architecture/domain-flows.md`
- [x] 5.2 Run `gofmt`, `go build ./...`, `go test ./...`, `git diff --check`, and OpenSpec validation
