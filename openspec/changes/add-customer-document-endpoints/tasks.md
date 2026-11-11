## 1. Handler

- [x] 1.1 Add `reportService` to `UserHandler` and `NewUserHandler`.
- [x] 1.2 Add `PromiseContractPDF` with contract ownership verification.
- [x] 1.3 Add `CustomerRecordPDF` with contract ownership verification.
- [x] 1.4 Add `BalancePDF` for the authenticated user.
- [x] 1.5 Return `400` for missing ids, `403` for foreign contracts, `404` when
      the contract does not exist, and stream `application/pdf` on success.
- [x] 1.6 Log generation failures with `logger.Error`, including the acting
      `user_id`, `contract_id`/`target_user_id` and the wrapped error.

## 2. Wiring and routes

- [x] 2.1 Pass `svcs.Report` in `internal/handlers/handlers.go`.
- [x] 2.2 Register the three routes in the `userData` group in
      `cmd/api/main.go`.

## 3. Docs

- [x] 3.1 Add Swagger annotations for the new endpoints.
- [x] 3.2 Regenerate Swagger (`make swagger`).

## 3b. Balance statement

- [x] 3b.1 Build typed statement data: customer, account totals, contract
      details, full payment schedule and ledger movements.
- [x] 3b.2 Format money with thousands separators and translate contract,
      payment, financing and ledger labels to Spanish.
- [x] 3b.3 Redesign `user_balance.html` with the Inversiones FAMA header,
      summary tiles, per-contract lot/financing details and a full schedule.
- [x] 3b.4 Add page numbers to the balance PDF footer.
- [x] 3b.5 Test the statement payload and template rendering, plus PDF
      generation when wkhtmltopdf is available.

## 4. Tests

- [x] 4.1 Owner can download all three documents.
- [x] 4.2 A different customer receives `403`.
- [x] 4.3 Admin and seller can download any customer's documents.
- [x] 4.4 Missing or invalid `contract_id` returns `400`.

## 5. Verification

- [x] 5.1 `go build ./...`
- [x] 5.2 `go test ./...`
- [x] 5.3 `golangci-lint run --timeout=5m`
- [x] 5.4 `openspec validate add-customer-document-endpoints`
- [x] 5.5 Download all three PDFs against the local API with a real customer
      token and confirm `200 application/pdf`.
