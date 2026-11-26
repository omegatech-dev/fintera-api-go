## Purpose

Let staff and customers capture, store, and retrieve the two identity documents
of a person — the DNI and the RTN — so a contract can never be created without
them, and so the staff can always see what the customer actually sent.

## ADDED Requirements

### Requirement: Identity documents live on the person

The API SHALL store identity documents against the person who owns them, and
SHALL support exactly two kinds: `dni` and `rtn`. A person SHALL have at most
one document of each kind, and every contract of that person SHALL show the
same two documents.

#### Scenario: A person's documents are listed as two slots

- **WHEN** an authorized caller lists the identity documents of a person
- **THEN** the response contains one `dni` entry and one `rtn` entry
- **AND** a slot with no document on file is reported as empty rather than omitted

#### Scenario: Only two kinds exist

- **WHEN** an upload requests a kind other than `dni` or `rtn`
- **THEN** the API responds `422` and no document is stored

### Requirement: An explicit upload replaces the previous one

An upload through the identity document upload endpoints SHALL overwrite the
stored file when the person already has a document of that kind. The API SHALL
NOT keep previous versions. Creating a contract SHALL NOT replace a document
that is already on file.

#### Scenario: Replace an existing DNI

- **WHEN** an admin uploads a new DNI for a person who already has one
- **THEN** the response contains the new document
- **AND** the previous file is no longer reachable
- **AND** the person's RTN is unchanged

#### Scenario: Every contract of the person shows the replacement

- **WHEN** a person's DNI is replaced
- **THEN** the identity documents of each of that person's contracts show the new DNI

#### Scenario: A contract creation does not replace an existing DNI

- **WHEN** a contract is created with a DNI attached and the applicant already
  has a DNI on file
- **THEN** the stored DNI is unchanged
- **AND** the uploaded file is not stored
- **AND** the `already_present` array of the response names `dni`

### Requirement: The creation response reports the kinds it did not store

The API SHALL answer a successful contract creation with `201` and an
`already_present` array that names every document kind the request carried and
the API did not store because the applicant already had that kind on file. The
array SHALL be present and empty when every uploaded document filled an empty
slot. A non-empty `already_present` array SHALL NOT turn the response into an
error.

#### Scenario: Every uploaded document was stored

- **WHEN** a contract is created and every uploaded document filled an empty
  slot
- **THEN** the API responds `201`
- **AND** the `already_present` array is empty

#### Scenario: An uploaded document was not stored

- **WHEN** a contract is created and an uploaded document was not stored
  because the applicant already had that kind on file
- **THEN** the API responds `201`
- **AND** the `already_present` array names that kind

#### Scenario: The response is a success

- **WHEN** the `already_present` array is not empty
- **THEN** the response carries the created contract
- **AND** is not an error

### Requirement: Accepted formats and size limit

The API SHALL accept `application/pdf`, `image/jpeg`, `image/jpg`, `image/png`,
`image/webp`, `image/heic` and `image/heif` for identity documents, and SHALL
reject any file larger than 10 MB.

#### Scenario: Accept a supported document

- **WHEN** an admin uploads a PDF, JPEG, PNG, WebP, HEIC or HEIF file
- **THEN** the API responds with the stored document

#### Scenario: Reject an oversized document

- **WHEN** an uploaded file is larger than 10 MB
- **THEN** the API responds `422`
- **AND** the error names the file and states the maximum size

### Requirement: An invalid upload is rejected instead of discarded

An upload whose type is not allowed SHALL be rejected with `422` that names the
offending file. The API SHALL NOT silently skip it and SHALL NOT create the
resource the upload belongs to.

#### Scenario: A contract carries a file of a disallowed type

- **WHEN** a contract is created with an attachment whose type is not allowed
- **THEN** the API responds `422`
- **AND** the error names the offending file
- **AND** no contract is created

#### Scenario: A contract carries an oversized attachment

- **WHEN** a contract is created with an attachment larger than 10 MB
- **THEN** the API responds `422`
- **AND** the error names the offending file
- **AND** no contract is created

#### Scenario: The invalid file is reported before the missing documents

- **WHEN** a contract is created with both an invalid file and a missing DNI
- **THEN** the API responds `422` describing the invalid file

