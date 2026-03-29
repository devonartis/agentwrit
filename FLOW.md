# FLOW.md — agentauth-core

Running decision log. Append after each meaningful action.

---

## 2026-03-29 — Repo Setup & Migration Planning

### Decision: Fork point is `2c5194e` (TD-006), not `3f9639f` (Phase 1C-alpha)

Phase 1C-alpha bakes `hitl_scopes` into the app data model (store, service, handler, CLI — 4 source files, 3 tests). TD-006 has identical core + app functionality with zero HITL references. Verified with `grep -ri "hitl|approval" internal/ cmd/` returning nothing at `2c5194e`.

### Decision: Clone from agentauth-internal, not copy

The original agentauth repo was a file copy that lost all 412 commits of history. This time we did `git clone agentauth-internal agentauth-core` to preserve the full incremental commit history.

### Decision: Open-core model with two archive repos

- Core (this repo) → will become open-source
- Add-ons (future separate repo) → HITL, OIDC, cloud, federation → stays private
- agentauth-internal → archive, reference for incremental history
- agentauth → archive, reference for cherry-pick commits + enterprise add-ons

### Action: Cloned repo, reset to fork point

```
git clone agentauth-internal agentauth-core
cd agentauth-core
git checkout -B main 2c5194e
```

### Decision: GitFlow branching for migration work

All cherry-pick work happens on `fix/*` branches off `develop`. Merged to `develop` after verification, then `develop` merged to `main` periodically.

### Action: Set up GitFlow (2026-03-29)

```
git branch -D develop  # old branch from agentauth-internal clone
git checkout -b develop  # new develop from main HEAD
git checkout -b fix/sidecar-removal
```

### Action: B0 — Sidecar removal cherry-pick (2026-03-29)

Cherry-picked `34bb887` and `909a777` from agentauth repo onto `fix/sidecar-removal`.

- `34bb887`: 25 files changed, -2220 lines. Removed token exchange handler, sidecar admin endpoints, sidecar store CRUD, SidecarID from claims/requests, sidecar metrics, sidecar audit events, sidecar Docker/compose/script config.
- `909a777`: 3 files changed. Renamed test fixtures, removed sidecar doc references.
- Conflicts resolved in: tkn_svc.go, renew_hdl.go, sql_store.go (3 regions), admin_hdl_test.go, docs/architecture.md, MEMORY.md
- Contamination check: PASS (zero sidecar/hitl/oidc/approval references in Go code)
- Evidence: `.plans/cherry-pick/B0-analysis.md`

### B0 Status: MERGED (2026-03-29)

B0 merged to develop. G1-G7 all PASS. G6 smoke at 3/7 (TD-S05 for remaining stale payloads).
Next: B1 cherry-pick.

### Current Step: B1 Cherry-Pick

**Guides:**
- Cherry-Pick Guide: `agentauth/.plans/modularization/Cherry-Pick-Guide.md`
- Feature Inventory: `agentauth/.plans/modularization/Cowork-Feature-Inventory.md`
- Repo Map: `agentauth/.plans/modularization/Repo-Directory-Map.md`
- Migration Audit: `.plans/cherry-pick/MIGRATION-AUDIT.md`
- Skill: `cherrypick-devflow`
