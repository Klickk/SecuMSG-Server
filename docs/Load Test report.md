# Load & Performance Test Report – E2EE Messaging Platform

## 1. Introduction

This report describes the load and performance testing of the E2EE messaging platform developed as part of my Semester 7 individual project. The goal of the tests is to evaluate how the system behaves under realistic and extreme load, identify performance bottlenecks, and document improvements.

The system under test consists of multiple Go microservices (auth, keys, messages, gateway) deployed on Kubernetes (minikube), with PostgreSQL as the primary data store.

---

## 2. Objectives & Success Criteria

### 2.1 Objectives

- Validate that core user flows (login, sending 1:1 messages, sending group messages) perform acceptably under expected load.
- Determine the maximum sustainable load before the system degrades (increased latency, errors).
- Collect evidence for performance-related learning outcomes (e.g. scalable architectures, CI/CD and quality).

### 2.2 Success Criteria / SLAs

Define clear, measurable targets, for example:

- **Availability:** Error rate (5xx responses) < 1% for expected load.
- **Latency:** p95 response time < 300 ms for `/messages/send` under expected load.
- **Throughput:** At least **X** messages per second under expected load.
- **Stability:** No crashes or restarts during a **N-minute** soak test at baseline load.

---

## 3. Test Environment

### 3.1 Infrastructure

- **Cluster:** minikube on local machine  
- **Kubernetes version:** `vX.Y.Z`
- **Node resources:** `XX` CPU cores, `YY` GB RAM
- **Database:** PostgreSQL `version`, single instance
- **Storage:** local volume (describe if relevant)

### 3.2 Application Version

- Git commit: `abcdef1234`
- Branch: `main`
- Container tags:
  - `gateway: ...`
  - `messages: ...`
  - `auth: ...`
  - etc.

### 3.3 Monitoring & Observability

- **Metrics:** Prometheus (if used)
- **Dashboards:** Grafana (if used)
- **Logs:** Loki / kubectl logs

Include screenshots of relevant dashboards here.

---

## 4. Test Scenarios & Workload Model

Describe the main scenarios you tested. Example:

### 4.1 Scenario A – Health Check

- **Endpoint:** `GET /healthz`
- **Purpose:** Verify basic availability under simple load.

### 4.2 Scenario B – Send 1:1 Message

- **Endpoint:** `POST /messages/send`
- **Description:** Simulates a user sending a message to another user.
- **Request shape:**
  ```json
  {
    "recipient_id": "user-123",
    "content": "Hello from load test"
  }
