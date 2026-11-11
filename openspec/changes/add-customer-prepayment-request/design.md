## Context

`ContractService.CapitalRepayment` already applies an admin-initiated capital
payment: it records a paid `capital_repayment` payment, writes a prepayment
ledger entry, and shortens the schedule by deleting or reducing future
installments from the end.

Customers can already upload a receipt for any payment they own. What is missing
is a payment record that represents an early capital payment: without it the
customer has nothing to attach a receipt to, and staff has nothing to approve.

See `proposal.md` and
`openspec/changes/add-customer-prepayment-request/specs/customer-prepayments/spec.md`.

## Goals / Non-Goals

**Goals:**

- Let customers request a partial or full capital payment from the portal.
- Reuse the existing receipt upload and admin approval paths.
- Apply the approved amount as capital, shortening the term.

**Non-Goals:**

- Changing the admin capital repayment endpoint or its UI.
- Automatic approval of customer transfers.
- Recalculating interest schedules beyond the current reduce-term behavior.

## Decisions

### Request endpoint creates a submitted payment

`POST /contracts/:guid/prepayments` creates a `capital_repayment` payment with
the requested amount, `due_date = now` and `status = submitted`. The existing
`POST /payments/:payment_id/upload_receipt` then attaches the proof. This keeps
one record per request and lets staff see it in the normal payment schedule.

### Validation in the service

`ContractService.CreatePrepaymentRequest` checks:

- the contract is `approved`;
- the ledger balance is negative (there is debt) and the amount does not exceed
  `-balance`;
- no other `capital_repayment` payment is already `submitted`.

Access control stays in the handler: owner, admin or seller.

### Approval branch

`PaymentService.Approve` now branches on payment type. For
`capital_repayment`:

- the whole approved amount is applied to pending installments from the end
  (`applyToPendingInstallments`), reducing or deleting them;
- the ledger gets only a `prepayment` credit (no regular `payment` entry).

Regular payments keep the existing behavior: only the excess above the
payment's own obligation becomes capital.

## Risks / Trade-offs

- **Two records for the same intent?** No: the request is the payment record and
  approval updates it in place.
- **Partial coverage.** If the amount exceeds the remaining installments, the
  leftover still credits the ledger; the balance refresh/auto-close handles the
  final state.
- **Schedule order dependency.** Reducing from the end assumes installments are
  loaded in due-date order, matching the existing `CapitalRepayment` behavior.

## Migration Plan

Additive endpoint and an approval branch; deploy with the normal API rollout. No
data migration.
