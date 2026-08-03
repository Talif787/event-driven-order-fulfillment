# Event-Driven E-Commerce Order and Fulfillment System

A production-oriented, event-driven order and fulfillment platform. This
repository is being built in phases. Each phase is independently buildable and
testable.

## Status

Phase 1 (delivered): the Order service order-intake vertical slice.

- Event-sourced Order aggregate with full domain invariants and unit tests.
- PlaceOrder use case with idempotency.
- Postgres event store plus a transactional outbox, written atomically, with
  optimistic concurrency.
- REST API, JWT bearer auth (toggleable), correlation ids, structured JSON
  logging, panic recovery, and OpenTelemetry tracing.
- Liveness and readiness probes, env-driven Twelve-Factor config, embedded SQL
  migrations with an up/down runner.
- Unit and Testcontainers integration tests, a multi-stage distroless
  Dockerfile, docker-compose, a Makefile, and an OpenAPI 3.1 specification.

Phase 2 (this drop): the event backbone walking skeleton.

- A Kafka producer (segmentio/kafka-go) that publishes with acknowledgement from
  all in-sync replicas and partitions by aggregate id, so events for one order
  keep their order.
- An outbox relay worker (`cmd/relay`) that drains the transactional outbox to
  Kafka and marks rows published, with at-least-once delivery.
- The first consumer: an order projection worker (`cmd/projector`) that builds a
  denormalized read model from the event stream.
- `GET /v1/orders/{id}` now reads the projection first (the CQRS read path) and
  falls back to folding the event store when the projection has not yet caught
  up, preserving read-your-writes.
- A shared integration-event contract (`internal/contracts`) so producer and
  consumers agree on the wire format. This becomes the schema-registry subject
  later.
- Kafka, relay, and projector added to the local compose stack.

Phase 3 (this drop): the Inventory service.

- A new bounded context with its own Go module and its own database (database
  per service), built as an independently buildable and testable vertical slice.
- Reservation lifecycle: reserve (HELD), then commit (COMMITTED) or release
  (RELEASED, the saga compensation). Reserving is all-or-nothing across lines.
- Optimistic concurrency on the stock ledger: a version column guards every
  write, and conflicting writes retry before surfacing a 409.
- Idempotency keyed on order id (reserve, release, and commit are all
  idempotent), enforced by a unique constraint plus idempotent aggregate
  transitions.
- REST API, toggleable JWT auth, correlation ids, structured logging, panic
  recovery, OpenTelemetry, health probes, embedded migrations, unit and
  Testcontainers tests, a multi-stage distroless Dockerfile, and compose wiring
  (inventory API on :8081, its Postgres on :5433).

The application use cases are transport-agnostic. The saga orchestrator will
call inventory synchronously, so a gRPC adapter over the same use cases is the
planned next increment; it is additive and leaves the domain untouched.

## Design decision: polling relay now, Debezium later

The outbox relay in this phase is the polling-publisher variant of the
transactional outbox pattern. A worker selects unpublished rows (with
`FOR UPDATE SKIP LOCKED` so multiple relay replicas can run), publishes them,
and marks them published, all in one transaction. If publication fails the
transaction rolls back and the rows are retried, which is what makes delivery
at-least-once.

The target architecture (see `docs/architecture.md`, ADR-2) uses log-based
change data capture with Debezium for higher throughput and lower database load.
That swap is a drop-in: the outbox table and the Kafka topic contract are
identical, so consumers do not change. The polling relay was chosen for the
walking skeleton because it is real, testable application code that runs without
standing up Kafka Connect or reconfiguring Postgres for logical replication.

## Important build note

This code was authored in an environment without the Go toolchain or network
access, so it was not compiled there. Before first build you must resolve
dependencies and generate `go.sum`:

```bash
cd services/order
go mod tidy
```

That step requires network access to fetch modules (including
`github.com/segmentio/kafka-go`). After it completes, the build, tests, Docker
images, and compose stack work as documented.

## Prerequisites

- Go 1.23 or newer
- Docker (for the integration tests and the local stack)

## Quick start (full local stack)

```bash
# from the repository root, after running go mod tidy in services/order
docker compose up --build -d
```

This starts Postgres and Kafka, applies migrations, and starts the API on
`http://localhost:8080`, the outbox relay, and the projection consumer.

## End-to-end smoke test

