SERVICE_DIR := services/order
DATABASE_URL ?= postgres://order:order@localhost:5432/order?sslmode=disable
KAFKA_BROKERS ?= localhost:29092

.PHONY: help tidy build test test-integration vet run run-relay run-projector run-orchestrator \
        migrate-up migrate-down docker-build compose-up compose-down smoke saga-smoke clean

help:
	@echo "Targets:"
	@echo "  tidy             Resolve module dependencies (requires network)"
	@echo "  build            Build api, migrate, relay, projector, and orchestrator binaries"
	@echo "  test             Run unit tests"
	@echo "  test-integration Run integration tests (requires Docker)"
	@echo "  vet              Run go vet"
	@echo "  run              Run the api locally"
	@echo "  run-relay        Run the outbox relay locally"
	@echo "  run-projector    Run the projection consumer locally"
	@echo "  run-orchestrator Run the saga orchestrator locally"
	@echo "  migrate-up       Apply database migrations"
	@echo "  migrate-down     Roll back the last migration"
	@echo "  compose-up       Build and start the full local stack (db, kafka, order + inventory services)"
	@echo "  compose-down     Stop the local stack"
	@echo "  smoke            Print the end-to-end smoke-test steps"
	@echo "  saga-smoke       Print the end-to-end saga demo (confirm and compensate)"
	@echo "  pay-build        Build the Payment service (mvn package)"
	@echo "  pay-test         Run the Payment service tests (mvn test)"
	@echo "  pay-run          Run the Payment service locally"
	@echo "  pay-smoke        Print the payment capture and webhook demo"
	@echo "  ful-tidy         Resolve Fulfillment module dependencies (requires network)"
	@echo "  ful-build        Build the Fulfillment api, migrate, and consumer binaries"
	@echo "  ful-test         Run the Fulfillment service tests"
	@echo "  ful-run-api      Run the Fulfillment api locally"
	@echo "  ful-run-consumer Run the Fulfillment consumer locally"
	@echo "  ful-smoke        Print the shipment lifecycle demo"
	@echo "  ntf-tidy         Resolve Notification module dependencies (requires network)"
	@echo "  ntf-build        Build the Notification consumer and migrate binaries"
	@echo "  ntf-test         Run the Notification service tests"
	@echo "  ntf-run-consumer Run the Notification consumer locally"
	@echo "  ntf-smoke        Print the notification fan-in demo"

tidy:
	cd $(SERVICE_DIR) && go mod tidy

build:
	cd $(SERVICE_DIR) && \
		go build -o ../../bin/api ./cmd/api && \
		go build -o ../../bin/migrate ./cmd/migrate && \
		go build -o ../../bin/relay ./cmd/relay && \
		go build -o ../../bin/projector ./cmd/projector && \
		go build -o ../../bin/orchestrator ./cmd/orchestrator

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

run-orchestrator:
	cd $(SERVICE_DIR) && DATABASE_URL="$(DATABASE_URL)" KAFKA_BROKERS="$(KAFKA_BROKERS)" INVENTORY_BASE_URL="http://localhost:8081" PAYMENT_STUB_OUTCOME=approve go run ./cmd/orchestrator

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

