## Purpose

Expose the application's global branding (name and logo) so any client can
render it and an administrator can update it without a code change or redeploy.

## ADDED Requirements

### Requirement: Public branding read

The API SHALL expose `GET /api/v1/branding` without authentication and return
`{"branding": {"app_name", "logo_url", "logo_dark_url", "updated_at"}}`.

#### Scenario: Unauthenticated read

- **WHEN** a client requests `GET /api/v1/branding` without a token
- **THEN** the response is `200` with the current branding

### Requirement: Branding defaults

When no branding row exists or a logo field is unset, the API SHALL return the
default application name `Fintera` and null logo URLs.

#### Scenario: Fresh database

- **WHEN** the branding table has no row
- **THEN** `GET /api/v1/branding` returns `app_name` "Fintera" and null
  `logo_url` and `logo_dark_url`

### Requirement: Admin updates the application name

`PUT /api/v1/branding` SHALL be restricted to admins and accept the new name as
`{"branding": {"app_name": "..."}}` (nested or flat).

#### Scenario: Admin renames the app

- **WHEN** an admin sends `PUT /api/v1/branding` with `app_name` "Acme"
- **THEN** the name is persisted and the response contains the updated branding

#### Scenario: Non-admin attempts rename

- **WHEN** a seller or regular user sends `PUT /api/v1/branding`
- **THEN** the response is `403` and nothing changes

#### Scenario: Invalid name

- **WHEN** `app_name` is empty, whitespace only, or longer than 60 characters
- **THEN** the response is `400` with a validation error and nothing changes

### Requirement: Admin uploads a logo

`POST /api/v1/branding/logo` (multipart fields `logo` and `variant` of `light`
or `dark`) SHALL be restricted to admins, store the file under
`uploads/branding/`, and return the updated branding.

#### Scenario: Upload light logo

- **WHEN** an admin uploads a PNG image with `variant=light`
- **THEN** `logo_url` points to the stored file under `/uploads/branding/` and
  `logo_dark_url` is unchanged

#### Scenario: Upload dark logo

- **WHEN** an admin uploads an image with `variant=dark`
- **THEN** `logo_dark_url` points to the stored file and `logo_url` is unchanged

#### Scenario: Rejected upload

- **WHEN** the file is missing, not PNG/JPEG/WebP, or larger than 2 MB
- **THEN** the response is `400`, no file is stored, and the previous branding
  is unchanged

#### Scenario: Replace removes the old file

- **WHEN** a logo variant that already has a file is replaced
- **THEN** the previous file is deleted from storage

### Requirement: Admin removes a logo

`DELETE /api/v1/branding/logo?variant=light|dark` SHALL be restricted to admins,
clear the stored path, and delete the file so clients fall back to defaults.

#### Scenario: Remove light logo

- **WHEN** an admin deletes the light logo
- **THEN** `logo_url` is null and the stored file is removed

#### Scenario: Remove unset logo

- **WHEN** an admin deletes a logo variant that has no file
- **THEN** the response is `200` with that logo URL still null

### Requirement: Branding changes are audited

Every successful branding change SHALL write an audit log entry referencing the
acting admin, the `Branding` entity, and what changed.

#### Scenario: Audit entry on rename

- **WHEN** an admin updates the application name
- **THEN** an audit log entry exists for the admin and the `Branding` entity
