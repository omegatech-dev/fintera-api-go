## 1. Schema and models

- [x] 1.1 Add `000048_commission_credit_applications.up.sql` / `.down.sql` with `applied_amount`, `source`, the applied-range and settled-full checks, `credit_applications`, the partial unique index, and the `source = 'legacy'` backfill
- [x] 1.2 Extend `internal/models/seller_commission_credit.go` with `AppliedAmount`, `Source`, `RemainingAmount`, source constants, and the new response fields
- [x] 1.3 Add `internal/models/credit_application.go` with the GORM model and payout/preview/application DTOs

## 2. Repository layer

- [x] 2.1 Extend `internal/repository/commission_repository.go` interface and implementation: payout transaction, preview, restore, active-application sum, remaining-based aggregate, credit `source`
- [x] 2.2 Make `SettleCredit` set `applied_amount = amount` so the settled-full check holds with netting off
- [x] 2.3 Load active applications for credit listings without per-row queries

## 3. Services and wiring

- [x] 3.1 Add `COMMISSION_CREDIT_NETTING_ENABLED` to `internal/config/config.go` and inject it in `internal/services/services.go`
- [x] 3.2 Implement `PayCommission`, `PreviewCommissionPayout`, and `RestoreAndCreateCredit` in `internal/services/commission_service.go`, including the manual-apply guard
- [x] 3.3 Delegate `ContractService.MarkCommissionPaid` to the payout flow and return the payout breakdown
- [x] 3.4 Update `ContractService.Cancel` to restore applications and create the credit only for cash paid
- [x] 3.5 Update `ContractService.Reassign` to restore applications, create the cash credit, and always clear the paid flags when the seller changes

## 4. API

- [x] 4.1 Extend `PayCommission` handler and add the payout-preview handler with Swagger annotations in `internal/handlers/contract_handler.go`
- [x] 4.2 Register `GET /contracts/:guid/commission/payout-preview` in the admin group of `cmd/api/main.go`
- [x] 4.3 Add `seller_balance` and remaining-based `pending_credits` to `internal/handlers/resource_handlers.go` and `internal/services/report_service.go`
- [x] 4.4 Update `internal/services/commissions_report.go` export to use remaining balances

## 5. Tests

- [x] 5.1 Add allocation-planning unit tests (exact, partial, FIFO, rounding, other-seller exclusion)
- [x] 5.2 Update service fakes/stubs for the new repository methods and add payout/reversal/apply-guard tests
- [x] 5.3 Add handler tests for the payout response and preview (enabled/disabled)
- [x] 5.4 Add a `DATABASE_URL`-gated repository test for concurrency/idempotency and application reversal

## 6. Verification and docs

- [x] 6.1 `go build ./...` and `go test ./...`
- [x] 6.2 `make swagger` and regenerate docs
- [x] 6.3 Update `docs/architecture/data-model.md`, `domain-flows.md`, and `api-and-auth.md`
- [x] 6.4 `gofmt` changed Go files and review `git diff`
