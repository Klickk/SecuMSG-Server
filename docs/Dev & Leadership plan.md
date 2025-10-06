# Development & Personal Leadership Plan (Snapshot)

**Student:** Ivan Bakalov  
**Project:** E2EE Messaging Platform  
**Period:** Sep – Jan (current semester)  
**Version:** 1.0 (2025‑10‑05)

## 1) Long‑Term Goals

- Become proficient with **Go** for production backend services (auth, messaging, files).
- Operate a small **microservices** stack end‑to‑end (design → CI/CD → deploy → operate).
- Build solid fundamentals in **security engineering** (E2EE, JWT/session hygiene, secrets).
- Demonstrate **enterprise software** competencies: architecture, quality attributes, DevOps.

## 2) Initial Development Time Plan

- **Sept - Oct (Weeks 1–5):** Finish Auth MVP (register/login/refresh/revoke), harden input validation, unit tests, CI green.
- **Oct (Week 5–6):** Stabilize **CI/CD** to self‑hosted runner (immutable image tags, rollback, healthchecks).
- **Oct - Nov (Weeks 6–8):** Messaging service (groups, envelopes, WebSocket fan‑out) + DB schema & events.
- **Nov (Week 8–10):** File service PoC (client‑side encryption, S3/MinIO, presigned flows).
- **Nov - Dec (Weeks 10–12):** Non‑functional: load test basics, token/key rotation, logs/metrics MVP.
- **Dec - Jan (Weeks 12–18):** Documentation & portfolio, ADRs, final polishing.

## 3) Professional Attitude

- Keep issues small and traceable; PRs with checklists; reproducible builds (`sha-<commit>`).
- Respect legal/ethical constraints (GDPR data minimization, zero plaintext content on server).
- Share weekly progress notes and risks with stakeholders (semester coach/technical teachers).

---

**Status:** Active. Will be updated with next major milestone.
