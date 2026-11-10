## Why

Step 2 of the customer reservation wizard offers **bank financing**, but the API
only stores a single `max_payment_date` for that modality. A bank does not commit
to a disbursement date when the customer reserves, so the portal is forced to
invent one. Meanwhile the information the sales team actually needs — which bank
the customer intends to use and how far the credit process has advanced — is not
persisted anywhere.

## What Changes

- Add nullable `bank_name` and `bank_credit_status` columns to `contracts`
  (migration `000021`).
- Restrict `bank_credit_status` to `not_started`, `pre_qualified`,
  `pre_approved` and `approved`. Empty/null means "not provided".
- Accept `bank_name` and `bank_credit_status` when a customer creates a
  reservation with `financing_type=bank`; ignore them for other modalities.
- Stop requiring `max_payment_date` when creating a **bank** reservation. Cash
  reservations keep requiring it because the buyer knows that date.
- Treat `max_payment_date` as a seller-managed field for bank contracts:
  optional at reservation time, editable through the admin PATCH endpoint, and
  required before a bank contract can be approved (the payment schedule needs
  it).
- Expose `bank_name` and `bank_credit_status` in `ContractResponse`.
- Allow admins to update both fields through `PATCH /contracts/:id`.
- Update tests and regenerate Swagger.

## Capabilities

### New Capabilities

- `contract-bank-financing`: bank-financing details captured at reservation
  time and the payment-date rules for the bank modality.

### Modified Capabilities

None.

## Impact

- `internal/database/migrations/000021_add_contract_bank_credit_fields.*.sql`
- `internal/models/contract.go`
- `internal/handlers/contract_handler.go`
- `internal/services/payment_schedule_service.go` (approval guard only)
- `internal/database/seeds/seeds.sql`
- `docs/` (regenerated Swagger)
- Consumers: FinteraWeb reservation wizard (`feat/bank-credit-step-web`) and
  FinteraUI contract details (`feat/bank-credit-details-ui`). New fields are
  additive and nullable, so existing clients keep working.
