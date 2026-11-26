## Context

`internal/handlers/contract_handler.go` already accepts multipart files when a
contract is created. The loop that stores them skips anything whose
`Content-Type` is not in `storage.ValidContentTypes()`, skips anything that
fails to open, and skips anything the storage layer rejects — all with a bare
`continue`. The surviving paths land in `contracts.document_paths`, a JSON
array of relative paths under `storage/contracts/YYYY/MM/`, and no endpoint
reads that column.

The consequences today:

- The staff has no way to see what a customer attached.
- A customer who photographed their DNI with an iPhone produced a HEIC file,
  which is not on the allow-list, so the file was dropped without a word.
- `storage.MaxFileSize()` exists and is applied to payment receipts
  (`internal/handlers/payment_handler.go`) but not to contract documents.

The API already has the pieces this change needs: `internal/storage` for local
files, `internal/services/image_processing.go` for the WebP encoder used by the
media, branding and receipt flows, `internal/backfills` for one-off data tasks
with a ledger, and `internal/services/image_service.go` for the
"process and save" pattern.

See `proposal.md` and
`openspec/changes/add-client-identity-documents/specs/client-identity-documents/spec.md`.

## Goals / Non-Goals

**Goals:**

- Give the staff a way to see and to upload the DNI and the RTN of a customer.
- Make it impossible to create a contract without both documents, and say
  precisely which one is missing.
- Stop losing files: a file the API cannot accept must be reported, not
  dropped.
- Keep the files private: an identity document must not be reachable by
  guessing a URL.
- Give the legacy files that are already on disk a visible home, without
  rewriting them and without pretending they are the person's DNI.

**Non-Goals:**

- Versioning. A new DNI replaces the old one; the old file is deleted.
- Deciding whether a document "looks like" a DNI. The API stores and serves
  the file; it does not read it.
- Rasterizing HEIC or PDF. The API does not decode HEIC and does not render
  PDFs; the browser normalizes HEIC before upload and the preview falls back
  to serving the original.
- Changing the free-form `documents[N]` attachments beyond rejecting the ones
  the API cannot store.
- Any change to the contract state machine or to the ledger.

## Decisions

### Two tables: the document belongs to the person, the link belongs to the
contract

`identity_documents` holds the file, one row per `(user_id, kind)`. It is the
person's DNI and RTN and it is the only place a file path lives for a
canonical document. `contract_identity_documents` links a contract to the
documents of its applicant and additionally stores the metadata of the
migrated legacy files, which have no person-level row because they cannot be
attributed to a slot.

The uniqueness rule is `UNIQUE (user_id, kind)` on `identity_documents`, plus
a `CHECK (kind IN ('dni','rtn'))`. A person therefore has at most one DNI and
one RTN, and no third kind can be introduced.

### The legacy entries carry no kind

A legacy contract may have one, two or more stored files, and nothing in the
data says which one was the DNI. Forcing them into the `dni`/`rtn` slots
would both guess wrong and break `UNIQUE (contract_id, kind)`.

`contract_identity_documents.kind` is therefore nullable, and the uniqueness
constraint is `UNIQUE (contract_id, kind)`. In PostgreSQL a unique constraint
treats `NULL` as distinct, so several legacy rows with `kind IS NULL` coexist
in the same contract while two rows claiming the same kind do not. The
`CHECK (kind IN ('dni','rtn'))` constraint accepts `NULL` as well, because a
check is satisfied when its expression is not false.

The result is that the two-kind rule is enforced strictly where it matters —
on the person's slots — while the legacy files stay unclassified and
read-only, exactly as decided.

### Legacy files are referenced in place, not copied

The migration writes metadata rows that point at the files that are already
under `storage/contracts/YYYY/MM/`. Nothing is copied and nothing is moved, so
a failure of the migration can never lose a file, and the storage cost of the
migration is a few rows. The relative paths continue to be resolved through
`storage.SafeFullPath`, so a malformed row still cannot escape the storage
root.

### The contract-scoped routes use the contract GUID

The change is addressed with `/api/v1/contracts/:guid/identity_documents`, not
`/api/v1/contracts/:contract_id/identity_documents`.

Gin builds one radix tree per method and refuses two different wildcard names
at the same position. The API already registers `/contracts/:guid`,
`/contracts/:guid/prepayments`, `/contracts/:guid/assignments`,
`/contracts/:guid/reminder` and `/contracts/:guid/commission/pay`, so adding
`/contracts/:contract_id/identity_documents` makes the router panic at
startup, in either registration order. The GUID is also what the staff UI
already holds when it renders a contract, and it is what the other
contract-scoped staff routes use.

