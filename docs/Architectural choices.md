# Architectural Choices

Project: E2EE Messaging Platform  
Date: 2025-10-05  
Scope: Captures the key architecture decisions made so far, including motivation, alternatives considered, and consequences.
---

## 1) Architecture Style: Microservices with an API Gateway

**Decision**  
Use a small set of Go microservices fronted by an API Gateway.

**Services (current & planned)**  
- **Gateway:** HTTP/WS entry point, CORS, auth delegation, routing.  
- **Auth:** Registration, login, JWT issuance/refresh, session management.  
- **Messaging (planned):** Rooms, message envelopes, WS fanout, delivery receipts.  
- **File (planned):** Encrypted file ingest/retrieval to S3-compatible storage.  
- **Event Bus:** NATS (or compatible) for domain events and decoupling.

**Motivation**  
- Clear separation of concerns (security, messaging, storage).  
- Independent build/deploy for rapid iteration and safe rollbacks.  
- Aligns with a scalable, enterprise-grade direction while remaining small enough to manage.

**Alternatives considered**  
- **Monolith:** simpler initially, but tighter coupling and slower iteration across concerns.  
- **Functions-only/serverless:** good for spiky workloads; adds vendor coupling and local dev complexity.

**Consequences**  
- More operational surface area (multiple services, images, env).  
- Requires consistent contracts and CI/CD discipline (addressed via Actions + Compose).

---

## 2) Language & Runtime: Go (Golang)

**Decision**  
All backend services implemented in **Go**.

**Motivation**  
- Performance and efficient concurrency (goroutines, channels).  
- Strong standard library for HTTP, crypto, and testing.  
- Simple deploy artifacts (static binaries) and good container fit.

**Alternatives considered**  
- **Node.js/TypeScript:** rich ecosystem; less optimal for CPU-bound crypto and concurrency.  
- **Rust:** safety/perf; steeper learning curve, slower iteration for the current team.

**Consequences**  
- Shared Go modules for common domain types/utilities.  
- Consistent tooling across services (linters, tests).

---

## 3) Security Model: End-to-End Encryption (Client-Side)

**Decision**  
Encryption and key management are **client-owned**. The server never sees plaintext message/file content.

**Core points**  
- Server stores **public keys** and **ciphertext** + metadata.  
- One-to-one: public-key cryptography for establishing shared secrets; messages encrypted symmetrically per session.  
- Groups (planned): shared symmetric “sender keys” with rotation on membership changes.  
- Files: client-side encryption; server stores encrypted blobs and minimal metadata.

**Consequences**  
- Backend focuses on **identity, routing, and storage**.  
- Zero access to content impacts search/anti-abuse features (must be designed with metadata and user controls).

**Open items**  
- Protocol choice & spec (e.g., Double Ratchet, X3DH variants) to be formalized.  
- Key rotation, re-key on device add/remove, attachment key wrapping.

---

## 4) Authentication & Sessions

**Decision**  
Use short-lived **JWT access tokens** and long-lived **refresh tokens** bound to **server-side sessions**.

**Details**  
- Access tokens: short TTL for safety.  
- Refresh tokens: server can revoke via session table.  
- Store **only the IP address** in the session (Postgres `inet`).  
  - Parse `RemoteAddr` and persist only the host part to avoid `inet` parse errors.

**Alternatives considered**  
- Stateless-only tokens (no server session): simple but hard to revoke.  
- Opaque tokens + session store: more round trips, less client autonomy.

**Consequences**  
- Predictable logout/revoke semantics.  
- A small DB write on login/refresh; acceptable trade-off for control.

---

## 5) Data & Persistence

**Decision**  
- **PostgreSQL** as the primary relational datastore.  
- **Object storage** (S3/MinIO) for encrypted files.  
- **GORM** currently used for rapid development; may evolve to `sqlc` for stricter control where helpful.

**Motivation**  
- Postgres provides strong consistency, rich types (e.g., `inet`) and indexing.  
- S3/MinIO standardizes file storage and scales separately from the DB.

**Consequences**  
- DB migrations managed via a migrations container in Compose (dev/prod).  
- Clear split between structured data (users, sessions, rooms) and blobs (files).

**Notes**  
- Lesson learned: ensure session IP is stored as plain IP, not `host:port` when using `inet`.

---

## 6) Communication Patterns & Protocols

**Decision**  
- **HTTP/REST** for management endpoints (auth, configuration).  
- **WebSockets** for real-time messaging fanout.  
- **Event bus (NATS)** for decoupled domain events and cross-service notifications.

**Motivation**  
- REST remains the simplest integration surface.  
- WS provides low-latency delivery for chats without polling.  
- Event bus enables independent services to react without tight coupling.

**Consequences**  
- Requires connection lifecycle handling (WS reconnects, backoff).  
- Message ordering and idempotency addressed at the envelope level (planned).

---

