SERVICE_DIR := services/order
DATABASE_URL ?= postgres://order:order@localhost:5432/order?sslmode=disable
KAFKA_BROKERS ?= localhost:29092

.PHONY: help tidy build test test-integration vet run run-relay run-projector \
        migrate-up migrate-down docker-build compose-up compose-down smoke clean

help:
	@echo "Targets:"
	@echo "  tidy             Resolve module dependencies (requires network)"
	@echo "  build            Build api, migrate, relay, and projector binaries"
	@echo "  test             Run unit tests"
	@echo "  test-integration Run integration tests (requires Docker)"
	@echo "  vet              Run go vet"
	@echo "  run              Run the api locally"
	@echo "  run-relay        Run the outbox relay locally"
	@echo "  run-projector    Run the projection consumer locally"
	@echo "  migrate-up       Apply database migrations"
	@echo "  migrate-down     Roll back the last migration"
	@echo "  compose-up       Build and start the full local stack (db, kafka, api, relay, projector)"
	@echo "  compose-down     Stop the local stack"
	@echo "  smoke            Print the end-to-end smoke-test steps"

tidy:
	cd $(SERVICE_DIR) && go mod tidy

build:
	cd $(SERVICE_DIR) && \
		go build -o ../../bin/api ./cmd/api && \
		go build -o ../../bin/migrate ./cmd/migrate && \
		go build -o ../../bin/relay ./cmd/relay && \
		go build -o ../../bin/projector ./cmd/projector

test:
	cd $(SERVICE_DIR) && go test ./...

test-integration:
	cd $(SERVICE_DIR) && RUN_INTEGRATION=1 go test ./test/integration/...

vet:
	cd $(SERVICE_DIR) && go vet ./...

run:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

run-relay:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" KAFKA_BROKERS="$(KAFKA_BROKERS)" go run ./cmd/relay

run-projector:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" KAFKA_BROKERS="$(KAFKA_BROKERS)" go run ./cmd/projector

migrate-up:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate up

migrate-down:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate down

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down -v

smoke:
	@echo "End-to-end smoke test (stack must be up: make compose-up):"
	@echo "1. Place an order:"
	@echo "   curl -s -XPOST localhost:8080/v1/orders -H 'Content-Type: application/json' \\"
	@echo "     -H 'Idempotency-Key: smoke-0001' -d @docs/sample-order.json"
	@echo "2. Watch the event land on the topic:"
	@echo "   docker compose exec kafka /opt/kafka/bin/kafka-console-consumer.sh \\"
	@echo "     --bootstrap-server localhost:9092 --topic orders.events --from-beginning --max-messages 1"
	@echo "3. Read the order back (served from the projection once the projector catches up):"
	@echo "   curl -s localhost:8080/v1/orders/<order-id-from-step-1>"

clean:
	rm -rf bin
