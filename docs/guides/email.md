# Email setup (Resend)

Transactional email is sent through [Resend](https://resend.com). The API loads
its email configuration from the environment at startup, so **restart the API
after changing any email variable**.

Relevant files:

- `internal/config/config.go` — configuration keys and defaults
- `internal/services/email_service.go` — templates, send wrapper and recipient override
- `.env.example` — copy this file to `.env`

## Variables

| Variable | Purpose |
| --- | --- |
| `RESEND_API_KEY` | Resend API key. Email is skipped/rejected without it. |
| `FROM_EMAIL` | Sender address. Must be verified in Resend, or use `onboarding@resend.dev` for testing. |
| `ENABLE_EMAIL_NOTIFICATIONS` | Global switch. Must be `true` to send anything. |
| `EMAIL_OVERRIDE_TO` | Optional test inbox. When set, **every** email is redirected here and the subject is prefixed with `[TEST → original@recipient]`. Leave empty in production. |
| `PORTAL_URL` | Customer portal origin used in email deep links. |
| `APP_URL` | Admin app origin used in internal email links. |

## Local development (test inbox)

1. Create a Resend account and an API key at <https://resend.com/api-keys>.
2. In `FinteraAPI/.env`:

   ```env
   ENABLE_EMAIL_NOTIFICATIONS=true
   RESEND_API_KEY=re_your_real_key
   FROM_EMAIL=onboarding@resend.dev
   EMAIL_OVERRIDE_TO=delivered@resend.dev
   PORTAL_URL=http://localhost:5175
   APP_URL=http://localhost:5173
   ```

   > `onboarding@resend.dev` is Resend's test sender and does not require domain
   > verification. `delivered@resend.dev` accepts every message and reports it
   > as delivered.

3. Restart the API:

   ```bash
   make build
   PORT=8081 ./bin/api
   ```

4. Trigger an email and check the API log. For example, a password recovery:

   ```bash
   curl -X POST http://localhost:8081/api/v1/users/send_recovery_code \
     -H "Content-Type: application/json" \
     -d '{"email":"alexander.ramos@example.com"}'
   ```

   Expected log lines:

   ```text
   📧 [Email Redirect] alexander.ramos@example.com → delivered@resend.dev
   📧 [Email Sent] To: alexander.ramos@example.com | Subject: Código de reseteo
   ```

   In the app, the same path is exercised by **Reenviar correo de acceso** on
   the reservation tracking page.

5. Check the Resend dashboard under **Emails** to confirm the message was
   accepted, or use the labels below.

### Test inbox addresses

| Address | Behavior |
| --- | --- |
| `delivered@resend.dev` | Always accepted and reported as delivered |
| `delivered+local@resend.dev` | Same, labelled `local` (useful per environment) |
| `delivered+staging@resend.dev` | Same, labelled `staging` |

Any `delivered+<label>@resend.dev` address works; use a different label per
environment so test mail is easy to tell apart.

## Staging

Use the same setup as local, with a staging label and a real/fallback sender:

```env
ENABLE_EMAIL_NOTIFICATIONS=true
RESEND_API_KEY=re_...
FROM_EMAIL=onboarding@resend.dev   # or a verified staging domain
EMAIL_OVERRIDE_TO=delivered+staging@resend.dev
PORTAL_URL=https://staging-portal.example.com
```

No real customer receives mail while `EMAIL_OVERRIDE_TO` is set.

## Production

```env
ENABLE_EMAIL_NOTIFICATIONS=true
RESEND_API_KEY=re_...
FROM_EMAIL=noreply@your-verified-domain.com
# EMAIL_OVERRIDE_TO is not set (or empty)
PORTAL_URL=https://portal.fintera.app
```

- Verify the sending domain in Resend (SPF/DKIM) before using it as
  `FROM_EMAIL`; Resend rejects unverified senders.
- **Do not set `EMAIL_OVERRIDE_TO` in production.** When it is set, all mail
  goes to the test inbox and customers receive nothing.

## How the override works

All email methods call `EmailService.sendEmail`, which calls the pure helper
`applyRecipientOverride`:

- empty override → the message is sent unchanged (production behavior);
- non-empty override → `To` becomes the override address and the subject gains
  the `[TEST → original recipients]` prefix;
- the redirect is logged.

Adding a new email type? Send it through `s.sendEmail(...)`, never call
`s.resendClient.Emails.Send` directly, or it will bypass the override.

## Troubleshooting

| Symptom | Cause / fix |
| --- | --- |
| `[Email Disabled] Skipping ...` | `ENABLE_EMAIL_NOTIFICATIONS` is `false`. |
| `email not configured: RESEND_API_KEY is not set` | Add a key to `.env` and restart. |
| `API key is invalid` | Resend rejected the key: create a new one and restart. |
| `The from address is not verified` | Verify the domain in Resend or use `onboarding@resend.dev`. |
| Nothing arrives in the test inbox | Check `EMAIL_OVERRIDE_TO` is set and the API was restarted. |
| Email went to a real customer in staging | `EMAIL_OVERRIDE_TO` is missing/empty. Set it. |

## Security

- `.env` is git-ignored; never commit real keys.
- Rotate a leaked key in Resend immediately.
- Use a dedicated Resend key per environment (local, staging, production).
