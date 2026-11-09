---
description: Format, build, test, and lint the Go API when the tools are available.
agent: build
---

Verify the current working tree without changing behavior.

Run, in order, and stop to report on the first hard failure:

1. `gofmt -l .` — list unformatted Go files. If any are listed, run
   `gofmt -w` only on the files you changed for this task.
2. `go build ./...`
3. `go test ./...`
4. `command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed; skipped"`
5. `git status --short` and `git diff --stat`

Do not run migrations, seeds, Docker builds, or destructive database targets.
Do not commit. Summarize failures with the exact command and the relevant
output. Additional context: $ARGUMENTS
