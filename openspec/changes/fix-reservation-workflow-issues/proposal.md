## Why

Three production-affecting defects block the reservation review flow: uploaded
receipts are stored under `{STORAGE_PATH}/receipts` but served from
`{STORAGE_PATH}/uploads`, so every "ver comprobante" link returns 404; an admin
can approve a reservation payment before the contract is approved, reversing
the intended review order; and rejected receipts keep their rejection reason
forever, so customers see a stale motive after re-submitting. In addition, the
admin panel cannot apply capital repayments because its payload field name does
not match the endpoint contract.

## What Changes

- Serve uploaded receipts at the path reported in `receipt_url` so existing and
  new files resolve instead of returning 404.
- Require an approved contract before a reservation payment can be approved,
  on every payment-approval path, and return a clear error otherwise.
- Clear the stored rejection reason when a new receipt is submitted, so the
  reason only reflects the latest review.
- Accept the capital-repayment amount in either `capital_repayment_amount` or
  `amount`, keeping `capital_repayment_amount` as the documented field.
- Add/update Go tests for the approval guard, the rejection lifecycle and the
  receipt URL, and update the Swagger annotations where the error responses
  change.

**BREAKING**: approving a reservation payment now requires the contract to be
approved first. Existing receipts become reachable through their stored URL
without changing the database.

## Capabilities

### New Capabilities

- `payment-review-workflow`: reservation payment approval ordering and the
  rejection reason lifecycle across re-submissions.
- `payment-receipt-files`: public serving of uploaded payment receipts.
- `capital-repayment-requests`: accepted request payload for admin capital
  repayments.

### Modified Capabilities

None (this repository has no synced base specs; the new capabilities capture
the required behavior).

## Impact

- `internal/services/payment_service.go` (`Approve`, `UpdateReceipt`)
- `internal/services/payment_business_logic.go` (`ApprovePaymentWithLedger`)
- `internal/handlers/payment_handler.go` / `contract_handler.go` (error
  mapping, payload binding)
- `internal/models/payment.go` (receipt URL)
- `cmd/api/main.go` (static route)
- `internal/models/contract_test.go`, payment service/handler tests
- `docs/` (Swagger regeneration with `make swagger`)
- Consumers: FinteraWeb `fix-reservation-workflow-issues` (receipt links,
  hidden reasons) and FinteraUI `fix-reservation-workflow-issues` (approval
  guard message, capital repayment payload).
