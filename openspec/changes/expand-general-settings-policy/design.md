## Context

`general_settings` is a singleton row that today holds the support contact,
the legal data, the terms content and `reservation_hold_hours`. The admin
portal (`fintera-ui-react`) sends 20 fields to `PUT /api/v1/settings`; the
handler binds 10 of them and silently drops the other 10. The dropped fields
are exactly the ones that describe a policy: the reservation minimum amount,
the three payment windows, the notification switches and the regional
defaults.

The policy is also only partially enforced. The hold duration reaches
`hold_expires_at` and the reminder email, but `ReleaseUnpaidReservations`
passes a literal 48 to `FindPendingReservations` and
`SendReservationVoucherReminders` passes a literal 40. The payment windows are
literals in `PaymentScheduleService.GenerateSchedule` (7, 14 and 21 days). The
upcoming-reminder window is `CURRENT_DATE + INTERVAL '1 day'` in SQL and the
overdue reminder job has no switch at all.

The endpoint also replaces the row instead of merging it. The admin portal
works around that by sending the full 20-field snapshot on every save, which is
why a section-level save appears to work today. A partial write against the
current endpoint erases every field that was not sent, so the endpoint cannot
be probed to learn whether it merges.

The admin portal's own OpenSpec change (`expand-admin-general-settings`) already
promised the additive API expansion, per-section saves and the currency/locale
defaults. That expansion was never implemented on this side, and the customer
portal (`fintera-customer-site`) hardcodes the same numbers the admin form
edits.

## Goals / Non-Goals

**Goals:**

- Persist the ten missing policy fields and return them from the public read.
- Merge the provided fields on update so a section-level save cannot erase the
  other sections.
- Advertise the merge behaviour explicitly so a client can detect it without a
  destructive write.
- Replace every hardcoded policy literal in the backend with the stored value.
- Enforce the configured minimum reservation amount on reservation creation.
- Apply the deployment-wide email switch on top of the existing per-user opt-out
  and environment kill switch.

**Non-Goals:**

- Recalculating or migrating the payment schedules of contracts that already
  exist. Existing rows keep their dates; only new schedules and new
  reservations use the new policy.
- Changing the per-user `email_notifications` preference or the environment
  kill switch. Both keep working exactly as they do.
- Implementing per-record overrides of the regional defaults in this repo. The
  contract currency already wins over the deployment default; the admin portal
  is the consumer of the currency, locale and timezone.
- Shipping the customer portal or admin portal changes. Those live in their own
  repositories and consume this API as a client.

## Decisions

### Additive migration with reversible down

The ten columns are added with `ADD COLUMN IF NOT EXISTS` in
`000049_expand_general_settings_policy.up.sql`, each with the same default the
admin portal already uses, so a deployment that upgrades without touching the
form keeps its current behaviour. The `down` drops the ten columns and leaves
the row intact. No existing column is modified, so the migration is safe to
apply to a live database and the rollback is a pure `DROP COLUMN`.

### Pointers in the update request to express "not sent"

`generalSettingsUpdateRequest` switches the ten new fields, and the fields that
can legitimately be cleared, to pointers. `nil` means "not sent, keep the
stored value"; a non-nil empty value means "clear it". A plain `int` cannot
express the difference between "not sent" and "sent zero", which is exactly
what makes a merge impossible to implement safely. The service reads the
existing row, applies only the fields that were sent, validates the result and
writes it back.

### Merge by reading the row and applying the patch

The service loads the current row, overlays the provided fields, validates the
merged result and persists it. This keeps the validation honest: a payload
that only changes one field is still validated as a whole, so a merge can never
leave the row in a state the update path would have rejected.

The admin portal sends the full snapshot today, so the merge produces the same
result it produces today. The change only becomes observable for clients that
send a subset.

### Explicit `supports_partial_updates` flag instead of client-side probing

A client cannot learn whether the endpoint merges without either sending a
partial write (destructive against a replace endpoint) or inferring it from
the shape of the response (which only proves the fields exist, not that they
merge). The response therefore carries `supports_partial_updates: true`, and
the admin portal keys its behaviour off that flag. Until this change is
deployed the flag is absent and the admin portal keeps sending the full
snapshot, which is safe against both endpoint shapes.

### A process-wide policy snapshot for the read paths

