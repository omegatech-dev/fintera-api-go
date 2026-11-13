## ADDED Requirements

### Requirement: Acceptance score computation

The system SHALL compute a seller score as `accepted / (accepted + expired)`
over the last 90 days and SHALL persist the value and level. A seller with no
history SHALL score 100 with level `high`.

#### Scenario: Score from history

- **WHEN** a seller has 12 accepted and 1 expired assignment in 90 days
- **THEN** the score is approximately 92
- **AND** the level is `high`

#### Scenario: No history

- **WHEN** a seller has no assignments
- **THEN** the score is 100
- **AND** the level is `high`

#### Scenario: Nightly recompute

- **WHEN** the nightly job runs
- **THEN** each seller's score and level are refreshed from the 90-day window

### Requirement: Score thresholds

The system SHALL map scores to levels: `high` for 85 or more, `medium` for 60 to
84 and `low` below 60.

#### Scenario: Boundary values

- **WHEN** the score is 85
- **THEN** the level is `high`
- **WHEN** it is 60
- **THEN** the level is `medium`
- **WHEN** it is 59
- **THEN** the level is `low`

### Requirement: Admin score override

An admin SHALL be able to override a seller score with a reason. The override
SHALL be audited and SHALL not be overwritten by the nightly job while active.

#### Scenario: Override is audited

- **WHEN** an admin overrides a score with a reason
- **THEN** the value and reason are stored
- **AND** an audit entry records actor, reason and date

#### Scenario: Nightly job respects override

- **WHEN** the nightly job runs for a seller with an active override
- **THEN** the overridden value is kept
