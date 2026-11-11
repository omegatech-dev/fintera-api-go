## Why

The customer portal's first-login page changes the temporary password with
`POST /api/v1/auth/change_password`, but the API only exposes
`PATCH /api/v1/users/{user_id}/change_password`. Every customer who follows the
welcome email hits `404 page not found` and cannot finish onboarding.

## What Changes

- Add an authenticated `POST /api/v1/auth/change_password` that resolves the
  user from the JWT (no user ID in the URL).
- Keep the existing `PATCH /users/{user_id}/change_password` for the admin UI.
- Validate the current password and the new password with the existing
  `UserService.ChangePassword` (minimum 6 characters), clearing
  `must_change_password`.
- Swagger documentation and handler tests.

## Capabilities

### New Capabilities

- `customer-password-change`: the authenticated endpoint used by the customer
  portal to set a password after the welcome email.

### Modified Capabilities

None.

## Impact

- `internal/handlers/user_handler.go`
- `cmd/api/main.go`
- `internal/handlers/user_password_handler_test.go`
- `docs/` (regenerated Swagger)
- No database or model changes. FinteraWeb needs no change.