`GeneralSettingsService` already owns the process-wide hold duration through
`models.SetReservationHoldHours`, and `services.go` warms it at startup. The
same mechanism is extended to a `SettingsPolicy` snapshot holding the whole
policy. The alternative, injecting the settings repository into every consumer,
would add a database round trip to the schedule generator, the reminder jobs
and the email service, which run on paths that are latency sensitive and
already read the policy once per request at most.

The snapshot is refreshed on every settings read and every settings update.
Within a single process the snapshot is authoritative; across replicas a
replica that has not served a settings read since startup keeps the startup
value until its next read. This mirrors the behaviour the hold duration
already has.

### The reservation payment window replaces the hold as the payment deadline

The reservation payment created at reservation time is currently due when the
hold expires, and the payment schedule regenerated on approval puts it at a
literal 7 days. The two disagreed already. The configured
`reservation_due_days` becomes the single source for the payment deadline, and
the hold keeps its own meaning: how long the lot stays apart. The customer
portal needs the hold for the countdown and the window for the due date, and
the admin form exposes both as separate settings.

### The down-payment window depends on the presence of a reservation

`down_payment_due_days_with_reservation` applies when the contract carries a
reservation amount, and `down_payment_due_days` otherwise. The wizard in the
customer portal always creates a reservation, so it reads the former; the
reservation schedule reads the latter when no reservation exists.

### Minimum reservation amount is enforced server-side

The customer wizard already clamps the amount to the configured minimum, but the
server accepted any value. The handler rejects an amount below the minimum with
`400`, the error code `RESERVE_AMOUNT_BELOW_MINIMUM` and the configured minimum
in `min_amount`. The code, not the message, is the contract: the customer
portal localises the message from the code and the minimum, and never parses
prose. A client that sends a request built by hand cannot bypass the rule.

### The deployment email switch is an additional AND

`ENABLE_EMAIL_NOTIFICATIONS` stays the environment kill switch and the per-user
`email_notifications` stays the individual opt-out. The new deployment-wide
setting is a third switch in series: an email is sent only when all three
allow it. This keeps the current behaviour when the new setting is left at its
default of enabled.

### The reminder lead time becomes a query parameter

`FindUpcomingReminders` hardcodes `CURRENT_DATE + INTERVAL '1 day'`. The
window becomes a parameter so the repository stays free of policy knowledge and
the value comes from the snapshot.

## Risks / Trade-offs

- **A partial write against a replace endpoint erases the other fields** →
  The endpoint merges before this change ships, and the admin portal only sends
  a partial payload when the `supports_partial_updates` flag says it is safe.
  The admin portal additionally verifies the response and restores the full
  snapshot if the endpoint did not honour the merge.
- **A stale snapshot in a replica applies the previous policy** → The policy
  only changes when an admin saves the settings, and the read path refreshes
  the snapshot on every settings read. The window is bounded by the replica's
  own settings traffic. Acceptable for a policy that changes rarely.
- **The merge changes the semantics of an existing admin endpoint** → The
  admin portal sends the full snapshot, so it is unaffected. The change is
  marked BREAKING in the proposal because a client that relied on the replace
  behaviour to clear a field must now send it explicitly.
- **Existing contracts keep their original payment dates** → Deliberate. The
  admin form changes the policy for future activity; rewriting the schedule of
  a contract the customer is already paying would be worse than the
  inconsistency.
- **The policy snapshot is read from memory** → A policy change is not visible
  to a process that never reads the settings. The startup warm-up in
  `services.go` covers the common case.
- **`reservation_due_days` and the hold are now independent** → A hold shorter
  than the payment window releases the lot while the payment is still pending.
  Both are admin-controlled, and the admin form validates the window ordering.

## Migration Plan

1. Apply `000049_expand_general_settings_policy.up.sql`. The new columns take
   the defaults the admin portal already uses, so behaviour is unchanged.
2. Deploy the API. The endpoint merges from this release, and
   `supports_partial_updates` is `true` from this release.
3. The admin portal switches to per-section partial writes once it observes the
   flag, and falls back to the full snapshot otherwise.
4. The customer portal replaces its hardcoded hold, minimum amount and currency
   with the values it reads from the public settings endpoint. Until it does,
   the customer portal keeps showing the previous constants.
5. Rollback: revert the API to the previous release. The ten columns remain in
   the database with their defaults and are ignored by the previous release.
   Rolling the migration back drops the columns.

## Open Questions

None.
