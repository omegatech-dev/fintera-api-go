## 1. Migration

- [x] 1.1 Add `internal/database/migrations/000049_expand_general_settings_policy.up.sql` with the ten columns (`reservation_min_amount NUMERIC(12,2) NOT NULL DEFAULT 5000`, `reservation_due_days INTEGER NOT NULL DEFAULT 7`, `down_payment_due_days INTEGER NOT NULL DEFAULT 14`, `down_payment_due_days_with_reservation INTEGER NOT NULL DEFAULT 21`, `email_notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE`, `overdue_reminders_enabled BOOLEAN NOT NULL DEFAULT TRUE`, `upcoming_reminder_days INTEGER NOT NULL DEFAULT 1`, `default_currency VARCHAR(3) NOT NULL DEFAULT 'HNL'`, `default_locale VARCHAR(8) NOT NULL DEFAULT 'es'`, `timezone VARCHAR(64) NOT NULL DEFAULT 'America/Tegucigalpa'`).
- [x] 1.2 Add the matching `000049_expand_general_settings_policy.down.sql` that drops only those ten columns.
- [x] 1.3 Verify the migration applies and rolls back against a disposable database (`make migrate-up` / `make migrate-down`).

## 2. Model and response

- [x] 2.1 Add the ten fields to `models.GeneralSettings` and to `models.GeneralSettingsResponse` in `internal/models/general_settings.go`.
- [x] 2.2 Add the default values to `models.DefaultGeneralSettings()` and a `models.SettingsPolicy` snapshot with `SetSettingsPolicy`/`SettingsPolicy` accessors, keeping `models.ReservationHoldHours()` working on top of it.
- [x] 2.3 Expose `supports_partial_updates` as `true` in the settings response.

## 3. Merge and validation

- [x] 3.1 Change `generalSettingsUpdateRequest` in `internal/handlers/general_settings_handler.go` to use pointers for the ten new fields and for the fields that can be cleared.
- [x] 3.2 Change `GeneralSettingsService.Update` to read the stored row, overlay only the fields that were sent, validate the merged result and persist it.
- [x] 3.3 Validate the minimum amount (>= 0), the three payment windows (1-365), the reminder lead time (0-30), the currency (three uppercase letters), the locale (`es`/`en`) and the timezone (valid IANA zone) in `internal/services/general_settings_service.go`.
- [x] 3.4 Reject the payment windows that are not ordered `reservation_due_days <= down_payment_due_days <= down_payment_due_days_with_reservation`.
- [x] 3.5 Refresh the process-wide policy snapshot on every settings read and every settings update.

## 4. Reservation policy enforcement

- [x] 4.1 Use the configured hold in `ContractService.ReleaseUnpaidReservations` instead of the literal 48 (`internal/services/contract_service.go`).
- [x] 4.2 Derive the voucher reminder lead time from the configured hold instead of the literal 40 (`internal/services/contract_service.go`).
- [x] 4.3 Use `reservation_due_days` for the reservation payment due date at reservation creation (`internal/services/contract_service.go`).
- [x] 4.4 Use `reservation_due_days`, `down_payment_due_days` and `down_payment_due_days_with_reservation` for the schedule windows in `internal/services/payment_schedule_service.go`, replacing the literals 7, 14 and 21.
- [x] 4.5 Reject a reservation below the configured minimum in `internal/handlers/contract_handler.go` with `400`, the code `RESERVE_AMOUNT_BELOW_MINIMUM` and the configured minimum in `min_amount`.

## 5. Notification policy enforcement

- [x] 5.1 Skip the overdue reminder emails when `overdue_reminders_enabled` is false in `internal/services/payment_service.go`.
- [x] 5.2 Pass the configured lead time to `FindUpcomingReminders` and replace the literal `CURRENT_DATE + INTERVAL '1 day'` in `internal/repository/contract_repository.go`.
- [x] 5.3 Apply `email_notifications_enabled` in `EmailService.checkNotificationPreconditions` on top of the per-user opt-out and the environment kill switch.

## 6. Wiring and startup

- [x] 6.1 Inject the settings service into the services that consume the policy (`internal/services/services.go`) and keep the startup warm-up that loads the policy.
- [x] 6.2 Confirm `GET /api/v1/settings` is public and `PUT /api/v1/settings` stays admin-only in `cmd/api/main.go`.

## 7. Tests

- [x] 7.1 Extend `internal/services/general_settings_service_test.go` with the merge, the validation ranges, the window ordering and the capability flag.
- [x] 7.2 Extend `internal/handlers/general_settings_handler_test.go` with a partial update that preserves the other fields and with the minimum reservation rejection.
- [x] 7.3 Add `internal/services/payment_schedule_service_test.go` covering the three configured payment windows.
- [x] 7.4 Update `internal/services/contract_service_test.go` for the configurable release window and voucher reminder.
- [x] 7.5 Update `internal/repository/contract_repository_test.go` for the configurable reminder window.
- [x] 7.6 Run `go build ./...` and `go test ./...`.

## 8. Docs and verification

- [x] 8.1 Regenerate the Swagger docs with `make swagger`.
- [x] 8.2 Update `docs/architecture/data-model.md` with the ten new columns and the merge semantics.
- [x] 8.3 Update `docs/architecture/api-and-auth.md` with the settings payload, the capability flag and the minimum reservation error code.
- [x] 8.4 Run `make fmt`, `make lint` and `make security`.
- [x] 8.5 Archive the completed `add-general-settings-and-tracking` change so the canonical `openspec/specs/customer-settings/spec.md` exists before this change is archived.
