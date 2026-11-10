## 1. Data model

- [x] 1.1 Add `internal/database/migrations/000021_add_contract_bank_credit_fields.up.sql` adding nullable `bank_name VARCHAR(120)` and `bank_credit_status VARCHAR(30)` to `contracts`.
- [x] 1.2 Add the paired `.down.sql` dropping both columns.
- [x] 1.3 Add `BankName *string` and `BankCreditStatus *string` to `models.Contract` and `models.ContractResponse`, and copy them in `ToResponse`.
- [x] 1.4 Add the bank credit status constants (`not_started`, `pre_qualified`, `pre_approved`, `approved`) and a normalizer/validator helper.

## 2. Reservation creation

- [x] 2.1 Read `contract[bank_name]` and `contract[bank_credit_status]` in `ContractHandler.create`.
- [x] 2.2 Validate the status and return `400` for unknown values.
- [x] 2.3 Store both fields only when `financing_type=bank`; otherwise store `null`.
- [x] 2.4 Require `max_payment_date` only for `cash`, not for `bank`.

## 3. Admin update

- [x] 3.1 Add `BankName` and `BankCreditStatus` to `UpdateContractRequest`.
- [x] 3.2 Apply the same validation and bank-only rule in `ContractHandler.Update`.
- [x] 3.3 Allow clearing both fields with empty values.

## 4. Approval guard

- [x] 4.1 Keep `max_payment_date` required when approving a bank contract; surface a clear error when it is missing.
- [x] 4.2 Add a test for approving a bank contract without a payment date.

## 5. Seeds and documentation

- [x] 5.1 Add realistic `bank_name`/`bank_credit_status` values for seeded bank contracts in `internal/database/seeds/seeds.sql`.
- [x] 5.2 Run `make swagger` and commit the regenerated `docs/` output.
- [x] 5.3 Update `docs/architecture/data-model.md` with the new `contracts` columns.

## 6. Tests

- [x] 6.1 Handler/service tests for bank reservation with and without details.
- [x] 6.2 Test invalid `bank_credit_status` returns `400`.
- [x] 6.3 Test non-bank reservations ignore bank fields.
- [x] 6.4 Test cash still requires `max_payment_date` and bank does not.
- [x] 6.5 Test admin PATCH updates and clears the bank fields.

## 7. Verification

- [x] 7.1 `go build ./...` passes.
- [x] 7.2 `go test ./...` passes.
- [x] 7.3 `openspec validate --changes` passes.
