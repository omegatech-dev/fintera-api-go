## Why

A commission paid with seller credits shows in the report as "Comisión L 16,800 · Pagada" even when only L 3,000 was actually paid in cash. The remaining L 13,800 came from credits of cancelled contracts, and the report gives no explanation, so the row looks like a fresh full payment.

## What Changes

- Expose `credit_applied` on every commission report row (JSON): the sum of active credit applications for that contract.
- Mark netted rows in the CSV/PDF export: "Pagada el <fecha> · Reajuste por crédito: L <aplicado> (efectivo L <efectivo>)".
- Unnetted rows render exactly as today; cancelled-contract notes keep working together with the adjustment.

## Capabilities

### New Capabilities

<!-- Reuses the `commissions-report-visibility` capability introduced by commissions-report-include-cancelled. -->

### Modified Capabilities

- `commissions-report-visibility`: commission rows now also expose how much of the paid commission was settled with seller credits, and the export states the adjustment.

## Impact

- `internal/repository/commission_repository.go`: grouped lookup of active credit applications per contract.
- `internal/services/report_service.go` (`CommissionReportItem` and `GenerateCommissions`), `internal/services/commissions_report.go` (detail row).
- Repository fakes in tests; `docs/architecture/api-and-auth.md`.
