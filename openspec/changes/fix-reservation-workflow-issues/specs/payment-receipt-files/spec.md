## Purpose

Guarantee that uploaded payment receipts are reachable at the URL the API
reports, so customers and staff can open the proof of payment they were asked
to send.

## ADDED Requirements

### Requirement: Reported receipt URL resolves to the stored file

Every payment response that includes a receipt SHALL expose a `receipt_url`
that maps to the file on disk and returns the file content when requested.
Files uploaded before this change SHALL also resolve, without a database
migration or renaming of stored documents.

#### Scenario: Existing receipt

- **WHEN** a client requests the `receipt_url` of a receipt stored under the
  receipts directory
- **THEN** the response is the stored file with a successful status
- **AND** the content type matches the file extension

#### Scenario: New receipt

- **WHEN** a customer uploads a receipt and the API returns the updated payment
- **THEN** the reported `receipt_url` opens the newly uploaded file

#### Scenario: Unknown file

- **WHEN** a client requests a receipt path that does not exist
- **THEN** the API responds with not found

### Requirement: Receipt files are readable without a session

Because reservation tracking is available to anonymous visitors, receipt files
SHALL be served without requiring a session token. Authenticated download
endpoints SHALL keep working unchanged.

#### Scenario: Anonymous tracking visitor

- **WHEN** a visitor without a session opens the receipt link from the tracking
  screen
- **THEN** the file is served without an authentication error

#### Scenario: Authenticated customer

- **WHEN** a signed-in customer downloads a receipt through the authenticated
  endpoint
- **THEN** the file is returned as before
