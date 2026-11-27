## Why

Paid seller commissions and cancellation credits never meet. Paying a commission
pays the full gross amount and ignores the seller's pending credits, while
"applying" a credit only flips a row to `settled` without recording an amount,
a destination contract, or any reversal path. The admin either pays gross or
settles the credit by hand, so the amount actually recovered is never known, and
if the consuming contract is later cancelled the credit cannot be restored.

## What Changes

- Add `applied_amount` and `source` to `seller_commission_credits`, plus a
  `credit_applications` table that records which credit paid which contract.
- Net pending seller credits automatically when an admin marks a commission as
  paid: FIFO, same seller only, inside one transaction with a contract row lock.
  Netting is unconditional; there is no feature flag.
- Return the payout breakdown (`commission`, `applied_total`, `net_cash`,
  `remaining_credit`, `applications`) from the existing pay endpoint.
- Add an admin payout-preview endpoint so the admin can confirm gross, applied
  credit, and cash before paying.
- Restore applied credits when a netted commission is cancelled or reassigned,
  and create the new recoverable credit only for the cash actually paid.
- Reverse applications softly (`reversed_at`/`reversed_by`) so the audit trail
  survives and the same credit can be applied again after a reversal.
- Change `pending_credits` to count the remaining balance (`amount -
  applied_amount`) in both the JSON report and the CSV/PDF export.
- Add `seller_balance` (global remaining seller credit) to the commissions
  report without changing the existing `net_pending` semantics.
- **BREAKING** for admins: `POST /commissions/credits/:id/apply` returns `422`
  while the credit still has a remaining balance, because netting happens at
  payout time. Only zero-balance pending credits can be settled manually.
- Explicit no-goals: the contract ledger, the commission rate/amount
  calculation, the accrual date, and the customer balance are untouched.

## Capabilities

### New Capabilities

- `commission-credit-netting`: how seller commission credits are consumed at
  payout, previewed, reversed on cancellation/reassignment, reported, and kept
  consistent between JSON and exports.

### Modified Capabilities

<!-- None. This repository has no archived specs yet. -->

## Impact

- Migrations: `000048_commission_credit_applications`.
- Models: `SellerCommissionCredit`, new `CreditApplication`, payout DTOs.
- Repositories: `CommissionRepository` (transactional payout, preview,
  reversal, remaining-based aggregation).
- Services: `CommissionService`, `ContractService` (`MarkCommissionPaid`,
  `Cancel`, reassignment), `ReportService`, `ReportExportService`.
- Handlers/routes: `PATCH /contracts/:guid/commission/pay`, new
  `GET /contracts/:guid/commission/payout-preview`,
  `GET /reports/commissions`, `POST /commissions/credits/:id/apply`.
- Docs: `docs/architecture/data-model.md`, `domain-flows.md`,
  `api-and-auth.md`, and regenerated Swagger.
