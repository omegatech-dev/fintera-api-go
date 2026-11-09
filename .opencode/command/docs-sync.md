---
description: Update documentation and Swagger to match the current code changes.
agent: build
---

Synchronize documentation with the working tree changes.

1. Inspect `git status` and `git diff` to identify changed files.
2. Map changes to docs:
   - routes, middleware, RBAC → `docs/architecture/api-and-auth.md`
   - models, migrations, enums → `docs/architecture/data-model.md`
   - services, FSMs, jobs, reports, storage → `docs/architecture/domain-flows.md`
   - config, commands, tooling → `docs/guides/development.md`
   - tests/CI → `docs/guides/testing.md`
   - deploy/runtime/security → `docs/guides/operations.md`
   - cross-cutting structure → `docs/architecture/overview.md`
3. Update `AGENTS.md` only if invariants, commands, or guardrails changed.
4. If handler Swagger annotations changed, run `make swagger` when `swag` is
   available; otherwise report that it must be run.
5. Verify internal markdown links still resolve.
6. Do not commit. Report which docs changed and any unresolved drift.

Context: $ARGUMENTS
