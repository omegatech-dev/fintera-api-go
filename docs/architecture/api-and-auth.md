# API and Authorization

All routes are registered in `cmd/api/main.go`. Base path: `/api/v1`.
Swagger UI is served at `/swagger/index.html`; `/` redirects there.

## Authentication

### Login flow

| Endpoint | Body | Result |
| --- | --- | --- |
| `POST /api/v1/auth/login` | `email`, `password` | JWT + refresh token + user |
| `POST /api/v1/auth/refresh` | `refresh_token` | New JWT + rotated refresh token |
| `POST /api/v1/auth/logout` | `refresh_token` | Deletes the refresh token |

- Passwords are bcrypt-hashed (`AuthService.HashPassword`).
- Inactive or suspended users cannot log in or refresh.
- JWT uses HS256, claims: `user_id`, `email`, `role`, `exp`, `iat`.
- JWT lifetime: `JWT_EXPIRATION_HOURS` (default 24).
- Refresh tokens are random 32-byte hex strings stored in `refresh_tokens`,
  expire after 30 days, and are rotated on each refresh.
- Signing secret: `JWT_SECRET`. In development an insecure default is used;
  production requires it to be set.

### Token sources

`middleware.Auth` accepts:

1. `Authorization: Bearer <token>`
2. `?token=<token>` (used for direct download links)

Middleware stores `userID`, `userEmail`, `userRole`, and `claims` in the Gin
context.

### Public endpoints

| Method | Path |
| --- | --- |
| `GET` | `/api/v1/health` |
| `POST` | `/api/v1/auth/login` |
| `POST` | `/api/v1/auth/refresh` |
| `POST` | `/api/v1/auth/logout` |
| `POST` | `/api/v1/users/send_recovery_code` |
| `POST` | `/api/v1/users/verify_recovery_code` |
| `POST` | `/api/v1/users/update_password_with_code` |

`GET /swagger/*any`, `/`, and `/uploads/*` are also public.

## RBAC

Role middleware lives in `internal/middleware/auth.go`:

- `RequireAdmin()` — `admin` only.
- `RequireRole(...)` — any listed role.
- `RequireAdminSellerOrOwner()` — `admin`, `seller`, or the resource owner.
- `RequireAdminOrOwner()` — `admin` or the resource owner (sellers excluded).

Claimed authorization errors return `403` with `No tienes acceso a esta sección`.

### Role matrix

| Capability | user | seller | admin |
| --- | --- | --- | --- |
| Own profile / password / locale / picture | yes | yes | yes |
| Own contracts and payments | yes | yes | yes |
| Create/update contracts | yes (own flow) | yes | yes |
| Upload/download own receipts | yes | yes | yes |
| List users | no | yes | yes |
| Create users | no | yes | yes |
| View all contracts/payments/projects/lots | no | yes | yes |
| Analytics, reports, audits, seller dashboard | no | yes | yes |
| Delete/toggle/restore users | no | no | yes |
| Approve/reject/cancel/reopen/capital repayment contracts | no | no | yes |
| Approve/reject/undo payments | no | no | yes |
| Create/update/delete projects and lots | no | no | yes |
| Job status | no | no | yes |

## Route reference

### Admin-only (`RequireAdmin`)

| Method | Path |
| --- | --- |
| `DELETE` | `/users/:user_id` |
| `PUT` | `/users/:user_id/toggle_status` |
| `POST` | `/users/:user_id/restore` |
| `DELETE` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id` |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/approve` |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/reject` |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/cancel` |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/reopen` |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/capital_repayment` |
| `POST` | `/payments/:payment_id/approve` |
| `POST` | `/payments/:payment_id/reject` |
| `POST` | `/payments/:payment_id/undo` |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/approve` |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/reject` |
| `POST` | `/projects` |
| `PUT` | `/projects/:project_id` |
| `DELETE` | `/projects/:project_id` |
| `POST` | `/projects/:project_id/approve` |
| `POST` | `/projects/import` |
| `POST` | `/projects/:project_id/lots` |
| `PUT` | `/projects/:project_id/lots/:lot_id` |
| `DELETE` | `/projects/:project_id/lots/:lot_id` |
| `GET` | `/jobs/status` |

### Admin, seller, or owner (`RequireAdminSellerOrOwner`)

