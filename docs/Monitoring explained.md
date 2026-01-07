# Monitoring & Logging Explained

This file summarizes how monitoring and logging are wired in this repo for local/dev use and for the services themselves.

---

## 1) Local Observability Stack (Compose)

File: `.docker/docker-compose.observability.yml`

- **Prometheus** (`prom/prometheus:v2.51.0`), port `9090`, loads config from `./prometheus.yml`, persists data to `prometheus-data` volume.  
- **Loki** (`grafana/loki:2.9.3`), port `3100`, config from `./loki-config.yml`, data in `loki-data` volume.  
- **Promtail** (`grafana/promtail:latest`), scrapes Docker logs via `/var/run/docker.sock` and `/var/log`, pushes to Loki.  
- **Grafana** (`grafana/grafana:10.4.4`), port `3000` (admin pwd `admin` in dev), pre-provisioned datasources/dashboards from `./grafana/provisioning` and `./grafana/dashboards`, data in `grafana-data`.

Usage: run alongside the dev stack to collect metrics and logs locally. All services expose `/metrics` for Prometheus scraping; logs go to stdout and are collected by Promtail.

---

## 2) Service-Level Metrics

All services expose:
- `GET /healthz` (readiness/basic health)
- `GET /metrics` (Prometheus)

Per-service metrics packages register Prometheus counters/histograms and attach middleware:

- **Gateway** (`services/gateway/internal/observability/metrics`): HTTP request counters/histograms; handler wrapped with metrics + request/trace ID middleware.  
- **Auth** (`services/auth/internal/observability/metrics`): HTTP metrics and auth-specific counters (token issuance, logins, registrations).  
- **Keys** (`services/keys/internal/observability/metrics`): HTTP metrics.  
- **Messages** (`services/messages/internal/observability/metrics`): HTTP metrics plus messaging-specific counters/histograms (stored messages, ciphertext sizes, history fetches).

Each service curies the `service` label when calling `MustRegister("service-name")`, so metrics are tagged by service.

---

## 3) Logging

- Services use structured JSON logs via `log/slog` with a per-service logger (`internal/observability/logging`).  
- Logs are written to stdout; Promtail scrapes container logs and ships to Loki (Compose observability stack).  
- Middleware injects `X-Request-ID` and `X-Trace-ID` for correlation across gateway → downstream services.

---

## 4) Tracing

- No distributed tracing backend is configured yet. Trace IDs are generated and propagated via middleware and logged for future correlation.

---

## 5) K8s / Argo CD Context

- The Kubernetes manifests keep `/metrics` and `/healthz` endpoints exposed inside the cluster.  
- For minikube/Argo CD deployments, reuse the same Prometheus/Loki/Grafana stack externally or via port-forwards; cluster-level scraping is not defined in the manifests.  
- Argo CD observes drift but does not ship metrics/logs; observability remains a separate stack run alongside the services.

---

## 6) How to View

1) Start the dev stack (`make up`) and observability stack:  
```bash
docker compose -f .docker/docker-compose.observability.yml up -d
```
2) Open Grafana: `http://localhost:3000` (admin/admin).  
3) Prometheus: `http://localhost:9090` (query `/metrics` endpoints).  
4) Loki logs in Grafana via the provisioned datasource.

---

## 7) Gaps / Next Steps

- Add distributed tracing backend (Tempo/Jaeger) and instrument services with OTEL if needed.  
- Add Kubernetes-native scraping (Prometheus Operator) if running observability in-cluster.  
- Harden Promtail/Loki/Grafana creds for non-dev use; current defaults are dev-friendly.  
- Create dashboards for auth/keys/messages/gateway latency, error rates, and message throughput.
