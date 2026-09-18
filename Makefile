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

migrate:
	go run ./pkg/database/migration/migration_script.go

run:
	go run ./cmd/main.go dev
