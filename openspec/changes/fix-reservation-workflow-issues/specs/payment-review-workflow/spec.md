## Purpose

Define the order and lifecycle rules for reviewing reservation payments and
receipts so customers and staff always see a coherent state.

## ADDED Requirements

### Requirement: Reservation payment approval requires an approved contract

The API SHALL reject the approval of a reservation payment while its contract
is not approved. The rejection SHALL leave the payment, the contract and any
ledger entries unchanged and SHALL return a clear error message explaining that
the contract must be approved first. Once the contract is approved, the same
approval request SHALL succeed.

#### Scenario: Contract still pending

- **WHEN** an admin approves a reservation payment whose contract is pending or
  submitted
- **THEN** the API responds with a client error
- **AND** the error message states the contract must be approved first
- **AND** the payment keeps its previous status

#### Scenario: Contract approved

- **WHEN** an admin approves a reservation payment whose contract is approved
- **THEN** the payment is approved and the response reports success

#### Scenario: Non-reservation payment

- **WHEN** an admin approves a down-payment, installment or capital payment for
  an approved contract
- **THEN** the approval follows the existing payment rules without requiring
  the reservation-order check

### Requirement: Rejection reason reflects the latest review

The API SHALL clear the stored rejection reason when a new receipt is submitted
for that payment, so the reason is only exposed while the payment is rejected.
The reason SHALL remain readable by the customer and staff while the payment is
rejected, and SHALL be absent after the payment is approved or paid.

#### Scenario: New receipt after a rejection

- **WHEN** a rejected payment receives a new receipt
- **THEN** the payment returns to submitted and its rejection reason is empty

#### Scenario: Rejected payment exposes its reason

- **WHEN** a payment is rejected with a reason
- **THEN** payment responses include that reason until the state changes

#### Scenario: Approved after re-submission

- **WHEN** a payment that was rejected and re-submitted is approved
- **THEN** payment responses no longer include a rejection reason

### Requirement: Reservation tracking state advances with the receipt

The API SHALL move a pending reservation contract to submitted when the
reservation receipt is uploaded, and SHALL keep the payment and contract states
consistent so clients can label the reservation as awaiting approval.

#### Scenario: Receipt uploaded for a pending reservation

- **WHEN** the customer uploads the reservation receipt
- **THEN** the reservation payment is submitted
- **AND** a pending contract becomes submitted

#### Scenario: Receipt re-uploaded

- **WHEN** the customer uploads a replacement receipt for a submitted or
  rejected reservation payment
- **THEN** the payment is submitted again and the contract remains submitted
