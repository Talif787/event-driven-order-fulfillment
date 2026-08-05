# Event schemas

The published contracts for the events that cross service boundaries. This is a
lightweight schema registry: the wire format stays JSON, and these files are the
versioned, machine-checkable source of truth for what each event looks like.

## Layout

- `<event>.schema.json`: a JSON Schema (draft 2020-12) for one event, named by
  its event type (for example `order.placed.v1.schema.json`). `additionalProperties`
  is false, so an unexpected field is a contract violation, not a silent addition.
- `examples/<event>.json`: one valid example per event, used as a golden sample.
- `validate.py`: checks every schema is a valid draft 2020-12 schema and that
  each example conforms. CI runs this on any change under `schemas/`.

## Covered events

- Order: `order.placed.v1`, `order.confirmed.v1`, `order.cancelled.v1`.
- Payment: `payment.event.v1`. The five payment event types
  (`payment.captured/declined/settled/refunded/failed.v1`) share one shape,
  distinguished by the `status` field, so they share this one schema.
- Shipment: `shipment.created.v1`, `shipment.dispatched.v1`,
  `shipment.delivered.v1`, `shipment.failed.v1`.

## Evolving a contract

The version is part of the event type and the file name. A backward-compatible
change (adding an optional field) can extend the existing schema. A breaking
change (removing or renaming a field, tightening a type) ships as a new version:
add `order.placed.v2` with its own schema and example, publish both until every
consumer has migrated, then retire v1. The `format` entries (`uuid`, `date-time`)
are documentary hints; `validate.py` enforces structure (types, required fields,
and no unexpected properties).