saga-smoke:
	@echo "End-to-end saga demo (stack up: make compose-up)."
	@echo "The orchestrator reacts to order.placed automatically: reserve, pay, commit, confirm."
	@echo ""
	@echo "SUCCESS PATH (stub approves by default):"
	@echo "1. Seed stock for the sample order's SKU:"
	@echo "   curl -s -XPUT localhost:8081/v1/stock/SKU-CLASSIC-TEE -H 'Content-Type: application/json' -d '{\"available\":100}'"
	@echo "2. Place the sample order:"
	@echo "   curl -s -XPOST localhost:8080/v1/orders -H 'Content-Type: application/json' \\"
	@echo "     -H 'Idempotency-Key: saga-ok-1' -d @docs/sample-order.json"
	@echo "3. Read it back after a second (status should be CONFIRMED):"
	@echo "   curl -s localhost:8080/v1/orders/<order-id-from-step-2>"
	@echo "4. The reservation should be COMMITTED:"
	@echo "   curl -s localhost:8081/v1/reservations/<order-id-from-step-2>"
	@echo ""
	@echo "COMPENSATION PATH (force a decline):"
	@echo "5. Recreate the Payment service to decline (the saga now calls the real service):"
	@echo "   PAYMENT_GATEWAY_OUTCOME=decline docker compose up -d payment-api"
	@echo "   (tip: docker compose logs -f orchestrator shows the saga cancel in real time)"
	@echo "6. Place another order:"
	@echo "   curl -s -XPOST localhost:8080/v1/orders -H 'Content-Type: application/json' \\"
	@echo "     -H 'Idempotency-Key: saga-decline-1' -d @docs/sample-order.json"
	@echo "7. Read it back (status should be CANCELLED; the held stock is released):"
	@echo "   curl -s localhost:8080/v1/orders/<order-id-from-step-6>"
	@echo "   curl -s localhost:8081/v1/stock/SKU-CLASSIC-TEE"
	@echo ""
	@echo "8. Restore approvals:"
	@echo "   docker compose up -d payment-api"

clean:
	rm -rf bin

# ============================================================================
# Inventory service (Phase 3). Own module and database; runs on ports 8081/5433.
# ============================================================================
INVENTORY_DIR := services/inventory
INVENTORY_DATABASE_URL ?= postgres://inventory:inventory@localhost:5433/inventory?sslmode=disable

.PHONY: inv-tidy inv-build inv-test inv-test-integration inv-vet inv-run \
        inv-migrate-up inv-migrate-down inv-smoke

inv-tidy:
	cd $(INVENTORY_DIR) && go mod tidy

inv-build:
	cd $(INVENTORY_DIR) && \
		go build -o ../../bin/inventory-api ./cmd/api && \
		go build -o ../../bin/inventory-migrate ./cmd/migrate

inv-test:
	cd $(INVENTORY_DIR) && go test ./...

inv-test-integration:
	cd $(INVENTORY_DIR) && RUN_INTEGRATION=1 go test ./test/integration/...

inv-vet:
	cd $(INVENTORY_DIR) && go vet ./...

inv-run:
	cd $(INVENTORY_DIR) && DATABASE_URL="$(INVENTORY_DATABASE_URL)" go run ./cmd/api

inv-migrate-up:
	cd $(INVENTORY_DIR) && DATABASE_URL="$(INVENTORY_DATABASE_URL)" go run ./cmd/migrate up

inv-migrate-down:
	cd $(INVENTORY_DIR) && DATABASE_URL="$(INVENTORY_DATABASE_URL)" go run ./cmd/migrate down

inv-smoke:
	@echo "Inventory smoke test (stack up: make compose-up). API on :8081."
	@echo "1. Seed stock for a SKU:"
	@echo "   curl -s -XPUT localhost:8081/v1/stock/SKU-1 -H 'Content-Type: application/json' -d '{\"available\":100}'"
	@echo "2. Reserve stock for an order (orderId must be a UUID):"
	@echo "   curl -s -XPOST localhost:8081/v1/reservations -H 'Content-Type: application/json' \\"
	@echo "     -d '{\"orderId\":\"11111111-1111-1111-1111-111111111111\",\"lines\":[{\"sku\":\"SKU-1\",\"quantity\":3}]}'"
	@echo "3. Read the ledger (available down 3, reserved up 3):"
	@echo "   curl -s localhost:8081/v1/stock/SKU-1"
	@echo "4. Commit (or swap 'commit' for 'release' to compensate):"
	@echo "   curl -s -XPOST localhost:8081/v1/reservations/11111111-1111-1111-1111-111111111111/commit"
	@echo "5. Read the reservation:"
	@echo "   curl -s localhost:8081/v1/reservations/11111111-1111-1111-1111-111111111111"


