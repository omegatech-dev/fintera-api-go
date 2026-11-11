## Purpose

Let authenticated customers download their own contract and account PDFs while keeping every other customer's documents private.

## ADDED Requirements

### Requirement: Owner-scoped promise of sale contract PDF

The API SHALL expose `GET /api/v1/users/:user_id/promise_contract_pdf?contract_id=` that returns the promise of sale contract PDF for a contract owned by the customer.

#### Scenario: Owner requests their contract

- **WHEN** the authenticated customer owns the requested contract
- **THEN** the API responds `200` with `Content-Type: application/pdf`

#### Scenario: Contract belongs to another customer

- **WHEN** the requested contract belongs to a different customer
- **THEN** the API responds `403`

#### Scenario: Missing contract identifier

- **WHEN** `contract_id` is absent or not a positive integer
- **THEN** the API responds `400`

#### Scenario: Contract does not exist

- **WHEN** `contract_id` does not match any contract
- **THEN** the API responds `404`

### Requirement: Owner-scoped customer record PDF

The API SHALL expose `GET /api/v1/users/:user_id/customer_record_pdf?contract_id=` that returns the customer record PDF for a contract owned by the customer.

#### Scenario: Owner requests their customer record

- **WHEN** the authenticated customer owns the requested contract
- **THEN** the API responds `200` with `Content-Type: application/pdf`

#### Scenario: Contract belongs to another customer

- **WHEN** the requested contract belongs to a different customer
- **THEN** the API responds `403`

### Requirement: Owner-scoped balance PDF

The API SHALL expose `GET /api/v1/users/:user_id/balance_pdf` that returns the account balance PDF for the authenticated customer.

#### Scenario: Owner requests their balance

- **WHEN** the authenticated customer requests their own balance
- **THEN** the API responds `200` with `Content-Type: application/pdf`

#### Scenario: Customer requests another balance

- **WHEN** a customer requests the balance of another user
- **THEN** the API responds `403`

### Requirement: Balance PDF content

The balance PDF SHALL be a complete account statement, not a raw list of payments. It SHALL include the customer's identity and contact details, an account summary, and a detailed section per contract.

#### Scenario: Customer statement

- **WHEN** the statement is generated
- **THEN** it includes the customer name, identity, phone, email and address
- **AND** an account summary with the total contract amount, total paid, outstanding balance and overdue amount
- **AND** per contract: project, lot, area, dimensions, registration, address and financing conditions
- **AND** a financial summary with price, reserve, down payment, financed amount, paid amount and balance
- **AND** the full numbered payment schedule with type, due date, amount, paid amount and Spanish status
- **AND** interest and prepayment ledger movements when present
- **AND** the issue date and page numbers

#### Scenario: Closed contract

- **WHEN** a contract is closed with no outstanding balance
- **THEN** it is still listed with its payment history and a zero balance

#### Scenario: No contracts

- **WHEN** the customer has no contracts
- **THEN** the PDF renders an explanatory empty state

### Requirement: Staff access

Admins and sellers SHALL be able to download any customer's documents so support can assist.

#### Scenario: Seller requests a customer document

- **WHEN** an authenticated seller requests a contract or balance PDF for any customer
- **THEN** the API responds `200` with the PDF

### Requirement: Authentication required

All document endpoints SHALL require authentication.

#### Scenario: Anonymous request

- **WHEN** the request has no valid bearer token
- **THEN** the API responds `401`
