# Inventory Service

Reservation-based inventory management for the order and fulfillment platform.
The service holds, releases, and commits stock against orders, with optimistic
concurrency on the ledger and order-keyed idempotency on reservations.

This is its own Go module and its own database (database per service). It runs
independently of the Order service.

## What it does

A reservation is a hold on stock for one order. The lifecycle is:

- Reserve: available stock decreases and reserved stock increases, atomically
  across every line. The reservation starts HELD. Reserving is all-or-nothing:
  if any line lacks stock, nothing is held.
- Commit: the order shipped, so the held stock leaves the warehouse. Reserved
  decreases; the reservation becomes COMMITTED.
- Release: the order was cancelled or failed downstream, so held stock returns
  to available. The reservation becomes RELEASED. This is the saga compensation.

Reserve, release, and commit are all idempotent. A repeat reserve for the same
order returns the existing reservation without decrementing stock again; a
repeat release or commit is a no-op that returns the current state.

## Optimistic concurrency

Each `stock_items` row carries a `version`. Every mutating write is issued as
`UPDATE ... SET version = version + 1 WHERE sku = $sku AND version = $observed`.
If a concurrent writer moved the version first, the update matches zero rows and
the whole transaction retries (bounded by a retry count) before surfacing a
409 conflict. This keeps writes correct under contention without holding row
locks across the request.

Reservation creation is made idempotent by a unique constraint on
`reservations.order_id`: a losing racer sees the unique violation and returns the
winner's reservation.

## Design note: transport

The application use cases are transport-agnostic. This drop ships an HTTP
adapter, which builds with no code generation. The saga orchestrator will call
inventory synchronously, so a gRPC adapter over the same use cases is the
natural next increment; it is additive and does not touch the domain or
application layers.

## API

All bodies are JSON. Auth is a toggleable bearer JWT (disabled by default for
local development). Every response carries an `X-Correlation-Id`.

- `PUT  /v1/stock/{sku}` seed or set available stock (admin). Body:
  `{"available": 100}`.
- `GET  /v1/stock/{sku}` read the ledger: available, reserved, version.
- `POST /v1/reservations` hold stock. Body:
  `{"orderId": "<uuid>", "lines": [{"sku": "SKU-1", "quantity": 3}]}`.
  Returns 201 on creation, 200 on idempotent replay.
- `GET  /v1/reservations/{orderId}` read a reservation.
- `POST /v1/reservations/{orderId}/release` release a held reservation.
- `POST /v1/reservations/{orderId}/commit` commit a held reservation.
- `GET  /livez`, `GET /readyz`, `GET /healthz` probes.

Error envelope: `{"error": {"code": "...", "message": "...", "correlationId": "..."}}`.
Codes: `VALIDATION_ERROR` (400), `NOT_FOUND` (404), `INSUFFICIENT_STOCK` (409),
`CONFLICT` (409, concurrency), `INVALID_STATE` (409, illegal transition).

## Layout

- `internal/domain/inventory` StockItem and Reservation aggregates, value
  objects, invariants. No infrastructure dependencies.
- `internal/app` transport-agnostic use cases and the persistence port.
- `internal/infra/postgres` the store: transactional reserve/release/commit with
  optimistic concurrency and idempotency, plus embedded migrations.
- `internal/infra/{config,logging,telemetry}` Twelve-Factor config, structured
  logging, OpenTelemetry setup.
- `internal/presentation/http` the HTTP adapter: router, handlers, middleware,
  error mapping.
- `cmd/api`, `cmd/migrate` the API server and the migration runner.
- `test/integration` Testcontainers tests for the full reservation lifecycle and
  a deterministic optimistic-concurrency proof.

## Running

From the repository root:

```
make inv-tidy              # resolve dependencies (requires network)
make inv-test              # unit tests (no Docker)
make inv-test-integration  # integration tests (requires Docker)
make compose-up            # start the whole stack; inventory API on :8081
make inv-smoke             # print the reservation smoke-test steps
```

Local defaults: API on `:8081`, Postgres on `:5433`, database, user, and
password all `inventory`.
