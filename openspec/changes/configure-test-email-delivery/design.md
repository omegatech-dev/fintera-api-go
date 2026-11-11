## Context

`EmailService` builds a `resend.SendEmailRequest` in each `Send*` method and
calls `s.resendClient.Emails.Send` directly (11 call sites). Production must keep
sending to real recipients; local/staging must be able to send through Resend to
`delivered@resend.dev`.

See `proposal.md` and
`openspec/changes/configure-test-email-delivery/specs/email-delivery-testing/spec.md`.

## Goals / Non-Goals

**Goals:**

- A single, production-safe switch that redirects mail to a test inbox.
- Keep the original recipient visible for debugging.
- Cover every email path with one wrapper.

**Non-Goals:**

- Per-email-address overrides or a full mail interceptor.
- Changes to templates, triggers or the Resend account.
- Automatic environment detection; the operator sets the env var.

## Decisions

### One send wrapper + pure override helper

`sendEmail` applies `applyRecipientOverride` and calls Resend. The helper is pure
and unit-tested, so no network is needed. All `Send*` methods use the wrapper, so
no email path can bypass the override.

### Subject prefix instead of a separate field

Resend test addresses support labels (`delivered+staging@resend.dev`), but a
single inbox can receive mail for many users. Prefixing the subject with the
original recipients makes each test email identifiable without changing
templates.

### Empty override = production

The config defaults to empty and the helper returns early, leaving `To` and
`Subject` untouched. Production requires no code change.

## Risks / Trade-offs

- **Misdirected test mail**: if `EMAIL_OVERRIDE_TO` is set in production, real
  customers get nothing. The env file documents it loudly and production leaves
  it empty.
- **Unverified sender**: tests need `FROM_EMAIL=onboarding@resend.dev` (or a
  verified domain), otherwise Resend rejects the request.

## Migration Plan

Add `EMAIL_OVERRIDE_TO` to local/staging env files and restart the API. Production
needs no change.
