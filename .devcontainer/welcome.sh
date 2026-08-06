#!/usr/bin/env bash
cat <<'BANNER'
============================================================
 Order Fulfillment: the full stack is starting
============================================================
Six services, Kafka, and a Postgres per service come up through
docker compose. Give it ~30s after the first start to settle.

  docker compose ps                  # all services Up / healthy

Seed stock so orders confirm:
  curl -X PUT localhost:8081/v1/stock/SKU-100 \
    -H 'content-type: application/json' -d '{"available": 100}'

Place an order and watch it flow:
  oid=$(curl -s -X POST localhost:8080/v1/orders \
    -H 'content-type: application/json' \
    -H "Idempotency-Key: $(cat /proc/sys/kernel/random/uuid)" \
    -d '{"customerId":"1a2b3c4d-5e6f-4071-8182-93a4b5c6d7e8","items":[{"sku":"SKU-100","quantity":2,"unitPriceMinor":1499,"currency":"USD"}],"shipTo":{"line1":"1 Main St","line2":"","city":"Boston","region":"MA","postalCode":"02118","country":"US"}}' \
    | python3 -c 'import sys,json;print(json.load(sys.stdin)["orderId"])')
  sleep 5
  curl -s localhost:8080/v1/orders/$oid; echo      # CONFIRMED
  curl -s localhost:8083/v1/shipments/$oid; echo   # shipment CREATED

To connect the web console: make ports 8080-8083 Public in the Ports
tab, then point the console at their URLs. See DEPLOYMENT.md.
============================================================
BANNER
