# CI/CD Overview

This document explains the continuous integration and delivery setup for the **E2EE Messaging Platform**, based on the repository’s current workflow and container stack files.

It covers:
- What each GitHub Actions workflow does;
- How images are built/published to GHCR;
- How production deploys run on the self-hosted runner using Docker Compose;
- Where secrets/variables live and how `docker-compose` uses them;

---

## 1) Repository CI/CD Artifacts

- **Workflows**
  - `.github/workflows/lint.yml` — *Lint* (GolangCI-Lint) on PRs & pushes to `main`.
  - `.github/workflows/ci.yml` — *CI* (build + tests) on PRs & pushes to `main`.
  - `.github/workflows/publish.yml` — *Build & Publish Images* to GHCR on pushes to `main` and tags `v*`.
  - `.github/workflows/deploy.yml` — *Deploy (self-hosted)* on `workflow_run` after *Build & Publish Images* or manual dispatch.

- **Compose stacks**
  - `.docker/docker-compose.dev.yml` — Developer stack (Postgres, migrations, services).
  - `.docker/docker-compose.prod.yml` — Production stack (env-driven).

- **Env & config**
  - `.docker/.env.prod` — created at deploy-time (filled from GitHub **Secrets/Variables**).
  - `.env.prod.example` — sample of what’s required on the server (owner/repo/tag and secrets).
  - `.golangci.yml` — linter configuration (enabled rules, formatters).
  - `Makefile` — helpers for dev compose (up/logs/down/migrate).

> The file paths above reflect how the deploy workflow references them (e.g., `.docker/docker-compose.prod.yml`). If your repo stores them at different paths, keep the workflow in sync.

---

## 2) Pipeline at a Glance

```mermaid
flowchart LR
  A[Push or PR] --> B[Lint]
  A --> C[CI]
  C --> D[Build and Publish Images]
  D -- workflow_run completed on main --> E[Deploy on self-hosted]
  F[Manual dispatch] --> E

  classDef gray fill:#eee,stroke:#999,color:#333;
  class B gray

```

| Workflow | Triggers | Runner | Purpose | Key steps |
|---|---|---|---|---|
| **Lint** (`lint.yml`) | PRs, push to `main` | `ubuntu-latest` | Static analysis | Run `golangci-lint` (Docker) over service modules |
| **CI** (`ci.yml`) | PRs, push to `main` | `ubuntu-latest` | Compile & test | `go mod tidy`, `go vet`, `go test -v`, `go build` for services |
| **Build & Publish Images** (`publish.yml`) | Push to `main`, tags `v*` | `ubuntu-latest` | Build images & push to GHCR | `docker/build-push-action@v6` with cache to `gha`, tags `latest` and `sha-<commit>` |
| **Deploy (self-hosted)** (`deploy.yml`) | `workflow_run` after *Build & Publish*, or manual | **self-hosted** | Pull & restart compose stack | Render `.docker/.env.prod`, login GHCR, `docker compose up -d` |

---

## 3) Lint (GolangCI-Lint)

**Workflow:** `.github/workflows/lint.yml`  
**Triggers:** PRs and pushes to `main`  
**Highlights:**
- Uses a **matrix** over service modules (e.g., `services/auth`) to lint each module in isolation.
- Runs `golangci/golangci-lint:latest` container, mounting the module folder and executing `golangci-lint run --timeout=5m`.
- Uses `.golangci.yml` to enable `gocyclo`, `misspell`, `revive`, `unparam`, and formatting with `gofumpt`.

**Benefits:** single, consistent linter runtime & config; no local installation required.

---

## 4) CI (Build & Tests)

**Workflow:** `.github/workflows/ci.yml`  
**Triggers:** PRs and pushes to `main`  
**Highlights:**
- Checks out repository and sets up Go per module using `actions/setup-go@v5` with `go-version-file` (e.g., `services/auth/go.mod`, `services/gateway/go.mod`).
- For each service: `go mod tidy` → `go vet` → `go test -v ./...` → `go build ./...`.

**Outcome:** early failure surface for compile and unit tests before building images or deploying.

---

## 5) Build & Publish Images (GHCR)

**Workflow:** `.github/workflows/publish.yml`  
**Triggers:** Push to `main`; Git tags matching `v*`  
**Permissions:** `contents: read`, `packages: write`  
**Environment:**
- `REGISTRY=ghcr.io`
- `SHA_TAG=sha-${{ github.sha }}` (immutable tag per commit)

**Highlights:**
- Builds multiple images (e.g., `gateway`, `auth`, **and** `auth-migrations`) using `docker/build-push-action@v6`.
- Pushes **two tags per image**:
  - `:latest`
  - `:sha-<commit>` (immutable, used for rollbacks/pinning)
- Enables **build cache** via `cache-from: type=gha` and `cache-to: type=gha,mode=max`.
- Logs in to GHCR using the built-in `GITHUB_TOKEN`.