### Requirement: Contract creation requires a DNI and an RTN

The API SHALL reject the creation of a contract whose applicant has no DNI or
no RTN on file, and SHALL say which kinds are missing. A document that the
applicant already has on file SHALL count as satisfied.

#### Scenario: The applicant has no identity documents

- **WHEN** a contract is created for an applicant with neither a DNI nor an RTN
- **THEN** the API responds `422`
- **AND** the response names both `dni` and `rtn` as missing
- **AND** no contract is created

#### Scenario: The applicant has only a DNI

- **WHEN** a contract is created for an applicant that has a DNI but no RTN
- **THEN** the API responds `422` naming `rtn` as missing

#### Scenario: The applicant already has both documents

- **WHEN** a contract is created for an applicant that already has a DNI and an
  RTN on file
- **THEN** the API responds `201`
- **AND** the contract links to both documents

#### Scenario: The documents are uploaded with the contract

- **WHEN** a contract is created with a DNI and an RTN attached and the
  applicant has neither on file
- **THEN** the API responds `201`
- **AND** both documents are stored for the applicant
- **AND** the contract links to both documents

#### Scenario: The contract carries a document the applicant already has

- **WHEN** a contract is created with a DNI and an RTN attached and the
  applicant already has one of them
- **THEN** the API responds `201`
- **AND** the document already on file is kept
- **AND** the `already_present` array of the response names that kind
- **AND** the contract links to both documents

#### Scenario: The requirement applies to the customer reservation

- **WHEN** a customer creates a reservation without a DNI or an RTN
- **THEN** the API responds `422` naming the missing kinds

### Requirement: Contract creation accepts typed identity document fields

The API SHALL read the DNI from the `identity_document` multipart field and the
RTN from the `rtn_document` multipart field, and SHALL use them to fill the
slots the applicant does not have yet. The anonymous `documents[N]` fields
SHALL keep working as free-form attachments.

#### Scenario: A typed DNI fills an empty slot

- **WHEN** a contract is created with an `identity_document` file and the
  applicant has no DNI on file
- **THEN** the file is stored as the applicant's DNI

#### Scenario: A typed DNI on a slot that is already covered

- **WHEN** a contract is created with an `identity_document` file and the
  applicant already has a DNI on file
- **THEN** the stored DNI is unchanged
- **AND** the uploaded file is not stored
- **AND** the `already_present` array of the response names `dni`

#### Scenario: A typed RTN fills an empty slot

- **WHEN** a contract is created with an `rtn_document` file and the applicant
  has no RTN on file
- **THEN** the file is stored as the applicant's RTN

#### Scenario: A typed RTN on a slot that is already covered

- **WHEN** a contract is created with an `rtn_document` file and the applicant
  already has an RTN on file
- **THEN** the stored RTN is unchanged
- **AND** the uploaded file is not stored
- **AND** the `already_present` array of the response names `rtn`

### Requirement: Contract identity documents are listed per contract

The API SHALL expose the identity documents of the applicant for a given
contract, together with the read-only entries migrated from the files that were
stored under the contract before this change.

#### Scenario: List the documents of a contract

- **WHEN** an authorized caller lists the identity documents of a contract
- **THEN** the response contains the applicant's `dni` and `rtn` slots
- **AND** each populated slot carries the original filename, the MIME type, the
  size in bytes, the upload timestamp, a preview URL and a download URL

#### Scenario: Legacy entries are listed with the contract

- **WHEN** an authorized caller lists the identity documents of a contract that
  has migrated legacy entries
- **THEN** the response also contains those entries
- **AND** each legacy entry is marked as read-only and without a kind

#### Scenario: A new contract's attachments become legacy entries immediately

- **WHEN** a contract is created with `documents[N]` attachments
- **THEN** the API creates one read-only legacy entry per stored attachment at
  creation time, pointing at the path the upload wrote
- **AND** the entry carries the file name and the size read from disk
- **AND** no file is copied or moved

#### Scenario: The contract owner is not a caller

- **WHEN** the owner of the contract lists its identity documents
- **THEN** the API responds `403`

### Requirement: Sellers only reach their own contracts

A seller SHALL only list, upload and download the identity documents of the
contracts it created or that are assigned to it and accepted, using the same
scope as the contract listing.

