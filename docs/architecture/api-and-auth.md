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
| `GET` | `/api/v1/branding` |

`GET /swagger/*any`, `/`, `/uploads/*`, and `/receipts/*` are also public.

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
| `POST` | `/projects/:project_id/publish` |
| `POST` | `/projects/:project_id/unpublish` |
| `POST` | `/projects/import` |
| `POST` | `/projects/:project_id/lots` |
| `PUT` | `/projects/:project_id/lots/:lot_id` |
| `DELETE` | `/projects/:project_id/lots/:lot_id` |
| `GET` | `/jobs/status` |
| `PUT` | `/branding` |
| `POST` | `/branding/logo` |
| `DELETE` | `/branding/logo` |

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
| `GET` | `/reports/commissions` |
| `GET` | `/reports/operations_export` |
| `GET` | `/reports/commissions_export` |
| `GET` | `/reports/overdue_export` |
| `GET` | `/reports/income_flow_export` |
| `GET` | `/reports/user_balance_pdf` |
| `GET` | `/reports/user_promise_contract_pdf` |
| `GET` | `/reports/user_rescission_contract_pdf` |
| `GET` | `/reports/user_information_pdf` |
| `GET` | `/reports/customer_record_pdf` |
| `GET` | `/dashboard/seller` |
| `GET` | `/audits` |

#### Historical sales (admin and seller)

Sellers only see and edit the records they created; admins see everything.
Both `evidence` endpoints serve the seller-uploaded "respaldo" file: `GET` with
`download=1` forces an attachment, otherwise the file streams inline so the
admin can view the image in the browser. Legacy records whose `evidence_file`
is an external `http(s)` URL return `{"evidence_url": "..."}` instead of a file.

| Method | Path |
| --- | --- |
| `GET` | `/historical_sales` |
| `GET` | `/historical_sales/:id` |
| `POST` | `/historical_sales` |
| `PUT` | `/historical_sales/:id` |
| `POST` | `/historical_sales/import` |
| `GET` | `/historical_sales/import/template` |
| `POST` | `/historical_sales/:id/evidence` |
| `GET` | `/historical_sales/:id/evidence` |

Admin-only historical-sale actions: `POST /historical_sales/:id/validate` and
`POST /historical_sales/:id/reject`.

#### Report export endpoints

All four exports require `format` (`xlsx`, `csv`, or `pdf`) and return a
downloadable file: `text/csv` for CSV, `application/octet-stream` for XLSX and
PDF. Sellers are always scoped to their own operations; admins see everything.

| Path | Report | Filters |
| --- | --- | --- |
| `/reports/operations_export` | Monthly operations summary for accounting | `project_id`, `start_date`, `end_date` |
| `/reports/commissions_export` | Commissions grouped by seller (paid/pending, credits, net pending) | `start_date`, `end_date`, `seller_id` |
| `/reports/overdue_export` | Overdue payments with 1-30/31-60/61-90/90+ aging buckets (cancelled contracts excluded) | none |
| `/reports/income_flow_export` | Monthly income flow: actual vs projected collections per month/project | `project_id`, `start_date`, `end_date` |

Dates accept `AAAA-MM-DD` or RFC3339. When both dates are omitted on the
operations and income-flow exports, the period defaults to the current calendar
month; commissions keeps all periods and overdue is a current snapshot.

On `/reports/commissions_export`, admins may pass `seller_id` to limit the whole
export (commissions, credits and net pending) to one seller; an invalid value or
`0` returns `400` with `{"error": "seller_id inválido"}`. Non-admin users ignore
`seller_id` and always export their own operations.

#### Commissions list (`GET /reports/commissions`)

Returns one page of commissions and one page of seller credits. Pages hold at
most 20 rows: `per_page` and `credits_per_page` default to 20 and values above 20
are capped at 20. `page` and `credits_page` below 1 are treated as 1, and a page
past the end returns an empty list with correct pagination metadata.

| Query parameter | Meaning |
| --- | --- |
| `start_date`, `end_date` | Commission period (`AAAA-MM-DD`) |
| `seller_id` | Admin only: commissions of this seller id. Rows without a seller id are excluded when set. Credits are filtered by the same id. Invalid values or `0` return `400`. For non-admin users the parameter is ignored: the server always filters by the authenticated user's id, so sellers never receive other sellers' rows or credits. |
| `search` | Case-insensitive substring on `client_name`, `project`, `lot`, `seller` and the contract id with or without `#` (`#43` and `43` both find contract 43); it also matches `historical_sale_id`. Blank or whitespace-only values do not filter. It only narrows `commissions` and `pagination.total` / `total_pages`. |
| `page`, `per_page` | Commission page and page size (max 20) |
| `credits_page`, `credits_per_page` | Credit page and page size (max 20) |

Response keys:

| Key | Content |
| --- | --- |
| `commissions` | The requested page of commission rows, after `search` |
| `pagination` | `page`, `per_page`, `total`, `total_pages` for the searched commissions, plus `period_total`: the commissions of the period before applying `search` |
| `totals` | `commission`, `amount`, `paid`, `pending`, `paid_count`, `average_rate` (`commission / amount * 100`, `0` when amount is 0), `net_pending` over the full filtered period, not the page and not affected by `search` |
| `sellers` | `id` and `name` of every seller with commissions in the period, sorted by name, built before the `seller_id` and `search` filters |
| `credits` | The requested page of seller commission credits |
| `credits_pagination` | `page`, `per_page`, `total`, `total_pages` for the credits |
| `pending_credits` | Pending total over all credits (not only the page); not affected by `search` |
| `net_pending` | Same value as `totals.net_pending`, kept for compatibility; not affected by `search` |

The totals are computed in Go after the full set is built, so the request still
loads every contract in the period; only the response size shrinks. `search`
keeps the dashboard KPIs stable while the seller narrows the visible rows.

**Migración de endpoints eliminados**

| Antes | Ahora |
| --- | --- |
| `/analytics/export?format=x` | `/reports/operations_export?format=x` |
| `/reports/commissions_csv` | `/reports/commissions_export?format=csv` |
| `/reports/total_revenue_csv` | `/reports/income_flow_export?format=csv` |
| `/reports/overdue_payments_csv` | `/reports/overdue_export?format=csv` |

`format` is required on every replacement endpoint.

The legacy paths above are no longer registered; clients still calling them
must migrate to the `*_export` equivalents.

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
- Admin project responses include `photos`, an ordered preview of up to ten
  gallery photos (`id`, `position`, `url`, optional `thumb_url`, optional
  `alt`), plus `cover_photo_url` matching the first preview photo.
- Uploaded raster images (gallery photos, raster plans, profile pictures,
  branding logos, image receipts) are re-encoded as lossy WebP and downscaled
  server-side; gallery thumbnails are generated by the server. SVG and PDF
  uploads are stored unchanged. Transcoding applies to new uploads only;
  `file_type` is `webp` for transcoded raster plans and `size_bytes` reflects
  the stored WebP size.
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
