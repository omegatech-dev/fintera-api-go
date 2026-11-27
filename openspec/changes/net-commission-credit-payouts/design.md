## Context

Seller commission credits are created in two flows: cancellation of a paid
commission (`ContractService.Cancel` → `CommissionService.CreateCancellationCredit`)
and reassignment of a paid commission (`ContractService.Reassign` →
`CreateCommissionCredit`). They are recovered manually with
`POST /commissions/credits/:id/apply`, which only flips `pending` to `settled`.
Commission payouts (`PATCH /contracts/:guid/commission/pay`) write the gross
amount and never look at credits.

Two invariants make the current model insufficient:

- `contracts.guid` is `VARCHAR(255)`, not a UUID, so applications must reference
  `contracts(id)`.
- Repositories own GORM access; services must not open transactions. Existing
  transactional methods live in the repository layer
  (`contract_repository.go`, `ledger_repository.go`).

## Goals / Non-Goals

**Goals:**

- Make the payout the single place where credits are consumed, atomically with
  marking the commission paid.
- Keep an auditable, reversible record of every credit application.
- Restore credits on cancellation/reassignment and recreate only the cash
  portion of the recovered commission.
- Fix the two consumers of pending credits (JSON report and export) at once.
- Ship safely: migration + query changes can be deployed with netting off.

**Non-Goals:**

- No contract ledger changes. The seller credit is an account between the
  company and the seller; the customer's balance does not move.
- No change to commission rate resolution, accrual date, or gross amount.
- No cash-collection / write-off workflow (phase 2).
- No UI changes in this repository.

## Decisions

**Transaction boundary in `commissionRepository`.**
One method performs the whole payout inside `db.Transaction`: lock the contract
row with `SELECT ... FOR UPDATE`, re-check `commission_paid_at IS NULL` and the
contract status, select the seller's pending credits `FOR UPDATE` ordered by
`created_at, id`, insert applications, update credits, then update the contract.
Alternative: expose a `UnitOfWork`/`WithTx` to services. Rejected: it breaks the
repository abstraction and there is no precedent in the codebase.

**Lock ordering.**
Always contract first, then credits ordered by id/created_at. This serializes
payouts for the same seller and avoids deadlocks between concurrent contracts.

**Soft reversal.**
Applications use `reversed_at`/`reversed_by` instead of `DELETE`. The unique
constraint is partial (`WHERE reversed_at IS NULL`) so the same credit can be
applied to the same contract again after a reversal, while duplicate active
applications remain impossible. Deleting would lose the audit trail.

**Credit balance is derived, settlement is guarded.**
`applied_amount` is the source of truth for consumption; `remaining =
amount - applied_amount`. A database CHECK enforces `settled ⇒ applied =
amount`. Because the legacy manual apply only updated status, `SettleCredit`
must also set `applied_amount = amount` in the same UPDATE, otherwise the
constraint would reject manual settlement while netting is off.
The migration also installs a compatibility trigger that promotes an old
server's status-only settlement to a full applied amount during a rolling
deployment; the new repository writes the amount explicitly.

**Reversal is data-driven.**
Cancellation and reassignment check for active applications, not the flag. If
applications exist: reverse them, restore balances, and create the new credit
for `gross - applied`. If none exist (legacy), keep the current gross behavior.
This keeps rollout order independent.
Cancellation persists its status under a contract row lock and preserves the
latest commission payment fields, so a payout that finishes concurrently is
still detected and recovered instead of being overwritten by a stale snapshot.

**Rename insurance.**
The credit's existing `contract_id`/`contract_guid` keep meaning "origin
contract". Applications expose `applied_contract_id`/`applied_contract_guid`
to avoid confusing origin with destination.

**Rounding.**
Allocations are rounded to 2 decimals; the last allocation absorbs the residue
(`gross - sum(previous)`), and comparisons use an epsilon of 0.005.

**Netting is unconditional.**
Payouts always consume the seller's pending credits. The preview always returns
`200` with the full breakdown so the UI can confirm before paying.

**Backfill coherence.**
After the migration, every pending credit has `applied_amount = 0`, so
`SUM(amount - applied_amount) = SUM(amount)`. Changing
`ListPendingCreditsBySeller` and the report loop keeps historical report totals
stable.

**`source` backfill.**
Existing rows get `source = 'legacy'` because the old code did not record
whether a credit came from cancellation or reassignment. Claiming
`cancellation` would invent data.

## Risks / Trade-offs

- **Cancellation/reassignment remain multi-statement.** The credit
  restore-and-recreate step is transactional on its own, but the contract
  update is outside it, as in today's flow. If the process dies between them the
  contract may be cancelled without its new credit. Mitigation: reversal is
  idempotent and the failing step is retryable with a repair script; a full
  unit-of-work refactor is out of scope.
- **Negative `net_pending` remains possible.** It mixes global credits with the
  selected period. It is kept for compatibility and marked deprecated; the UI
  moves to `seller_balance`.
- **Manual apply is restricted.** `POST /commissions/credits/:id/apply`
  returns `422` for any credit with a remaining balance; only zero-balance
  pending credits can be settled manually. The legacy escape hatch of settling
  a full credit by hand is gone, which is the intended breaking change.
- **Legacy credits cannot be reversed.** They have no applications; the backfill
  marks them `settled` and they are excluded from netting.
- **Rollback.** There is no runtime flag; rolling back means reverting the
  deploy. Payout behavior is deterministic and every application is recorded
  with its reversal path, so a revert does not corrupt state. The migration's
  `down` drops applications and the new columns; reversed history is lost,
  which is acceptable only if no netting happened in production.

## Migration Plan

1. Deploy migration `000048` together with the API changes; netting is active
   immediately, and legacy settled rows are excluded from it.
2. The UI ships the payout breakdown and preview rendering with the backend.

## Open Questions

None. The API contract with the frontend is closed.
