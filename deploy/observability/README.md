# Observability

The monitoring and tracing stack for the platform (Phase 6d, cluster slice).
Three upstream charts wired together, installed into the `observability`
namespace.

## What runs

- kube-prometheus-stack: Prometheus, Grafana, node-exporter, kube-state-metrics.
  Prometheus is configured to discover ServiceMonitors across all namespaces, so
  it scrapes the app services once their metrics are enabled.
- OpenTelemetry Collector: receives OTLP traces from the services (the endpoint
  they already point at, otel-collector.observability:4318) and forwards them to
  Jaeger.
- Jaeger: all-in-one with in-memory storage for dev, collector plus query UI.

## Data flow

- Metrics: each service exposes /metrics, Prometheus scrapes it via a
  ServiceMonitor (rendered by the app chart when metrics are enabled), Grafana
  reads Prometheus.
- Traces: services export OTLP to the collector, the collector forwards to
  Jaeger, viewable in the Jaeger UI or through the Jaeger datasource in Grafana.

## Install

```bash
./install.sh
```

Or step through the same commands manually; it adds the three Helm repos and
upgrades each release with the values here.

## Verify without applying

The values follow each chart's documented schema. Render them offline first:

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
helm repo update
helm template kps prometheus-community/kube-prometheus-stack -f kube-prometheus-stack.values.yaml >/dev/null
helm template otel-collector open-telemetry/opentelemetry-collector -f otel-collector.values.yaml >/dev/null
helm template jaeger jaegertracing/jaeger -f jaeger.values.yaml >/dev/null
```

Service names (jaeger-query, jaeger-collector, kps-grafana) come from the charts;
confirm them with `kubectl get svc -n observability` after install and adjust the
datasource URL or port-forward targets if a chart version names them differently.

## Metric conventions

The custom dashboard and the next slice's instrumentation agree on these names:

- `http_requests_total{service,method,path,status}` and
  `http_request_duration_seconds_bucket{service,method,path}` on the HTTP
  services.
- `saga_outcomes_total{outcome}` from the orchestrator.
- `shipments_total{transition}` from fulfillment.
- `notifications_sent_total{event_type}` from notification.
- `events_consumed_total{service,topic}` from the consumers.

Until the app slice lands these, the dashboard panels render empty. The
kube-prometheus-stack bundled Kubernetes dashboards populate immediately.