With the stack up, this exercises the whole path: place an order, watch the
relay publish it to Kafka, and read it back from the projection.

```bash
# 1. place an order
curl -sS -X POST http://localhost:8080/v1/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: smoke-1' \
  -d @docs/sample-order.json

# 2. see the event on the topic (the relay published it)
docker compose exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 --topic orders.events \
  --from-beginning --max-messages 1

# 3. read the order back; once the projector has consumed the event this is
#    served from the projection, otherwise from the event-store fallback
curl -sS http://localhost:8080/v1/orders/<orderId-from-step-1>
```

`make smoke` prints these steps.

## Local development

```bash
cd services/order
go mod tidy          # once, to resolve deps and write go.sum
go test ./...        # unit tests (no Docker required)
RUN_INTEGRATION=1 go test ./test/integration/...   # integration tests (Docker)

# run the pieces against a local Postgres and the compose Kafka
export DATABASE_URL='postgres://order:order@localhost:5432/order?sslmode=disable'
export KAFKA_BROKERS='localhost:29092'   # host-facing listener from compose
go run ./cmd/migrate up
go run ./cmd/api
go run ./cmd/relay
go run ./cmd/projector
```

The Makefile at the repository root wraps these: `make tidy`, `make test`,
`make test-integration`, `make run`, `make run-relay`, `make run-projector`,
`make migrate-up`, `make compose-up`, `make smoke`.

## Configuration

All configuration is environment based. See
`services/order/config/config.example.env` for the complete list with defaults.
Notable values:

| Variable | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | API listen address |
| `DATABASE_URL` | local dsn | Postgres connection string |
| `AUTH_ENABLED` | `false` | Enable JWT bearer auth |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty | Enables tracing when set |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated brokers (compose services use `kafka:9092`, host tools use `localhost:29092`) |
| `RELAY_POLL_INTERVAL` | `1s` | Outbox poll cadence when idle |
| `RELAY_BATCH_SIZE` | `100` | Max outbox rows drained per cycle |
| `PROJECTOR_GROUP_ID` | `order-projection` | Consumer group id |
| `PROJECTOR_TOPICS` | `orders.events` | Topics the projector consumes |

## Project structure

```
order-fulfillment/
  go.work                     Go workspace (adds services as phases land)
  docker-compose.yml          Local stack: postgres, kafka, migrate, api, relay, projector
  Makefile                    Common developer tasks
  docs/
    openapi.yaml              OpenAPI 3.1 contract
    architecture.md           Architecture notes and ADRs
    sample-order.json         Smoke-test request body
  services/order/
    cmd/api                   API composition root
    cmd/migrate               Embedded migration runner
    cmd/relay                 Outbox relay worker
    cmd/projector             Order projection consumer
    internal/contracts        Versioned integration event schemas (Kafka wire format)
    internal/domain/order     Event-sourced aggregate, value objects, events
    internal/app              Use cases and ports (command, query, projection)
    internal/infra            Postgres, kafka, config, logging, telemetry adapters
    internal/worker           Relay and projector run loops
    internal/presentation     HTTP edge, middleware, health
    test/integration          Testcontainers integration tests
```

## Testing

- Unit tests cover domain invariants, the PlaceOrder handler, and the
  integration-event contract round trip.
- Integration tests start real Postgres via Testcontainers and cover: placement
  persistence and idempotency; the relay draining the outbox, marking rows
  published, and rolling back (leaving rows unpublished) when publication fails;
  and the projection applying `order.placed.v1` idempotently.

The relay and projection logic are tested against Postgres with a fake
publisher, and the wire contract is unit tested. The broker round trip itself
(producer to real Kafka to consumer) is verified by the compose smoke test above
rather than an automated broker test, to keep the suite fast and hermetic. An
automated end-to-end Kafka test can be added next if wanted.

## Roadmap

- Event backbone hardening: saga orchestrator with compensations, the Cancel
  command, the schema registry, and a dead-letter topic for poison messages.
- Phase 3: Inventory service (Go) with reservations and optimistic concurrency.
- Phase 4: Payment service (Java, Spring Boot) with idempotent capture and
  webhook reconciliation.
- Phase 5: Fulfillment and Notification services.
- Phase 6: Kubernetes manifests and Helm chart, Terraform, CI/CD, and the full
  observability stack.
