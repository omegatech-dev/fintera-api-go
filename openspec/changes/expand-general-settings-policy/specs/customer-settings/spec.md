## MODIFIED Requirements

### Requirement: Configurable hold duration

Pending reservation responses SHALL compute `hold_expires_at` from the configured
`reservation_hold_hours`. The reservation payment due date SHALL follow the
configured `reservation_due_days`, the reminder email SHALL show the hold
expiry, and the release of unpaid reservations and the voucher reminder SHALL
follow the configured hold.

#### Scenario: Default hold

- **WHEN** no hold is configured
- **THEN** `hold_expires_at` is 48 hours after the contract creation

#### Scenario: Custom hold

- **WHEN** an admin sets the hold to 72 hours
- **THEN** new and pending reservations expose a 72-hour hold

#### Scenario: Reservation payment window follows the policy

- **WHEN** an admin sets the reservation payment window to 3 days and the hold
  to 72 hours
- **THEN** the reservation payment is due after 3 days and the hold still
  expires 72 hours after the contract creation
