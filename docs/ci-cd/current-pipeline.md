# Current CI/CD pipeline overview

This document captures what the monolithic GitHub Actions workflows currently do and how they map to each service. It will be the starting point for splitting pipelines by service.

## Workflows in `.github/workflows`

| Workflow | Trigger | Purpose | Jobs/notes |
| --- | --- | --- | --- |
| `ci.yml` (CI) | `push` to `main`, all `pull_request` | Runs Go setup, vet, build, and unit tests for every backend module in a single job. | Sequential steps for gateway, auth, crypto-core, keys, and messages. Uses `go mod tidy`, `go vet`, `go test`, and `go build` (no caching, no matrix). |
| `lint.yml` (Lint Services) | `push` to `main`, all `pull_request` | golangci-lint for each Go module. | Matrix over services/auth, gateway, crypto-core, keys, messages. Runs `golangci-lint run --timeout=5m` inside the official Docker image. |
| `publish.yml` (Build & Publish Images) | `push` to `main` and tags matching `v*` | Builds and pushes container images to GHCR. | Builds images for gateway, auth, keys, messages, and migration images for auth, keys, and messages. Uses `docker/build-push-action@v6` with cache-to/from GHA. Tags `sha-<commit>` and `latest`. |
| `deploy.yml` (Deploy self-hosted) | Manual `workflow_dispatch` or after successful `Build & Publish Images` on `main` | Pulls and deploys the full stack on a self-hosted runner using Docker Compose. | Ensures Docker Compose is present, writes `.docker/.env.prod` from secrets/vars, logs into GHCR, then pulls and starts services via `.docker/docker-compose.prod.yml`. |

## Shared/global behaviors

- All unit tests run in a single `CI` job rather than per-service jobs; failures block all services.
- Linting is centralized in one matrix job (`golangci`).
- Image publishing is centralized in one job that builds/pushes all service and migration images.
- Deployment is global: one compose-based rollout after the publish workflow completes successfully.

These are candidates for splitting so that services can build/test/publish independently while potentially keeping select global checks.

## Candidate split: per-service jobs

Based on the current workflows, each service should have the following pipeline stages when split:

| Service | Build/Test | Lint | Sonar | Docker build/publish | Security scan | Deploy |
| --- | --- | --- | --- | --- | --- | --- |
| Gateway | Go vet, unit tests, and build (from `ci.yml`) | golangci-lint matrix run | _Not present currently_ | Image build/push in `publish.yml` | _Not present currently_ | Deployed via compose in `deploy.yml` |
| Auth | Go vet, unit tests, and build (from `ci.yml`) | golangci-lint matrix run | _Not present currently_ | Image build/push in `publish.yml` + `auth-migrations` image | _Not present currently_ | Deployed via compose in `deploy.yml` |
| Crypto-core | Go unit tests (no build step) in `ci.yml` | golangci-lint matrix run | _Not present currently_ | _No image build today_ | _Not present currently_ | _Not deployed directly in compose_ |
| Keys | Go vet, unit tests, and build (from `ci.yml`) | golangci-lint matrix run | _Not present currently_ | Image build/push in `publish.yml` + `keys-migrations` image | _Not present currently_ | Deployed via compose in `deploy.yml` |
| Messages | Go vet, unit tests, and build (from `ci.yml`) | golangci-lint matrix run | _Not present currently_ | Image build/push in `publish.yml` + `messages-migrations` image | _Not present currently_ | Deployed via compose in `deploy.yml` |

## Global jobs to keep vs duplicate

- **Per-service to duplicate:**
  - Build/test steps currently in `ci.yml` should become per-service CI pipelines.
  - Lint steps from the `golangci` matrix can be split so each service owns its lint job.
  - Docker image build/push steps from `publish.yml` should be scoped per service (including migration images where they exist).
  - Deployments should trigger per service (or per stack) rather than after all images build.

- **Global to consider keeping:**
  - There is no Sonar coverage, container scanning, or Trivy/Snyk today; if introduced, they could remain global or be added per service.
  - Shared environment preparation for compose-based deployments might stay in a global “full stack” workflow if stack-level rollouts are still needed.
