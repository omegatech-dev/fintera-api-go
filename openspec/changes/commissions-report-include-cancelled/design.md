## Context

`ReportService.GenerateCommissions` builds its query with
`Filters["status_in"] = approved,closed`. When a contract with a paid
commission is cancelled, the row disappears from the JSON report and the
CSV/PDF export, even though `contracts.commission_paid_at` remains set and the
cancellation created a recoverable `seller_commission_credits` row. The seller
dashboard and the credits list keep their data; only the commission report
loses the row.

## Goals / Non-Goals

**Goals:**

- Keep paid commissions visible after cancellation, in JSON and exports.
- Expose the contract lifecycle status per commission row.
- Keep totals consistent: paid totals follow the payout; credits are reported
  separately so the seller balance is not double counted.

**Non-Goals:**

- No change to the credit netting behavior or the payout flow.
- No change to the seller dashboard.
- Rejection status stays out of the report: rejected contracts never had an
  approved reservation to accrue a commission.

## Decisions

**Include `cancelled` in the existing `status_in` filter.** Cancelled contracts
keep their `commission_accrual_date`, so the existing period filter still
applies. Adding a new filter or a separate section would split the report.

**Expose raw `contract_status` on each row.** The JSON keeps one additive
field (`contract_status`); the UI can translate it. Historical sale rows omit
it because they have no contract lifecycle.

**Export marks the contract state inside the existing "Estado" column.** A
cancelled contract renders `Pagada el <fecha> (contrato cancelado)` or
`Cancelada (sin pago)`; the row keeps the paid/pending visual style. Adding a
new column would widen every seller table for a rare state.

**Totals stay payout-driven.** `commissionsGroupItems` and
`CommissionTotalsFor` already classify rows by `CommissionPaidAt`; cancelled
rows flow through unchanged. The recoverable credit is reported in the credits
section, so no double counting: the commission is paid, and its credit is
pending until applied.

## Risks / Trade-offs

- **Paid totals grow when a paid contract is cancelled.** → Correct from an
  expense standpoint; the credit section shows the recoverable amount.
- **Reports gain rows that were previously hidden.** → Search, seller filter
  and pagination already handle arbitrary rows; no UI contract breaks because
  `contract_status` is additive.

## Migration Plan

Code-only change; no schema migration. Rollback is a binary rollback.

## Open Questions

None.
