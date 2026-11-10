## 1. Handler and Swagger

- [x] 1.1 Add Swagger annotations to `Index`, `Create`, `Update` and `Delete` in `internal/handlers/payment_instruction_handler.go`.
- [x] 1.2 Return `[]models.PaymentInstructionResponse` from `Index` instead of `[]interface{}`.
- [x] 1.3 Document the request body shape for create/update.
- [x] 1.4 Run `make swagger` and commit the regenerated `docs/`.

## 2. Validation

- [x] 2.1 Trim bank, beneficiary, account number and notes in `validateInstruction`.
- [x] 2.2 Enforce max lengths: bank 120, beneficiary 160, account 60, notes 500.
- [x] 2.3 Normalize `account_type` to lowercase and default to `savings`.
- [x] 2.4 Return a typed validation error that the handler maps to `422`.

## 3. Seeds

- [x] 3.1 Add idempotent accounts for the demo projects in `internal/database/seeds/seeds.sql` (keyed by project + account number).
- [x] 3.2 Keep the seeded values clearly demo-oriented.

## 4. Documentation

- [x] 4.1 Add `project_payment_instructions` to `docs/architecture/data-model.md` with columns and the account-type enum.
- [x] 4.2 Mention the public payload fields in the data-model notes.

## 5. Tests

- [x] 5.1 Add `internal/services/payment_instruction_service_test.go` for required fields, account type, length caps and defaults.
- [x] 5.2 Add `internal/handlers/payment_instruction_handler_test.go` for list, create validation and cross-project access.
- [x] 5.3 Verify the public payload excludes ids, positions and timestamps.

## 6. Verification

- [x] 6.1 `go build ./...` passes.
- [x] 6.2 `go test ./...` passes.
- [x] 6.3 `openspec validate --changes` passes.
