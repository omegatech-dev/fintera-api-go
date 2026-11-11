## Why

Local and staging environments need to verify that transactional emails render
and are accepted by Resend, without emailing real customers. Resend provides a
test inbox (`delivered@resend.dev`) that always reports delivery. The API currently
sends every message to the customer's real address, so there is no safe way to
exercise the email flow outside production.

## What Changes

- Add `EMAIL_OVERRIDE_TO` config (empty by default).
- Route every outbound email through a single `EmailService.sendEmail` wrapper.
- When the override is set:
  - all recipients are replaced with the test inbox;
  - the subject is prefixed with `[TEST → <original recipients>]` for traceability;
  - the redirect is logged.
- When it is empty (production default), behavior is unchanged.
- Document the local/staging setup, including `FROM_EMAIL=onboarding@resend.dev`
  and `ENABLE_EMAIL_NOTIFICATIONS=true`.

## Capabilities

### New Capabilities

- `email-delivery-testing`: safe redirection of outbound email in non-production
  environments.

### Modified Capabilities

None.

## Impact

- `internal/config/config.go`
- `internal/services/email_service.go` and `email_service_test.go`
- `.env.example`, `docs/guides/development.md`
- No API contract, database or template changes.