## 7) Containerization & Environments

**Decision**  
All services packaged as Docker images. **Docker Compose** used for dev and prod stacks.

**Dev**  
- Compose spins up Postgres, migrations, and services with local ports exposed.  
- Fast inner loop via rebuilds and selective service restart.

**Prod**  
- Compose orchestrates services on a self-hosted host.  
- Images pulled from GHCR with a configurable `TAG` (supports rollbacks via `sha-<commit>`).

**Consequences**  
- Single-node orchestration for now; easy to evolve to Swarm/Kubernetes later.  
- Clear separation via `.env.dev` and `.env.prod` files.

---

## 8) CI/CD Strategy

**Decision**  
Use **GitHub Actions** for linting, CI, image builds, and deploy to a **self-hosted runner** using Docker Compose.

**Workflows**  
- **Lint:** GolangCI-Lint across service modules.  
- **CI:** `go vet`, `go test`, `go build`.  
- **Publish:** build & push images to **GHCR** with `:latest` and immutable `:sha-<commit>` tags (build cache enabled).  
- **Deploy:** renders `.env.prod` from **Secrets/Variables**, logs into GHCR, and runs Compose `pull` + `up -d`.

**Motivation**  
- Fast feedback cycle; reproducible images; safe rollbacks by pinning to `sha-<commit>`.  
- Straightforward ops with a single Compose host.

**Consequences**  
- Keep workflow and compose paths in sync.  
- Self-hosted runner must have Docker/Compose and GHCR network access.

---

## 9) Configuration & Secrets

**Decision**  
- Environment variables for configuration; `.env.prod` generated at deploy time.  
- GitHub **Secrets** for sensitive values (DB password, signing key).  
- GitHub **Variables** for non-sensitive values (CORS origins, owner/repo/tag).

**Motivation**  
- Twelve-Factor alignment; no secrets committed to VCS.  
- Quoted secrets in `.env` to handle `#` and spaces safely.

**Consequences**  
- Single source of truth in Actions UI; reproducible deploy manifests.  
- Easy rotation by updating Secrets and re-deploying.

---

## 10) Observability, Logging, and Error Handling (Initial)

**Decision**  
- Structured logs from services (stdout).  
- CI surfacing of failing container logs during deploy.  
- Healthchecks in Compose for basic readiness/liveness.

**Planned**  
- Centralized logs (e.g., Loki/ELK) and metrics/traces (OpenTelemetry).  
- Request IDs and correlation across gateway → services → event bus.

**Consequences**  
- Current setup adequate for early stages; observability will be expanded as features land.

---

## 11) Testing Strategy (Initial)

**Decision**  
- Unit tests in CI for each service.  
- Integration tests (planned) with ephemeral Postgres and service containers.  
- Contract tests for gateway ↔ services (planned).

**Motivation**  
- Prevent regressions as services evolve independently.  
- Validate security-sensitive flows (auth, token refresh, session revocation).

**Consequences**  
- CI runtime increases slightly with integration tests; acceptable trade-off.

---

## 12) Risks & Mitigations

- **Operational complexity:** Mitigated by Compose, a minimal set of services, and clear CI/CD.  
- **Security protocol drift:** Will formalize E2EE protocol (spec/doc + test vectors).  
- **Key/secret handling mistakes:** Centralized via Actions Secrets; add secret scanning later.  
- **Vendor lock-in (object storage):** Using S3 API allows switching providers.  
- **Schema evolution:** Migrations container and versioned SQL; plan for zero-downtime patterns.

---

## 13) Open Decisions / Next ADRs

- **E2EE protocol specifics** (Double Ratchet, X3DH, sender keys for groups).  
- **Message envelope schema** (ordering, idempotency, replay protection).  
- **ORM direction** (continue GORM vs. adopt `sqlc` for stricter compile-time checks).  
- **Event bus choice** (NATS config vs. Kafka if durability/ordering needs grow).  
- **Kubernetes** (when to migrate beyond Compose; ingress, secrets, autoscaling).  
- **Observability stack** (OpenTelemetry collector, log aggregation choice).

---

## Appendix: Current Component Responsibilities

- **Gateway**: TLS termination (environment), CORS, auth delegation, WS upgrade/routing.  
- **Auth**: identity lifecycle, password hashing (Argon2id/bcrypt), JWT/refresh, session storage (`inet` for IP), event emission.  
- **Messaging (planned)**: user/device registration for WS, rooms & membership, message persistence and delivery, events.  
- **File (planned)**: pre-signed upload/download flows, metadata, encrypted blob storage on S3/MinIO.  
- **PostgreSQL**: users, sessions, keys/metadata, room membership.  
- **Object Storage**: encrypted file blobs.  
- **Event Bus**: domain events (user registered, session created/revoked, message stored).

---

**Status:** Accepted for the current iteration. This document will be refined into individual ADRs as features are implemented.