The alternative — hanging the routes off
`/projects/:project_id/lots/:lot_id/contracts/:contract_id/identity_documents`
— was rejected because it forces the caller to know three ids to reach a file
that is conceptually a property of one contract.

### Identity documents are stored outside the static routes

`cmd/api/main.go` serves `router.Static("/uploads", cfg.StoragePath+"/uploads")`
and `router.Static("/receipts", cfg.StoragePath+"/receipts")` without
authentication. Identity documents are written under
`identity_documents/YYYY/MM/`, which has no static route, so the only way to
read one is `GET /api/v1/identity_documents/:id/content` after the
authorization check.

The legacy files under `contracts/` are likewise not statically served today,
and the content endpoint serves them through the same authorization path.

### Content and preview are two endpoints

`GET /api/v1/identity_documents/:id/content` returns the stored original with
its real MIME type, as an attachment by default and inline when the caller
asks for it. `GET /api/v1/identity_documents/:id/preview` returns an inline
`image/webp` raster preview for the formats the Go decoders understand
(JPEG, PNG, WebP) and falls back to serving the original inline for the ones
they do not (PDF, HEIC, HEIF).

Serving a preview and an original from one endpoint would make the response
type depend on a query flag, and the staff UI needs both at once: a thumbnail
in the slot and the original on download.

### Previews reuse the existing WebP encoder

`internal/services/image_processing.go` already decodes raster images and
re-encodes them as lossy WebP. The identity flow adds
`IdentityDocumentMaxDimension` (1600) and `IdentityDocumentToWebP`, reusing
`ResizeToWebP`. HEIC and PDF fail `image.Decode`, so those uploads are stored
without a preview rather than being rejected: the file is still valid and the
caller still gets a download.

### The format allow-list for identity documents is its own list

`storage.ValidContentTypes()` is shared with the payment-receipt flow, whose
uploads are converted to WebP by a decoder that cannot read HEIC. Widening
that list would accept receipts that can never be previewed. The identity flow
gets `storage.ValidIdentityContentTypes()` with `application/pdf`,
`image/jpeg`, `image/jpg`, `image/png`, `image/webp`, `image/heic` and
`image/heif`, and the identity service enforces the 10 MB cap with
`storage.MaxFileSize()`.

HEIC is accepted even though the browser is expected to normalize it, because
Safari is the only engine that decodes HEIC natively and a customer on
Chrome will send the raw file. Accepting it is the difference between a
stored document and a silent loss.

### The requirement is enforced by the service, in one place

`ContractService` gains an `identityDocumentService` dependency and exposes a
single entry point for contract creation that:

1. resolves the applicant,
2. computes the kinds that would be missing, given the documents already on
   file for that applicant plus the ones carried by the request,
3. returns `ErrIdentityDocumentsRequired{Missing: []Kind}` when the set is not
   `{dni, rtn}`,
4. fills the empty slots with the uploaded documents, leaving the documents
   that are already on file untouched and reporting those kinds as already
   present,
5. creates the contract and its links in one transaction.

The check runs before anything is written, so a rejected request leaves no
half-uploaded document behind. The handler maps the error to `422` with a
`missing` array naming the kinds.

Both the staff endpoint and the public reservation endpoint go through the
same service method, so the customer and the staff get the same rule and the
same error.

### Creating a contract never replaces a document that is already on file

The requirement is about *having* the documents, not about re-capturing them.
When a contract is created with an `identity_document` or an `rtn_document` and
the applicant already has that kind on file, the stored document is left
untouched, the uploaded file is not written, and the response reports the kind
as already present. Replacing a document is an explicit act, performed through
the upload endpoints, where the staff sees the current document before it
changes.

Without this rule the seller flow would silently destroy the customer's file:
the seller cannot read the person's documents, the wizard forces an upload, and
the upload would overwrite the DNI the customer sent from the portal — for
every contract that person has.

### A seller reads a person's documents only within its scope

The person-scoped read is admin-only for the upload, but a seller needs to know
whether the customer it is about to write a contract for already has the
documents, otherwise the wizard can only offer an upload. The person-scoped
`GET` therefore answers `200` for a seller that has at least one contract with
that applicant in scope, and `403` otherwise.

This does not widen what a seller can see: those are the same documents, of the
same applicants, that the seller already reaches through the contract-scoped
route for the contracts in scope. The staff profile keeps the documents hidden
from sellers; the wizard is the only surface that uses the scoped read.

### The requirement is behind a configuration flag during the rollout