# ============================================================================
# Payment service (Phase 4). Java/Spring Boot module; runs on ports 8082/5434.
# ============================================================================
PAYMENT_DIR := services/payment

.PHONY: pay-build pay-test pay-run pay-smoke

pay-build:
	cd $(PAYMENT_DIR) && mvn -q -DskipTests package

pay-test:
	cd $(PAYMENT_DIR) && mvn -q test

pay-run:
	cd $(PAYMENT_DIR) && DATABASE_URL="jdbc:postgresql://localhost:5434/payment" mvn -q spring-boot:run

pay-smoke:
	@echo "Payment smoke test (stack up: make compose-up). API on :8082."
	@echo "The saga calls this service on every order; these steps drive it directly."
	@echo ""
	@echo "1. Place an order; the saga captures a payment for it:"
	@echo "   curl -s -XPOST localhost:8080/v1/orders -H 'Content-Type: application/json' -H 'Idempotency-Key: pay-demo-1' -d @docs/sample-order.json"
	@echo "2. Read the payment back (status CAPTURED, approved true, a psp_ reference):"
	@echo "   curl -s localhost:8082/v1/payments/<order-id-from-step-1>"
	@echo "3. Sign a settlement webhook for that payment's providerReference and post it:"
	@echo "   BODY='{\"eventId\":\"evt-1\",\"type\":\"payment.settled\",\"providerReference\":\"<ref>\"}'"
	@echo "   SIG=\$$(printf '%s' \"\$$BODY\" | openssl dgst -sha256 -hmac whsec_local_dev_secret | sed 's/^.* //')"
	@echo "   curl -s -XPOST localhost:8082/v1/payments/webhooks -H \"X-Signature: \$$SIG\" -H 'Content-Type: application/json' -d \"\$$BODY\""
	@echo "4. Read it back (status now SETTLED):"
	@echo "   curl -s localhost:8082/v1/payments/<order-id-from-step-1>"
	@echo ""
	@echo "An unsigned webhook returns 401; a duplicate eventId is a no-op. Skip the webhook"
	@echo "and the reconciliation sweep settles it on its own after about two minutes."

# ============================================================================
# Fulfillment service (Phase 5). Go module; HTTP api on 8083, own db on 5435.
# The consumer reacts to order.confirmed and opens a shipment; the api advances
# it. Both emit to fulfillment.events.
# ============================================================================
FULFILLMENT_DIR := services/fulfillment
FULFILLMENT_DATABASE_URL ?= postgres://fulfillment:fulfillment@localhost:5435/fulfillment?sslmode=disable

.PHONY: ful-tidy ful-build ful-test ful-run-api ful-run-consumer ful-smoke

ful-tidy:
	cd $(FULFILLMENT_DIR) && go mod tidy

ful-build:
	cd $(FULFILLMENT_DIR) && \
		go build -o ../../bin/fulfillment-api ./cmd/api && \
		go build -o ../../bin/fulfillment-migrate ./cmd/migrate && \
		go build -o ../../bin/fulfillment-consumer ./cmd/consumer

ful-test:
	cd $(FULFILLMENT_DIR) && go test ./...

ful-run-api:
	cd $(FULFILLMENT_DIR) && DATABASE_URL="$(FULFILLMENT_DATABASE_URL)" KAFKA_BROKERS="$(KAFKA_BROKERS)" go run ./cmd/api

ful-run-consumer:
	cd $(FULFILLMENT_DIR) && DATABASE_URL="$(FULFILLMENT_DATABASE_URL)" KAFKA_BROKERS="$(KAFKA_BROKERS)" go run ./cmd/consumer

