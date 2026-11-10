## 1. Endpoint

- [x] 1.1 Add `UserHandler.ChangeOwnPassword` using the JWT user ID.
- [x] 1.2 Register `POST /auth/change_password` in the protected group.
- [x] 1.3 Keep `PATCH /users/{user_id}/change_password`.
- [x] 1.4 Add Swagger annotations.

## 2. Tests

- [x] 2.1 Cover missing session, missing current password, short new password and invalid JSON.
- [x] 2.2 Verify the happy path end-to-end against the local API.

## 3. Verification

- [x] 3.1 `go build ./...` and `go test ./...`.
- [x] 3.2 `make swagger`.
- [x] 3.3 `openspec validate --changes`.
