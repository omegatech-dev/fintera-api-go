## Why

Customers can see their financing and upload receipts for scheduled
installments, but there is no way to request an early (partial or full) capital
payment. Capital repayments are currently an admin-only action, so a customer
who wants to pay ahead must contact staff. The portal needs a first-class early
payment request that captures the amount and the transfer receipt, and that
staff can validate and apply.

## What Changes

- Add `POST /contracts/:guid/prepayments` for the contract owner (or
  admin/seller) to request a partial or full capital payment.
- Validate the contract is approved, has outstanding balance, the amount is
  positive and does not exceed the balance, and no other request is already
  under review.
- Create the request as a `capital_repayment` payment in `submitted` status so
  the customer can upload the receipt through the existing
  `POST /payments/:payment_id/upload_receipt`.
- When staff approves a `capital_repayment` payment, apply the whole amount as a
  prepayment: shorten the schedule (delete or reduce future installments from
  the end), write a prepayment ledger entry, and refresh the balance instead of
  booking it as a regular installment payment.

## Capabilities

### New Capabilities

- `customer-prepayments`: customer-requested partial or full capital payments.

### Modified Capabilities

None.

## Impact

- `internal/services/contract_service.go` (`CreatePrepaymentRequest`)
- `internal/services/payment_service.go` (capital-repayment approval branch)
- `internal/handlers/contract_handler.go` (`RequestPrepayment`)
- `cmd/api/main.go` (route registration)
- `docs/` (Swagger)
- Service tests for request validation and prepayment distribution
- Consumers: FinteraWeb `redesign-customer-reservations` early payment modal.
