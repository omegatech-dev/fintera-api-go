## Purpose

Configure the support contact and reservation hold, and expose the timestamps the
customer portal needs to render a complete reservation timeline.

## ADDED Requirements

### Requirement: General settings

The API SHALL store and expose a singleton general settings row with
`support_whatsapp`, `support_email` and `reservation_hold_hours` (default 48).

#### Scenario: Public read

- **WHEN** anyone requests `GET /api/v1/settings`
- **THEN** the response includes the support contact and the hold hours

#### Scenario: Admin update

- **WHEN** an admin updates the settings with a valid WhatsApp, email and hold
- **THEN** the new values are persisted and returned

#### Scenario: Invalid values

- **WHEN** the WhatsApp is not 8–20 digits, the email is malformed, or the hold
  is outside 1–720 hours
- **THEN** the API responds `400` and stores nothing

### Requirement: Configurable hold duration

Pending reservation responses SHALL compute `hold_expires_at` from the configured
`reservation_hold_hours`, and the reservation payment due date and reminder email
SHALL use the same value.

#### Scenario: Default hold

- **WHEN** no hold is configured
- **THEN** `hold_expires_at` is 48 hours after the contract creation

#### Scenario: Custom hold

- **WHEN** an admin sets the hold to 72 hours
- **THEN** new and pending reservations expose a 72-hour hold

### Requirement: Tracking timestamps

`ContractResponse` SHALL include `submitted_at`, `rejected_at`, `approved_at` and
`lot_status`.

#### Scenario: Receipt submitted

- **WHEN** the reservation receipt moves the contract to `submitted`
- **THEN** `submitted_at` is set

#### Scenario: Rejected contract

- **WHEN** an admin rejects a contract
- **THEN** `rejected_at` is set and the response includes the lot status

### Requirement: Resend account setup

`POST /api/v1/public/reservations/:guid/resend-access` SHALL regenerate the
temporary password and resend the account setup email when the customer has not
completed onboarding.

#### Scenario: Pending onboarding

- **WHEN** the applicant still has `must_change_password`
- **THEN** a new temporary password is generated, stored hashed, and emailed

#### Scenario: Already configured

- **WHEN** the applicant already changed their password
- **THEN** the API responds `409` and does not reset it
