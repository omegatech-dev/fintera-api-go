## 1. Service

- [x] 1.1 Add `ContractService.CreatePrepaymentRequest` with ownership-agnostic
      validation: approved contract, outstanding balance, amount bounds and no
      duplicate submitted request.
- [x] 1.2 Create the request as a submitted `capital_repayment` payment.
- [x] 1.3 Extract `applyToPendingInstallments` in `PaymentService`.
- [x] 1.4 Approve `capital_repayment` payments as prepayments: shorten the
      schedule and write only a prepayment ledger entry.

## 2. Handler and routes

- [x] 2.1 Add `ContractHandler.RequestPrepayment` with owner/admin/seller access
      checks.
- [x] 2.2 Register `POST /contracts/:guid/prepayments` in the protected group.
- [x] 2.3 Add Swagger annotations and regenerate the docs.

## 3. Tests

- [x] 3.1 Cover successful request creation.
- [x] 3.2 Cover non-positive amount, unapproved contract, no balance, amount
      above balance and duplicate request.
- [x] 3.3 Cover the prepayment distribution (partial and full coverage).
- [x] 3.4 Cover approval of a `capital_repayment` payment (schedule shortened,
      prepayment ledger entry, no regular payment entry).

## 4. Verification

- [x] 4.1 `go build ./...`
- [x] 4.2 `go test ./...`
- [x] 4.3 `golangci-lint run ./...`
- [x] 4.4 `openspec validate add-customer-prepayment-request`
