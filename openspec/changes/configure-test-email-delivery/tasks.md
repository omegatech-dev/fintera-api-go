## 1. Config

- [x] 1.1 Add `EmailOverrideTo` to `Config` (`EMAIL_OVERRIDE_TO`, default empty).
- [x] 1.2 Document it in `.env.example` and `docs/guides/development.md`.

## 2. Email service

- [x] 2.1 Add the pure `applyRecipientOverride` helper.
- [x] 2.2 Add `sendEmail` and route all 11 send calls through it.
- [x] 2.3 Log the redirect with the original recipients.

## 3. Local setup

- [x] 3.1 Set local `.env` to `ENABLE_EMAIL_NOTIFICATIONS=true`, `FROM_EMAIL=onboarding@resend.dev`, `EMAIL_OVERRIDE_TO=delivered@resend.dev`.
- [ ] 3.2 Verify a real send through Resend returns success. _(Blocked: the local `RESEND_API_KEY` returns `API key is invalid`; the redirect is confirmed working in the logs.)_

## 4. Tests and verification

- [x] 4.1 Unit test the override and the production (empty) path.
- [x] 4.2 `go build ./...` and `go test ./...`.
- [x] 4.3 `openspec validate --changes`.
