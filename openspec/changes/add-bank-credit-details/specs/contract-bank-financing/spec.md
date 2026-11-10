## Purpose

Capture bank-financing information at reservation time without forcing the
customer to invent a disbursement date, and keep the contract payment schedule
valid.

## ADDED Requirements

### Requirement: Bank financing details on reservations

When a reservation is created with `financing_type=bank`, the API SHALL accept
optional `bank_name` and `bank_credit_status` fields, persist them, and return
them in the contract response.

`bank_credit_status` SHALL accept only `not_started`, `pre_qualified`,
`pre_approved` or `approved`; any other non-empty value SHALL be rejected with
`400`.

#### Scenario: Bank reservation with details

- **WHEN** a customer creates a bank reservation with `bank_name` and
  `bank_credit_status=pre_qualified`
- **THEN** the reservation is created and the contract response includes both
  values

#### Scenario: Bank reservation without details

- **WHEN** a customer creates a bank reservation without `bank_name` or
  `bank_credit_status`
- **THEN** the reservation is created and both response fields are `null`

#### Scenario: Invalid credit status

- **WHEN** a bank reservation sends `bank_credit_status=maybe`
- **THEN** the API responds `400` and no contract is created

#### Scenario: Non-bank reservation sends bank fields

- **WHEN** a `direct` or `cash` reservation includes bank fields
- **THEN** the fields are ignored and stored as `null`

### Requirement: Payment date rules per modality

`max_payment_date` SHALL be required for `cash` reservations, optional for
`bank` reservations, and ignored for `direct` reservations. A bank contract
without `max_payment_date` SHALL NOT be approvable until an admin sets it.

#### Scenario: Bank reservation without a payment date

- **WHEN** a customer creates a bank reservation without `max_payment_date`
- **THEN** the reservation is created as pending

#### Scenario: Cash reservation without a payment date

- **WHEN** a customer creates a cash reservation without `max_payment_date`
- **THEN** the API responds `400`

#### Scenario: Approve a bank contract without a payment date

- **WHEN** an admin approves a bank contract whose `max_payment_date` is null
- **THEN** approval fails with an error explaining that the payment date is
  required

#### Scenario: Admin sets the payment date

- **WHEN** an admin patches `max_payment_date` on a pending bank contract
- **THEN** the date is stored and the contract can be approved

### Requirement: Admin updates bank details

`PATCH /projects/:project_id/lots/:lot_id/contracts/:contract_id` SHALL accept
`bank_name` and `bank_credit_status` and return the updated values in the
contract response.

#### Scenario: Admin edits the bank and status

- **WHEN** an admin updates `bank_name` and `bank_credit_status` on a bank
  contract
- **THEN** the contract response reflects the new values

#### Scenario: Admin clears the bank name

- **WHEN** an admin sends an empty `bank_name`
- **THEN** the stored value becomes `null`
