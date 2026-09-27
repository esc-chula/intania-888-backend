SHELL := /usr/bin/env bash

.DEFAULT_GOAL := help

GO ?= go
AIR ?= air
AIR_CONFIG ?= .air.toml
DOCKER_COMPOSE ?= docker compose
GOLANGCI_LINT ?= golangci-lint
APP_ENV ?= dev
TEST_COMPOSE_FILE ?= docker-compose.test.yml
TEST_COMPOSE_PROJECT ?= intania888-test
TEST_POSTGRES_PORT ?= 55432
TEST_REDIS_PORT ?= 56379
TEST_DATABASE_URL ?= postgres://root:1234@localhost:$(TEST_POSTGRES_PORT)/intania888_test?sslmode=disable

# Resolve the generator from go.mod so docs checks use the project's pinned version.
SWAG_CMD := $(GO) run github.com/swaggo/swag/cmd/swag
GO_PACKAGES := ./cmd/... ./docs/... ./internal/... ./pkg/... ./utils/...
DESTRUCTIVE_MIGRATION_CONFIRMATION := I_UNDERSTAND_DATA_WILL_BE_LOST

.PHONY: help dev deps migrate migrate-up migrate-status migrate-down migrate-reset seed test test-race test-integration \
	build fmt-check lint docs docs-check tidy ci check-env check-air check-docker check-golangci

help:
	@printf '%s\n' \
		'Available commands:' \
		'  make dev                                      Start Compose dependencies, migrate, and run Air' \
		'  make deps                                     Start PostgreSQL and Redis dependencies' \
		'  make migrate                                   Apply migrations (alias for migrate-up)' \
		'  make migrate-status                            Show migration status' \
		'  make migrate-down                              Roll back one migration (guarded)' \
		'  make migrate-reset                             Roll back all migrations (guarded)' \
		'  make seed                                     Seed stable catalogue data' \
		'  make test                                     Run the test suite' \
		'  make test-race                                Run tests with the race detector' \
		'  make test-integration                         Run PostgreSQL acceptance tests' \
		'  make build                                    Compile all Go packages' \
		'  make lint                                     Check formatting, vet, and run golangci-lint' \
		'  make docs                                     Regenerate API documentation' \
		'  make docs-check                               Verify generated documentation files are current' \
		'  make tidy                                     Tidy Go modules' \
		'  make ci                                       Run the local CI checks'

check-env:
	@test -f .env || { \
		echo 'missing .env; copy .env.example to .env and configure local values'; \
		exit 1; \
	}

check-air:
	@command -v "$(AIR)" >/dev/null 2>&1 || { \
		echo 'air is required for make dev; install it or set AIR=/path/to/air'; \
		exit 1; \
	}

check-docker:
	@command -v "$(firstword $(DOCKER_COMPOSE))" >/dev/null 2>&1 || { \
		echo 'Docker Compose is required for make deps, make dev, or make test-integration'; \
		exit 1; \
	}

check-golangci:
	@command -v "$(GOLANGCI_LINT)" >/dev/null 2>&1 || { \
		echo 'golangci-lint is required for make lint; install it or set GOLANGCI_LINT=/path/to/golangci-lint'; \
		exit 1; \
	}

deps: check-docker
	$(DOCKER_COMPOSE) up --detach --wait postgres redis

dev: check-env check-air deps
	$(MAKE) APP_ENV=dev migrate
	APP_ENV=dev $(AIR) -c $(AIR_CONFIG)

migrate: migrate-up

migrate-up: check-env
	APP_ENV=$(APP_ENV) $(GO) run ./cmd/migrate up

migrate-status: check-env
	APP_ENV=$(APP_ENV) $(GO) run ./cmd/migrate status

migrate-down: check-env
	@test "$${ALLOW_DESTRUCTIVE_MIGRATIONS}" = "$(DESTRUCTIVE_MIGRATION_CONFIRMATION)" || { \
		echo 'destructive migration refused; set ALLOW_DESTRUCTIVE_MIGRATIONS=I_UNDERSTAND_DATA_WILL_BE_LOST'; \
		exit 1; \
	}
	APP_ENV=$(APP_ENV) $(GO) run ./cmd/migrate down

migrate-reset: check-env
	@test "$${ALLOW_DESTRUCTIVE_MIGRATIONS}" = "$(DESTRUCTIVE_MIGRATION_CONFIRMATION)" || { \
		echo 'destructive migration refused; set ALLOW_DESTRUCTIVE_MIGRATIONS=I_UNDERSTAND_DATA_WILL_BE_LOST'; \
		exit 1; \
	}
	APP_ENV=$(APP_ENV) $(GO) run ./cmd/migrate reset

seed: check-env
	APP_ENV=$(APP_ENV) $(GO) run ./cmd/seed

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-integration: check-docker
	@set -e; \
		cleanup() { \
			TEST_POSTGRES_PORT="$(TEST_POSTGRES_PORT)" TEST_REDIS_PORT="$(TEST_REDIS_PORT)" $(DOCKER_COMPOSE) -f "$(TEST_COMPOSE_FILE)" -p "$(TEST_COMPOSE_PROJECT)" down --volumes --remove-orphans; \
		}; \
		trap cleanup EXIT; \
		TEST_POSTGRES_PORT="$(TEST_POSTGRES_PORT)" TEST_REDIS_PORT="$(TEST_REDIS_PORT)" $(DOCKER_COMPOSE) -f "$(TEST_COMPOSE_FILE)" -p "$(TEST_COMPOSE_PROJECT)" up --detach --wait postgres redis; \
		INTANIA888_TEST_DATABASE_URL="$(TEST_DATABASE_URL)" INTANIA888_TEST_REDIS_ADDR="localhost:$(TEST_REDIS_PORT)" $(GO) test -tags=integration -p 1 -count=1 ./...

build:
	$(GO) build ./...

fmt-check:
	@files="$$(find . -type f -name '*.go' -not -path './vendor/*' -print)"; \
		if test -n "$$files" && test -n "$$(gofmt -l $$files)"; then \
			echo 'Go files are not gofmt-formatted:'; \
			gofmt -l $$files; \
			exit 1; \
		fi

lint: fmt-check check-golangci
	$(GO) vet ./...
	$(GOLANGCI_LINT) run --allow-parallel-runners $(GO_PACKAGES)

docs:
	$(SWAG_CMD) init -g cmd/main.go -o docs

docs-check:
	@set -e; \
		tmp_root=$$(mktemp -d); \
		tmp_dir="$$tmp_root/docs"; \
		mkdir "$$tmp_dir"; \
		trap 'rm -rf "$$tmp_root"' EXIT; \
		$(SWAG_CMD) init -g cmd/main.go -o "$$tmp_dir"; \
		diff -u docs/docs.go "$$tmp_dir/docs.go"; \
		diff -u docs/swagger.json "$$tmp_dir/swagger.json"; \
		diff -u docs/swagger.yaml "$$tmp_dir/swagger.yaml"

tidy:
	$(GO) mod tidy

ci: lint test test-race build docs-check
