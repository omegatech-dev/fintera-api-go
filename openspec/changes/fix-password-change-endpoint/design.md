## Context

`AuthFirstLoginPage` in FinteraWeb calls
`POST /auth/change_password` with `{ current_password, new_password }`. The API
registers only `PATCH /users/:user_id/change_password`, so the request returns
404 before reaching any handler.

See `proposal.md` and
`openspec/changes/fix-password-change-endpoint/specs/customer-password-change/spec.md`.

## Goals / Non-Goals

**Goals:**

- Make the first-login password change work without a Web release.
- Keep the portal API under `/auth/*` and avoid requiring the user ID.
- Preserve the admin/seller password-reset route.

**Non-Goals:**

- Changing password policies.
- New email flows.
- Reworking the recovery-code flow.

## Decisions

### Add a new route instead of changing the Web call

FinteraWeb already ships `POST /auth/change_password`; adding the route fixes
existing deployments immediately. A `/users/:id` call would also require the
client to know its user ID and would drift from the rest of the `/auth/*` surface.

### Reuse `UserService.ChangePassword`

The service already verifies the current password, hashes the new one, clears
`must_change_password` and writes an audit entry. The new handler only resolves
the user from the JWT and maps errors.

## Risks / Trade-offs

- **Two password endpoints**: intentional — one is self-service (`/auth`), the
  other supports admins editing a specific user.
- **Wrong current password returns 422**: consistent with the existing endpoint.

## Migration Plan

Deploy the API; no data or Web changes.
