## Context

See `proposal.md`. The backend already stores `commission_amount` on contracts,
has per-project rates (`commission_rate_direct|_bank|_cash`), a notification
service, an audit service and a scheduled worker. This change extends those
pieces instead of adding a parallel stack.

## Goals / Non-Goals

**Goals:**

- Persist referral attribution, assignments, scores and historical sales.
- Keep reservation approval, payment and commission logic working with the new
  seller fields.
- Make automatic assignment deterministic, fair and testable.

**Non-Goals:**

- Email/WhatsApp delivery for assignments (in-app notification only here).
- Custom seller domains.
- Redesigning the existing payment or PDF pipelines.

## Decisions

### Data model

Migrations (immutable, paired up/down):

- `000032_seller_referrals_and_scores`: users gain `commission_rate_override`
  (nullable), `score` (default 100), `score_level` (default `high`),
  `score_override` (nullable), `score_override_reason`, `score_override_by`,
  `score_override_at`.
- `000033_contract_assignments`: a new `assignments` table (one row per attempt:
  `id`, `contract_id`, `seller_id`, `origin`, `status`, `assigned_at`,
  `accept_deadline_at`, `responded_at`, `created_at`) plus contract fields for
  fast reads: `seller_id`, `assignment_origin`, `assignment_status`,
  `assigned_at`, `accept_deadline_at`, `responded_at`. The `assignments` history
  is what the score counts.
- `000034_referral_visits`: `id`, `seller_id`, `referral_guid`, `visited_at`,
  `path`, `session_hash` (hashed IP/user agent), indexes on
  `(seller_id, visited_at)`.
- `000035_historical_sales`: client data, `project_name`, `lot_label`,
  `area_m2`, `sale_price`, `sale_date`, `payment_mode`, `sale_status`,
  `installments_paid`, `rate`, `commission_amount`, `commission_status`,
  `evidence_file`, `validation_status`, `created_by`, `validated_by`,
  `validated_at`, timestamps.
- `000036_commission_origin_and_rates`: contracts gain `commission_origin`
  (default `direct`) and `commission_accrual_date`; settings gain
  `commission_default_rate` (5), `commission_unassigned_rate` (3) and a
  `seller_commission_rates` exception table (`seller_id`, `rate`).
- `000038_seller_referral_guid`: `users.referral_guid` (UUID, unique) and
  `contracts.referral_guid` (text); backfills sellers and contracts and drops the
  superseded `referral_slug` columns. The public link uses the opaque GUID.

### Referral resolution

`GET /r/:guid` is public and rate-limited. It resolves the seller by
`referral_guid`, writes a `referral_visits` row, sets a signed attribution
cookie/param with the GUID and `expires_at = now + 30d`, and returns the portal
redirect. The portal can also call `POST /public/referrals/:guid/visit` (204, no
redirect) to record the visit and carry the GUID into its reservation flow. A
reservation create request that carries the attribution (cookie or explicit
`referral_guid`) sets `seller_id`, `origin = referral`,
`assignment_status = accepted`, and clears the attribution. Last interaction wins
because each visit overwrites the cookie.

### Assignment lifecycle

`POST` reservation create accepts `assignment_origin` and optional `seller_id`.
Server rules:

- `manual` with `seller_id` → `pending`, `accept_deadline_at = now + window`.
- `automatic` or no seller → run the selection algorithm, `pending`.
- `referral` → `accepted` immediately, no deadline.
- `seller_id` absent after expiry → reservation continues with no seller.

Endpoints: `GET /assignments/pending`, `POST /assignments/:id/accept`,
`POST /assignments/:id/reject`. Accept/reject are idempotent and reject expired
or already-answered assignments.

### Selection algorithm (round-robin weighted by score)

1. Candidates: active sellers, `score_level ∈ {high, medium}`, not already
   expired for this reservation.
2. Order by `score_level` (high first), then `pending_load` asc (count of active
   `pending` assignments), then `last_assigned_at` asc.
3. A per-level rotation cursor (persisted or derived from `last_assigned_at`)
   guarantees a full cycle before repeating.
4. Random tiebreak inside the same level and load.
5. New sellers start `high` with `pending_load = 0`.

The UI never computes this; it only displays the result.

### Background jobs

- **Expire assignments**: every 5 minutes, mark `pending` rows past
  `accept_deadline_at` as `expired`, write the audit event, penalise the score
  window, and enqueue a reassignment for the contract (same algorithm, excluding
  expired sellers). Stop when a seller accepts or the pool is exhausted.
- **Recompute scores**: nightly, `accepted / (accepted + expired)` over 90 days;
  no history → 100 (`high`). Persist the value and level; never overwrite an
  active admin override.

### Commission resolution

When a contract is approved, resolve the rate: seller exception → project rate
for `financing_type` → global default. Contracts without a seller use the
unassigned rate. Store `commission_amount`, `commission_origin` and
`commission_accrual_date` on the contract. Existing contracts keep their stored
amount.

Origin mapping: `referral` when the contract came from a referral attribution,
`historical` for migrated sales, `direct` otherwise.

### Historical sales

`POST /historical_sales`, `PUT /historical_sales/:id`,
`POST /historical_sales/import` (CSV multipart), `GET /historical_sales` filtered
by creator/admin, and `POST /historical_sales/:id/validate|reject` (admin).
Import supports mark-historical, skip-duplicates-by-identity and
commission-with-sale-date toggles and returns a valid/skipped summary.
Historical records never create a reservation, never touch lots or stock, never
change the score, and store the commission with `sale_date` as accrual date.

## Risks / Trade-offs

- [Reassignment can loop through the whole pool] → Cap attempts per contract and
  stop when the pool is exhausted; the reservation continues without a seller.
- [Score override race with the nightly job] → The job skips sellers with an
  active override and records the recomputed value separately.
- [Attribution cookie cleared too early] → Clear it only after a reservation is
  created, and store the GUID on the contract for traceability.
- [CSV import duplicates] → Unique key on `(created_by, identity, sale_date)` and
  the skip toggle; invalid rows are reported, not fatal.
- [Existing project rates] → Kept as the fallback, so no data migration is
  required for current contracts.

## Migration Plan

- Apply migrations in order; all are additive.
- Backfill `score = 100`, `score_level = high` for existing sellers.
- Backfill `commission_origin = direct` and `commission_accrual_date =
  approved_at` for existing contracts where possible.
- Deploy this change before the UI change; the UI reads the new fields
  defensively.
- Rollback: the migrations have down files; the UI can be rolled back
  independently.

## Open Questions

- Whether an expired **manual** assignment should fall back to the automatic
  pool or leave the reservation without a seller. Proposed: fall back.
