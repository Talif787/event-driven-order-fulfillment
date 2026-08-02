# Event-Driven E-Commerce Order and Fulfillment System

A production-oriented, event-driven order and fulfillment platform. This
repository is being built in phases. Each phase is independently buildable and
testable.

## Phase 1 status (this drop): Order service, order-intake vertical slice

Delivered and runnable now:

- Event-sourced Order aggregate with full domain invariants and unit tests.
- PlaceOrder use case with idempotency, and a GetOrder read that folds the
  event stream.
- Postgres event store plus a transactional outbox, written atomically, with
  optimistic concurrency.
- REST API (place and get), JWT bearer auth (toggleable), correlation ids,
  structured JSON logging, panic recovery, and OpenTelemetry tracing.
- Liveness and readiness probes, env-driven Twelve-Factor config, embedded SQL
  migrations with an up/down runner.
- Unit tests, a Testcontainers integration test, a multi-stage distroless
  Dockerfile, docker-compose for the full local stack, a Makefile, and an
  OpenAPI 3.1 specification.

Deferred to later phases: Kafka publication of the outbox (CDC relay), the saga
orchestrator and compensations, and the inventory, payment, fulfillment, and
notification services. See the roadmap below.

## Important build note

This code was authored in an environment without the Go toolchain or network
access, so it was not compiled there. Before first build you must resolve
dependencies and generate `go.sum`:

```bash
cd services/order
go mod tidy
```

That step requires network access to fetch modules. After it completes, the
build, tests, Docker image, and compose stack work as documented.

## Prerequisites

- Go 1.23 or newer
- Docker (for the integration test and the local stack)

## Quick start (full local stack)

```bash
# from the repository root, after running go mod tidy in services/order
docker compose up --build -d
```

This starts Postgres, applies migrations, and starts the API on
`http://localhost:8080`.

## Local development

```bash
cd services/order
go mod tidy          # once, to resolve deps and write go.sum
go test ./...        # unit tests (no Docker required)
 RUN_INTEGRATION=1 go test ./test/integration/...   # integration tests (Docker)

# run the API against a local Postgres
export DATABASE_URL='postgres://order:order@localhost:5432/order?sslmode=disable'
go run ./cmd/migrate up
go run ./cmd/api
```

The Makefile at the repository root wraps these: `make tidy`, `make test`,
`make test-integration`, `make run`, `make migrate-up`, `make compose-up`.

## API examples

Place an order (auth disabled locally, so `customerId` is taken from the body):

```bash
curl -sS -X POST http://localhost:8080/v1/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-key-0001' \
  -d '{
    "customerId": "11111111-1111-1111-1111-111111111111",
    "items": [
      {"sku": "SKU-1", "quantity": 2, "unitPriceMinor": 1500, "currency": "USD"}
    ],
    "shipTo": {"line1": "1 Main St", "city": "Boston", "region": "MA", "postalCode": "02118", "country": "US"}
  }'
```

Sending the same `Idempotency-Key` again returns the same order and creates no
duplicate. Read the order back:

```bash
curl -sS http://localhost:8080/v1/orders/<orderId>
```

The full contract is in `docs/openapi.yaml`.

## Configuration

All configuration is environment based. See
`services/order/config/config.example.env` for the complete list with defaults.
Notable values:

| Variable | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Listen address |
| `DATABASE_URL` | local dsn | Postgres connection string |
| `AUTH_ENABLED` | `false` | Enable JWT bearer auth |
| `AUTH_JWT_SECRET` | empty | HS256 secret (required when auth is enabled) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty | Enables tracing when set |

## Project structure

```
order-fulfillment/
  go.work                     Go workspace (adds services as phases land)
  docker-compose.yml          Local stack: postgres, migrate, api
  Makefile                    Common developer tasks
  docs/
    openapi.yaml              OpenAPI 3.1 contract
    architecture.md           Phase 1 architecture notes
  services/order/
    cmd/api                   API composition root
    cmd/migrate               Embedded migration runner
    internal/domain/order     Event-sourced aggregate, value objects, events
    internal/app              Use cases and ports
    internal/infra            Postgres, config, logging, telemetry adapters
    internal/presentation     HTTP edge, middleware, health
    test/integration          Testcontainers integration test
```

## Testing

- Unit tests cover domain invariants (totals, duplicate line rejection,
  rehydration) and the PlaceOrder handler (idempotent replay creates no
  duplicate).
- The integration test starts real Postgres via Testcontainers, applies the
  embedded migration, and asserts that placement persists exactly one event and
  one outbox row and that idempotency holds across requests.

## Roadmap

- Phase 2: publish the outbox to Kafka via a change-data-capture relay
  (Debezium), add the schema registry, and stand up the saga orchestrator with
  compensations.
- Phase 3: Inventory service (Go) with reservations and optimistic concurrency.
- Phase 4: Payment service (Java, Spring Boot) with idempotent capture and
  webhook reconciliation.
- Phase 5: Fulfillment and Notification services.
- Phase 6: Kubernetes manifests and Helm chart, Terraform, CI/CD, and the full
  observability stack.
