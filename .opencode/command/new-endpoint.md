---
description: Scaffold a new API endpoint across handler, service, and repository layers.
agent: build
---

Add an endpoint for: $ARGUMENTS

Follow the existing architecture and conventions:

1. Read a comparable handler/service/repository trio before editing.
2. Add a handler method in `internal/handlers/` with Swagger annotations,
   using `middleware.GetUserID` / `GetUserRole` as needed.
3. Add a service method in `internal/services/` for validation and business
   logic. Use the FSM for contract/payment state changes, and write audit logs
   for admin state changes.
4. Add a repository interface method plus implementation in
   `internal/repository/` only if new persistence is required.
5. Register the route in `cmd/api/main.go` under the correct middleware group
   (`RequireAdmin`, seller/admin, owner, or authenticated).
6. Return wrapped JSON: `{"<resource>": ...}` or `{"error": "..."}`. Use
   `ToResponse()` for output.
7. Add or update tests.
8. Run `make swagger` if Swagger annotations changed.
9. Update `docs/architecture/api-and-auth.md`.

Do not commit. Report changed files and any follow-up work.
