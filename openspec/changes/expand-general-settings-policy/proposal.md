## Why

The admin portal exposes a reservation, notification and regional policy in
`/settings/general`, but the API silently discards 10 of the 20 fields it
sends: `reservation_min_amount`, `reservation_due_days`,
`down_payment_due_days`, `down_payment_due_days_with_reservation`,
`email_notifications_enabled`, `overdue_reminders_enabled`,
`upcoming_reminder_days`, `default_currency`, `default_locale` and `timezone`.
An admin who sets a 25,000 minimum reserve, a 72-hour hold and 3-day payment
windows gets a success toast and nothing changes: the deployment keeps running
the hardcoded 48-hour hold, the L 5,000 minimum and the 7/14/21-day windows.
The customer portal hardcodes the same numbers, so customers are promised a
"48-hour hold for L 5,000" no matter what the policy says.

The values that do reach the API are only partially honoured: the job that
releases unpaid reservations still cancels at 48 hours, the voucher reminder
still fires at 40 hours, and the three payment windows are literals in the
schedule generator.

## What Changes

- Add ten columns to `general_settings` and expose them through
  `GET /api/v1/settings`: the reservation policy (minimum amount and the three
  payment windows), the notification policy (email switch, overdue reminders,
  upcoming-reminder lead time) and the regional defaults (currency, locale,
  timezone).
- Make `PUT /api/v1/settings` merge the provided fields into the singleton row
  instead of replacing it, so a section-level save cannot erase the other
  sections. The request fields become pointers so "not sent" is distinguishable
  from "sent empty".
- Advertise the merge behaviour with `supports_partial_updates`, so a client can
  detect it without a destructive probe against the endpoint.
- Enforce the configured policy in the backend, replacing the hardcoded values:
  the unpaid-reservation release window, the reservation voucher reminder lead
  time, the reservation payment due date, the two down-payment windows, the
  upcoming-reminder window and the overdue reminder switch.
- Enforce the configured minimum reservation amount on reservation creation and
  answer with a stable `RESERVE_AMOUNT_BELOW_MINIMUM` code plus the configured
  minimum, so the client can localise the message.
- Apply the deployment-wide email switch on top of the per-user opt-out and the
  environment kill switch.
- **BREAKING** `PUT /api/v1/settings` no longer replaces the row: a payload that
  omits a field leaves the stored value untouched. A client that relied on the
  replace behaviour to clear a field must now send it explicitly as `null` or
  an empty string. The admin portal already sends the full snapshot on every
  save, so it is unaffected.

## Capabilities

### New Capabilities
- `deployment-settings-policy`: stores, validates and applies the
  deployment-wide reservation, notification and regional policy; owns the merge
  semantics of the settings endpoint and the capability flag that advertises
  them.

### Modified Capabilities
- `customer-settings`: the reservation payment due date follows the configured
  payment window instead of the lot-hold duration, and the hold duration now
  also drives the release of unpaid reservations and the voucher reminder.

## Impact

- `internal/database/migrations/000049_expand_general_settings_policy.{up,down}.sql`
  — additive migration and its reversible `down`.
- `internal/models/general_settings.go` — new fields, defaults, response DTO and
  the process-wide policy snapshot.
- `internal/services/general_settings_service.go` — validation, merge, policy cache.
- `internal/handlers/general_settings_handler.go` — pointer request fields.
- `internal/services/payment_schedule_service.go` — configurable payment windows.
- `internal/services/contract_service.go` — configurable release window and
  voucher reminder lead time.
- `internal/services/payment_service.go` — overdue reminder switch.
- `internal/services/email_service.go` — deployment-wide email switch.
- `internal/repository/contract_repository.go` — configurable upcoming-reminder
  window.
- `internal/handlers/contract_handler.go` — minimum reservation amount.
- `docs/` — regenerated Swagger and updated architecture documentation.
- Consumers outside this repo: the admin portal (`fintera-ui-react`) and the
  customer portal (`fintera-customer-site`) read the new fields from
  `GET /api/v1/settings`.
