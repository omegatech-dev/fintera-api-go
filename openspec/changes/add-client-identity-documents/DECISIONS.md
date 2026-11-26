# Decision record — `add-client-identity-documents`

This note records what was decided **after** this change was first written and
reviewed, and what the backend has to build as a result. It does not replace
`design.md`; it explains what changed there and why, so a reviewer does not
have to reverse-engineer the intent from the diff.

Review date: 2026-11-23. Affected repositories: `FinteraAPI` (this change),
`FinteraUI` (`add-client-identity-documents`), `FinteraWeb`
(`add-mobile-identity-capture`, unchanged by this round).

---

## What did not change

The original design was sound and most of it is confirmed as written:

- the two-table model, with the file on the person and the link on the
  contract;
- `UNIQUE (user_id, kind)` plus `CHECK (kind IN ('dni','rtn'))`, which keeps
  the "exactly two documents" rule strict;
- the contract-scoped routes addressed with `:guid` rather than
  `:contract_id` — Gin builds one radix tree per method and refuses two
  different wildcard names at the same position, and `admin` is a subgroup of
  `protected` (`cmd/api/main.go:311`), so both live in the same tree;
- the `IDENTITY_DOCUMENTS_REQUIRED` flag defaulting to `false`, so the backend
  can ship before the two frontends;
- a separate `storage.ValidIdentityContentTypes()` instead of widening the
  list shared with the receipt flow, whose decoder cannot read HEIC;
- the original and the preview served from two endpoints;
- the legacy migration referencing the files already on disk, copying nothing,
  registered as a backfill that counts first;
- the write order that updates the row before deleting the replaced file, so a
  failed delete leaves an orphan file and never a row pointing at a missing
  one;
- `GET /api/v1/contracts/:guid/identity_documents` instead of the three-id
  path.

---

## The final access matrix

| Route | admin | seller | contract owner | other customer |
| --- | --- | --- | --- | --- |
| `GET /users/:user_id/identity_documents` | yes | in scope | `403` | `403` |
| `POST /users/:user_id/identity_documents` | yes | `403` | `403` | `403` |
| `GET /contracts/:guid/identity_documents` | yes | in scope | `403` | `403` |
| `POST /contracts/:guid/identity_documents` | yes | in scope | `403` | `403` |
| `GET /identity_documents/:id/content` | yes | in scope | `403` | `403` |
| `GET /identity_documents/:id/preview` | yes | in scope | `403` | `403` |

"Seller in scope" is the predicate the contract listing already uses:
`creator_id = ? OR (seller_id = ? AND assignment_status = 'accepted')`.

**The contract owner is never a caller.** The identity documents are internal
staff records. The customer uploaded them; the dossier is the staff's. This is
a deliberate privacy decision and it is now pinned by scenarios in the spec,
so it cannot be reintroduced by a later reader who assumes symmetry with
`ShowByGUID`.

**The two statuses are deliberately different, and they go together.** The
contract-scoped routes are addressed by a `guid` the caller may already be
looking at, so an out-of-scope seller answers `403` there — the resource is
known to them. The content and preview endpoints are addressed by a document
id, which is the only enumeration surface, so they answer `404` for anyone who
is not an admin and not a seller in scope: a `404` does not confirm that the
document exists, and for the owner that is the strongest form of "no route to
read them back". Both serve no bytes.

---

## The corrections

### C1 — The customer had been granted access to their own documents

**Was:** the design matrix gave the contract owner `yes` on the contract
listing and on the content endpoint, the tasks put the contract-scoped `GET`
in the `protected` group, and task 4.3 resolved authorization for "the owner of
the document".

**Now:** the owner is `403` everywhere. `GET /contracts/:guid/identity_documents`
and `POST /contracts/:guid/identity_documents` both live in the `sellerAdmin`
group; the content and preview endpoints stay in `protected` with the role and
scope checks in the handler.

**Where:** `specs/client-identity-documents/spec.md` (two new scenarios, "The
contract owner is not a caller"), `design.md` (matrix and the paragraph about
the route groups), `tasks.md` 4.3 and 5.2.

### C2 — The migration number depends on the target branch

`000046` is free on `main` today, where the latest migration is
`000045_backfill_runs`. But the pending branch
`fix/historical-clients-rtn-and-evidence` already contains
`000046_users_unique_empty_normalization`. If that branch lands first, the
identity migration becomes `000047`.

**Now:** both `tasks.md` 1.2 and the `design.md` migration plan say "the next
free number on the target branch" and name the conflict.

### C4 — A risk contradicted its own mitigation

The risk was titled *"A contract that exists today without documents becomes
uneditable"* and its mitigation said the opposite ("the requirement only
applies at creation; editing an existing contract is not affected").

