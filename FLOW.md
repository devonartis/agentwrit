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

### Action: B1 — P0 cherry-pick (2026-03-29)

Cherry-picked 6 commits from agentauth onto `fix/p0-persistent-key`. Zero conflicts.
Commits: `9c1d51d`, `f96549f`, `6d0d77d`, `cec8b34`, `0fef76b`, `e823bea`
- New package: `internal/keystore/` (persistent Ed25519 key management)
- Graceful shutdown with signal handling in `cmd/broker/serve.go`
- Docker volume for key persistence in `docker-compose.yml`

### B1 Status: ACCEPTANCE TESTS PASS — ready to merge (2026-03-29)

Gates: G1-G7 all PASS. G6 smoke at 3/7 (threshold 3, TD-S05).
Acceptance tests: 7/7 PASS (K1-K5, S1-S2) from `agentauth/tests/p0-production-foundations/`.
Evidence: `tests/p0-production-foundations/evidence/`

### Decision: Acceptance tests required before merge (2026-03-29)

Every batch must run its acceptance tests from the agentauth repo before merging. The process:
1. Cherry-pick commits onto fix/ branch
2. Run gate checks (G1-G7) via `test_batch.sh`
3. Copy acceptance tests from `agentauth/tests/<feature>/` to `agentauth-core/tests/<feature>/`
4. Run each story against Docker per `tests/LIVE-TEST-TEMPLATE.md`
5. All stories must PASS before merge to develop

Acceptance test availability by batch:
- B2 (P1): `agentauth/tests/p1-admin-secret/` — 9 stories + 3 security reviews
- B3 (SEC-L1): `agentauth/tests/fix-sec-l1/` — has evidence
- B4 (SEC-L2a): NONE — must write before merge
- B5 (SEC-L2b): `agentauth/tests/fix-sec-l2b/` — has evidence
- B6 (SEC-A1): NONE — must write before merge

### Decision: tracker.jsonl created for migration (2026-03-29)

`.plans/tracker.jsonl` tracks all batches, gates, and acceptance test stories. Status updates go here. FLOW.md and tracker must stay consistent.

### B2 Status: MERGED (2026-03-29)

Branch: `fix/p1-admin-secret` → develop
10 commits (8 cherry-picks + 2 fixes), 35 files changed
Gates G1-G7: ALL PASS (15 packages)
Acceptance tests: 9/9 PASS (S1-S9), 3 security reviews done
Security findings: 8/10 addressed on branch. I-4 (logging) and I-5 (rate limiting) deferred as TD-S06/TD-S07
Contamination: CLEAN
Conflicts resolved: cfg.go (HITL fields dropped, P1 fields added), admin_hdl_test.go (HITL tests dropped), CHANGELOG.md, docs/api.md
Key change: cfg.Load() now returns (Cfg, error) — all callers updated

### B3 Status: READY FOR MERGE REVIEW (2026-03-29)

Branch: `fix/sec-l1`
5 commits, 17 files changed
Gates G1-G6: ALL PASS (G7 skipped — no B3-specific tests)
Acceptance tests: 12/12 PASS (C5 OIDC skipped — not in core)
Contamination: CLEAN
Conflicts resolved: .gitignore (merged entries), configfile_test.go (added weak secret test), cmd/broker/main.go (added background goroutines + bind address, dropped HITL pruner + OIDC log)
Key changes: bind address defaults 127.0.0.1 (VPS), docker-compose.yml overrides to 0.0.0.0 (container), weak secret denylist, HTTP timeouts, TLS 1.2 minimum
Waiting on Cowork merge review

### Current Step: B3 merge review → B4 cherry-pick
3. Tracker: `.plans/tracker.jsonl`

**Guides:**
- Cherry-Pick Guide: `agentauth/.plans/modularization/Cherry-Pick-Guide.md`
- Feature Inventory: `agentauth/.plans/modularization/Cowork-Feature-Inventory.md`
- Repo Map: `agentauth/.plans/modularization/Repo-Directory-Map.md`
- Migration Audit: `.plans/cherry-pick/MIGRATION-AUDIT.md`
- Skill: `cherrypick-devflow`
