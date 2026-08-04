# Deploy

Kubernetes and Helm for the order-fulfillment platform (Phase 6a).

## Layout

- `helm/app` is a reusable application chart. One release per bounded context,
  parameterized by a list of workloads, so a service with several processes
  (order runs api, relay, projector, orchestrator) deploys from one values file.
- `helm/values/<service>.yaml` holds each service's values.
- `k8s/namespace.yaml` creates the `order-fulfillment` namespace and enforces
  the restricted Pod Security Standard.

The chart assumes managed Kafka and Postgres (provisioned by the Terraform
phase), reached over connection strings supplied through a per-service Secret.
It does not deploy in-cluster databases.

## What the chart renders

Per workload: a Deployment (hardened pod and container security contexts,
rolling updates with zero unavailable, zone topology spread), a Service and
HTTP probes when the workload declares a port, an HPA when autoscaling is on,
and a PodDisruptionBudget when it runs more than one replica. Per release: a
ConfigMap from `env`, a ServiceAccount (IRSA-annotated when a role ARN is set),
an optional Ingress, a migrate Job as a pre-upgrade hook, and a gated
ServiceMonitor (enabled in the observability phase).

## Secrets

Credentials never live in values. Each release reads a Secret named by
`existingSecret` (for example `order-secrets`), created out of band. For a local
cluster:

```bash
kubectl -n order-fulfillment create secret generic order-secrets \
  --from-literal=DATABASE_URL=postgres://order:order@<host>:5432/order?sslmode=disable
```

In a real cluster these come from the External Secrets Operator backed by AWS
Secrets Manager, wired in the Terraform phase.

## Deploy

```bash
kubectl apply -f k8s/namespace.yaml

helm upgrade --install order      helm/app -f helm/values/order.yaml        -n order-fulfillment
helm upgrade --install inventory  helm/app -f helm/values/inventory.yaml    -n order-fulfillment
helm upgrade --install payment    helm/app -f helm/values/payment.yaml      -n order-fulfillment
helm upgrade --install fulfillment helm/app -f helm/values/fulfillment.yaml -n order-fulfillment
helm upgrade --install notification helm/app -f helm/values/notification.yaml -n order-fulfillment
```

Before deploying, set `global.registry`, `global.imageTag`, each `KAFKA_BROKERS`,
and the IRSA `serviceAccount.roleArn` (all produced by the Terraform phase).

## Verify locally without a cluster

```bash
helm lint helm/app -f helm/values/order.yaml
helm template order helm/app -f helm/values/order.yaml -n order-fulfillment | kubectl apply --dry-run=client -f -
```
