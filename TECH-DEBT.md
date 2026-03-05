# TECH-DEBT.md

Active tech debt. Append new entries as debt is taken. Never remove — mark as RESOLVED with date.

Full details for each item live in the referenced file. This is the index.

---

| ID | What | Phase | Severity | When to fix | Reference |
|----|------|-------|----------|-------------|-----------|
| TD-001 | `app_rate_limited` audit event not emitted — rate limiter fires before handler audit call | 1a | Low | Before Phase 1C | `.plans/phase-1a/ADR-Phase-1a-Tech-Debt.md` |
| TD-002 | No operator onboarding — no `aactl init`, admin secret origin undocumented | 1a | Low | Future | `tests/phase-1a/lessons-learned.md` |
| TD-003 | Sidecar has no defined use case — removed from infra, code still in `cmd/sidecar/` | 1a | Medium | When PRD defines a use case | `tests/phase-1a/lessons-learned.md` |
| TD-006 | App JWT TTL hardcoded to 5 min via global `AA_DEFAULT_TTL` — too short for machine-to-machine. Default should be 30 min minimum. Operator should set per-app TTL at registration (`aactl app register --token-ttl`) or update it later (`aactl app update --token-ttl`). Requires splitting `AA_DEFAULT_TTL` into per-token-type defaults. | 1b | Medium | Before Phase 1C | `internal/cfg/cfg.go`, `internal/app/app_svc.go` |