#### Scenario: Seller reaches a contract in scope

- **WHEN** a seller lists the identity documents of a contract it created or
  that is assigned to it and accepted
- **THEN** the API responds `200` with the documents

#### Scenario: Seller reaches a contract out of scope

- **WHEN** a seller lists the identity documents of a contract it neither
  created nor has accepted
- **THEN** the API responds `403`

#### Scenario: Seller downloads a document out of scope

- **WHEN** a seller downloads the content of a document that belongs to a
  contract out of its scope
- **THEN** the API responds `404`
- **AND** serves no bytes

### Requirement: Person-level identity documents are admin only, or within a seller's scope

The API SHALL expose the identity documents of a person to admins. A seller
SHALL reach them only when it has a contract with that person in its scope, and
SHALL NOT reach them through the person-scoped upload route.

#### Scenario: Admin lists a person's documents

- **WHEN** an admin lists the identity documents of a person
- **THEN** the API responds `200` with the two slots

#### Scenario: Seller lists a person it serves

- **WHEN** a seller lists the identity documents of a person it has a contract
  with in scope
- **THEN** the API responds `200` with the two slots

#### Scenario: Seller lists a person it does not serve

- **WHEN** a seller lists the identity documents of a person it has no contract
  with in scope
- **THEN** the API responds `403`

#### Scenario: Admin uploads a person's document

- **WHEN** an admin uploads a DNI or an RTN for a person
- **THEN** the API responds with the stored document

#### Scenario: Seller uploads a person's document

- **WHEN** a seller uploads a DNI or an RTN through the person-scoped route
- **THEN** the API responds `403`

### Requirement: Identity document content is served through authorized routes

The API SHALL serve the stored original and an inline preview of an identity
document only to authorized callers, and SHALL name the format in the response
so the browser can render it.

#### Scenario: Download the original

- **WHEN** an authorized caller requests the content of an identity document
- **THEN** the API responds `200` with the stored bytes and the stored MIME type
- **AND** the response is sent as an attachment when a download is requested

#### Scenario: View the preview inline

- **WHEN** an authorized caller requests the preview of an identity document
- **THEN** the API responds `200` with an inline `image/webp` preview

#### Scenario: A document without a raster preview

- **WHEN** an authorized caller requests the preview of a PDF or HEIC document
- **THEN** the API responds `200` with the original served inline
- **AND** the response is not labeled as a WebP preview

#### Scenario: An anonymous caller

- **WHEN** a caller without a session requests the content
- **THEN** the API responds `401`

#### Scenario: An out-of-scope caller

- **WHEN** a caller who is not an admin and not a seller in scope requests the
  content or the preview of a document
- **THEN** the API responds `404`
- **AND** serves no bytes

#### Scenario: The contract owner is not a caller

- **WHEN** the owner of the contract requests the content or the preview of an
  identity document of that contract
- **THEN** the API responds `404`
- **AND** serves no bytes

### Requirement: Identity documents are not publicly served

The stored identity document files SHALL NOT be reachable through the public
static file routes. They SHALL only be reachable through the authorized content
endpoints.

#### Scenario: The public upload route does not expose identity documents

- **WHEN** a caller guesses the path of a stored identity document under the
  public upload or receipt routes
- **THEN** the API does not serve the file

### Requirement: Legacy contract documents are migrated read-only

The API SHALL migrate the files that were already stored for a contract into
read-only entries that appear only in that contract's tab. They SHALL NOT be
promoted into the person's `dni` or `rtn` slot, and the API SHALL NOT offer any
action that promotes them.

#### Scenario: Legacy files are migrated

- **WHEN** the migration runs on a contract that has stored files
- **THEN** an entry is created for each stored file
- **AND** each entry is marked as read-only and without a kind

#### Scenario: The migration skips when there is nothing to migrate

- **WHEN** the migration runs and no contract has stored files
- **THEN** it reports that there was nothing to migrate
- **AND** it creates no entries

#### Scenario: Legacy files are not promoted

- **WHEN** an admin views a contract with migrated legacy entries
- **THEN** no action is offered to turn a legacy entry into the person's DNI or
  RTN
- **AND** the person's slots still report whatever documents were uploaded
  through the identity document routes
