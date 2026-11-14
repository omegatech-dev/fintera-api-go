## ADDED Requirements

### Requirement: Public referral link resolution

The system SHALL expose a public `GET /r/:guid` that resolves the seller by their
opaque `referral_guid` and records a visit with a 30-day attribution. The most
recent visit SHALL win. A non-redirecting `POST /public/referrals/:guid/visit`
SHALL record the visit and return 204.

#### Scenario: Resolve a valid GUID

- **WHEN** a visitor opens `/r/{guid}` for an active seller
- **THEN** the seller is resolved
- **AND** a referral visit is recorded
- **AND** the visitor is redirected to the portal with the attribution

#### Scenario: Record a visit from the portal

- **WHEN** the customer portal posts to `/public/referrals/{guid}/visit`
- **THEN** the visit is recorded
- **AND** the response is 204 without a redirect

#### Scenario: Unknown GUID

- **WHEN** the GUID does not match an active seller
- **THEN** the response is a 404 without recording a visit

#### Scenario: Last interaction wins

- **WHEN** a visitor opens two seller links within the attribution window
- **THEN** the second visit overwrites the first attribution

### Requirement: Referral attribution on reservation

A reservation created with an active attribution SHALL store the seller, use
`origin = referral` and be accepted immediately. The attribution SHALL be
cleared after the reservation is created.

#### Scenario: Reservation inside the window

- **WHEN** a reservation is created with an active attribution
- **THEN** the contract stores the seller and `origin = referral`
- **AND** the assignment status is `accepted`
- **AND** the attribution is cleared

#### Scenario: Reservation after expiry

- **WHEN** the attribution is older than 30 days
- **THEN** no seller is assigned from the link
- **AND** the normal assignment path runs
