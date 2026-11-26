## Why

The API already accepts identity documents at contract-creation time and stores
them on disk, but it loses them: `contracts.document_paths` is never exposed by
any endpoint, no route serves the stored bytes, and a file whose MIME type is
not on the allow-list is dropped with a silent `continue`. The result is that
the staff cannot see a customer's DNI or RTN, cannot tell the customer which
document is missing, and the customer cannot tell the staff what to re-upload.
On top of that, a photo taken with an iPhone is usually HEIC, which the API
rejects outright, so the capture flow silently loses the file.

## What Changes

- Add per-person identity documents with exactly two kinds, `dni` and `rtn`,
  stored in a new `identity_documents` table with one row per `(user_id, kind)`
  and a new `contract_identity_documents` table that links a contract to the
  documents of its applicant.
- Add read/write endpoints for identity documents:
  - `GET /api/v1/users/:user_id/identity_documents` and
    `POST /api/v1/users/:user_id/identity_documents` (admin only).
  - `GET /api/v1/contracts/:guid/identity_documents` and
    `POST /api/v1/contracts/:guid/identity_documents` (admin and seller; the
    seller is limited to contracts it created or that are assigned and
    accepted).
  - `GET /api/v1/identity_documents/:id/content` for the stored original and
    `GET /api/v1/identity_documents/:id/preview` for an inline WebP preview.
- Require both documents to create a contract. A contract whose applicant is
  missing a DNI or an RTN is rejected with `422` that names the missing kinds.
  Documents already on file for the applicant count as satisfied.
- **BREAKING** Stop discarding files silently. An upload whose type is not
  allowed, or whose size exceeds 10 MB, is rejected with `422` that names the
  offending file instead of being skipped.
- **BREAKING** Accept `webp`, `heic` and `heif` in addition to the current
  `pdf`, `jpeg`, `jpg` and `png`, and apply the existing 10 MB cap to contract
  document uploads, which are not limited today.
- Accept typed `identity_document` and `rtn_document` multipart fields on
  contract creation so the API can tell a DNI from an RTN. The anonymous
  `documents[N]` fields keep working for the free-form attachments.
- Migrate the files that were already stored under `contracts.document_paths`
  into `contract_identity_documents` as read-only legacy entries, visible only
  in the contract tab. The migration is a backfill task that first counts the
  legacy files and returns immediately when there is nothing to migrate.
- Store identity documents outside the publicly served `uploads` and
  `receipts` directories so they are only reachable through the authorized
  content endpoints.

## Capabilities

### New Capabilities

- `client-identity-documents`: per-person DNI and RTN documents, their
  contract-scoped exposure, the contract-creation requirement, and the upload
  validation rules.

### Modified Capabilities

None. The repository has no baseline specs in `openspec/specs/` yet, so every
requirement of this change lives in the new capability.

## Impact

- `internal/models/identity_document.go` (new), `internal/models/contract.go`
- `internal/repository/identity_document_repository.go` (new)
- `internal/services/identity_document_service.go` (new),
  `internal/services/contract_service.go`,
  `internal/services/image_processing.go`
- `internal/handlers/identity_document_handler.go` (new),
  `internal/handlers/contract_handler.go`
- `internal/storage/local.go` (accepted types)
- `internal/backfills/tasks.go` (legacy migration task)
- `internal/database/migrations/000046_identity_documents.{up,down}.sql` (new)
- `cmd/api/main.go` (route registration)
- `internal/handlers/handlers.go`, `internal/services/services.go` (wiring)
- `docs/` (Swagger), `docs/architecture/api-and-auth.md`,
  `docs/architecture/data-model.md`, `docs/architecture/backfills.md`
- Consumers: FinteraUI `add-client-identity-documents`,
  FinteraWeb `add-mobile-identity-capture`
