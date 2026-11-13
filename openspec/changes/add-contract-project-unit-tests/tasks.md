## 1. Contract state machine tests

- [x] 1.1 Create `internal/statemachine/contract_fsm_test.go` with a table-driven test asserting `Approve` succeeds from `pending`, `submitted`, and `rejected`, and returns an error from `approved`, `signed`, `cancelled`, and `closed`; verify with `go test ./internal/statemachine/ -run TestContractFSM`.
- [x] 1.2 Add cases asserting `Reject` succeeds from `pending` and `submitted` and fails from `approved`, `rejected`, `cancelled`, and `closed`; update the same test file and verify with `go test ./internal/statemachine/`.
- [x] 1.3 Add cases asserting `Cancel` succeeds from `pending`, `submitted`, and `rejected`, and fails from `approved`, `cancelled`, and `closed`; verify with `go test ./internal/statemachine/`.
- [x] 1.4 Add cases asserting `Close` succeeds only from `approved`, `Reopen` succeeds only from `closed`, and `Submit` succeeds from `pending` and `rejected`; assert `Status` is updated on success and unchanged on error; verify with `go test ./internal/statemachine/`.
- [x] 1.5 Create `internal/statemachine/payment_fsm_test.go` covering payment `submit`, `approve`, `reject`, and `undo` transitions and their invalid sources; verify with `go test ./internal/statemachine/`.

## 2. Contract service tests

- [x] 2.1 In `internal/services/contract_service_test.go`, add `TestContractService_Approve` covering: success from `pending` (status becomes `approved`, `ApprovedAt`/`Active` set, initial ledger entry created, schedule payments created, existing reservation payment not duplicated) and failure from `approved`; verify with `go test ./internal/services/ -run TestContractService_Approve`.
- [x] 2.2 Add `TestContractService_Close` covering: rejection when the ledger balance is negative, success when balance is `>= 0` (status `closed`, `ClosedAt` set, lot marked `fully_paid`), and failure from a non-`approved` state; verify with `go test ./internal/services/ -run TestContractService_Close`.
- [x] 2.3 Add `TestContractService_DeleteRejected` covering: success deletes payments, ledger entries, contract, and releases the lot; rejection when status is not `rejected`; verify with `go test ./internal/services/ -run TestContractService_DeleteRejected`.
- [x] 2.4 Add `TestContractService_ResendAccountSetup` covering: success when `MustChangePassword` is true (email sent before the user update), `ErrAccountAlreadyConfigured` when false, and `ErrNotFound` when the GUID is unknown; verify with `go test ./internal/services/ -run TestContractService_ResendAccountSetup`.
- [x] 2.5 Add `TestContractService_CapitalRepayment_Success` covering the happy path: payment row created as `paid`, positive `prepayment` ledger entry linked to the payment, schedule reduced/removed by the amount, contract balance updated, and auto-close triggered when the repayment settles the balance; verify with `go test ./internal/services/ -run TestContractService_CapitalRepayment_Success`.
- [x] 2.6 Add `TestContractService_Update` and `TestContractService_ReleaseUnpaidReservations` covering the repository delegation and the cancel loop for stale reservations; verify with `go test ./internal/services/ -run 'TestContractService_(Update|ReleaseUnpaidReservations)'`.

## 3. Contract handler tests

- [x] 3.1 Create `internal/handlers/contract_handler_test.go` with helpers that build a `ContractHandler` from stub repositories (service constructed with mocks, following `project_handler_test.go`); verify the helper compiles with `go test ./internal/handlers/ -run TestContractHandler`.
- [x] 3.2 Add `TestContractHandler_ShowByGUID` covering: `200` for admin/seller, `200` for the applicant, `403` for an unrelated user, and `404` when the GUID is missing; verify with `go test ./internal/handlers/ -run TestContractHandler_ShowByGUID`.
- [x] 3.3 Add `TestContractHandler_RequestPrepayment` covering: `201` on success, `400` for a non-positive amount, `403` for an unrelated user, and `404` for an unknown GUID; verify with `go test ./internal/handlers/ -run TestContractHandler_RequestPrepayment`.
- [x] 3.4 Add `TestContractHandler_Update` covering: nested and flat JSON bodies, schedule fields blocked outside `pending`/`rejected`/`submitted`, negative amounts rejected, invalid `max_payment_date` rejected, and bank fields applied/cleared for bank financing; verify with `go test ./internal/handlers/ -run TestContractHandler_Update`.
- [x] 3.5 Add `TestContractHandler_StateActions` covering `Approve`, `Reject`, `Cancel`, and `Delete` success responses and their `422`/`403`/`404` error mappings; verify with `go test ./internal/handlers/ -run TestContractHandler_StateActions`.
- [x] 3.6 Add `TestContractHandler_IndexStatsLedger` covering the wrapped `contracts`/`pagination` payload, stats passthrough, ledger payload, and the `404` when the contract does not exist; verify with `go test ./internal/handlers/ -run TestContractHandler_IndexStatsLedger`.

## 4. Project handler tests

- [x] 4.1 Extend `internal/handlers/project_handler_test.go` with `TestProjectHandler_Index` asserting the `projects`/`pagination` envelope and total; verify with `go test ./internal/handlers/ -run TestProjectHandler_Index`.
- [x] 4.2 Add `TestProjectHandler_Create` covering a successful creation (`201`, generated lots, GUID) and the invalid-body `400` path; verify with `go test ./internal/handlers/ -run TestProjectHandler_Create`.
- [x] 4.3 Add `TestProjectHandler_Update` covering the successful update, the invalid-body `400` path, and the repository-error `500` path; verify with `go test ./internal/handlers/ -run TestProjectHandler_Update`.
- [x] 4.4 Add `TestProjectHandler_Delete` covering the successful `200` deletion and the repository-error `422` path; verify with `go test ./internal/handlers/ -run TestProjectHandler_Delete`.

## 5. Repository tests

- [x] 5.1 Create `internal/repository/contract_repository_test.go` using the existing `setupMockDB` helper; cover `FindActiveByLot`, `FindPendingReservations`, `FindReservationsAwaitingVoucher`, and `GetStats`, asserting the status/age filters in the generated SQL; verify with `go test ./internal/repository/ -run TestContractRepository`.
- [x] 5.2 Add tests for `PaymentRepository.FindOverdue`, `FindOverdueForActiveContracts`, and `FindPaymentsDueTomorrowForActiveContracts` asserting the date/status filters; verify with `go test ./internal/repository/ -run TestPaymentRepository`.
- [x] 5.3 Add tests for `ProjectRepository.ListPublished`/`FindPublishedByGUID` and `LotRepository.ReserveIfAvailable` (including the "already reserved" zero-rows-affected path); verify with `go test ./internal/repository/ -run 'Test(Project|Lot)Repository'`.

## 6. Verification

- [x] 6.1 Run `gofmt -w` on every added/modified test file and confirm `go build ./...` succeeds.
- [x] 6.2 Run `go test ./...` and confirm all packages pass with the new tests and no test depends on a live database.
- [x] 6.3 Run `make lint` (or `golangci-lint run`) and resolve any test-only lint findings.
- [x] 6.4 Run `openspec validate --changes` and confirm `add-contract-project-unit-tests` validates.
