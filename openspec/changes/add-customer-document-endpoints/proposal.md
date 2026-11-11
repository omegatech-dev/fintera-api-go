## Why

Customers can list their contracts, payments and financing summary and can
download payment receipts, but the three documents they care about — the promise
of sale contract, the customer record and the account balance — are only
registered under the `sellerAdmin` role group. A customer cannot download their
own documents from the portal.

## What Changes

- Add owner-scoped document endpoints that reuse the existing report generators:
  - `GET /users/:user_id/promise_contract_pdf?contract_id=`
  - `GET /users/:user_id/customer_record_pdf?contract_id=`
  - `GET /users/:user_id/balance_pdf`
- Enforce ownership: the authenticated user must own the requested contract and
  user unless the caller is an admin or seller.
- Register the routes in the existing `userData` group so the
  `RequireAdminSellerOrOwner` middleware applies.
- Add Swagger annotations and handler tests.

## Capabilities

### New Capabilities

- `customer-documents`: owner-scoped PDF document downloads for customers.

### Modified Capabilities

None.

## Impact

- `internal/handlers/user_handler.go` (new methods + `reportService` dependency)
- `internal/handlers/handlers.go` (constructor wiring)
- `cmd/api/main.go` (route registration)
- `docs/` (Swagger)
- `internal/handlers/user_handler_test.go`
- Consumers: FinteraWeb `redesign-customer-reservations`.
