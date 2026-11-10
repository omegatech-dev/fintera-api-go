## Purpose

Let administrators control whether a project is visible in the customer portal
and identify the gallery photo that represents the project on admin cards.

## ADDED Requirements

### Requirement: Publish a project

An admin SHALL be able to publish a project without changing its active state.

#### Scenario: Publish an unpublished project

- **WHEN** an admin sends `POST /api/v1/projects/:id/publish`
- **THEN** the project's `is_published` becomes `true`
- **AND** `is_active` keeps its previous value
- **AND** the response contains the updated project
- **AND** an audit entry records the change

#### Scenario: Publish an already published project

- **WHEN** an admin publishes a project that is already published
- **THEN** the request succeeds and the project stays published

#### Scenario: Non-admin cannot publish

- **WHEN** a non-admin calls the publish endpoint
- **THEN** the request is rejected by the admin authorization middleware

### Requirement: Unpublish a project

An admin SHALL be able to unpublish a project without changing its active state.

#### Scenario: Unpublish a published project

- **WHEN** an admin sends `POST /api/v1/projects/:id/unpublish`
- **THEN** the project's `is_published` becomes `false`
- **AND** `is_active` keeps its previous value
- **AND** an audit entry records the change

#### Scenario: Unknown project

- **WHEN** an admin unpublishes a project that does not exist
- **THEN** the API responds with `404`

### Requirement: Admin project responses expose a cover photo

Admin project responses SHALL include `cover_photo_url`, derived from the first
gallery photo in position order.

#### Scenario: Project has gallery photos

- **WHEN** an admin lists or reads a project that has photos
- **THEN** `cover_photo_url` is the public URL of the first photo by position

#### Scenario: Project has no photos

- **WHEN** an admin lists or reads a project without photos
- **THEN** `cover_photo_url` is `null`