**Now:** the title states the actual situation — those contracts stay editable,
keep working, and show the empty slots plus the migrated legacy entries.

---

## The two behavioral rules

### Rule A — Creating a contract never replaces a document that is on file

When a contract is created with an `identity_document` or an `rtn_document` and
the applicant already has that kind on file:

- the stored document is left untouched;
- the uploaded file is **not** written;
- the response reports the kind as already present.

Replacing a document stays an **explicit** act, performed through the upload
endpoints, where the staff sees the current document before it changes.

**Why:** without this rule the seller flow silently destroys the customer's
file. The seller cannot read the person's documents, the wizard forces an
upload, and with person-level storage and replace semantics that upload
overwrites the DNI the customer sent from the portal — for every contract that
person has. The stored file was deleted in the process. The original file the
customer provided was gone, replaced by a phone photo taken by a seller, and
nothing in the response said so.

**Where:** `design.md` (new decision), `spec.md` (requirement "An explicit
upload replaces the previous one" plus its scenario), `tasks.md` 4.1b
(`FillMissing`), 6.3b and the test in 8.3.

Two places carried the same trap and were fixed after this record was written:

- The typed-field requirement asserted "a contract is created with an
  `identity_document` file → the file is stored as the applicant's DNI" with no
  mention of the slot, so a reader could reintroduce the overwrite from that
  requirement alone. Its scenarios now carry the empty-slot precondition, each
  kind has a complementary "already covered" scenario, and the requirement says
  the fields fill the slots the applicant does not have yet.
- Step 4 of the service entry point said "stores the uploaded documents" and
  task 6.4 repeated it, contradicting 6.3b two lines above. Both now describe
  the fill-missing behavior.
- The response shape was never pinned. "The response reports the kind as
  already present" left the field name to the implementer, so the three repos
  could have shipped three different names, and a `201` carrying a notice
  could have been read as a partial failure. The spec now has a requirement,
  "The creation response reports the kinds it did not store", that fixes the
  field as `already_present`, always present and empty when nothing was
  skipped, on a `201` that stays a success.

### Rule B — A seller reads a person's documents only within its scope

`GET /users/:user_id/identity_documents` answers `200` for a seller that has
at least one contract with that applicant in scope, and `403` otherwise. The
person-scoped **upload** stays admin-only.

**Why:** the wizard has to know whether the customer already has the
documents. Without this read, the wizard can only offer an upload, and the
seller is pushed into Rule A's failure mode for a customer they already serve.

**Why it does not widen the seller's reach:** these are the same documents, of
the same applicants, that the seller already reaches through the
contract-scoped route for the contracts in scope. It adds no document the
seller could not already see.

**Why the profile is still admin-only:** that is a UI decision in FinteraUI.
The drawer section stays gated on `currentUser?.role === "admin"`. The wizard
is the only surface that uses the scoped read.

**Where:** `design.md` (new decision), `spec.md` (requirement "Person-level
identity documents are admin only, or within a seller's scope" plus its two
scenarios), `tasks.md` 2.4 and the test in 8.2b.

---

## The seller flow, before and after

**Before:** seller → cannot read the customer → wizard blocks the submit →
seller uploads → the upload replaces the customer's DNI across all their
contracts → the customer's original file is deleted.

**After:** seller → reads the customer's documents when a contract in scope
exists → the wizard offers "reutilizar" and uploads nothing → nothing is
replaced. When no contract in scope exists, the seller must upload, and that
upload fills the empty slot without touching anything else.

---

## Implementation order

1. The migration, the models, the repository, the service and the endpoints,
   with `IDENTITY_DOCUMENTS_REQUIRED=false`.
2. The legacy backfill registered and run. It counts first and does nothing
   when there is nothing to migrate.
3. FinteraUI and FinteraWeb deployed.
4. `IDENTITY_DOCUMENTS_REQUIRED=true`.

Rolling back to the previous API version leaves the two tables in place and
harmless, and the flag can be flipped back to `false` without a deploy.

---

## Checklist for the implementer

- [ ] No route authorizes the contract owner. Check the handler, not only the
      route group.
- [ ] The content endpoint resolves the document, then its contract, then
      applies the scope predicate — a seller must not read a file by guessing
      its id.
- [ ] Contract creation fills an empty slot and never replaces a stored one;
      the response reports the kinds that were already present.
- [ ] The creation response carries `already_present`, always present and
      empty when every uploaded document filled an empty slot, and the `201`
      stays a success.
- [ ] The person-scoped read resolves the seller's scope through the
      repository, using the same predicate as the contract listing.
- [ ] The migration number is checked against the target branch before the
      file is created.
- [ ] The legacy task is idempotent: a contract that already has a row for a
      path is skipped, and the summary reports the counted contracts, the
      created rows and the skipped ones.
