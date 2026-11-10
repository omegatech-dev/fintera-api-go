## Purpose

Give the admin project list enough ordered photo data to render a card
carousel without sending each card a separate gallery request.

## ADDED Requirements

### Requirement: Admin project responses expose an ordered gallery preview

Admin project responses SHALL include a `photos` array ordered by `position`
and then `id`, containing at most the first ten photos.

#### Scenario: Project has gallery photos

- **WHEN** an admin lists or reads a project with photos
- **THEN** `photos` contains the first ten photos in position order
- **AND** each photo has `id`, `position`, `url`, and optional `thumb_url` and `alt`
- **AND** `cover_photo_url` matches the first `photos` URL

#### Scenario: Project has more than ten photos

- **WHEN** a project has more than ten photos
- **THEN** `photos` contains only the first ten

#### Scenario: Project has no photos

- **WHEN** an admin lists or reads a project without photos
- **THEN** `photos` is an empty array
- **AND** `cover_photo_url` is `null`
