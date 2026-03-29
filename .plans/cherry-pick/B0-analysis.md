# B0 Analysis: Sidecar Removal

**Date:** 2026-03-29
**Branch:** `fix/sidecar-removal`
**Commits:** `34bb887`, `909a777` (from legacy agentauth repo)

---

## Commit 34bb887 — `feat(broker): remove sidecar subsystem entirely`

29 files changed, +304 / -2395 lines.

### What It Does

Removes the entire sidecar proxy model (superseded by app-based architecture). Affects:
- Token exchange handler (deleted entirely)
- Storage layer (sidecar tables, ceiling CRUD removed)
- Admin handlers (5 sidecar endpoints removed)
- Token service (SidecarID field removed from IssueReq and claims)
- Renewal handler (store dependency and ScopeCeiling removed)
- Observability (sidecar metrics removed)
- Audit (5 sidecar event types removed)
- Infrastructure (Dockerfile, compose files, test scripts)

### Add-on Code Touch: NONE

Zero modifications to approval/, oidc/, cloud/, hitl/ paths. Safe to pick.

### Conflict Expectations: HIGH

Written against agentauth's squashed initial release, not agentauth-internal's incremental history. Key conflicts:

| File | Severity | Why |
|------|----------|-----|
| `internal/token/tkn_svc.go` | HIGH | SidecarID in IssueReq struct |
| `internal/store/sql_store.go` | HIGH | Sidecar tables/CRUD throughout |
| `internal/admin/admin_hdl.go` | HIGH | 5 sidecar handler methods + routes |
| `internal/admin/admin_svc.go` | HIGH | Sidecar service logic |
| `internal/handler/renew_hdl.go` | LOW | Store dependency removal |

### Resolution Strategy

Keep agentauth-core's app-level code (Phase 1A/1B/TD-006 additions) intact. Only remove sidecar-specific code. Do NOT adopt agentauth's files wholesale — they may be missing app features from the incremental history.

---

## Commit 909a777 — `fix(cleanup): remove final sidecar references`

2 files changed, +3 / -3 lines. Cosmetic cleanup:
- `docs/architecture.md` — removed historical sidecar reference
- `internal/token/tkn_svc_test.go` — renamed fixture strings ("sidecar-123" → "session-123")

### Conflict Expectations: LOW

Trivial changes. May auto-apply cleanly.
