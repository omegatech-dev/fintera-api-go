# Fintera API - Go Backend

Go implementation of the Fintera API, a real-estate contract management backend
migrated from Ruby on Rails.

- **Stack:** Go 1.25, Gin, GORM, PostgreSQL, JWT, `looplab/fsm`
- **Docs:** [docs/README.md](docs/README.md)
- **Agent guidance:** [AGENTS.md](AGENTS.md)

## Quick start

```bash
cp .env.example .env
# Edit DATABASE_URL, JWT_SECRET, and any email/storage settings

make deps
make migrate-up
make seed        # optional sample data
make dev         # hot reload; requires air (installed on demand)
```

Useful URLs (assuming `PORT=8081` from `.env.example`):

- Health: `http://localhost:8081/api/v1/health`
- Swagger UI: `http://localhost:8081/swagger/index.html`

## Documentation

| Topic | Document |
| --- | --- |
| Documentation index | [docs/README.md](docs/README.md) |
| Architecture overview | [docs/architecture/overview.md](docs/architecture/overview.md) |
| Data model | [docs/architecture/data-model.md](docs/architecture/data-model.md) |
| Contract/payment flows and jobs | [docs/architecture/domain-flows.md](docs/architecture/domain-flows.md) |
| Routes and authorization | [docs/architecture/api-and-auth.md](docs/architecture/api-and-auth.md) |
| Development setup | [docs/guides/development.md](docs/guides/development.md) |
| Testing | [docs/guides/testing.md](docs/guides/testing.md) |
| Operations and security | [docs/guides/operations.md](docs/guides/operations.md) |
| Contributing | [docs/guides/contributing.md](docs/guides/contributing.md) |

## Project structure

```
.
├── cmd/api/                 # main: config, wiring, router, jobs, shutdown
├── cmd/test_email/          # manual email smoke test
├── internal/
│   ├── config/              # environment configuration
│   ├── database/            # connection, migrations, seeds
│   ├── models/              # GORM models, constants, response DTOs
│   ├── repository/          # data access interfaces and implementations
│   ├── services/            # business logic, reports, email, analytics
│   ├── handlers/            # Gin HTTP handlers
│   ├── middleware/          # JWT auth, RBAC, CORS, request logging
│   ├── jobs/                # in-process worker pool and scheduler
│   ├── statemachine/        # contract and payment FSMs
│   ├── storage/             # local file storage and path safety
│   ├── mailers/             # placeholder package
│   ├── reports/             # placeholder package
│   └── utils/               # placeholder package
├── pkg/logger/              # slog setup and GORM logger adapter
├── docs/                    # canonical Swagger and documentation
├── scripts/                 # ad-hoc SQL maintenance scripts
├── storage/                 # runtime uploads (contracts, receipts, uploads)
├── Makefile
├── Dockerfile
└── README.md
```

## Available commands

```bash
make deps          # download and tidy dependencies
make dev           # hot reload with air
make build         # build bin/api
make run           # build and run
make test          # run all tests
make test-cover    # coverage report
make fmt           # go fmt
make lint          # golangci-lint
make security      # gosec
make migrate-up    # apply migrations
make migrate-down  # roll back one migration
make migrate-create
make seed          # run SQL seeds
make swagger       # regenerate docs/ Swagger
make docker-build
make docker-run
```

## Key features

- JWT authentication with refresh tokens and role-based authorization
- Contract and payment state machines with a double-entry-style ledger
- Payment schedule generation, overdue interest, and automatic contract closure
- In-process background worker for reminders, interest, credit scores, and cache
- Local file storage for receipts, contract documents, and profile pictures
- CSV/PDF reports and analytics exports
- Email notifications through Resend and error tracking through Rollbar

## API

All endpoints are under `/api/v1`. See
[docs/architecture/api-and-auth.md](docs/architecture/api-and-auth.md) for the
full route and RBAC reference, or run the service and open Swagger UI.

## Configuration

See `.env.example` for all supported variables and
[docs/guides/development.md](docs/guides/development.md#environment-variables)
for defaults and requirements. `.env` is loaded automatically; `.production.env`
is not.
