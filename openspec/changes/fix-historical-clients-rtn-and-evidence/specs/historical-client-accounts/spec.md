## ADDED Requirements

### Requirement: Empty RTN and identity values do not collide

The system SHALL store `NULL` (not the empty string) in `users.rtn` and
`users.identity` when no value is provided, on both user creation and user
update. Uniqueness SHALL be enforced only for non-empty values.

#### Scenario: Two historical clients without RTN are both edited

- **WHEN** a user without RTN is edited twice while other users also have no RTN
- **THEN** the update succeeds and no unique-constraint error is raised

#### Scenario: Creating a client without RTN or identity

- **WHEN** a client is created without an RTN or identity
- **THEN** the stored row has `NULL` in those columns

#### Scenario: Editing a historical sale relinks its client

- **WHEN** a seller edits a historical sale to add evidence and the linked
  historical client has no RTN
- **THEN** the client update succeeds and the sale update completes

#### Scenario: Distinct RTN values remain unique

- **WHEN** a client is created or edited with an RTN that another user already has
- **THEN** the API rejects the operation with a duplicate-value error

### Requirement: Existing empty values are normalized

The database migration SHALL convert existing empty-string `rtn` and `identity`
values to `NULL` before uniqueness is re-applied, and SHALL replace the strict
UNIQUE constraints with partial unique indexes that exclude empty and NULL
values.

#### Scenario: Migration over a table with one empty-string RTN

- **WHEN** the migration runs on a database where some users have `rtn = ''`
- **THEN** those values become `NULL` and the partial unique indexes are created

#### Scenario: Migration is reversible

- **WHEN** the down migration runs
- **THEN** the partial indexes are dropped and the original UNIQUE constraints
  are restored
