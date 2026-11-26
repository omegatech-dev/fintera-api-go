## 1. Data

- [x] 1.1 Add `ListAppliedTotalByContractIDs` to `CommissionRepository` (active applications summed per contract)
- [x] 1.2 Add `credit_applied` to `CommissionReportItem` and populate it in `GenerateCommissions` for paid contract rows

## 2. Export

- [x] 2.1 Render the "Reajuste por crédito" note with the cash portion in `commissionsDetailRow`

## 3. Tests

- [x] 3.1 Update repository fakes with the new method and add report/export tests for netted, fully netted and unnetted rows

## 4. Docs and verification

- [x] 4.1 Document `credit_applied` and the export note in `docs/architecture/api-and-auth.md`
- [x] 4.2 Run `gofmt`, `go build ./...`, `go test ./...`, `git diff --check`, and OpenSpec validation
