# Deployment

This system runs six services (order, inventory, payment, fulfillment, plus the
order saga and notification workers) on Kafka, with a Postgres per service. That
is a heavyweight, always-on footprint. It has two deployment paths: a free,
on-demand launch of the real system, and a paid, production-grade path on AWS
that this repository already encodes.

## Why these two paths

A permanently-running deployment of six always-on services plus a Kafka broker
cannot be free: free tiers are designed for workloads that scale to zero or use
minimal resources, which a standing Kafka cluster and multiple long-lived
services do not. Rather than refactor the architecture to fit a free tier (which
would deploy a different system than the one built here), the free path deploys
the true system on demand, and the paid path deploys it continuously.

## Path 1: free, on-demand launch (full parity)

The repository is itself a one-command deployment via a dev container. In GitHub
Codespaces (free monthly hours) or any dev-container host:

1. Open the repository in a Codespace ("Code" -> "Codespaces" -> "Create").
2. The dev container builds the images on creation and runs `docker compose up`
   on start, so the whole stack comes up automatically. First build takes a few
   minutes (it compiles four Go services and one Java service).
3. When it attaches, a welcome banner prints the seed and demo commands.

This launches the exact architecture, unchanged: all six services, Kafka, and
the per-service databases, reachable on ports 8080 to 8083. It is free within
Codespaces' monthly included hours and reproducible by anyone from the repo.

Config lives in `.devcontainer/`. The `hostRequirements` request a 4-core
machine because the multi-service build needs the headroom.

### Connecting the web console

The console is a separate repository deployed to Cloudflare Pages (see that
repo's DEPLOYMENT.md). To point it at a launched backend, make ports 8080 to
8083 Public in the Codespace Ports tab and set the console's service URLs to
those public forwarded URLs. The services send CORS headers, so the browser
calls them cross-origin directly. This is the same split-origin model the
console already uses.

## Path 2: production on AWS (paid, already coded)

The production-grade path is in this repository and is deliberately gated off so
it costs nothing until applied:

- `infra/terraform/` provisions the AWS footprint (network, EKS, and supporting
  resources). It validates and plans for free; `terraform apply` is what incurs
  cost.
- `deploy/helm/` is a reusable chart that deploys every service with pod
  security, autoscaling (HPA), disruption budgets (PDB), and migrate hooks.
- `.github/workflows/` builds and pushes images to GHCR and has a gated deploy
  workflow.

For production, the in-cluster Postgres and Kafka used by compose would be
replaced by managed services (RDS or Aurora for Postgres, MSK or Confluent Cloud
for Kafka) and secrets moved to AWS Secrets Manager or SSM. Those substitutions
are configuration, not code changes.

## What is free vs. what costs

- Free, permanent: the web console on Cloudflare Pages (separate repo).
- Free, on-demand: the full backend via Codespaces, within included hours.
- Paid, on apply: the AWS production path (EKS, RDS, MSK). Estimate before
  applying; a minimal EKS-plus-RDS-plus-MSK footprint is on the order of low
  hundreds of USD per month and should be torn down when not demoing.

## What requires manual action

- Creating the Codespace (or dev-container host) to launch the backend.
- Making the Codespace ports Public to reach them from the deployed console.
- Creating the Cloudflare Pages project and setting the backend URLs (console
  repo).
- For AWS: supplying credentials and running `terraform apply` / the deploy
  workflow. Nothing here applies AWS infrastructure automatically.

Nothing in this repository deploys paid infrastructure on its own. The free
paths are safe to run; the paid path requires your explicit credentials and
apply.
