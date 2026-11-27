## Context

`BrandingService` is the reference singleton-config pattern. Contract holds are
computed as `created_at + 48h` in the response mapper, the reservation payment
due date and the reminder email. The customer portal reads contracts through
`GET /contracts/:guid`.

See `proposal.md` and
`openspec/changes/add-general-settings-and-tracking/specs/customer-settings/spec.md`.

## Goals / Non-Goals

**Goals:**

- One admin-managed place for support contact and hold duration.
- Accurate tracking timestamps and lot status without extra queries.
- A safe resend path for customers who never finished account setup.

**Non-Goals:**

- Per-project support numbers.
- Changing the reservation payment amount or approval state machine.
- A new tracking endpoint; the existing contract response is extended.

## Decisions

### Process-wide hold hours

`Contract.ToResponse` is a model method called from many handlers, so threading
the settings service through every call would be invasive. The settings service
loads the value at startup and on update into a thread-safe package variable in
`models`; `HoldExpiresAt` and related helpers read it.

### Resend only before onboarding completes

The resend endpoint regenerates the temporary password only while
`must_change_password` is true. Once the customer set their own password it
returns `409`, avoiding an accidental password reset.

### FSM remains the source of truth

`submitted_at` is set when the reservation receipt moves the contract to
`submitted`; `rejected_at` is set by `Reject`. `approved_at` already exists.

## Risks / Trade-offs

- **Package-level hold setting**: acceptable for a singleton deployment config;
  tests reset it explicitly.
- **Resend reveals whether a reservation exists** via 404/409. The endpoint is
  rate limited and requires the contract GUID.

## Migration Plan

Apply migrations `000022` and `000023`; existing contracts keep null timestamps
and the default 48-hour hold.
