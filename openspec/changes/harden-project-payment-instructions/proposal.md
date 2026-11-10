## Why

FinteraAPI already stores per-project bank accounts (`project_payment_instructions`)
and serves them through an admin CRUD API and a public endpoint, but the
capability is effectively invisible:

- the admin endpoints are missing from Swagger;
- `project_payment_instructions` is absent from the data-model docs;
- demo environments have no accounts, so the portal has nothing real to show.

The admin UI is being added in FinteraUI (`add-project-payment-instructions-admin`),
and once accounts are the source of truth the portal must never fall back to
demo data (FinteraWeb `remove-payment-instruction-fallback`).

## What Changes

- Add Swagger annotations and typed request/response models for:
  - `GET /admin/projects/:project_id/payment-instructions`
  - `POST /admin/projects/:project_id/payment-instructions`
  - `PATCH /admin/projects/:project_id/payment-instructions/:instruction_id`
  - `DELETE /admin/projects/:project_id/payment-instructions/:instruction_id`
- Return `[]models.PaymentInstructionResponse` instead of `[]interface{}`.
- Harden validation: trim inputs, cap `bank_name` (120), `beneficiary_name`
  (160), `account_number` (60) and `notes` (500), normalize `account_type`.
- Seed one or two accounts for each demo project, idempotently.
- Document the table and the account-type enum in
  `docs/architecture/data-model.md`.
- Add service and handler tests.

## Capabilities

### New Capabilities

- `project-payment-instructions`: the bank accounts a project exposes to
  customers, their validation, and their admin/public contract.

### Modified Capabilities

None.

## Impact

- `internal/handlers/payment_instruction_handler.go`
- `internal/services/payment_instruction_service.go`
- `internal/services/payment_instruction_service_test.go` (new)
- `internal/handlers/payment_instruction_handler_test.go` (new)
- `internal/database/seeds/seeds.sql`
- `docs/architecture/data-model.md`
- `docs/` (regenerated Swagger)
- Consumers: FinteraUI admin page and FinteraWeb reservation payment step.
  The public payload is unchanged, so existing clients keep working.
