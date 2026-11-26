## ADDED Requirements

### Requirement: Contract reassignment is atomic with commission compensation

The system SHALL perform a contract reassignment as one database transaction
that locks and re-reads the contract before deciding how to assign it. The
transaction SHALL include the contract assignment fields, assignment history,
superseding prior pending assignments, and any required commission-credit
application reversal, credit restoration, and cash-recovery credit creation.
If any write fails, all changes in the reassignment transaction SHALL roll
back. Automatic seller selection and commission-compensation decisions SHALL
use the locked contract state.

#### Scenario: Successful reassignment of a netted commission

- **WHEN** an admin reassigns a contract whose commission consumed seller credit
- **THEN** the old applications are reversed, their credit balances are restored, any cash recovery credit is created, and the new seller and assignment history are committed together

#### Scenario: Reassignment with a partially paid commission

- **WHEN** a paid commission used both credit and cash
- **THEN** the original credit portion is restored and a credit for the cash portion is created in the same transaction as the reassignment

#### Scenario: Reassignment with no paid commission

- **WHEN** the contract commission has not been paid
- **THEN** the reassignment and assignment history commit atomically without creating or changing commission credits

#### Scenario: Assignment write fails

- **WHEN** superseding an assignment, creating or updating the new assignment, compensation, or saving the contract fails
- **THEN** the transaction rolls back and the seller, assignment history, credit balances, applications, and payment markers remain unchanged

#### Scenario: Automatic seller selection uses fresh contract state

- **WHEN** an automatic reassignment begins after the contract seller changed since the initial read
- **THEN** the current seller is read under the contract lock and excluded from the automatic selection according to the reassignment rules

### Requirement: Assignment acceptance and reassignment share a lock protocol

Assignment acceptance SHALL lock the contract row before locking or changing the
assignment row. It SHALL re-read the contract and assignment under those locks,
then update the accepted assignment and contract assignment fields in the same
transaction. Reassignment SHALL use the same contract-first ordering so the two
operations serialize without overwriting each other's contract assignment
state.

#### Scenario: Acceptance wins the contract lock

- **WHEN** acceptance obtains the contract lock before a concurrent reassignment
- **THEN** acceptance commits its assignment and contract fields first, and reassignment proceeds using that committed state

#### Scenario: Reassignment wins the contract lock

- **WHEN** reassignment obtains the contract lock before a concurrent acceptance
- **THEN** reassignment supersedes the pending offer and commits the new assignment, and the stale acceptance is rejected without changing the contract

#### Scenario: Assignment is no longer pending

- **WHEN** acceptance re-reads an assignment that has already been accepted, expired, rejected, or superseded
- **THEN** it returns the existing idempotent result for already accepted assignments or `ErrAssignmentNotPending` for other terminal states, without changing contract assignment fields

### Requirement: Assignment-side audit and notification writes follow transaction outcome

The system SHALL persist audit and notification records created by reassignment
or assignment acceptance in the same database transaction as their state
changes. No notification or audit record for an uncommitted assignment change
SHALL remain if the transaction rolls back. Reassignment-level audit entries
that are intentionally written after commit SHALL only be emitted after
successful commit.

#### Scenario: Assignment operation rolls back

- **WHEN** an assignment operation fails after producing audit or notification records
- **THEN** those records are rolled back with the assignment and contract writes

#### Scenario: Assignment operation commits

- **WHEN** an assignment operation commits successfully
- **THEN** its corresponding audit and notification records are persisted

### Requirement: Unconverted assignment flows remain explicitly out of scope

Reject, expiration, cancellation, and automated backfill flows SHALL retain
their existing behavior in this change and SHALL be documented as follow-up
work for the shared transaction protocol.

#### Scenario: Rejected or expired offer

- **WHEN** a seller rejects an offer or the offer expires
- **THEN** the existing rejection/expiration flow remains in effect and is not implicitly routed through the new reassignment transaction

#### Scenario: Contract cancellation

- **WHEN** an admin cancels a contract
- **THEN** the existing cancellation flow remains in effect; this change does not claim atomicity between cancellation, ledger writes, and commission-credit compensation
