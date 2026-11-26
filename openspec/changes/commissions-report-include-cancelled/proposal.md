## Why

The commissions report filters contracts to `approved` and `closed`, so the moment a contract with an already-paid commission is cancelled its paid-commission row disappears from the report and exports. That reads as if the payment record was deleted, even though the payout remains on the contract and the recoverable credit exists.

## What Changes

- Include `cancelled` contracts in the commissions report (JSON and CSV/PDF export) so paid commissions stay visible.
- Expose the contract status on every commission row (`contract_status`) and mark cancelled rows in the export's state column.
- Keep the paid/pending totals semantics: commissions of cancelled contracts count as paid when their payout happened; the recoverable credit continues to surface separately in `pending_credits` and `seller_balance`.
- Document the behavior and the staging rollout of `COMMISSION_CREDIT_NETTING_ENABLED` for automatic credit application.

## Capabilities

### New Capabilities

- `commissions-report-visibility`: which contracts the commissions report and its exports include and how cancelled contracts are represented.

### Modified Capabilities

<!-- No canonical specs exist yet in openspec/specs. -->

## Impact

- `internal/services/report_service.go` (status filter, row field).
- `internal/services/commissions_report.go` (export state column).
- `internal/handlers/resource_handlers.go` Swagger note; `docs/architecture/api-and-auth.md`.
- Tests for the report rows, totals and export rendering.
