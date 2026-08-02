SERVICE_DIR := services/order
DATABASE_URL ?= postgres://order:order@localhost:5432/order?sslmode=disable

.PHONY: help tidy build test test-integration vet run migrate-up migrate-down docker-build compose-up compose-down clean

help:
	@echo "Targets:"
	@echo "  tidy             Resolve module dependencies (requires network)"
	@echo "  build            Build api and migrate binaries"
	@echo "  test             Run unit tests"
	@echo "  test-integration Run integration tests (requires Docker)"
	@echo "  vet              Run go vet"
	@echo "  run              Run the api locally"
	@echo "  migrate-up       Apply database migrations"
	@echo "  migrate-down     Roll back the last migration"
	@echo "  compose-up       Build and start the full local stack"
	@echo "  compose-down     Stop the local stack"

tidy:
	cd $(SERVICE_DIR) && go mod tidy

build:
	cd $(SERVICE_DIR) && go build -o ../../bin/api ./cmd/api && go build -o ../../bin/migrate ./cmd/migrate

test:
	cd $(SERVICE_DIR) && go test ./...

test-integration:
	cd $(SERVICE_DIR) && RUN_INTEGRATION=1 go test ./test/integration/...

vet:
	cd $(SERVICE_DIR) && go vet ./...

run:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

migrate-up:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate up

migrate-down:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate down

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down -v

clean:
	rm -rf bin
