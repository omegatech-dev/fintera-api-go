## ADDED Requirements

### Requirement: Assignment lifecycle

The system SHALL create an assignment with a status and origin when a
reservation is created. Manual and automatic assignments SHALL start `pending`
with an acceptance deadline; referral assignments SHALL start `accepted`.

#### Scenario: Manual assignment

- **WHEN** a reservation is created with a chosen seller
- **THEN** the origin is `manual`
- **AND** the status is `pending`
- **AND** `accept_deadline_at` is set to the configured window

#### Scenario: Automatic assignment

- **WHEN** a reservation is created without a seller or with "nobody attended"
- **THEN** the origin is `automatic`
- **AND** an eligible seller is selected and the status is `pending`

#### Scenario: Referral assignment

- **WHEN** a reservation carries a referral attribution
- **THEN** the origin is `referral`
- **AND** the status is `accepted`

### Requirement: Accept and reject

The system SHALL let the assigned seller accept or reject a pending assignment.
Accepting SHALL commit the commission to that seller. Rejecting SHALL expire
the assignment. Both operations SHALL be idempotent and SHALL reject answered or
expired assignments.

#### Scenario: Seller accepts

- **WHEN** the seller accepts before the deadline
- **THEN** the status becomes `accepted`
- **AND** the commission is committed to that seller

#### Scenario: Seller rejects

- **WHEN** the seller rejects a pending assignment
- **THEN** the status becomes `expired`

#### Scenario: Answered assignment

- **WHEN** the seller acts on an already answered or expired assignment
- **THEN** the request is rejected without changing state

### Requirement: Expiry and reassignment

The system SHALL expire overdue pending assignments and reassign the reservation
to the next eligible seller until one accepts or the pool is exhausted.

#### Scenario: Deadline passes

- **WHEN** a pending assignment passes its deadline
- **THEN** it becomes `expired`
- **AND** the seller's score window is penalised
- **AND** the reservation is offered to the next eligible seller

#### Scenario: Pool exhausted

- **WHEN** no eligible seller remains
- **THEN** the reservation continues without a seller
- **AND** its status is not cancelled

### Requirement: Selection fairness

The system SHALL prefer green sellers, then yellow, and SHALL never assign to a
red seller. Within a level it SHALL prefer the lowest pending load and rotate so
the same seller is not always selected.

#### Scenario: Green before yellow

- **WHEN** green and yellow candidates exist
- **THEN** a green candidate is selected

#### Scenario: Red excluded

- **WHEN** only red candidates exist
- **THEN** none is assigned

#### Scenario: Rotation

- **WHEN** equal candidates receive successive assignments
- **THEN** the choice rotates between them
