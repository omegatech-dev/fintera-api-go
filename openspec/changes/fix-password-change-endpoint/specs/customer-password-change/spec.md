## Purpose

Let an authenticated customer set a new password after receiving the temporary
one by email.

## ADDED Requirements

### Requirement: Self-service password change

`POST /api/v1/auth/change_password` SHALL change the authenticated user's
password using `current_password` and `new_password`, and SHALL clear
`must_change_password`.

#### Scenario: Valid change

- **WHEN** an authenticated user posts the correct current password and a new
  password of at least 6 characters
- **THEN** the password is updated, `must_change_password` becomes false and the
  API responds `200`

#### Scenario: Wrong current password

- **WHEN** the current password does not match
- **THEN** the API responds `422` and the password is unchanged

#### Scenario: Missing fields

- **WHEN** `current_password` is missing or `new_password` is shorter than 6
  characters
- **THEN** the API responds `400`

#### Scenario: No session

- **WHEN** the request has no valid token
- **THEN** the API responds `401`

### Requirement: Existing admin route kept

`PATCH /api/v1/users/{user_id}/change_password` SHALL keep working for the admin
and seller UI.

#### Scenario: Admin changes another user

- **WHEN** an admin posts a new password for another user
- **THEN** the password is reset without requiring the old password