The backend is deployed before the two frontends, and the current staff UI
creates contracts without typed identity documents. Turning the requirement on
in the first backend deploy would break every reservation created from the
current UI.

`internal/config` therefore gains `IDENTITY_DOCUMENTS_REQUIRED`, defaulting to
`false`. It is flipped to `true` once FinteraUI and FinteraWeb are in
production. The target behaviour — the behaviour the spec describes — is the
one that runs once the flag is on, and the flag is removed once it is no longer
needed.

### Authorization matrix

| Route | admin | seller | contract owner | other customer |
| --- | --- | --- | --- | --- |
| `GET /users/:user_id/identity_documents` | yes | in scope | `403` | `403` |
| `POST /users/:user_id/identity_documents` | yes | `403` | `403` | `403` |
| `GET /contracts/:guid/identity_documents` | yes | in scope | `403` | `403` |
| `POST /contracts/:guid/identity_documents` | yes | in scope | `403` | `403` |
| `GET /identity_documents/:id/content` | yes | in scope | `403` | `403` |
| `GET /identity_documents/:id/preview` | yes | in scope | `403` | `403` |

"Seller in scope" is the same predicate the contract listing uses:
`creator_id = seller_id OR (seller_id = seller_id AND assignment_status =
'accepted')`, applied in the repository query rather than in the handler, so
the listing and the detail cannot drift apart.

The person-scoped routes live in the `admin` group, which already carries
`RequireAdmin`. The contract-scoped listing and upload live in the `sellerAdmin`
group, and the content and preview endpoints live in the `protected` group with
the role and scope checks in the handler.

The contract owner is not a caller. The identity documents are internal staff
records: the customer uploaded them, but the dossier is the staff's, and the
customer has no route to read them back.

The content endpoint resolves the document, then resolves the contract it
belongs to, then applies the same scope predicate, so a seller cannot read a
file by guessing its id.

## Risks / Trade-offs

- **The staff UI breaks between the backend deploy and the flag flip.** →
  The flag defaults to `false`, and the flip is scheduled after FinteraUI and
  FinteraWeb are live. The `422` body names the missing kinds so the UI can
  show what to upload.
- **A contract created before this change has no identity documents.** → The
  requirement only applies at creation, so those contracts stay editable and
  keep working. Their documents tab shows the empty slots and the legacy
  entries the migration gives them, and the staff can upload a proper DNI and
  RTN for the applicant at any time.
- **A seller's upload can no longer replace a document by accident.** → The
  contract creation does not write over a document that is on file; the
  response reports the kind as already present, and the staff replaces it
  deliberately from the slot, where the current file is visible first.
- **The legacy entries cannot be classified as a DNI or an RTN.** → They are
  shown without a kind and marked read-only. The staff can upload a real DNI
  through the slot, which becomes the person's canonical DNI.
- **A HEIC preview is not available.** → The preview endpoint falls back to
  the original inline. Chrome cannot display HEIC either, so the staff sees
  the browser's own behaviour. The customer portal normalizes HEIC to JPEG in
  the browser, so in practice the stored file is a JPEG.
- **The legacy migration reads every contract with `document_paths`.** → The
  task counts first and returns without touching a row when the count is
  zero, and it is registered as a normal backfill with a ledger row, so it
  runs once and its outcome is recorded.
- **Deleting a replaced document leaves the file on disk if the delete fails.**
  → The row update and the file delete happen in that order, so a failed
  delete leaves an orphan file that no row points at, never a row pointing at
  a missing file.

## Observability

Every rejected upload is logged with `logger.Warn` and includes the acting
`user_id`, the contract id, the file name and the reason, so a `422` can be
traced back to the file that caused it. The legacy migration records the
counted contracts, the created rows and the skipped ones in the `details`
column of its `backfill_runs` row.

## Migration Plan

1. Deploy the migration — additive, two new tables, no change to existing
   columns. Use the next free number on the target branch: `000046` is free on
   `main` today, but the pending branch
   `fix/historical-clients-rtn-and-evidence` already claims
   `000046_users_unique_empty_normalization`, so the number becomes `000047`
   if that branch lands first.
2. Deploy the API with `IDENTITY_DOCUMENTS_REQUIRED=false`. The new endpoints
   are live; the requirement is not enforced.
3. Run the `legacy_identity_documents` backfill. It counts first and does
   nothing when there is nothing to migrate.
4. Deploy FinteraUI and FinteraWeb.
5. Flip `IDENTITY_DOCUMENTS_REQUIRED=true`.

Rolling back to the previous API version leaves the two tables in place and
harmless; the flag can be flipped back to `false` to stop the `422` without a
deploy.
