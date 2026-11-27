## Why

The customer reservation experience needs deployment-wide settings and a richer
tracking payload:

- the support WhatsApp number is hardcoded in the portal;
- the 48-hour hold is hardcoded in three places and cannot be tuned;
- the tracking timeline lacks submission/rejection timestamps and the lot status;
- customers who lose the account-setup email cannot request it again.

## What Changes

- Add a `general_settings` singleton with `support_whatsapp`, `support_email`
  and `reservation_hold_hours` (default 48, 1–720).
- Public `GET /api/v1/settings`; admin `PUT /api/v1/settings`.
- Drive `hold_expires_at`, the reservation payment due date and the reminder
  email from `reservation_hold_hours`.
- Add `contracts.submitted_at` and `contracts.rejected_at`, populate them on the
  existing transitions, and expose them plus `lot_status` in `ContractResponse`.
- Add public `POST /api/v1/public/reservations/:guid/resend-access` to
  regenerate the temporary password and resend the account setup email when the
  customer has not completed onboarding.
- Tests, Swagger and data-model docs.

## Capabilities

### New Capabilities

- `customer-settings`: support contact and hold configuration.
- `reservation-tracking`: the customer-facing tracking payload.

### Modified Capabilities

None.

## Impact

- `internal/models/general_settings.go`, `general_settings_test.go`
- `internal/database/migrations/000022_*`, `000023_*`
- `internal/repository/general_settings_repository.go`, `repositories.go`
- `internal/services/general_settings_service.go`, `services.go`
- `internal/handlers/general_settings_handler.go`, `handlers.go`
- `internal/models/contract.go`, `internal/services/contract_service.go`,
  `internal/services/payment_service.go`, `internal/services/email_service.go`
- `internal/handlers/contract_handler.go`, `cmd/api/main.go`
- `docs/` (Swagger and data model)
- Consumers: FinteraWeb tracking/profile and FinteraUI general settings.
