## ADDED Requirements

### Requirement: Credit balances are tracked per seller credit

Each seller commission credit SHALL persist its total amount, the amount already
applied to commissions (`applied_amount`), its origin (`cancellation`,
`reassignment`, or `legacy`), and its settlement state. The remaining balance of
a credit SHALL be `amount - applied_amount`. A credit SHALL be `settled` only
when `applied_amount` equals `amount`. Every application of a credit SHALL be
recorded with the credit, the contract that consumed it, the amount, the actor,
and a creation timestamp.

#### Scenario: New cancellation credit starts unapplied

- **WHEN** a paid commission is cancelled and a recoverable credit is created
- **THEN** the credit is `pending`, its `applied_amount` is `0`, its remaining
  balance equals its amount, and its source is `cancellation`

#### Scenario: Legacy settled credit is backfilled

- **WHEN** the migration runs on credits already settled by hand
- **THEN** those credits keep `status = settled`, receive `applied_amount = amount`
  and `source = legacy`, and are never reversed automatically

#### Scenario: Settlement requires full application

- **WHEN** any code path sets a credit to `settled`
- **THEN** the persisted `applied_amount` equals the credit amount, otherwise the
  write is rejected

### Requirement: Commission payout applies pending credits automatically

The system SHALL consume a seller's pending credit balance before paying cash
when an admin marks the commission of an approved or closed contract as paid.
Credits SHALL be consumed in
ascending creation order (FIFO), only from the contract's current seller, and
never across different sellers. The payout SHALL be atomic with the
`commission_paid_at` update and SHALL report the gross commission, the total
applied, the cash paid, and the seller's remaining credit.

#### Scenario: Credit covers the whole commission

- **WHEN** the seller has a 10,000 pending credit and the contract commission is
  8,000
- **THEN** 8,000 of the credit is applied, `net_cash` is 0, the credit keeps
  2,000 remaining and stays `pending`, and the application is recorded

#### Scenario: Credit partially covers the commission

- **WHEN** the seller has a 3,000 pending credit and the contract commission is
  8,000
- **THEN** 3,000 are applied, `net_cash` is 5,000, and the credit becomes
  `settled` with `applied_amount = 3,000`

#### Scenario: Multiple credits are consumed FIFO

- **WHEN** the seller has pending credits of 3,000 and 10,000 (oldest first) and
  the commission is 8,000
- **THEN** the 3,000 credit is fully applied, 5,000 of the 10,000 credit is
  applied, and the 10,000 credit keeps 5,000 remaining

#### Scenario: Credits from another seller are not consumed

- **WHEN** the contract seller has no pending credits but another seller does
- **THEN** the full commission is paid in cash and no credit is modified

#### Scenario: Commission without credits

- **WHEN** the seller has no pending credits
- **THEN** `net_cash` equals the gross commission and the response lists no
  applications

### Requirement: Commission payout is idempotent and concurrency-safe

The system SHALL reject a second payout request for the same contract with
`422`, and SHALL re-check the unpaid state and contract status inside the same
transaction that consumes credits. Two concurrent payout requests for the same
contract SHALL result in exactly one payout.

#### Scenario: Paying twice

- **WHEN** an admin calls the pay endpoint twice for the same contract
- **THEN** the first call succeeds, the second returns `422`, and no additional
  credit is consumed

#### Scenario: Concurrent pay requests

- **WHEN** two payout requests for the same contract run concurrently
- **THEN** one request wins, the other returns `422`, and the credit balance is
  consumed only once

#### Scenario: Contract not payable

- **WHEN** the contract is not approved or closed
- **THEN** the endpoint returns `422` without touching credits

### Requirement: Admin can preview a commission payout

The system SHALL expose an admin-only payout preview for a contract with the
gross commission, the seller's available credit, the applicable amount, the net
cash, and the remaining credit. The preview SHALL never mutate credits. For a
contract without a seller it SHALL return the gross commission as `net_cash`
with zeroed credit figures.

#### Scenario: Preview with available credit

- **WHEN** an admin requests the preview for an unpaid contract whose seller has
  10,000 in pending credits and the commission is 8,000
- **THEN** the response has `available_credit = 10,000`, `applicable = 8,000`,
  `net_cash = 0`, and `remaining_credit = 2,000`

#### Scenario: Preview without a seller

