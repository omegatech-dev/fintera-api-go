## 1. Data model and migrations

- [x] 1.1 Add seller fields to `internal/models/user.go` and the response DTO: `referral_guid`, `commission_rate_override`, `score`, `score_level`, `score_override*`
- [x] 1.2 Add assignment and commission fields to `internal/models/contract.go`: `seller_id`, `assignment_origin/status`, `assigned_at`, `accept_deadline_at`, `responded_at`, `referral_guid`, `commission_origin`, `commission_accrual_date`
- [x] 1.3 Add `internal/models/assignment.go`, `internal/models/referral_visit.go` and `internal/models/historical_sale.go`
- [x] 1.4 Add migrations `000032`–`000036` (paired up/down) for the fields, the visit/historical tables, the settings columns and the seller rate exceptions
- [x] 1.5 Add model and migration tests, including defaults and nullable fields

## 2. Referrals

- [x] 2.1 Add `internal/repository/referral_repository.go` and `internal/services/referral_service.go` (slug generation, resolve, visit recording, attribution window)
- [x] 2.2 Add a public `GET /r/:guid` route in `cmd/api/main.go` with rate limiting and redirect
- [x] 2.3 Accept `referral_guid`/attribution on reservation create and set `seller_id`, `origin = referral`, `status = accepted`
- [x] 2.4 Add `GET /vendedor/referrals` metrics (visits, attributed reservations, conversion, generated commission) and `GET /vendedor/referrals/reservations`
- [x] 2.5 Test slug uniqueness, visit recording, 30-day expiry and last-interaction-wins

## 3. Assignments

- [x] 3.1 Add `internal/repository/assignment_repository.go` and `internal/services/assignment_service.go`
- [x] 3.2 Accept `assignment_origin` and `seller_id` on reservation create; set `pending` + deadline for manual/automatic and `accepted` for referral
- [x] 3.3 Add `GET /assignments/pending`, `POST /assignments/:id/accept`, `POST /assignments/:id/reject` (idempotent, reject expired/answered)
- [x] 3.4 Implement the selection algorithm (green→yellow, lowest load, rotation cursor, random tiebreak, red excluded) with unit tests
- [x] 3.5 Add the expiry job in `internal/jobs/worker.go` and `scheduleJobs` in `cmd/api/main.go`; expire overdue assignments, audit them and reassign
- [x] 3.6 Emit in-app notifications for assignment created, accepted and expired
- [x] 3.7 Test accept/reject/expire and pool exhaustion (reservation continues without seller)

## 4. Score

- [x] 4.1 Add `internal/services/score_service.go`: `accepted/(accepted+expired)` over 90 days, 100 for no history
- [x] 4.2 Add the nightly recompute job, skipping active overrides
- [x] 4.3 Add the admin override endpoint with reason, actor and audit
- [x] 4.4 Expose `score`/`score_level` in the user list, detail and seller responses
- [x] 4.5 Test thresholds (85/84/60/59), no-history, override and job immutability

## 5. Commissions

- [x] 5.1 Extend `CalculateCommission` resolution: seller exception → project rate → global default; unassigned rate when no seller
- [x] 5.2 Persist `commission_origin` and `commission_accrual_date` at approval; map referral/direct/historical
- [x] 5.3 Include historical commissions in the report period using `accrual_date`
- [x] 5.4 Add `commission_default_rate`/`commission_unassigned_rate` settings and the seller exception CRUD
- [x] 5.5 Update `internal/services/report_service.go` and the commission tests

## 6. Historical sales

- [x] 6.1 Add `internal/repository/historical_sale_repository.go` and `internal/services/historical_sale_service.go`
- [x] 6.2 Add CRUD handlers and routes (`GET/POST/PUT /historical_sales`, admin validate/reject)
- [x] 6.3 Add CSV import with template, toggles and a valid/skipped summary
- [x] 6.4 Ensure historical records never create reservations, touch lots or change scores
- [x] 6.5 Add handler/service tests for manual create, CSV errors, duplicates and validation

## 7. Verification

- [x] 7.1 Run `go build ./...`
- [x] 7.2 Run `go test ./...` (including `-race` where the CI does)
- [x] 7.3 Run `go vet ./...`
- [x] 7.4 Run `make swagger` and commit the regenerated docs
- [x] 7.5 Update `docs/architecture/` for the new tables, routes and jobs

## Known follow-ups (coherence audit)

Remaining items from the coherence audit that are intentionally deferred:

- Expose `score_override_by_name` in user responses so `07c` can name the admin who overrode the score.
- Mount `AssignmentCard`'s accepted state on the seller dashboard/queue (today only pending is actionable in the bell).
- Decide whether `accrued` / `cancelled` commission states are part of the contract or the design should be updated to `pending/paid`.
- Complete the CSV column mapping (all 15 fields) and normalize `payment_mode` / `sale_status` to the design values.
- Upload the historical evidence file instead of storing only its filename.
- Paginate historical sales and surface load errors instead of silently showing an empty list.
- Format period boundaries in local time (avoid `toISOString`) so UTC+ zones do not shift the month.
- Localize assignment notifications (ES/EN) or send structured type + data and render on the client.
- Add `assignment_id` to notifications and do not silence `/assignments/pending` failures.
- Align the referral rules copy ("ventas cerradas" vs approval) and remove "podrás cambiarlo después" if no manual reassignment endpoint exists.
- Add the average rate as a fourth admin commissions KPI card.
- Generate the QR locally / download it as a blob instead of depending on an external QR service.
- Confirm the share action with a toast.
- Consume `location.state.openHistoricalImport` so the sheet cannot reopen on its own.
- Add conversion to the `21` referral card to mirror `21f`.
- Add tests for acceptance → portfolio, CSV 207/duplicates, customer payload redaction, `ref` attribution and the configurable acceptance window.
