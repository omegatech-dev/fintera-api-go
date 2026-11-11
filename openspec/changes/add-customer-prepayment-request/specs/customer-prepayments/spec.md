## Purpose

Let customers request a partial or full early capital payment and attach the
transfer receipt, while staff keeps approval control and the schedule shortens
only after validation.

## ADDED Requirements

### Requirement: Request an early capital payment

The API SHALL expose `POST /api/v1/contracts/:guid/prepayments` so the contract
owner (or admin/seller) can request an early payment of part or all of the
outstanding balance.

#### Scenario: Owner requests a partial prepayment

- **WHEN** the authenticated customer owns an approved contract with a balance
  and sends a positive amount up to that balance
- **THEN** the API responds `201` with a `submitted` `capital_repayment` payment
- **AND** the customer can upload its receipt through the existing payment
  receipt endpoint

#### Scenario: Full balance

- **WHEN** the requested amount equals the outstanding balance
- **THEN** the request is accepted

#### Scenario: Amount above the balance

- **WHEN** the requested amount exceeds the outstanding balance
- **THEN** the API responds `422` and creates nothing

#### Scenario: Contract without debt or not approved

- **WHEN** the contract is not approved or has no outstanding balance
- **THEN** the API responds `422`

#### Scenario: Duplicate request

- **WHEN** the contract already has a submitted `capital_repayment` payment
- **THEN** the API responds `422` and does not create a second request

#### Scenario: Another customer's contract

- **WHEN** the contract belongs to a different customer
- **THEN** the API responds `403`

#### Scenario: Unauthenticated caller

- **WHEN** the request has no valid session
- **THEN** the API responds `401`

### Requirement: Approve a prepayment as capital

When staff approves a submitted `capital_repayment` payment, the API SHALL apply
the approved amount to capital instead of booking it as a regular installment.

#### Scenario: Approved prepayment

- **WHEN** staff approves the submitted prepayment
- **THEN** the payment becomes `paid`
- **AND** future installments are reduced or deleted from the end of the schedule
- **AND** the ledger records a `prepayment` credit for the amount
- **AND** the contract balance is refreshed

#### Scenario: Regular payment with excess

- **WHEN** staff approves a regular payment whose paid amount exceeds its own obligation
- **THEN** only the excess is applied to capital, preserving the existing behavior