ful-smoke:
	@echo "Fulfillment smoke test (stack up: make compose-up). API on :8083."
	@echo "The consumer opens a shipment when an order is confirmed; the API advances it."
	@echo ""
	@echo "1. Seed stock, place an order, and let the saga confirm it (approve path):"
	@echo "   curl -s -XPUT localhost:8081/v1/stock/SKU-CLASSIC-TEE -H 'Content-Type: application/json' -d '{\"available\":100}'"
	@echo "   curl -s -XPOST localhost:8080/v1/orders -H 'Content-Type: application/json' -H 'Idempotency-Key: ful-demo-1' -d @docs/sample-order.json"
	@echo "2. The fulfillment consumer reacts to order.confirmed. Read the shipment (status CREATED):"
	@echo "   curl -s localhost:8083/v1/shipments/<order-id-from-step-1>"
	@echo "3. Dispatch it (status DISPATCHED, emits shipment.dispatched):"
	@echo "   curl -s -XPOST localhost:8083/v1/shipments/<order-id>/dispatch -H 'Content-Type: application/json' -d '{\"carrier\":\"UPS\",\"trackingCode\":\"1Z999\"}'"
	@echo "4. Deliver it (status DELIVERED, emits shipment.delivered):"
	@echo "   curl -s -XPOST localhost:8083/v1/shipments/<order-id>/deliver"
	@echo "5. Read it back to confirm the terminal state:"
	@echo "   curl -s localhost:8083/v1/shipments/<order-id>"
	@echo ""
	@echo "Creation is idempotent on order id, so a redelivered order.confirmed is a no-op."
	@echo "Re-dispatch or re-deliver is an idempotent no-op; illegal jumps (deliver before"
	@echo "dispatch) return 409 INVALID_STATE. Tail events with the console consumer:"
	@echo "   docker compose exec kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic fulfillment.events --from-beginning"

# ============================================================================
# Notification service (Phase 5). Go module; pure consumer, own db on 5436, no
# HTTP surface. Fans in orders.events, payments.events, and fulfillment.events
# and records one deduped notification per event (simulated send is a log line).
# ============================================================================
NOTIFICATION_DIR := services/notification
NOTIFICATION_DATABASE_URL ?= postgres://notification:notification@localhost:5436/notification?sslmode=disable

.PHONY: ntf-tidy ntf-build ntf-test ntf-run-consumer ntf-smoke

ntf-tidy:
	cd $(NOTIFICATION_DIR) && go mod tidy

ntf-build:
	cd $(NOTIFICATION_DIR) && \
		go build -o ../../bin/notification-consumer ./cmd/consumer && \
		go build -o ../../bin/notification-migrate ./cmd/migrate

ntf-test:
	cd $(NOTIFICATION_DIR) && go test ./...

ntf-run-consumer:
	cd $(NOTIFICATION_DIR) && DATABASE_URL="$(NOTIFICATION_DATABASE_URL)" KAFKA_BROKERS="$(KAFKA_BROKERS)" go run ./cmd/consumer

ntf-smoke:
	@echo "Notification smoke test (stack up: make compose-up). No HTTP surface;"
	@echo "you observe it through its logs and its notifications table."
	@echo ""
	@echo "1. Drive activity: place an order and let the saga confirm it, then advance"
	@echo "   its shipment (see saga-smoke and ful-smoke). Notifications flow as events land."
	@echo "2. Watch the consumer log a simulated send per event:"
	@echo "   docker compose logs -f notification-consumer"
	@echo "   (look for \"notification sent\" lines across order, payment, and shipment events)"
	@echo "3. Inspect the recorded, deduped notifications:"
	@echo "   docker compose exec notification-postgres \\"
	@echo "     psql -U notification -d notification \\"
	@echo "     -c \"SELECT event_type, order_id, subject FROM notifications ORDER BY created_at;\""
	@echo ""
	@echo "Dedup is on (event_type, order_id): replaying an event or restarting the"
	@echo "consumer records no duplicate rows and sends nothing twice. Unknown event"
	@echo "types are ignored, so adding a producer needs no change here until you map it."
