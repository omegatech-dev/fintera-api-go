## 1. Data model and migration

- [x] 1.1 Add `internal/models/identity_document.go` with the `IdentityDocument`
      model (`UserID`, `Kind`, `FilePath`, `PreviewPath`, `OriginalFilename`,
      `MimeType`, `SizeBytes`, `UploadedBy`, timestamps), the
      `ContractIdentityDocument` model (`ContractID`, `IdentityDocumentID`,
      `Kind`, `Source`, `FilePath`, `OriginalFilename`, `MimeType`,
      `SizeBytes`), the kind constants `dni` / `rtn`, the source constants
      `upload` / `legacy`, the `ToResponse()` DTOs and the `TableName()`
      methods.
- [x] 1.2 Add `internal/database/migrations/000047_identity_documents.up.sql`
      with `identity_documents` (`UNIQUE (user_id, kind)`,
      `CHECK (kind IN ('dni','rtn'))`) and `contract_identity_documents`
      (`UNIQUE (contract_id, kind)`, nullable `kind` and nullable
      `identity_document_id` for the legacy rows) plus their indexes. Use the
      next free number on the target branch: `000046` is free on `main` today,
      but `fix/historical-clients-rtn-and-evidence` already claims
      `000046_users_unique_empty_normalization` — renumber to `000047` if that
      branch lands first.
- [x] 1.3 Add `internal/database/migrations/000047_identity_documents.down.sql`
      dropping both tables, and confirm `migrate-down` leaves the schema as it
      was.
- [x] 1.4 Add `internal/models/identity_document_test.go` covering the kind
      constants and the response DTOs.

## 2. Repository

- [x] 2.1 Add `internal/repository/identity_document_repository.go` with the
      `IdentityDocumentRepository` interface: find by user and kind, find by
      id, list by user, list by contract, upsert, delete the link for a
      contract and kind, and a count of contracts that still carry legacy
      `document_paths`.
- [x] 2.2 Implement the repository over GORM following the style of
      `internal/repository/user_repository.go`, and register it in
      `Repositories` / `NewRepositories` in
      `internal/repository/repositories.go`.
- [x] 2.3 Add the seller-scoped query for the contract documents, reusing the
      predicate of `internal/repository/contract_repository.go:150-156`
      (`creator_id = ? OR (seller_id = ? AND assignment_status = 'accepted')`)
      so the listing and the detail cannot drift apart.
- [x] 2.4 Add the seller-scoped query for the person documents: a seller
      reaches the documents of a person when it has at least one contract with
      that applicant in the scope of 2.3, and nothing otherwise.

## 3. Storage and image processing

- [x] 3.1 Add `storage.ValidIdentityContentTypes()` to
      `internal/storage/local.go` with `application/pdf`, `image/jpeg`,
      `image/jpg`, `image/png`, `image/webp`, `image/heic` and `image/heif`,
      and a helper that reports the rejected file name.
- [x] 3.2 Add `IdentityDocumentMaxDimension` and `IdentityDocumentToWebP` to
      `internal/services/image_processing.go`, reusing `ResizeToWebP`, and
      return no preview for the formats `image.Decode` cannot read.
- [x] 3.3 Confirm the identity documents are written under a subdirectory of
      `STORAGE_PATH` that has no static route in `cmd/api/main.go`, so the
      files are only reachable through the authorized content endpoints.

## 4. Service

- [x] 4.1 Add `internal/services/identity_document_service.go` with
      `Upsert` (store the file, generate the preview, replace the row and
      delete the replaced file), `ListByUser`, `ListByContract`,
      `MissingKinds` and `Content` / `Preview` readers.
- [x] 4.1b Add a `FillMissing` entry point used by contract creation: it
      stores a document only when the applicant has no document of that kind,
      and reports the kinds that were already on file instead of replacing
      them.
- [x] 4.2 Enforce `storage.MaxFileSize()` and
      `storage.ValidIdentityContentTypes()` in the service, returning errors
      that name the offending file.
- [x] 4.3 Make the service resolve the authorization of a document: admins
      always, sellers whose contract is in their assignment scope, and nobody
      else. The contract owner is not authorized.
- [x] 4.4 Add `ErrIdentityDocumentsRequired` with the list of missing kinds
      to `internal/services/errors.go`, and the `422` mapping in the handler.
- [x] 4.5 Register the service in `internal/services/services.go` and give
      `ContractService` the dependency it needs to check the documents.

## 5. Handlers and routes

- [x] 5.1 Add `internal/handlers/identity_document_handler.go` with
      `ListByUser`, `UploadForUser`, `ListByContract`, `UploadForContract`,
      `Content` and `Preview`, each with the Swagger annotations.
- [x] 5.2 Register the routes in `cmd/api/main.go`:
      `GET` and `POST /users/:user_id/identity_documents` in the `admin`
      group, `GET` and `POST /contracts/:guid/identity_documents` in the
      `sellerAdmin` group with the scope checks in the handler, and
      `GET /identity_documents/:id/content` plus
      `GET /identity_documents/:id/preview` in the `protected` group with the
      role and scope checks in the handler. No route grants access to the
      contract owner.
