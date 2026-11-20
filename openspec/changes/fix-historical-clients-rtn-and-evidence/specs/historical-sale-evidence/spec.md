## ADDED Requirements

### Requirement: Evidence file upload

The system SHALL expose `POST /api/v1/historical_sales/:id/evidence` accepting a
multipart file (field `file`). The file MUST be PDF, JPEG or PNG and at most
10 MB. The system SHALL store the file under local storage (evidence
year/month directory) and persist the relative path in
`historical_sales.evidence_file`.

#### Scenario: Seller uploads evidence to own record

- **WHEN** the seller who created a historical sale uploads a valid image
- **THEN** the API responds `200` and the stored path is saved in `evidence_file`

#### Scenario: Admin uploads evidence to any record

- **WHEN** an admin uploads a valid file to any historical sale
- **THEN** the API responds `200` and the stored path is saved in `evidence_file`

#### Scenario: Seller uploads evidence to another seller's record

- **WHEN** a seller uploads evidence to a historical sale created by another seller
- **THEN** the API responds with `403 Forbidden` and no file is stored

#### Scenario: Unsupported file type

- **WHEN** the uploaded file is not PDF, JPEG or PNG
- **THEN** the API responds with an error and no file is stored

#### Scenario: File too large

- **WHEN** the uploaded file exceeds 10 MB
- **THEN** the API responds with an error and no file is stored

#### Scenario: Missing file

- **WHEN** the request contains no file in the `file` field
- **THEN** the API responds with `400 Bad Request`

### Requirement: Evidence view and download

The system SHALL expose `GET /api/v1/historical_sales/:id/evidence` that streams
the stored evidence file. The response SHALL use `Content-Disposition: inline`
by default so the image renders in the browser, and `attachment` when the
request includes `download=1`. Admins SHALL access any record; sellers SHALL
access only records they created.

#### Scenario: Admin views the evidence inline

- **WHEN** an admin requests the evidence of any historical sale
- **THEN** the API streams the file with `Content-Disposition: inline`

#### Scenario: Admin downloads the evidence

- **WHEN** an admin requests the evidence with `download=1`
- **THEN** the API streams the file as an attachment

#### Scenario: Seller views own record's evidence

- **WHEN** the creating seller requests the evidence of their historical sale
- **THEN** the API streams the file

#### Scenario: Seller requests another seller's evidence

- **WHEN** a seller requests the evidence of a historical sale created by another seller
- **THEN** the API responds with `403 Forbidden`

#### Scenario: Evidence file is missing on disk

- **WHEN** the stored path no longer exists in storage
- **THEN** the API responds with `404 Not Found`

### Requirement: Legacy external evidence URLs

The system SHALL keep working for historical sales whose `evidence_file` holds
an external `http(s)` URL instead of a stored path: the evidence endpoint SHALL
return that URL to the caller instead of failing.

#### Scenario: Evidence points to an external URL

- **WHEN** the evidence of a historical sale is an external `http(s)` URL
- **THEN** the evidence endpoint returns the URL so the client can open it
