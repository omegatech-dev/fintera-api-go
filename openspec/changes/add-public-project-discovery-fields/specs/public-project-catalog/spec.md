## Purpose

Expose the unpublished-authentication project catalog used by the customer
portal, with the discovery fields required by the approved directory design and
without leaking administrative data.

## ADDED Requirements

### Requirement: Public project discovery payload

`GET /api/v1/public/projects` and `GET /api/v1/public/projects/:guid` SHALL
return published, active projects with, per project:

- `guid`, `name`, `description`, `project_type`, `address`, `city`, `code`
- `delivery_date`, `created_at`
- `price_per_square_unit`, `measurement_unit`
- `price_from`, `price_to`
- `drawing_count`
- `stats` and `photos` / `photo_count`

`city`, `code`, `price_from` and `price_to` SHALL be `null` when the value is
unknown.

#### Scenario: Published project with complete discovery data

- **WHEN** a visitor requests the public project list
- **THEN** every published, active project includes its city, code, description,
  creation date, price range, drawing count and availability stats

#### Scenario: Legacy project without city or code

- **WHEN** a published project has no `city` or `code`
- **THEN** those fields are `null` and the rest of the payload is unchanged

#### Scenario: Internal fields are never exposed

- **WHEN** a visitor requests the public project payload
- **THEN** the response does not contain `interest_rate`, commission rates,
  internal notes or drawing uploader identifiers

### Requirement: Effective price range

The API SHALL compute `price_from` and `price_to` from the `effective_price`
of the project's non-inactive lots (override price when present, otherwise the
stored lot price), ignoring lots with a non-positive price.

#### Scenario: Project with lots

- **WHEN** a published project has lots priced between L 90,000 and L 250,000
- **THEN** `price_from` is 90000 and `price_to` is 250000

#### Scenario: Project without eligible lots

- **WHEN** a published project has no non-inactive lots with a positive price
- **THEN** `price_from` and `price_to` are `null`

### Requirement: Drawing count

The API SHALL expose `drawing_count` with the number of project drawings
published for the project.

#### Scenario: Project with plans

- **WHEN** a published project has three project drawings
- **THEN** `drawing_count` is 3

#### Scenario: Project without plans

- **WHEN** a published project has no drawings
- **THEN** `drawing_count` is 0

### Requirement: Financed and paid availability split

`stats` SHALL include `total_lots`, `available_lots`, `reserved_lots`,
`financed_lots`, `paid_lots` and `sold_lots`, where `sold_lots` equals
`financed_lots + paid_lots`.

#### Scenario: Mixed project

- **WHEN** a project has available, reserved, financed and fully paid lots
- **THEN** each count is reported separately and `sold_lots` is the sum of the
  financed and paid counts

### Requirement: Admin-managed city and code

The admin create and update project endpoints SHALL accept and persist `city`
and `code`, and return them in the project response.

#### Scenario: Admin sets city and code

- **WHEN** an admin creates or updates a project with `city` and `code`
- **THEN** both values are persisted and returned by the admin project response

#### Scenario: Optional values

- **WHEN** an admin omits `city` or `code`
- **THEN** the project is saved successfully with `null` for the omitted field