- **WHEN** the contract has no assigned seller and an admin requests the preview
- **THEN** the response is `200` with the gross commission as `net_cash` and
  zeroed credit figures

#### Scenario: Preview of an already paid contract

- **WHEN** the contract commission is already marked as paid
- **THEN** the preview returns `422` and does not modify credits

### Requirement: Cancelling a netted commission restores applied credits

Cancelling a contract whose commission was paid with credits SHALL reverse the
active applications, restore the applied amount to the original credits, return
them to `pending` and clear their settlement metadata
while they still have a balance, and create a new recoverable credit only for
the cash actually paid. The restored amount plus the new credit SHALL equal the
gross commission of the cancelled contract. Reversal SHALL be idempotent.

#### Scenario: Fully netted commission is cancelled

- **WHEN** an 8,000 commission was fully paid with credit and the contract is
  cancelled
- **THEN** the 8,000 is restored to the original credit, no new credit is
  created, and the seller's balance is 8,000

#### Scenario: Partially netted commission is cancelled

- **WHEN** an 8,000 commission was paid with 3,000 of credit and 5,000 in cash
  and the contract is cancelled
- **THEN** 3,000 is restored to the original credit, a 5,000 credit is created
  for the cash paid, and the seller's balance is 8,000

#### Scenario: Legacy cancellation without applications

- **WHEN** a paid commission with no credit applications is cancelled
- **THEN** a recoverable credit is created for the gross commission as before

### Requirement: Reassigning a netted commission restores applied credits

Reassigning a contract whose commission was paid with credits SHALL reverse the
active applications and create a recoverable credit for the previous seller
equal to the cash actually paid. The
reassignment SHALL clear the paid commission date and actor even when the cash
amount is zero, so the new seller's commission is not inherited as paid.

#### Scenario: Reassign a fully netted commission

- **WHEN** the previous seller's 8,000 commission was fully paid with credit and
  the contract is reassigned
- **THEN** the 8,000 is restored to the original credit, no zero-amount credit is
  created, and the contract is no longer marked as commission-paid

#### Scenario: Reassign a partially netted commission

- **WHEN** the previous seller received 5,000 in cash after applying 3,000 of
  credit and the contract is reassigned
- **THEN** 3,000 is restored and a 5,000 credit is created for the previous
  seller

#### Scenario: Reassign an unpaid commission

- **WHEN** the contract commission was never paid
- **THEN** no credit is created and the new seller's commission is calculated
  from the current rules

### Requirement: Manual credit application cannot bypass netting

The manual apply endpoint SHALL reject a credit that still has a remaining
balance with `422`, because credits are consumed by the payout flow. A pending
credit with no remaining balance (for example, a legacy row that was already
consumed) SHALL still be settled manually, recording `applied_amount` equal to
the amount.

#### Scenario: Apply a credit with remaining balance

- **WHEN** an admin applies a credit with a remaining balance
- **THEN** the request returns `422`, the credit stays unchanged, and no
  application is recorded

#### Scenario: Apply a zero-balance pending credit

- **WHEN** an admin applies a pending credit whose remaining balance is zero
- **THEN** the credit becomes `settled` with `applied_amount = amount`

#### Scenario: Apply an already settled credit

- **WHEN** the credit is already settled
- **THEN** the endpoint returns `422`

### Requirement: Credit reports use remaining balances

The commissions report and its CSV/PDF export SHALL count pending credits by
their remaining balance, not their original amount, so both outputs agree. The
commissions report SHALL expose a global `seller_balance` with the seller's
remaining credit, while `net_pending` keeps its current period-minus-credits
semantics for backward compatibility. Each credit in the API SHALL expose
`applied_amount`, `remaining_amount`, `source`, `applications_total`, and its
active applications, including the applied contract id and GUID.

#### Scenario: Partially applied credit in the report

- **WHEN** a credit of 10,000 has 8,000 applied
- **THEN** the JSON report and the export both count 2,000 as pending credit and
  `seller_balance` includes 2,000

#### Scenario: Credit detail lists applications

- **WHEN** a credit was applied to two contracts
- **THEN** the credit payload has `applications_total = 2` and two active
  application entries with their contract ids, GUIDs, and amounts

#### Scenario: Reversed applications are hidden by default

- **WHEN** an application was reversed
- **THEN** it is excluded from the default credit payload and does not count
  toward `applications_total`
