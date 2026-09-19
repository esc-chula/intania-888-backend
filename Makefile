tidy:
	go mod tidy

SWAG_VERSION ?= v1.16.3
SWAG_CMD = go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

swagger:
	$(SWAG_CMD) init -g cmd/main.go -o docs

swagger-check:
	@set -e; \
	tmp_root=$$(mktemp -d); \
	tmp_dir="$$tmp_root/docs"; \
	mkdir "$$tmp_dir"; \
	trap 'rm -rf "$$tmp_root"' EXIT; \
	$(SWAG_CMD) init -g cmd/main.go -o "$$tmp_dir"; \
	diff -u docs/docs.go "$$tmp_dir/docs.go"; \
	diff -u docs/swagger.json "$$tmp_dir/swagger.json"; \
	diff -u docs/swagger.yaml "$$tmp_dir/swagger.yaml"

migrate-up:
	APP_ENV=dev go run ./cmd/migrate up

migrate-status:
	APP_ENV=dev go run ./cmd/migrate status

migrate-down:
	@test "$${ALLOW_DESTRUCTIVE_MIGRATIONS}" = "I_UNDERSTAND_DATA_WILL_BE_LOST" || (echo "refusing destructive migration; set ALLOW_DESTRUCTIVE_MIGRATIONS=I_UNDERSTAND_DATA_WILL_BE_LOST"; exit 1)
	APP_ENV=dev go run ./cmd/migrate down

migrate-reset:
	@test "$${ALLOW_DESTRUCTIVE_MIGRATIONS}" = "I_UNDERSTAND_DATA_WILL_BE_LOST" || (echo "refusing destructive migration; set ALLOW_DESTRUCTIVE_MIGRATIONS=I_UNDERSTAND_DATA_WILL_BE_LOST"; exit 1)
	APP_ENV=dev go run ./cmd/migrate reset

migrate: migrate-status
	@echo "'make migrate' is non-mutating; use 'make migrate-up' explicitly"

seed:
	APP_ENV=dev go run ./cmd/seed

run:
	go run ./cmd/main.go dev
