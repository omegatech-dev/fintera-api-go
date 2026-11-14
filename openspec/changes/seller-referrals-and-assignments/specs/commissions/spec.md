## ADDED Requirements

### Requirement: Commission origin and accrual

The system SHALL store a commission origin (`referral|direct|historical`) and an
accrual date on each contract commission. The accrual date SHALL be the contract
approval date for reservations and the sale date for historical records.

#### Scenario: Referral commission

- **WHEN** a contract approved from a referral is commissioned
- **THEN** the origin is `referral`

#### Scenario: Historical commission

- **WHEN** a historical sale is recorded
- **THEN** the origin is `historical`
- **AND** the accrual date is the sale date

### Requirement: Commission rate resolution

The system SHALL resolve the commission rate as: the seller exception, else the
project rate for the financing type, else the global default. Contracts without
a seller SHALL use the unassigned rate. The resolved amount SHALL be frozen on
the contract at approval.

#### Scenario: Seller exception

- **WHEN** the seller has an exception and the contract is approved
- **THEN** the exception rate is used

#### Scenario: Project rate fallback

- **WHEN** the seller has no exception
- **THEN** the project rate for the financing type is used

#### Scenario: Unassigned contract

- **WHEN** the contract has no seller
- **THEN** the unassigned rate is used

#### Scenario: History is stable

- **WHEN** the rate rules change after approval
- **THEN** existing contracts keep their stored amount

### Requirement: Historical commissions in reports

The commission report SHALL include historical commissions in the selected
period using their accrual date and SHALL expose their origin.

#### Scenario: Historical in period

- **WHEN** a historical sale has a sale date inside the selected period
- **THEN** its commission is included in the period totals
- **AND** its origin is `historical`
