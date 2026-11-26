# Client cancellation handling

## Purpose

When a client disconnects before the server answers, the request is treated as an
abandoned interaction rather than a failure: it ends with status 499, is never
reported as a missing resource, and never appears as a SQL error in the logs.

## ADDED Requirements

### Requirement: Aborted requests are answered with 499

The public catalog endpoints SHALL respond to a request whose client context was
canceled with status `499` (Client Closed Request) and no response body. They
SHALL NOT respond 500, and SHALL NOT report a resource as missing, when the only
reason a lookup did not complete is that the caller disconnected.

#### Scenario: Client disconnects while the project list is being read

- **WHEN** a client requests the public project list and disconnects before the
  query completes
- **THEN** the request ends with status 499 and no body
- **AND** the request log records the route, method, duration, and 499

#### Scenario: Real database failure still reports a server error

- **WHEN** the same lookup fails for a reason other than client cancellation
- **THEN** the endpoint responds as it did before, with its existing 500 and a
  Spanish error message

#### Scenario: Timeout is not treated as a cancellation

- **WHEN** a query exceeds its deadline instead of being canceled by the caller
- **THEN** the request is handled as an error, not as 499

### Requirement: An aborted lookup is not a missing resource

A failed lookup caused by client cancellation SHALL reach the handler carrying the
cancellation, so the response reflects the disconnect. Genuine misses SHALL keep
returning 404 with the existing Spanish message, and other lookup errors that are
not cancellations SHALL continue to be reported as not found.

#### Scenario: Project lookup interrupted by a disconnect

- **WHEN** a published project is requested and the client disconnects during the
  lookup
- **THEN** the request ends with 499 rather than 404
- **AND** the project remains retrievable by a later request for the same GUID

#### Scenario: Unknown project GUID

- **WHEN** a client requests a project GUID that is not published or does not exist
- **THEN** the endpoint responds 404 with `Proyecto no encontrado`

### Requirement: Canceled queries are not SQL errors

The SQL logger SHALL record a statement that stopped because the caller canceled
at debug level, labelled as a client cancellation, and SHALL NOT emit an `SQL
Error` entry for it. Statements failing for any other reason SHALL continue to log
at error level with the parameterized query, duration, and rows.

#### Scenario: Aborted browser request during page load

- **WHEN** a query fails with a canceled context
- **THEN** the log contains a debug `SQL client canceled` entry with the query,
  duration, and rows
- **AND** no `SQL Error` entry is emitted for it

#### Scenario: Genuine query failure

- **WHEN** a statement fails for a database reason unrelated to cancellation
- **THEN** an `SQL Error` entry is logged at error level with the query and error
