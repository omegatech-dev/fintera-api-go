## Context

The report package already generates the promise of sale contract, customer
record and user balance PDFs, but the routes are registered only in the
`sellerAdmin` group (`cmd/api/main.go`). The customer-facing `userData` group
already applies `RequireAdminSellerOrOwner`, and `DownloadReceipt` establishes
the ownership-check pattern for customer files.

`UserHandler` currently holds `userService`, `paymentService` and an optional
`contractService`; `ReportHandler` holds only `reportService`.

See `proposal.md` and
`openspec/changes/add-customer-document-endpoints/specs/customer-documents/spec.md`.

## Goals / Non-Goals

**Goals:**

- Give customers their own contract, customer-record and balance PDFs.
- Reuse the existing report generators and ownership middleware.
- Keep staff access unchanged.

**Non-Goals:**

- Changing the seller/admin report routes or generators.
- Adding new PDF templates.
- Changing receipt upload/download behavior.

## Decisions

### Register under the `userData` group

The `userData` group (`/users/:user_id`) already runs
`RequireAdminSellerOrOwner`, which covers the balance PDF and the `user_id`
ownership check for the contract documents. Adding the three routes there avoids
duplicating middleware and keeps customer endpoints together.

### Contract ownership verification

For the two contract-scoped documents, the handler loads the contract through
`contractService` and compares `contract.ApplicantUserID` with the path
`user_id`, mirroring `PaymentHandler.DownloadReceipt`. Admin and seller roles
bypass the comparison. A contract that does not exist returns `404`; a contract
owned by someone else returns `403`.

### Inject the report service into `UserHandler`

`NewUserHandler` gains a fourth dependency (`*services.ReportService`), wired in
`handlers.go`. This keeps the PDF generation in `ReportService` and avoids moving
template code into `UserHandler`.

### Stream PDFs with explicit headers

Each handler writes `Content-Type: application/pdf` and
`Content-Disposition: attachment; filename=...`, matching the existing report
handlers, so browsers download instead of rendering inline.

## Risks / Trade-offs

- **New dependency edge.** `UserHandler` now depends on `ReportService`; the
  constructor is variadic today, so update the single call site in
  `handlers.go` carefully.
- **PDF cost.** Generating PDFs is CPU-bound; the endpoints are per-request only
  and authenticated, so no new caching is introduced.
- **PDF rendering.** Report PDFs are rendered natively with
  `gofpdf`; no external PDF binary is required.

## Observability

PDF generation failures are logged with `logger.Error` and include the acting
`user_id`, the `contract_id` (contract documents) or `target_user_id` (balance)
and the wrapped error, so a `500` can be diagnosed from the server log without
reproducing the request.

## Migration Plan

Additive endpoints; deploy with the normal API rollout. Existing seller/admin
routes and report generators are untouched.
