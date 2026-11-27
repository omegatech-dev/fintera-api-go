## ADDED Requirements

### Requirement: Commissions report keeps cancelled contracts visible

The commissions report and its CSV/PDF exports SHALL include contracts in
`cancelled` status alongside `approved` and `closed` contracts, so a commission
that was already paid remains visible after its contract is cancelled. Every
commission row SHALL expose the contract status. Paid/pending commission totals
SHALL follow the payout state of each row; recoverable seller credits continue
to be reported separately through the credits, `pending_credits`, and
`seller_balance` figures.

#### Scenario: Paid commission of a cancelled contract

- **WHEN** a contract with a paid commission is cancelled and the commissions report is generated
- **THEN** the contract's commission row remains in the report and export with the commission marked as paid and the contract status exposed as `cancelled`

#### Scenario: Unpaid commission of a cancelled contract

- **WHEN** a cancelled contract never had a commission payout
- **THEN** its row appears with the commission marked as pending and does not change paid totals

#### Scenario: Totals and credits reconcile

- **WHEN** the report includes cancelled contracts
- **THEN** paid and pending totals include their commissions, and the recoverable credit created by the cancellation continues to be reported through the credits section so the seller's balance is not double counted

### Requirement: Contract status is exposed on commission rows

Each commission row of the JSON report SHALL include the raw contract status of
its source contract. Historical sale rows SHALL omit the field.

#### Scenario: Contract row

- **WHEN** a commission row comes from a contract
- **THEN** it includes `contract_status` with the contract's lifecycle status

#### Scenario: Historical sale row

- **WHEN** a commission row comes from a historical sale
- **THEN** `contract_status` is omitted
