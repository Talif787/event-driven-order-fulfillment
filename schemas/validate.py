#!/usr/bin/env python3
"""Validate every event JSON Schema and check that each example conforms.

This is the enforcement half of the event-contract governance: it keeps the
published schemas well-formed and keeps the checked-in examples honest, so a
schema change that would break an example (or an example that drifts from its
schema) fails CI instead of reaching consumers.
"""
import json
import pathlib
import sys

from jsonschema import Draft202012Validator

root = pathlib.Path(__file__).parent
examples = root / "examples"
failures = []

schema_paths = sorted(root.glob("*.schema.json"))
for schema_path in schema_paths:
    schema = json.loads(schema_path.read_text())

    # The schema itself must be a valid Draft 2020-12 schema.
    try:
        Draft202012Validator.check_schema(schema)
    except Exception as exc:  # noqa: BLE001
        failures.append(f"{schema_path.name}: invalid schema: {exc}")
        continue

    # Its matching example must conform.
    example_name = schema_path.name.replace(".schema.json", ".json")
    example_path = examples / example_name
    if not example_path.exists():
        failures.append(f"{schema_path.name}: missing example {example_name}")
        continue

    example = json.loads(example_path.read_text())
    for err in sorted(Draft202012Validator(schema).iter_errors(example), key=lambda e: list(e.path)):
        failures.append(f"{example_name}: {list(err.path)}: {err.message}")

if failures:
    print("Schema validation FAILED:")
    for failure in failures:
        print("  -", failure)
    sys.exit(1)

print(f"Validated {len(schema_paths)} event schemas and their examples: OK")
