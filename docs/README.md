# Fintera API Documentation

This directory is the entry point for project documentation. It describes the
Go API as it exists in code today, not the historical Rails implementation.

## Start here

| Audience | Read |
| --- | --- |
| New developer | [guides/development.md](guides/development.md) |
| Agent / automation | [`../AGENTS.md`](../AGENTS.md) |
| Architecture overview | [architecture/overview.md](architecture/overview.md) |
| Data model | [architecture/data-model.md](architecture/data-model.md) |
| Business flows | [architecture/domain-flows.md](architecture/domain-flows.md) |
| API and authorization | [architecture/api-and-auth.md](architecture/api-and-auth.md) |
| Testing | [guides/testing.md](guides/testing.md) |
| Email setup (Resend) | [guides/email.md](guides/email.md) |
| Deployment and operations | [guides/operations.md](guides/operations.md) |
| Conventions and PR checklist | [guides/contributing.md](guides/contributing.md) |

## Documentation map

```
docs/
├── README.md                       # this index
├── architecture/
│   ├── overview.md                 # stack, layering, bootstrap, package map
│   ├── data-model.md               # tables, enums, relationships, ledger
│   ├── domain-flows.md             # contract/payment lifecycles, jobs, reports
│   └── api-and-auth.md             # routes, RBAC, auth, response conventions
├── guides/
│   ├── development.md              # local setup and commands
│   ├── email.md                    # Resend setup, test inbox and production
│   ├── testing.md                  # tests and CI
│   ├── operations.md               # deploy, runtime, observability, security
│   └── contributing.md             # conventions and definition of done
├── reviews/
│   └── payment-receipt-backend-review.md
├── archive/
│   └── go_migration_plan.md        # historical Rails → Go plan
├── docs.go                         # generated Swagger package
├── swagger.json                    # generated OpenAPI
└── swagger.yaml                    # generated OpenAPI
```

## Canonical sources of truth

| Topic | Source |
| --- | --- |
| Routes and middleware order | `cmd/api/main.go` |
| Configuration keys and defaults | `internal/config/config.go` |
| Database schema | `internal/database/migrations/*.up.sql` |
| Models and JSON shapes | `internal/models/*.go` |
| Business rules | `internal/services/*.go` |
| State transitions | `internal/statemachine/*.go` |
| Background scheduling | `cmd/api/main.go` (`scheduleJobs`) |
| HTTP API reference | `docs/swagger.yaml` |

## Generated Swagger

`docs/` is the **only** canonical Swagger location. Regenerate it with:

```bash
make swagger
```

The previous duplicates under `api/openapi/` and `openapi/` were stale and have
been removed. Do not recreate them.

## Documentation rules

- Keep documentation aligned with code. Update the relevant file in the same
  change that alters routes, statuses, config, schema, or scheduled jobs.
- Prefer linking to source files instead of copying large code blocks.
- Mark historical material clearly and keep it under `archive/` or `reviews/`.
- Do not commit secrets, `.env` files, or generated binaries. See
  [guides/operations.md](guides/operations.md#security).
