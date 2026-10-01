DB_URL ?= "postgres://postgres:password@localhost:5433/golearning?sslmode=disable"
MIGRATIONS_PATH ?= "file://migrations"

.PHONY: migrate-up migrate-down sqlc-generate test test-race vet seed gen-docs

migrate-up:
	go run ./cmd/migrate -direction=up -db-url=$(DB_URL) -path=$(MIGRATIONS_PATH)

migrate-down:
	go run ./cmd/migrate -direction=down -db-url=$(DB_URL) -path=$(MIGRATIONS_PATH)

sqlc-generate:
	sqlc generate

test:
	go test -v ./...

test-race:
	go test -v -race ./...

vet:
	go vet ./...

seed:
	go run ./cmd/seed -db-url=$(DB_URL)

SWAG ?= $(shell which swag 2>/dev/null || echo $(shell go env GOPATH)/bin/swag)

gen-docs:
	$(SWAG) fmt -d ./internal/api,./cmd/api
	$(SWAG) init -g ./cmd/api/main.go -o ./docs --parseDependency --parseInternal
