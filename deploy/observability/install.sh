#!/usr/bin/env bash
# Install the observability stack into the observability namespace.
# Idempotent: re-running upgrades in place. Requires helm and kubectl with a
# cluster context already selected.
set -euo pipefail

cd "$(dirname "$0")"

kubectl apply -f namespace.yaml

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
helm repo update

# Jaeger first, so the collector's export target exists when it starts.
helm upgrade --install jaeger jaegertracing/jaeger \
  -n observability -f jaeger.values.yaml --wait

helm upgrade --install otel-collector open-telemetry/opentelemetry-collector \
  -n observability -f otel-collector.values.yaml --wait

helm upgrade --install kps prometheus-community/kube-prometheus-stack \
  -n observability -f kube-prometheus-stack.values.yaml --wait

# Load the custom dashboard: the Grafana sidecar imports any ConfigMap labelled
# grafana_dashboard.
kubectl create configmap order-fulfillment-overview \
  -n observability \
  --from-file=order-fulfillment-overview.json=dashboards/order-fulfillment-overview.json \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl label configmap order-fulfillment-overview \
  -n observability grafana_dashboard=1 --overwrite

echo
echo "Installed. Port-forward to reach the UIs:"
echo "  Grafana:  kubectl -n observability port-forward svc/kps-grafana 3000:80   (admin / admin)"
echo "  Jaeger:   kubectl -n observability port-forward svc/jaeger-query 16686:16686"
echo "  Prometheus: kubectl -n observability port-forward svc/kps-prometheus 9090:9090"
