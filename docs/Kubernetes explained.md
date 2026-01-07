# Kubernetes Stack Explained

This file describes how the Kubernetes manifests in this repo deploy the E2EE messaging platform on a local minikube cluster (or any single-cluster setup), and how Argo CD is used to sync them.

---

## 1) High-Level Layout

- **Base manifests (`k8s/base/*`)**: Deployments + Services + migration Jobs for each service (auth, keys, messages, gateway).  
- **Overlays (`k8s/overlays/minikube-*`)**: Environment-specific Kustomize overlays that add config/secret generators and point at the base.  
- **Postgres (`infra/k8s/base/postgres`)**: StatefulSet + Service + credentials/DB config, included by the minikube overlay.  
- **Argo CD Application (`infra/argocd/sem7-platform-minikube.yaml`)**: Syncs the minikube overlay into the cluster (with prune/self-heal and image updater annotations).

---

## 2) Postgres

Path: `infra/k8s/base/postgres`  
- **StatefulSet** (1 replica) with PVC (`postgres-data`, 10Gi, `standard` storage class).  
- **Service:** ClusterIP on 5432.  
- **Secret:** `postgres-credentials` with user/password and DB names.  
- **ConfigMap:** `postgres-databases` listing DB names (`auth_db`, `keys_db`, `messages_db`).  
- **Sync wave:** annotated `-1` so Postgres comes up before app jobs/services (Argo CD friendly).

---

## 3) Auth Service

Path: `k8s/base/auth`  
- **Deployment:** 1 replica, image `ghcr.io/klickk/secumsg-server/auth:latest`, container port 8081.  
- **Env:** DB connection via `DATABASE_URL` (from `postgres-credentials` secret) plus values from `auth-config` / `auth-secret`.  
- **Service:** ClusterIP on 8081.  
- **Migration Job:** `auth-migrations` runs at sync (Argo hook, wave 0); initContainers wait for Postgres and create `auth_db` if missing, then run migrate image `auth-migrations:latest`.

Overlay: `k8s/overlays/minikube-auth`  
- ConfigMap generator sets issuer, JWKS path, TTLs, etc.  
- Secret generator sets `SIGNING_KEY`.  
- Namespace: `default`.

---

## 4) Keys Service

Path: `k8s/base/keys`  
- **Deployment:** 1 replica, image `ghcr.io/klickk/secumsg-server/keys:latest`, port 8082.  
- **Env:** `DATABASE_URL` from Postgres secret; `keys-config` ConfigMap.  
- **Service:** ClusterIP on 8082.  
- **Migration Job:** `keys-migrations` waits for Postgres, creates `keys_db`, runs migrate image `keys-migrations:latest`.

Overlay: `k8s/overlays/minikube-keys`  
- ConfigMap generator sets `ADDR=:8082`.  
- Secrets: none added here (DB creds come from Postgres secret).

---

## 5) Messages Service

Path: `k8s/base/messages`  
- **Deployment:** 1 replica, image `ghcr.io/klickk/secumsg-server/messages:latest`, port 8084.  
- **Env:** `MESSAGES_DATABASE_URL` from Postgres secret; `messages-config` ConfigMap.  
- **Service:** ClusterIP on 8084.  
- **Migration Job:** `messages-migrations` waits for Postgres, creates `messages_db`, runs migrate image `messages-migrations:latest`.

Overlay: `k8s/overlays/minikube-messages`  
- ConfigMap generator sets `MESSAGES_ADDR=:8084`.

---

## 6) Gateway

Path: `k8s/base/gateway`  
- **Deployment:** 1 replica, image `ghcr.io/klickk/secumsg-server/gateway:latest`, port 8080.  
- **Env:** From `gateway-config` + `gateway-secret` (includes HS256 shared secret).  
- **Service:** ClusterIP on 8080.  
- **Migration Job:** placeholder job (no DB migrations) with a wait-for-Postgres initContainer; hook-annotated for Argo.

Overlay: `k8s/overlays/minikube-gateway`  
- ConfigMap generator sets upstream URLs (auth/keys/messages), CORS, JWKS URL, debug flag.  
- Secret generator sets `GATEWAY_SHARED_HS256_SECRET`.

---

## 7) Minikube Overlay Composition

Path: `k8s/overlays/minikube/kustomization.yaml`  
- Assembles: Postgres base + minikube-auth + minikube-keys + minikube-messages + minikube-gateway.  
- Namespace: `default`.  
- Intended for local cluster use (minikube), but portable to other single-cluster environments.

---

## 8) Argo CD Application

Path: `infra/argocd/sem7-platform-minikube.yaml`  
- Points to repo `main` branch and `k8s/overlays/minikube`.  
- Automated sync with prune + self-heal.  
- Argo CD Image Updater annotations track GHCR images (`auth`, `keys`, `messages`, `gateway`) using `latest` update strategy and write back to `main`.  
- Destination: in-cluster API (`https://kubernetes.default.svc`), namespace `default`.

Usage: apply this manifest to the Argo CD namespace (`argocd`) to let Argo manage the stack on minikube.

---

## 9) Operational Notes

- Images default to `:latest` in manifests; Image Updater can bump tags automatically. Use SHA tags if you need pinned rollbacks.  
- Migration jobs run on each sync (hooked) to ensure DB schemas exist.  
- Postgres creds/DB names live in the `postgres-credentials` secret; adjust for production-grade secrets management if needed.  
- All services are ClusterIP; expose externally via an Ingress/port-forward/minikube service tunnel as needed.
