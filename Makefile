DB_URL ?= "postgres://postgres:password@localhost:5433/golearning?sslmode=disable"
MIGRATIONS_PATH ?= "file://migrations"

.PHONY: migrate-up migrate-down sqlc-generate test vet

migrate-up:
	go run ./cmd/migrate -direction=up -db-url=$(DB_URL) -path=$(MIGRATIONS_PATH)

migrate-down:
	go run ./cmd/migrate -direction=down -db-url=$(DB_URL) -path=$(MIGRATIONS_PATH)

sqlc-generate:
	sqlc generate

test:
	go test -v ./...

vet:
	go vet ./...
