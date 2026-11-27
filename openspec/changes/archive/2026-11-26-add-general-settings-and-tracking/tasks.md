## 1. General settings

- [x] 1.1 Add the `general_settings` model and runtime hold helpers.
- [x] 1.2 Add migration `000022_create_general_settings`.
- [x] 1.3 Add the repository, service and handler (public read, admin update).
- [x] 1.4 Wire routes and services and load the hold at startup.
- [x] 1.5 Validate WhatsApp, email and hold hours.

## 2. Configurable hold

- [x] 2.1 Replace hardcoded 48h in the contract response.
- [x] 2.2 Replace hardcoded 48h in the reservation payment due date.
- [x] 2.3 Replace hardcoded 48h in the reminder email.

## 3. Tracking data

- [x] 3.1 Add `submitted_at` and `rejected_at` (migration `000023`).
- [x] 3.2 Set them on the submit and reject transitions.
- [x] 3.3 Expose them plus `lot_status` in `ContractResponse`.

## 4. Resend access

- [x] 4.1 Add `ContractService.ResendAccountSetup`.
- [x] 4.2 Add the rate-limited public endpoint.
- [x] 4.3 Return `409` when onboarding is already complete.

## 5. Docs and verification

- [x] 5.1 Regenerate Swagger.
- [x] 5.2 Update `docs/architecture/data-model.md`.
- [x] 5.3 `go build ./...` and `go test ./...`.
- [x] 5.4 `openspec validate --changes`.
