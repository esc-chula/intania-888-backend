tidy:
	go mod tidy

swagger:
	swag init -g cmd/main.go -o docs

migrate:
	go run ./pkg/database/migration/migration_script.go