- [x] 5.3 Register the handler in `internal/handlers/handlers.go`.
- [x] 5.4 Stream the file with the stored MIME type, an `inline` preview and
      an `attachment` download, resolving the path through
      `storage.SafeFullPath`.

## 6. Contract creation

- [x] 6.1 Read the `identity_document` and `rtn_document` multipart fields
      in `internal/handlers/contract_handler.go`, and keep the `documents[N]`
      fields as free-form attachments.
- [x] 6.2 Replace the silent `continue` of the upload loop with a `422` that
      names the file when the type is not allowed or the file is larger than
      10 MB, and check the files before the contract is created.
- [x] 6.3 Compute the missing kinds from the documents already on file for the
      applicant plus the ones carried by the request, and return `422` with
      the `missing` array when either kind is absent.
- [x] 6.3b Store the uploaded documents with the fill-missing behavior of
      4.1b: a document that is already on file is kept, the uploaded file is
      not written, and the response reports the kind as already present.
- [x] 6.4 Fill the empty slots with the uploaded documents, keeping the
      documents that are already on file untouched (the fill-missing behavior
      of 6.3b), create the contract and write the
      `contract_identity_documents` links in one transaction, for both the
      staff endpoint and the public reservation endpoint.
- [x] 6.5 Add `IDENTITY_DOCUMENTS_REQUIRED` to `internal/config/config.go`,
      defaulting to `false` so the backend can be deployed before the two
      frontends, and enforce the check in the service when it is on.
- [x] 6.6 Apply the same treatment to `CreateGuest`, the public reservation
      endpoint used by the customer portal.

## 7. Legacy migration (task 0)

- [x] 7.1 Add the `legacy_identity_documents` task to
      `internal/backfills/tasks.go`, registered with `Version = "1"`, and
      give it a service method in
      `internal/services/identity_document_service.go`.
- [x] 7.2 The task counts first: when no contract carries a non-empty
      `document_paths`, it returns `RowsAffected: 0` with a detail that says
      there was nothing to migrate, and it creates no row.
- [x] 7.3 When there is something to migrate, create one
      `contract_identity_documents` row per stored file with
      `source = 'legacy'`, `kind = NULL` and the file metadata read from the
      file itself, pointing at the path that is already on disk. Copy no file.
- [x] 7.4 Make the task idempotent: a contract that already has a legacy row
      for a path is skipped, and the summary reports the counted contracts,
      the created rows and the skipped ones.
- [x] 7.5 Verify the task with `make backfill ARGS="-list"` (registered as
      `legacy_identity_documents`, version 1, pending) and
      `cmd/backfill -task=legacy_identity_documents -force` (empty DB reports
      `nothing to migrate`; a contract with attachments creates one row).

## 8. Tests

- [x] 8.1 Add `internal/services/identity_document_service_test.go` with
      small mocks that embed the repository interface: upsert replaces the
      previous file, the two kinds are enforced, the missing kinds are
      computed, and an oversized or unsupported file is rejected with the file
      name.
- [x] 8.2 Add `internal/handlers/identity_document_handler_test.go` covering
      the admin-only person routes, the seller scope on the contract routes
      and the content and preview endpoints.
- [x] 8.2b Test that the contract owner gets `403` on the contract listing, on
      the content endpoint and on the preview endpoint, and that a seller with
      a contract in scope with the person gets `200` on the person listing
      while a seller without one gets `403`.
- [x] 8.3 Add a contract-creation test: a request without the documents
      returns `422` naming the missing kinds, a request with an unsupported
      file returns `422` naming the file, and a request with both documents
      creates the contract and its links. A request that carries a document the
      applicant already has keeps the stored one, stores nothing and reports
      the kind as already present.
- [x] 8.4 Add a backfill test: with no legacy files the task creates nothing
      and reports that there was nothing to migrate; with legacy files it
      creates one read-only row per file.
- [x] 8.5 Add a test that the identity documents are not served by the public
      static routes.

## 9. Docs

- [x] 9.1 Regenerate the Swagger documentation (`make swagger`) and commit
      `docs/`.
- [x] 9.2 Update `docs/architecture/api-and-auth.md` with the new routes,
      their groups and their roles.
- [x] 9.3 Update `docs/architecture/data-model.md` with the two new tables.
- [x] 9.4 Update `docs/architecture/backfills.md` with the
      `legacy_identity_documents` task.

## 10. Verification

- [x] 10.1 `go build ./...`
- [x] 10.2 `go test ./...`
- [x] 10.3 `gofmt -l .` returns nothing.
- [x] 10.4 `golangci-lint run --timeout=5m` passes, when the linter is
      installed.
- [x] 10.5 `make swagger` produces no diff after the second run.
- [x] 10.6 `openspec validate add-client-identity-documents` passes.
- [x] 10.7 Upload a DNI and an RTN against the local API, create a contract
      without them and confirm the `422`, then create it with them and
      confirm the documents are listed and served.
