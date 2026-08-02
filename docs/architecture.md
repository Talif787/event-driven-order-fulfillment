# Order Service Architecture (Phase 1)

The order service is the first vertical slice of the Event-Driven E-Commerce
Order and Fulfillment System. Phase 1 delivers a complete, independently
buildable and testable order-intake path. Downstream coordination (Kafka
publication, saga orchestration, and the inventory, payment, fulfillment, and
notification services) arrives in later phases.

## Layering (Clean / Hexagonal)

Dependencies point inward. The domain has no knowledge of HTTP, Postgres, or
any framework.

- `internal/domain/order`: the event-sourced Order aggregate, value objects,
  domain events, and domain errors. Pure business logic, no I/O.
- `internal/app`: use cases (PlaceOrder command, GetOrder query) plus the ports
  (`Repository`, `IdempotencyStore`, `IDGenerator`, `Clock`) that the domain and
  application depend on but do not implement.
- `internal/infra`: adapters that implement the ports (Postgres event store and
  outbox, idempotency store), plus configuration, structured logging, and
  OpenTelemetry setup.
- `internal/presentation/http`: the REST edge, middleware (correlation id,
  recovery, logging, JWT auth), DTOs, the error envelope, and health probes.
- `cmd/api` and `cmd/migrate`: composition roots (manual dependency injection)
  and the embedded migration runner.

## Key patterns

- **Event sourcing** for the order aggregate: state is a fold over
  `order_events`, giving a native audit trail and a foundation for replay.
- **Transactional outbox**: the domain event and its outbox row are written in
  one database transaction, so state change and event emission are atomic. A
  change-data-capture relay publishes the outbox to Kafka in Phase 2.
- **Idempotency**: an `Idempotency-Key` header plus a uniquely constrained
  `idempotency_keys` table make retried submissions safe. A replay returns the
  original order and persists nothing new.
- **Optimistic concurrency**: the unique `(order_id, version)` constraint
  rejects conflicting concurrent appends, surfaced as a 409.

## What is intentionally deferred

Phase 1 does not publish to Kafka, run the saga, or reserve inventory. The
outbox is written to Postgres only. This keeps the phase self-contained and
compilable without a broker, while the outbox rows are exactly what the Phase 2
relay will publish.
