## Purpose

Define how staff apply a capital repayment to an approved contract, including
the accepted request payload, so the admin panel and any other client can apply
the payment reliably.

## ADDED Requirements

### Requirement: Capital repayment accepts the documented and legacy field names

The admin capital-repayment endpoint SHALL accept the amount in
`capital_repayment_amount` and SHALL also accept `amount` for compatibility
with existing clients. The amount SHALL be required and SHALL be greater than
zero.

#### Scenario: Documented field name

- **WHEN** an admin posts a positive capital repayment using
  `capital_repayment_amount`
- **THEN** the repayment is applied to the approved contract

#### Scenario: Legacy field name

- **WHEN** an admin posts a positive capital repayment using `amount`
- **THEN** the repayment is applied with the same result as the documented
  field

#### Scenario: Missing or non-positive amount

- **WHEN** the request has no amount, zero, or a negative amount
- **THEN** the API responds with a client error
- **AND** no ledger entry or payment is created

#### Scenario: Contract not approved

- **WHEN** an admin posts a capital repayment for a contract that is not
  approved
- **THEN** the API responds with a client error explaining the contract must be
  approved
- **AND** no ledger entry or payment is created
