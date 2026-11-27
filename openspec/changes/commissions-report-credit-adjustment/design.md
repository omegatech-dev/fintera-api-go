## Context

With credit netting, a payout can settle a commission partly (or fully)
with the seller's credits. `credit_applications` records exactly which credits
were consumed per contract, but the commissions report shows only the gross
commission and its paid/pending state, so a netted row reads like a fresh full
cash payment.

## Goals / Non-Goals

**Goals:**

- Report rows expose `credit_applied` (sum of active applications for the
  contract) so the UI can show gross, credit, and cash.
- The export states the adjustment in the existing "Estado" column.
- Zero extra queries per row: one grouped query for the whole report.

**Non-Goals:**

- No change to payout, preview, or the credit lifecycle.
- No change to the paid/pending totals semantics: the commission is still paid
  for its gross amount; the credits section reconciles the seller balance.

## Decisions

**One grouped query per report.** `ListAppliedTotalByContractIDs` sums active
applications grouped by `contract_id` for the report's contract IDs, mirroring
how credits are loaded for the page. Historical sale rows have no contract id
and are skipped.

**Adjustment lives in the state column.** The detail table already carries the
payment state; adding the credit note there ("Reajuste por crédito: L X
(efectivo L Y)") explains the difference without widening every seller table.
The JSON exposes `credit_applied` so the UI can render its own layout.

**Reversed applications are excluded.** Only active applications count,
matching the credit balance arithmetic (`amount - applied_amount`).

## Risks / Trade-offs

- Rows gain a longer state text when netting is used. → The note only appears
  on rows that actually consumed credits.
- Reports require the commission repository to be wired. → Optional dependency
  degrades to `credit_applied = 0`, matching pre-netting behavior.

## Migration Plan

Code-only; no schema change. Binary rollback.

## Open Questions

None.
