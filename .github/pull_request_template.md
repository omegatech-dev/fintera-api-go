# Pull Request

## 📋 Description

Summarize the change, the motivation, and the related issue.

Examples:
- Fix overdue interest calculation for bank-financed contracts
- Add capital repayment handling to the contract ledger
- Refactor payment approval to use the payment FSM

---

## 🔗 Ticket Link

---

## 🧩 Type of Change

Delete options that are not relevant.

- [ ] 🐛 Bug fix
- [ ] ✨ New feature
- [ ] ♻️ Refactor
- [ ] 📝 Documentation update
- [ ] ⚡ Performance improvement
- [ ] 🗄️ Database migration
- [ ] 🔧 Configuration / build / CI
- [ ] 🧹 Chore

---

## 🔌 API / Data Changes

- [ ] New or changed API endpoints
- [ ] Request or response shape changed
- [ ] Database migration included
- [ ] New or changed environment variable
- [ ] Background job or schedule changed
- [ ] None

If any are checked, list the affected routes, migrations, env vars, or jobs:

---

## 🧪 How Has This Been Tested?

Describe the tests you ran to verify the change.

```bash
make fmt
go build ./...
go test ./...
go test ./internal/services -run TestName -v
```

- [ ] Unit tests added or updated
- [ ] Manual API testing (Swagger UI or curl)
- [ ] Database migration tested up and down
- [ ] Not applicable

---

## ⚠️ Breaking Changes

- [ ] This PR introduces breaking changes

If yes, describe the impact and the migration or deployment steps.

---

## ✅ Checklist

Before requesting review, confirm the following:

- [ ] Code is formatted (`make fmt` / `gofmt`)
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] `make lint` passes if `golangci-lint` is installed
- [ ] Swagger regenerated with `make swagger` if handler annotations changed
- [ ] Documentation updated under `docs/` if routes, statuses, schema, config, or jobs changed
- [ ] Migrations are reversible and no applied migration was edited
- [ ] No secrets, `.env` files, binaries, or uploaded files committed
- [ ] Async notifications/emails use `worker.EnqueueAsync`
- [ ] Admin state changes write an audit log
- [ ] Reviewed for N+1 queries and unnecessary preloads
- [ ] Performed a self-review of the diff

---

## 📸 Screenshots (if applicable)

Add screenshots or recordings for API/UI-visible changes.

---

## 🔁 Steps to Reproduce / Verify

Example:
1. `cp .env.example .env` and set `DATABASE_URL`
2. `make migrate-up && make seed`
3. `make run`
4. `POST /api/v1/auth/login`
5. Call the affected endpoint and verify the response and ledger/state changes

---

## 💬 Additional Notes

Add any other context for reviewers or QA, such as rollback notes or operational impact.
