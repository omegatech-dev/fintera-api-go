## ADDED Requirements

### Requirement: Historical sale records

The system SHALL store historical sales with client data, free-text project,
lot, area, price, sale date, payment mode, sale status, commission and optional
evidence. A historical sale SHALL NOT create a reservation, block a lot, change
availability or affect the seller score.

#### Scenario: Create a historical sale

- **WHEN** a seller submits a historical sale
- **THEN** the record is stored with `validation_status = pending`
- **AND** the commission accrual date equals the sale date
- **AND** the commission origin is `historical`
- **AND** no reservation or lot block is created
- **AND** the seller score is unchanged

#### Scenario: Duplicate identity

- **WHEN** the skip-duplicates option is active and the identity already exists
- **THEN** the row is skipped and reported

### Requirement: CSV import of historical sales

The system SHALL import historical sales from a CSV with the sale date as the
commission accrual date and SHALL return a summary of valid and skipped rows.

#### Scenario: Successful import

- **WHEN** a valid CSV is uploaded
- **THEN** the records are created
- **AND** the summary reports the imported and skipped counts

#### Scenario: Partial failure

- **WHEN** some rows are invalid
- **THEN** valid rows are imported
- **AND** invalid rows are returned with their reason

### Requirement: Admin validation

An admin SHALL be able to validate or reject a historical sale without creating
a reservation.

#### Scenario: Admin validates

- **WHEN** an admin validates a historical sale
- **THEN** `validation_status` becomes `validated`
- **AND** the record stays a historical sale
