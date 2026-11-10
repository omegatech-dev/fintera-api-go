## Context

`ProjectPaymentInstruction` and its repository/service/handlers were introduced
with the project media work. The model, migration `000015`, admin CRUD routes
(admin-only) and `GET /public/projects/:guid/payment-instructions` already
exist. The remaining gaps are documentation, demo data and input hardening.

See `proposal.md` and
`openspec/changes/harden-project-payment-instructions/specs/project-payment-instructions/spec.md`.

## Goals / Non-Goals

**Goals:**

- Make the admin API discoverable in Swagger with accurate request/response
  shapes.
- Prevent oversized or malformed values from reaching the database.
- Give demo environments realistic accounts so the portal can be exercised.
- Keep the public payload unchanged and privacy-safe.

**Non-Goals:**

- New endpoints, an `active` flag or bulk reorder. Position updates already work
  through PATCH.
- Bank-account verification or bank integrations.
- Changing the reservation/payment flow.
- Multi-currency or per-lot accounts.

## Decisions

### Reuse the existing CRUD surface

The admin UI needs only list/create/update/delete, and the routes already exist
under the admin group. Adding annotations and a typed slice is enough; no new
handlers are needed.

### Validation caps in the service

Lengths are enforced in `validateInstruction` so every entry point (handler,
seeds via API, future imports) shares the same rules. The database columns stay
as they are; the caps are a safety net, not a schema change.

`account_type` is normalized to lowercase and empty values default to
`savings`, matching the existing behavior.

### Idempotent seeds

Seed accounts are inserted only when the project has none with the same account
number, so re-running seeds never duplicates rows and manual edits survive.

### Public payload stays minimal

Only `bank_name`, `beneficiary_name`, `account_number`, `account_type` and
`notes` are public. IDs, positions and timestamps stay admin-only.

## Risks / Trade-offs

- **Seeded accounts could look real.** They use clearly labelled demo banks and
  beneficiaries; production environments should not run the demo seeds.
- **Length caps can reject previously accepted data.** Only via the API; the
  migration is unchanged and existing rows are not truncated.

## Migration Plan

No schema migration. Deploy the API before the FinteraUI page; the endpoints are
unchanged and only better documented and validated. Rollback is a simple revert.