| Method | Path |
| --- | --- |
| `GET` | `/users/:user_id` |
| `GET` | `/users/:user_id/contracts` |
| `GET` | `/users/:user_id/payments` |
| `GET` | `/users/:user_id/payment_history` |
| `GET` | `/users/:user_id/summary` |

### Admin or seller (`RequireRole("admin", "seller")`)

| Method | Path |
| --- | --- |
| `GET` | `/users` |
| `POST` | `/users` |
| `GET` | `/contracts` |
| `GET` | `/contracts/stats` |
| `GET` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id` |
| `GET` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/ledger` |
| `GET` | `/payments` |
| `GET` | `/payments/statistics` |
| `GET` | `/payments/stats` |
| `GET` | `/payments/:payment_id` |
| `GET` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments` |
| `GET` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id` |
| `GET` | `/projects` |
| `GET` | `/projects/:project_id` |
| `GET` | `/projects/:project_id/lots` |
| `GET` | `/projects/:project_id/lots/:lot_id` |
| `GET` | `/analytics/overview` |
| `GET` | `/analytics/distribution` |
| `GET` | `/analytics/performance` |
| `GET` | `/analytics/sellers` |
| `GET` | `/analytics/export` |
| `GET` | `/reports/commissions` |
| `GET` | `/reports/commissions_csv` |
| `GET` | `/reports/total_revenue_csv` |
| `GET` | `/reports/overdue_payments_csv` |
| `GET` | `/reports/user_balance_pdf` |
| `GET` | `/reports/user_promise_contract_pdf` |
| `GET` | `/reports/user_rescission_contract_pdf` |
| `GET` | `/reports/user_information_pdf` |
| `GET` | `/reports/customer_record_pdf` |
| `GET` | `/dashboard/seller` |
| `GET` | `/audits` |

### Any authenticated user

| Method | Path | Guard |
| --- | --- | --- |
| `PUT` | `/users/:user_id` | admin or owner |
| `PATCH` | `/users/:user_id/change_password` | authenticated |
| `POST` | `/users/:user_id/resend_confirmation` | authenticated |
| `PATCH` | `/users/:user_id/update_locale` | authenticated |
| `POST` | `/users/:user_id/picture` | authenticated |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts` | authenticated |
| `PATCH` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id` | authenticated |
| `POST` | `/payments/:payment_id/upload_receipt` | authenticated |
| `POST` | `/projects/:project_id/lots/:lot_id/contracts/:contract_id/payments/:payment_id/upload_receipt` | authenticated |
| `GET` | `/payments/:payment_id/download_receipt` | authenticated |
| `GET` | `/notifications` | authenticated |
| `POST` | `/notifications/mark_all_as_read` | authenticated |
| `POST` | `/notifications/:notification_id/mark_as_read` | authenticated |
| `GET` | `/notifications/:notification_id` | authenticated |
| `DELETE` | `/notifications/:notification_id` | authenticated |

## Request and response conventions

- JSON field names are `snake_case`.
- Responses usually wrap the resource:
  - `{ "contract": { ... } }`
  - `{ "projects": [...], "pagination": { ... } }`
  - `{ "message": "..." }`
- Errors use `{ "error": "<message>" }`. Many messages are Spanish because the
  frontend is Spanish.
- List endpoints accept `page`, `per_page`, and often `search_term`, `status`,
  `sort`, `start_date`, `end_date`, and resource-specific filters.
- Pagination shapes vary slightly by handler. Contract and lot lists return
  `page`, `per_page`, `total`, `total_pages`; some lists only return `total`.
- `sort` format is `field-direction`, e.g. `created_at-desc`.
- Date filters use `YYYY-MM-DD` unless the handler documents otherwise.
- Contract creation uses `multipart/form-data` with a JSON `contract` field,
  optional JSON `user` field, and optional `documents` files.
- CSV/PDF endpoints set `Content-Disposition: attachment` and stream the file.

## Swagger

Annotations are declared on handler methods. Regenerate with:

```bash
make swagger
```

Output is written to `docs/` (`docs.go`, `swagger.json`, `swagger.yaml`).
`docs/` is the only canonical location; do not add duplicate specs elsewhere.

## Known gaps and caveats

- `POST /projects/:project_id/approve` is a placeholder and does not change
  project state.
- `GET /reports/user_information_pdf` currently reuses the user balance PDF.
- `GET /health` is exposed under `/api/v1` only.
- There is no request rate limiting.
- Role checks are route-level; ownership checks for payments and contracts are
  performed inside handlers/services rather than uniformly in middleware.
