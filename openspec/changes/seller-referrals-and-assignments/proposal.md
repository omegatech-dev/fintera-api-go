## Why

The seller redesign needs server-side data and behavior that does not exist:
seller referral slugs and visit attribution, reservation seller assignment with
a 2-hour acceptance window and background reassignment, an acceptance score that
drives automatic assignment, per-seller commission exceptions, and historical
sale migration. Without these the UI would have to invent clients, duplicate
state or hardcode scores.

## What Changes

- **Seller profile**: add `referral_guid` (unique), `commission_rate_override`,
  `score`, `score_level` and the override fields (`score_override`,
  `score_override_reason/by/at`) to users with the `seller` role.
- **Contract assignment**: add `seller_id`, `assignment_origin`
  (`referral|manual|automatic|none|historical`), `assignment_status`
  (`none|pending|accepted|expired`), `assigned_at`, `accept_deadline_at`,
  `responded_at` and `referral_guid` to contracts.
- **Commissions**: add `commission_origin` (`referral|direct|historical`) and
  `commission_accrual_date`; add a global default rate, an unassigned rate and
  per-seller exceptions, resolved as: seller exception → project rate for the
  financing type → global default.
- **Referral visits**: new `referral_visits` table, public `GET /r/:guid` that
  resolves the seller and records a visit with a 30-day, last-interaction-wins
  attribution.
- **Assignments**: list pending assignments, accept and reject endpoints, and a
  background job that expires overdue assignments and reassigns to the next
  eligible seller with round-robin weighted by score.
- **Score**: nightly recomputation from `accepted / (accepted + expired)` over
  90 days; admin override endpoint with audit.
- **Historical sales**: new `historical_sales` table with CRUD, CSV import and
  admin validation/rejection.
- Swagger annotations and Go tests for every new path.

**BREAKING**: a reservation can now exist with a seller assignment that is
pending or expired; approval and commission logic must use the accepted seller.

## Capabilities

### New Capabilities

- `seller-referrals`: slug, public link resolution, visit tracking and
  attribution.
- `seller-assignments`: assignment lifecycle, acceptance window, reassignment
  and notification hooks.
- `seller-score`: score computation, levels and admin override.
- `historical-sales`: historical sale records, CSV import and validation.
- `commissions`: origin, accrual date and rate resolution.

## Impact

- `internal/models/user.go`, `contract.go`, `payment.go`
- `internal/models/historical_sale.go`, `referral_visit.go` (new)
- `internal/repository/*` (new repositories + queries)
- `internal/services/*` (assignment, referral, score, historical, commission)
- `internal/handlers/*` (referral, assignment, historical, user, contract)
- `internal/jobs/worker.go` and `cmd/api/main.go` (schedules)
- `internal/database/migrations/000032+`
- `docs/` (Swagger regeneration)
- Consumers: FinteraUI `seller-referrals-and-assignments`.
