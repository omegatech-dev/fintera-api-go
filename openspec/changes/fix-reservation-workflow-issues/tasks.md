## 1. Receipt file serving

- [x] 1.1 Change the receipt URL builder in `internal/models/payment.go` to emit `/receipts/<document_path>` and update the expectation in `internal/models/contract_test.go`; verify with `go test ./internal/models/...`
- [x] 1.2 Register `router.Static("/receipts", cfg.StoragePath+"/receipts")` in `cmd/api/main.go` next to the existing `/uploads` route; verify `go build ./...` and a manual `curl` against an existing file under `storage/receipts/2026/...`
- [x] 1.3 Confirm the authenticated `GET /payments/:payment_id/download_receipt` handler and its tests still pass without changes; verify with `go test ./internal/handlers/...`

## 2. Reservation approval order

- [x] 2.1 Add an `ErrContractNotApproved` sentinel with a Spanish message in `internal/services/errors.go` and a helper that loads the contract and rejects non-approved contracts for reservation payments
- [x] 2.2 Apply the helper in `internal/services/payment_service.go` `Approve` and in `internal/services/payment_business_logic.go` `ApprovePaymentWithLedger` before any mutation; add service tests covering pending/submitted contract rejection and approved contract success
- [x] 2.3 Map `ErrContractNotApproved` to HTTP 422 with its message in `internal/handlers/payment_handler.go` and update the Swagger annotation; verify with a handler test for both the reservation and non-reservation paths

## 3. Rejection reason lifecycle

- [x] 3.1 Clear `RejectionReason` when a new receipt is persisted in `PaymentService.UpdateReceipt`; add a service test asserting the reason is empty while status is `submitted` and that rejecting again sets a new reason
- [x] 3.2 Verify the reservation contract still advances from `pending` to `submitted` on upload, including the re-upload case, with service tests

## 4. Capital repayment payload

- [x] 4.1 Accept both `capital_repayment_amount` and `amount` in `internal/handlers/contract_handler.go` with explicit `> 0` validation and the existing 400 message; add handler tests for the documented field, the legacy field, and a missing/zero/negative amount
- [x] 4.2 Verify a non-approved contract still returns the existing error from `ContractService.CapitalRepayment` and creates no ledger entry, with a service test

## 5. Docs and verification

- [x] 5.1 Run `make swagger` and commit the regenerated files under `docs/`
- [x] 5.2 Update `docs/architecture/` if the public route list or payment lifecycle documentation changes
- [x] 5.3 Run `go build ./...` and `go test ./...` and fix any failure
- [x] 5.4 Run `make lint` (or `golangci-lint run`) and fix any reported issue