**Resulting image names:**
```
ghcr.io/<owner>/<repo>/gateway:<tag>
ghcr.io/<owner>/<repo>/auth:<tag>
ghcr.io/<owner>/<repo>/auth-migrations:<tag>
```
Where `<tag>` is either `latest` or `sha-<commit>`.

---

## 6) Deploy (Self-Hosted Runner + Docker Compose)

**Workflow:** `.github/workflows/deploy.yml`  
**Triggers:**  
- `workflow_run` after **Build & Publish Images** completes on `main`  
- or manual **workflow_dispatch**

**Runner:** `self-hosted` (with Docker/Compose installed)

**What it does:**
1. **Generate `.docker/.env.prod`** from GitHub **Secrets/Variables** (example values):
   ```bash
   {
     echo "REGISTRY=ghcr.io"
     echo "OWNER=${{ env.OWNER_LC }}"
     echo "REPO=${{ env.REPO_LC }}"
     echo "TAG=latest"
     # Quote secrets so '#' and spaces can't break parsing
     echo "POSTGRES_PASSWORD=\"${{ secrets.PROD_POSTGRES_PASSWORD }}\""
     echo "SIGNING_KEY=\"${{ secrets.PROD_SIGNING_KEY }}\""
     echo "CORS_ORIGINS=\"${{ vars.CORS_ORIGINS }}\""
   } > .docker/.env.prod
   ```
   A **sanitized preview** is printed (values replaced with `***`) so you can confirm keys exist.
2. **Login to GHCR** using the workflow token:
   ```bash
   echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin
   ```
3. **Pull and start** the stack with production compose:
   ```bash
   docker compose -f .docker/docker-compose.prod.yml --env-file .docker/.env.prod pull
   docker compose -f .docker/docker-compose.prod.yml --env-file .docker/.env.prod up -d
   ```
4. On failure, the workflow prints `ps` and **last 200 lines of logs** for fast diagnostics.

**Self-hosted host requirements:**
- Docker + Docker Compose
- Network access to GHCR
- User running the runner in the `docker` group (or root)

---

## 7) Compose Stacks

### Development (`.docker/docker-compose.dev.yml`)

- **Services:** `postgres`, `migrate-auth`, `migrate-keys`, `migrate-messages`, `auth`, `keys`, `messages`, `gateway`.
- **Migrations:** migration containers (auth/keys/messages) apply schemas before services start.
- **Auth env:** example values (dev secrets), healthchecks, published ports.
- **Makefile shortcuts:**
  ```Makefile
  up:         docker compose -f .docker/docker-compose.dev.yml up --build -d
  logs:       docker compose -f .docker/docker-compose.dev.yml logs -f gateway auth
  down:       docker compose -f .docker/docker-compose.dev.yml down -v
  migrate-up: docker compose -f .docker/docker-compose.dev.yml run --rm migrate-auth ... up
  ```

### Production (`.docker/docker-compose.prod.yml`)

- Uses **env-file substitution** for secrets and options (from `.docker/.env.prod`).
- **Depends_on** ensures each migration container (auth/keys/messages) finishes before dependent services.
- Typical **env** variables for `auth` include:
  - `DATABASE_URL=postgres://app:${{POSTGRES_PASSWORD}}@postgres:5432/appdb?sslmode=disable`
  - `SIGNING_KEY` (HS256 secret)
  - `ACCESS_TTL`, `REFRESH_TTL`, `TRUST_PROXY`, etc.
- Postgres configured with healthcheck and persisted volume.

---

## 8) Secrets & Variables

These are defined in **GitHub → Repo → Settings → Secrets and variables**.

- **Actions → Secrets** (encrypted):
  - `PROD_POSTGRES_PASSWORD`
  - `PROD_SIGNING_KEY`

- **Actions → Variables** (non-secret):
  - `CORS_ORIGINS` (comma-separated list of allowed origins)
  - (Optional) `OWNER_LC`, `REPO_LC` if your workflow lowercases owner/repo for GHCR paths

> The **deploy** job writes these into `.docker/.env.prod`. See `.env.prod.example` for a reference:

```env
OWNER=Klickk
REPO=Sem7Individual
TAG=latest

POSTGRES_PASSWORD=change-me
SIGNING_KEY=change-me-super-long-random-secret

CORS_ORIGINS=http://localhost:5173,http://localhost:3000
```
---

## 9) File Inventory (Quick Reference)

- `lint.yml` — lints Go modules via `golangci-lint` (Docker)
- `ci.yml` — compiles and tests services (`go vet`, `go test`, `go build`)
- `publish.yml` — builds & pushes images (`:latest` and `:sha-<commit>`) to GHCR
- `deploy.yml` — renders env file, logs into GHCR, `docker compose up -d` on self-hosted
- `docker-compose.dev.yml` — local dev stack with migrations and healthchecks
- `docker-compose.prod.yml` — production stack (env-driven), suitable for the self-hosted runner
- `.env.prod.example` — guidance for server-side `.env` values
- `.golangci.yml` — linter settings
- `Makefile` — dev convenience commands

---