## Purpose

Define which plan files the API accepts for a project and what it returns.

## ADDED Requirements

### Requirement: Accepted plan formats

`POST /projects/:project_id/drawings` SHALL accept files with the extensions
`.svg`, `.png`, `.jpg`, `.jpeg` and `.pdf`, up to 50 MB each, and SHALL reject
other extensions with `422`.

#### Scenario: Upload an SVG plan

- **WHEN** an admin uploads a `.svg` file
- **THEN** the drawing is stored with `file_type=svg`

#### Scenario: Upload a PNG or JPG plan

- **WHEN** an admin uploads a `.png`, `.jpg` or `.jpeg` file
- **THEN** the drawing is stored with the matching `file_type`

#### Scenario: Upload a PDF plan

- **WHEN** an admin uploads a `.pdf` file
- **THEN** the drawing is stored with `file_type=pdf`

#### Scenario: Reject CAD files

- **WHEN** an admin uploads a `.dwg` or `.dxf` file
- **THEN** the API responds `422` and stores nothing

#### Scenario: Size limit

- **WHEN** a plan exceeds 50 MB
- **THEN** the API responds `422` with the size error

### Requirement: Plan content types

The API SHALL validate the uploaded content type for drawings and SHALL reject
unsupported types even when the extension is allowed.

#### Scenario: Allowed content type

- **WHEN** a PNG file is uploaded with `Content-Type: image/png`
- **THEN** it is accepted

#### Scenario: Unsupported content type

- **WHEN** a `.svg` upload declares an unrelated binary content type
- **THEN** the API responds `422` and stores nothing

### Requirement: Legacy plan rows

Drawings stored before this change SHALL remain listed and downloadable
regardless of their stored `file_type`.

#### Scenario: Legacy DWG row

- **WHEN** the public drawings endpoint returns a drawing stored as `dwg`
- **THEN** it keeps its `file_type` and `download_url`
