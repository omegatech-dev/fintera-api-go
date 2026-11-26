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

**Flag.**
`COMMISSION_CREDIT_NETTING_ENABLED` (default `false`) follows the existing
`IDENTITY_DOCUMENTS_REQUIRED` pattern. It gates only automatic netting and the
manual-apply guard. The preview always returns `200` with `enabled` so the UI
can deploy first.

**Query changes are flag-independent.**
After the migration, every pending credit has `applied_amount = 0`, so
`SUM(amount - applied_amount) = SUM(amount)`. Changing
`ListPendingCreditsBySeller` and the report loop is safe with the flag off.

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
- **Manual apply with the flag off can still settle a credit by hand.** That is
  the current behavior and preserves the legacy escape hatch. Once netting is
  enabled the endpoint returns `422` for any credit with a balance.
- **Legacy credits cannot be reversed.** They have no applications; the backfill
  marks them `settled` and they are excluded from netting.
- **Rollback.** Turn the flag off: payouts return to gross behavior and the
  manual apply path still works. The migration's `down` drops applications and
  the new columns; reversed history is lost, which is acceptable only if no
  netting happened in production.

## Migration Plan

1. Deploy migration `000048` and the remaining-based queries with the flag off.
2. Deploy the payout/preview endpoints and reversals (inactive while the flag is
   off, except data-driven reversals for legacy contracts).
3. Enable `COMMISSION_CREDIT_NETTING_ENABLED=true` in staging, run the
   acceptance tests, then in production.
4. The UI can ship before or after step 3 and falls back to gross rendering when
   `payout` is absent.

## Open Questions

None. The API contract with the frontend is closed.
