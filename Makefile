# Load .env file if it exists, but don't override existing environment variables
# Using -include ignores the file if it doesn't exist
-include .env

# Ensure DATABASE_URL is exported and respects environment overrides
# If DATABASE_URL was in .env, 'include' set it. If it was already in env,
# 'include' might have overridden it depending on 'make' version.
export DATABASE_URL

# Allow using a specific Go binary, e.g.:
#   make GO=/usr/local/go/bin/go test
GO ?= go

# Install Go tools into GOBIN (or GOPATH/bin) and make them available to
# recipes even when the shell PATH does not include that directory.
GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(strip $(GOBIN)),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif
export PATH := $(GOBIN):$(PATH)

# Resolve tools to GOBIN explicitly. GNU Make 3.81 (macOS) may bypass the
# exported PATH for simple recipe commands, so avoid relying on bare names.
AIR ?= $(GOBIN)/air
SWAG ?= $(GOBIN)/swag
MIGRATE ?= $(GOBIN)/migrate
GOLANGCI_LINT ?= $(GOBIN)/golangci-lint
GOSEC ?= $(GOBIN)/gosec

.PHONY: build run dev test test-cover clean deps tools \
	migrate-up migrate-down migrate-create ensure-migrate \
	drop-db-seed-migrate drop-db-seed-migrate-force seed \
	swagger lint fmt security docker-build docker-run

# Build the application
build:
	$(GO) build -o bin/api ./cmd/api

# Run the application
run: build
	./bin/api

# Run in development mode with hot reload (installs air when missing)
dev:
	@test -x $(AIR) || { echo "Installing air..."; $(GO) install github.com/air-verse/air@latest; }
	$(AIR)

# Run tests
test:
	$(GO) test -v ./...

# Run tests with coverage
test-cover:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Install the development tools used by the make targets
tools:
	@test -x $(AIR) || $(GO) install github.com/air-verse/air@latest
	@test -x $(SWAG) || $(GO) install github.com/swaggo/swag/cmd/swag@latest
	@test -x $(MIGRATE) || $(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@test -x $(GOLANGCI_LINT) || $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@test -x $(GOSEC) || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "Tools installed in $(GOBIN)"

# Ensure golang-migrate is available
ensure-migrate:
	@test -x $(MIGRATE) || { echo "Installing golang-migrate..."; $(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; }

# Run database migrations
migrate-up: ensure-migrate
	$(MIGRATE) -path internal/database/migrations -database "$(DATABASE_URL)" up

migrate-down: ensure-migrate
	$(MIGRATE) -path internal/database/migrations -database "$(DATABASE_URL)" down 1

migrate-create: ensure-migrate
	@read -p "Migration name: " name; \
	$(MIGRATE) create -ext sql -dir internal/database/migrations -seq $$name

# Drop database and run seed and migrations
drop-db-seed-migrate: ensure-migrate
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "DROP DATABASE IF EXISTS fintera_api_development;"
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "CREATE DATABASE fintera_api_development;"
	$(MIGRATE) -path internal/database/migrations -database "$(DATABASE_URL)" up
	@for f in $$(ls internal/database/seeds/*.sql | sort); do echo "Running seed: $$f"; psql "$(DATABASE_URL)" -f $$f; done

# Drop database and run seed and migrations and force drop database
drop-db-seed-migrate-force: ensure-migrate
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "SELECT pg_terminate_backend(pg_stat_activity.pid) FROM pg_stat_activity WHERE pg_stat_activity.datname = 'fintera_api_development' AND pid <> pg_backend_pid();"
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "DROP DATABASE IF EXISTS fintera_api_development;"
	psql "$(subst fintera_api_development,postgres,$(DATABASE_URL))" -c "CREATE DATABASE fintera_api_development;"
	$(MIGRATE) -path internal/database/migrations -database "$(DATABASE_URL)" up
	@for f in $$(ls internal/database/seeds/*.sql | sort); do echo "Running seed: $$f"; psql "$(DATABASE_URL)" -f $$f; done

# Run database seeds
seed:
	@if [ -z "$(DATABASE_URL)" ]; then echo "DATABASE_URL is not set"; exit 1; fi
	@for f in $$(ls internal/database/seeds/*.sql | sort); do echo "Running seed: $$f"; psql "$(DATABASE_URL)" -f $$f; done

# Generate Swagger docs (installs swag when missing)
swagger:
	@test -x $(SWAG) || { echo "Installing swag..."; $(GO) install github.com/swaggo/swag/cmd/swag@latest; }
	$(SWAG) init -g cmd/api/main.go -o docs

# Lint the code (installs golangci-lint when missing)
lint:
	@test -x $(GOLANGCI_LINT) || { echo "Installing golangci-lint..."; $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; }
	$(GOLANGCI_LINT) run

# Format code
fmt:
	$(GO) fmt ./...

# Check for security issues (installs gosec when missing)
security:
	@test -x $(GOSEC) || { echo "Installing gosec..."; $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest; }
	$(GOSEC) ./...

# Docker commands
docker-build:
	docker build -t fintera-api .

docker-run:
	docker run -p 8080:8080 --env-file .env fintera-api
