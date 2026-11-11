## Purpose

Let non-production environments exercise real Resend delivery without messaging
customers, while production remains unchanged.

## ADDED Requirements

### Requirement: Test recipient override

When `EMAIL_OVERRIDE_TO` is set, the API SHALL send every outbound email to that
address instead of the original recipient, and SHALL prefix the subject with the
original recipients.

#### Scenario: Override configured

- **WHEN** `EMAIL_OVERRIDE_TO=delivered@resend.dev` and an email is triggered
- **THEN** Resend receives only `delivered@resend.dev` as the recipient
- **AND** the subject starts with `[TEST → <original recipients>]`

#### Scenario: Override empty

- **WHEN** `EMAIL_OVERRIDE_TO` is empty
- **THEN** the email keeps its real recipients and subject

### Requirement: All email paths redirected

Every transactional email SHALL go through the same send wrapper so no email
bypasses the override.

#### Scenario: Any transactional email

- **WHEN** an override is configured and any email is triggered (account setup,
  recovery code, contract, payment, reminder)
- **THEN** it is redirected to the test inbox

### Requirement: Test delivery prerequisites

The documentation SHALL describe the local/staging setup.

#### Scenario: Local setup

- **WHEN** an operator configures local email testing
- **THEN** the docs show `ENABLE_EMAIL_NOTIFICATIONS=true`,
  `FROM_EMAIL=onboarding@resend.dev`,
  `EMAIL_OVERRIDE_TO=delivered@resend.dev`, and a valid `RESEND_API_KEY`
