# Local log capture

## Purpose

Give a local run a durable, greppable copy of its structured logs without
changing what the terminal shows or what deployed environments send to the log
collector.

## ADDED Requirements

### Requirement: LOG_FILE duplicates every record to a file

When the `LOG_FILE` variable names a usable path, the application SHALL write
every log record to that file in addition to stdout, keeping the configured
format (`JSON` by default, `text` when `LOG_FORMAT=text`) and level
(`LOG_LEVEL`) for both destinations. The file SHALL be created if missing and
appended to if it exists, so records from earlier runs are preserved across
restarts and hot reloads.

#### Scenario: Debugging a local run with a log file

- **WHEN** the server starts with `LOG_FILE=trace.log`
- **THEN** each record appears on stdout and in `trace.log` with the same fields,
  including the full structured attributes that the terminal truncates
- **AND** the file can be filtered by level or message after the run

#### Scenario: Server restarts while keeping history

- **WHEN** the process restarts with the same `LOG_FILE` path
- **THEN** new records are appended after the existing ones instead of
  overwriting them

#### Scenario: Variable unset

- **WHEN** `LOG_FILE` is unset or empty
- **THEN** records go to stdout only and no file is created

### Requirement: An unusable log path must not stop the application

When `LOG_FILE` names a path that cannot be opened, the application SHALL report
the problem once on stderr and continue logging to stdout. No request or job
SHALL fail because of the log destination.

#### Scenario: Missing parent directory

- **WHEN** the server starts with `LOG_FILE` pointing into a directory that does
  not exist
- **THEN** a single stderr warning names the unusable path
- **AND** logging continues on stdout and the server serves requests

### Requirement: The log file is released on shutdown

The application SHALL close the log file when it shuts down, and re-configuring
the logger SHALL replace the previously opened file rather than leaving it open.

#### Scenario: Graceful shutdown

- **WHEN** the server shuts down after writing to a `LOG_FILE`
- **THEN** the file handle is closed
- **AND** records written before shutdown are present in the file

### Requirement: Development runs can opt in through the build tooling

`make dev` SHALL pass `LOG_FILE` through to the application, create the file's
parent directory when needed, and print the path it is writing to, so a developer
can capture a full log with a single command. Local log files SHALL NOT be tracked
in version control.

#### Scenario: Capturing a reproduction

- **WHEN** a developer runs `LOG_FILE=trace.log make dev`
- **THEN** the target reports that it is writing to that path and the dev server
  starts normally

#### Scenario: Log files stay out of git

- **WHEN** a local run writes `trace.log` or any `*.log` file in the repository
- **THEN** git ignores it
