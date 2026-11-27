## ADDED Requirements

### Requirement: Deployment policy storage

The API SHALL store a singleton general settings row that also holds the
deployment-wide reservation, notification and regional policy, and SHALL return
those fields from the public settings read.

The stored fields SHALL be `reservation_min_amount`, `reservation_due_days`,
`down_payment_due_days`, `down_payment_due_days_with_reservation`,
`email_notifications_enabled`, `overdue_reminders_enabled`,
`upcoming_reminder_days`, `default_currency`, `default_locale` and `timezone`.

#### Scenario: Public read returns the policy

- **WHEN** anyone requests `GET /api/v1/settings`
- **THEN** the response includes every policy field with its stored value

#### Scenario: Admin update persists the policy

- **WHEN** an admin updates the settings with valid policy values
- **THEN** the values are persisted and returned in the response

#### Scenario: Defaults when the row is missing

- **WHEN** the singleton row does not exist yet
- **THEN** the API responds with the default policy instead of an error

### Requirement: Policy validation

The API SHALL validate every policy field before storing it and SHALL reject an
invalid payload with `400` without writing anything.

The minimum reservation amount SHALL be zero or greater, the three payment
windows SHALL be integers between 1 and 365 days, the upcoming-reminder lead
time SHALL be an integer between 0 and 30 days, the currency SHALL be a
three-letter uppercase code, the locale SHALL be `es` or `en`, and the timezone
SHALL be a valid IANA zone.

#### Scenario: Out-of-range values are rejected

- **WHEN** the minimum amount is negative, a payment window is outside 1-365
  days, or the reminder lead time is outside 0-30 days
- **THEN** the API responds `400` and stores nothing

#### Scenario: Unsupported currency, locale or timezone is rejected

- **WHEN** the currency is not a three-letter code, the locale is not `es` or
  `en`, or the timezone is not a valid IANA zone
- **THEN** the API responds `400` and stores nothing

#### Scenario: Payment windows out of order are rejected

- **WHEN** the reservation window is longer than the down-payment window, or the
  down-payment window is longer than the down-payment-with-reservation window
- **THEN** the API responds `400` and stores nothing

### Requirement: Settings merge semantics

`PUT /api/v1/settings` SHALL merge the provided fields into the singleton row
and SHALL leave the fields that were not provided untouched.

#### Scenario: Partial update preserves the other fields

- **WHEN** an admin updates only the reservation policy
- **THEN** the support contact, legal data, terms content, notification policy
  and regional defaults keep their stored values

#### Scenario: An explicit empty value clears a field

- **WHEN** an admin sends an empty string or `null` for a field
- **THEN** that field is cleared and the other fields are untouched

#### Scenario: A full update still applies every field

- **WHEN** an admin sends every field
- **THEN** all of them are stored

### Requirement: Merge capability advertisement

`GET /api/v1/settings` SHALL expose `supports_partial_updates` so a client can
learn that the endpoint merges without issuing a partial write to probe it.

#### Scenario: Capability flag is exposed

- **WHEN** anyone requests `GET /api/v1/settings`
- **THEN** the response includes `supports_partial_updates` set to `true`

### Requirement: Minimum reservation amount

The API SHALL reject a reservation whose amount is below the configured minimum
reservation amount.

#### Scenario: Reservation below the minimum is rejected

- **WHEN** a reservation is created with an amount below the configured minimum
- **THEN** the API responds `400` with the error code
  `RESERVE_AMOUNT_BELOW_MINIMUM` and the configured minimum in `min_amount`,
  and no reservation is created

#### Scenario: Reservation at or above the minimum is accepted

- **WHEN** a reservation is created with an amount equal to or above the
  configured minimum
- **THEN** the reservation is created

#### Scenario: No minimum configured

- **WHEN** the configured minimum is zero
- **THEN** any non-negative amount is accepted

### Requirement: Configurable payment windows

The API SHALL derive the reservation payment due date from
`reservation_due_days`, and the down-payment due date from
`down_payment_due_days` when the contract carries no reservation amount, or from
`down_payment_due_days_with_reservation` when it does.

#### Scenario: Reservation payment window

- **WHEN** a payment schedule is generated for a contract with a reservation
  amount
- **THEN** the reservation payment is due after the configured
  `reservation_due_days`

#### Scenario: Down payment without a reservation

- **WHEN** a payment schedule is generated for a contract without a reservation
  amount
- **THEN** the down payment is due after the configured `down_payment_due_days`

#### Scenario: Down payment with a reservation

- **WHEN** a payment schedule is generated for a contract with a reservation
  amount
- **THEN** the down payment is due after the configured
  `down_payment_due_days_with_reservation`

### Requirement: Configurable hold enforcement

The API SHALL use the configured `reservation_hold_hours` to release unpaid
reservations and to schedule the reservation voucher reminder.

#### Scenario: Unpaid reservation is released after the configured hold

- **WHEN** the release job runs for a reservation created more than the
  configured hold hours ago
- **THEN** the reservation is cancelled and the lot becomes available again

#### Scenario: A reservation inside the hold window is kept

- **WHEN** the release job runs for a reservation created less than the
  configured hold hours ago
- **THEN** the reservation stays active

#### Scenario: Voucher reminder fires before the hold expires

- **WHEN** the voucher reminder job runs
- **THEN** it contacts the reservations whose hold expires soon, using the
  configured hold as the reference

### Requirement: Notification policy

The API SHALL apply the configured notification policy to the customer-facing
reminder emails.

#### Scenario: Upcoming reminder lead time

- **WHEN** the upcoming reminder lead time is set to a number of days
- **THEN** the upcoming-payment reminders are based on that lead time instead
  of a fixed one-day window

#### Scenario: Overdue reminders are disabled

- **WHEN** overdue reminders are disabled
- **THEN** the daily job sends no overdue reminder email

#### Scenario: Deployment email switch

- **WHEN** customer emails are disabled in the policy
- **THEN** no customer notification email is sent, even when the per-user
  preference is enabled

#### Scenario: The environment switch still wins

- **WHEN** the deployment disables email globally through the environment
- **THEN** customer emails remain disabled regardless of the policy

### Requirement: Regional defaults

The API SHALL expose the configured currency, locale and timezone as the
deployment-wide defaults, and SHALL keep using a record-specific value when
the record defines one.

#### Scenario: Public read exposes the defaults

- **WHEN** anyone requests `GET /api/v1/settings`
- **THEN** the response includes the configured currency, locale and timezone

#### Scenario: Record-specific values win

- **WHEN** a contract defines its own currency
- **THEN** the contract keeps its own currency instead of the deployment default
