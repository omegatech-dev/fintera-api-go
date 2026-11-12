## Context

See `proposal.md` for motivation. Relevant current state:

- `LocalStorage.Upload(file, header, "receipts")` writes to
  `{STORAGE_PATH}/receipts/<YYYY>/<MM>/<hash>.<ext>`.
- `Payment.ToResponse()` builds `receipt_url` as
  `"/uploads/" + document_path`, i.e. `/uploads/receipts/...`.
- `setupRouter` only registers `router.Static("/uploads", cfg.StoragePath+"/uploads")`,
  and media services intentionally store project/lot files under
  `uploads/projects` and `uploads/lots`, which is why the static prefix exists.
- Payment approval flows: `PaymentService.Approve` (used by both
  `/payments/:id/approve` and the contract-scoped route) and
  `PaymentService.ApprovePaymentWithLedger`. Neither reads `contract.Status`.
- `PaymentService.UpdateReceipt` sets `DocumentPath`, `Status = submitted` and
  advances a pending reservation contract, but never clears
  `RejectionReason`.
- `CapitalRepaymentRequest` binds only `capital_repayment_amount` with
  `required,gt=0`.

## Goals / Non-Goals

**Goals:**

- Make existing and future receipts reachable through the URL the API reports.
- Enforce the contract-before-payment review order in the service layer, so
  every route that approves a payment inherits it.
- Make the rejection reason reflect only the latest review.
- Keep the capital-repayment endpoint compatible with the admin panel's current
  payload.

**Non-Goals:**

- Changing where other uploads (projects, lots, branding, contracts) are stored.
- Adding signed URLs or expiring links; receipt filenames remain unguessable
  hashes served publicly, matching the current tracking-link design.
- Moving receipt storage to object storage.
- Changing contract approval rules.

## Decisions

### Serve `/receipts` from the receipts directory

Change the receipt URL builder in `internal/models/payment.go` to emit
`"/receipts/" + document_path` and register
`router.Static("/receipts", cfg.StoragePath+"/receipts")` in `setupRouter`.
Existing files keep their relative document paths, so no migration or backfill
is needed. The authenticated `GET /payments/:id/download_receipt` endpoint is
unchanged.

Alternatives considered:

- Store new receipts under `uploads/receipts` and keep the URL shape. This
  would require copying the historical `storage/receipts` tree; more moving
  parts and a data migration for a purely cosmetic URL.
- Serve the whole storage root at `/uploads`. This breaks project/lot media,
  which is stored one level deeper (`uploads/projects`).

Trade-off: receipts become readable by anyone with the exact URL. That is the
same exposure the tracking screen already assumes and filenames are random;
the alternative public static block is intentional here.

### Enforce the approval order in the payment service

Add a sentinel error `ErrContractNotApproved` with a Spanish message, and a
private helper used by both `Approve` and `ApprovePaymentWithLedger`:

- if `payment.PaymentType != reservation`, continue;
- load the contract;
- if the contract status is not `approved`, return `ErrContractNotApproved`.

`ErrContractNotApproved` maps to HTTP 422 with its message in
`PaymentHandler.Approve`. No ledger entry, notification or lot update runs
because the check happens before any mutation.

Contracts remain approvable on their own; the only new coupling is payment
approval → contract state.

### Clear the rejection reason on re-submission

In `UpdateReceipt`, set `payment.RejectionReason = nil` before persisting. This
covers the customer re-upload path and the admin upload path that shares the
service method. Contract-level rejection reasons are untouched because a
rejected contract follows its own lifecycle.

### Accept both capital-repayment field names

Replace the strict binding with explicit resolution:

```go
type CapitalRepaymentRequest struct {
    CapitalRepaymentAmount float64 `json:"capital_repayment_amount"`
    Amount                 float64 `json:"amount"`
}
```

Use `capital_repayment_amount` when non-zero, otherwise `amount`; validate
`> 0` manually and keep the existing 400 message. `ContractService.CapitalRepayment`
continues to validate the contract is approved and applies the ledger entry.

## Risks / Trade-offs

- [Public receipt URLs] → Filenames are 32-hex random; tracking already exposes
  the link. If stricter access is required later, switch the public link to a
  short-lived signed token.
- [Clients caching the old `/uploads/receipts` URL] → Existing `receipt_url`
  values were 404 anyway; new responses use `/receipts`. FinteraUI previews
  receipts through the authenticated endpoint, and FinteraWeb reads the field
  on each load.
- [Approval-order guard surprises staff] → The API returns an explicit message
  and FinteraUI shows it next to the disabled action; audit logs keep the
  attempted action.
- [Alias masking a client typo] → Keep `capital_repayment_amount` documented in
  Swagger and validate `> 0`; the alias only covers the known admin payload.
- [Tests depending on the old URL] → Update
  `internal/models/contract_test.go` to assert `/receipts/...`.

## Migration Plan

1. Deploy FinteraAPI first; receipts start resolving and the guard activates.
2. Deploy FinteraUI and FinteraWeb, which adapt copy, payloads and visibility.
3. No database migration. Rollback is a code revert; stored receipt paths are
   unchanged, and the previous URL builder resumes producing 404 links.

## Open Questions

None.
