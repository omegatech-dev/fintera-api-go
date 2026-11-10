## Context

`Contract` already models the financing modality (`direct`, `bank`, `cash`) and
`max_payment_date`. For `bank` and `cash`, `PaymentScheduleService` builds a
two-payment schedule (reservation + full balance) and requires
`max_payment_date`. The customer wizard currently fills that date at reservation
time, which is only realistic for cash.

See `proposal.md` and
`openspec/changes/add-bank-credit-details/specs/contract-bank-financing/spec.md`.

## Goals / Non-Goals

**Goals:**

- Persist the bank and the credit-application stage the customer reports.
- Remove the fake-date requirement for bank reservations without breaking the
  payment schedule.
- Keep cash behavior unchanged.
- Keep the change additive and backwards compatible.

**Non-Goals:**

- Integrating with bank APIs or verifying pre-approval documents.
- Modeling loan terms (rate, LTV, amortization) — the bank keeps those.
- Changing the 48-hour reservation hold or the L-5,000 reservation payment.
- A new endpoint.

## Decisions

### Nullable `bank_name` / `bank_credit_status` columns

Both columns are nullable so the migration is additive and existing contracts
stay valid. `bank_name` is free text (`VARCHAR(120)`) because the set of
lenders is open and includes financial companies. `bank_credit_status` is a
small closed set (`VARCHAR(30)`) validated in the handler; storing free text
would make reporting and UI labels unreliable.

### Bank fields only apply to `financing_type=bank`

When the reservation is `direct` or `cash`, the handler clears both fields. This
avoids meaningless data if a client sends all fields regardless of the selected
modality.

### `max_payment_date` is seller-managed for bank

Bank reservations no longer require the date at creation. The customer reports
the bank and the credit stage instead. The sales/admin team sets the real
deadline through PATCH once the bank confirms it. Approval keeps requiring the
date because `GenerateSchedule` needs a due date for the balance payment; the
existing error from the schedule service remains the guard, surfaced as an
approval error with a clear message.

### Validation

`bank_credit_status` must be one of the four known values or empty. An unknown
value returns `400`. `bank_name` is trimmed and capped by the column length.

## Risks / Trade-offs

- **Bank contracts can be submitted without a due date.** Mitigation: they
  cannot be approved without one, and the contracts list already flags pending
  bank contracts.
- **Free-text bank names vary.** Acceptable for a first version; a canonical
  catalog can come later without a breaking change.

## Migration Plan

1. Apply `000021_add_contract_bank_credit_fields`.
2. Deploy the API before the portal/UI changes; the new request fields are
   optional.
3. Rollback: the down migration drops both columns; older API versions ignore
   the extra request fields.

## Open Questions

None.
