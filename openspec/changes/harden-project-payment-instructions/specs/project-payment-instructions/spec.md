## Purpose

Define the project bank accounts that customers use to pay a reservation, and
the admin API that manages them.

## ADDED Requirements

### Requirement: Admin bank-account management

The API SHALL expose admin-only CRUD endpoints for a project's bank accounts and
SHALL document them in Swagger.

#### Scenario: List accounts

- **WHEN** an admin requests the payment instructions of an existing project
- **THEN** the response contains its accounts ordered by position and then id

#### Scenario: Create an account

- **WHEN** an admin posts a bank, beneficiary, account number and account type
- **THEN** the account is stored with the project and an audit entry is written

#### Scenario: Update an account

- **WHEN** an admin patches an account that belongs to the project
- **THEN** the provided fields are updated and the response reflects them

#### Scenario: Account from another project

- **WHEN** an admin patches or deletes an account that belongs to a different
  project
- **THEN** the API responds `404`

#### Scenario: Delete an account

- **WHEN** an admin deletes an account
- **THEN** it disappears from the admin list and from the public payload

### Requirement: Bank-account validation

The API SHALL require `bank_name`, `beneficiary_name` and `account_number`, SHALL
accept only `savings` or `checking` as `account_type`, and SHALL reject values
longer than the supported limits with `422`.

#### Scenario: Missing required fields

- **WHEN** a request omits the bank, beneficiary or account number
- **THEN** the API responds `422` and stores nothing

#### Scenario: Invalid account type

- **WHEN** a request sends `account_type=credit`
- **THEN** the API responds `422`

#### Scenario: Oversized values

- **WHEN** a bank name exceeds 120 characters, the beneficiary exceeds 160, the
  account number exceeds 60 or the notes exceed 500
- **THEN** the API responds `422`

#### Scenario: Default account type

- **WHEN** a request omits the account type
- **THEN** the account is stored as `savings`

### Requirement: Public bank-account payload

`GET /public/projects/:guid/payment-instructions` SHALL return the accounts of a
published, active project with only the fields a customer needs.

#### Scenario: Published project with accounts

- **WHEN** a visitor requests the instructions of a published project
- **THEN** each account includes bank, beneficiary, account number, account type
  and notes

#### Scenario: Internal fields stay private

- **WHEN** a visitor requests the public payload
- **THEN** it does not include ids, positions or timestamps

#### Scenario: Unpublished or empty project

- **WHEN** the project is not published
- **THEN** the API responds `404`
- **AND** a published project with no accounts returns an empty list

### Requirement: Demo seed accounts

The demo seeds SHALL create at least one bank account for each seeded project
without duplicating accounts on repeated runs.

#### Scenario: First seed run

- **WHEN** the seeds run against an empty database
- **THEN** each demo project has at least one bank account

#### Scenario: Repeated seed run

- **WHEN** the seeds run again
- **THEN** no duplicate account numbers are created
