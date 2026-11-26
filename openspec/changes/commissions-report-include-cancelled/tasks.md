## 1. Report rows and filter

- [x] 1.1 Include `cancelled` in the `status_in` filter of `GenerateCommissions` in `internal/services/report_service.go`
- [x] 1.2 Add `contract_status` to `CommissionReportItem` and populate it for contract rows; leave historical sale rows without it

## 2. Export rendering

- [x] 2.1 Render cancelled contracts in `commissionsDetailRow` (`internal/services/commissions_report.go`) with the paid/pending state plus the cancelled contract note

## 3. Tests

- [x] 3.1 Add report tests: cancelled paid row visible with `contract_status`, cancelled unpaid row pending, totals and credits reconcile
- [x] 3.2 Add export row test for cancelled contracts

## 4. Docs and config

- [x] 4.1 Document cancelled-row behavior in `docs/architecture/api-and-auth.md`
- [x] 4.2 Note the staging rollout of `COMMISSION_CREDIT_NETTING_ENABLED` in `.env.example`

## 5. Verification

- [x] 5.1 Run `gofmt`, `go build ./...`, `go test ./...`, `git diff --check`, and OpenSpec validation
