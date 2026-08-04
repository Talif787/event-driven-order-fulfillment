# CI/CD

GitHub Actions for the order-fulfillment platform (Phase 6c). Path-filtered so a
change to one service does not rebuild the others.

## Workflows

- `ci.yml` runs on every pull request and on push to main. It detects which
  paths changed and runs only the relevant jobs: Go services (gofmt check, vet,
  build, test) on a matrix built from the changed services, the payment service
  (mvn test), Terraform (fmt, init without backend, validate), and Helm (lint
  and template every service).
- `release.yml` runs on push to main. For each changed service it calls the
  reusable builder to build and push that service's image targets to GHCR,
  tagged with the commit SHA and latest.
- `_docker-build.yml` is the reusable image builder: one service in, its
  Dockerfile targets built and pushed with layer caching.
- `deploy.yml` is a manual, gated deploy. It runs helm upgrade for every service
  against the cluster in the KUBECONFIG secret. It is guarded by a GitHub
  Environment (so it can require a reviewer) and stays inert until you add the
  secret and point it at a cluster.

## Registry

Images go to GitHub Container Registry at
`ghcr.io/<owner>/order-fulfillment/<service>-<target>`, which matches the Helm
`global.registry`. Pushing uses the built-in `GITHUB_TOKEN`, so no external
credentials are needed. The first push creates each package as private; make
them public or grant the cluster a pull secret to deploy.

## Enabling deploy

`deploy.yml` needs a `KUBECONFIG` repository secret (base64 of a kubeconfig for
the target cluster) and a `dev` Environment. Until then, CI and release run on
their own; deploy is a no-op you trigger by hand once a cluster exists.
