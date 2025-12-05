# Service Inventory

The table below maps each backend service to its code location, container build file, and any Kubernetes manifests currently present in the repository.

| Service | Code path | Dockerfile | K8s base path | Minikube overlay path |
|---------|-----------|------------|---------------|-----------------------|
| auth | services/auth | services/auth/Dockerfile | Not present in repo | Not present in repo |
| keys | services/keys | services/keys/Dockerfile | Not present in repo | Not present in repo |
| messages | services/messages | services/messages/Dockerfile | Not present in repo | Not present in repo |
| gateway | services/gateway | services/gateway/Dockerfile | Not present in repo | Not present in repo |
| crypto-core | services/crypto-core | Not present in repo | Not present in repo | Not present in repo |
