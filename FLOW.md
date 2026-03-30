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

### B3 Status: MERGED (2026-03-29)

Branch: `fix/sec-l1` → develop
5 cherry-picks + 4 fix commits, 28 files changed
Gates G1-G6: ALL PASS (G7 skipped — no B3-specific tests)
Acceptance tests: 12/12 PASS (C5 OIDC skipped — not in core)
Contamination: CLEAN
Tech debt TD-S08 through TD-S15 added for doc drift (two CRITICAL: wrong API field names + rejected secret in examples)
Waiting on Cowork merge review

### B4 Status: ACCEPTANCE TESTS PASS — ready for merge review (2026-03-29)

Branch: `fix/sec-l2a`
8 cherry-picks + 5 fix commits (revoker wiring, error sanitization, docstring, comment, RevokeByJTI method)
Gates G1-G7: ALL PASS (build, lint, unit tests — gosec WARN non-blocking)
Acceptance tests: 13/13 PASS (S1-S7, N1-N5, SEC1)
- S4/S5 initially FAILED — root cause: TknSvc.revoker nil at runtime (C1 finding)
- Fixed by adding `tknSvc.SetRevoker(revSvc)` in main.go + `RevokeByJTI()` on RevSvc
- H1 fix: renewal error info leakage sanitized in renew_hdl.go
- M1/M3 fixes: docstring and design comment updates
Container mode: 7/7 PASS (S1, S4, S5, S7, N1, N2, N4)
Contamination: CLEAN
Evidence: `tests/sec-l2a/evidence/`
Waiting on Cowork merge review.

### Action: B5 — SEC-L2b cherry-pick (2026-03-30)

Branch: `fix/sec-l2b` off `develop`
Cherry-picked 5 commits from agentauth: `daf2995`, `e592acc`, `2857b3a`, `247727c`, `c5da6c4`

**Results:**
- 4 commits applied, 1 skipped (247727c — empty after conflict resolution, content already present)
- Conflicts resolved:
  - `main.go`: OIDC/cloud routes dropped (add-on code)
  - `handler_test.go`: Kept comprehensive test suite from prior commits, dropped simpler incoming version
  - `renew_hdl.go`: Kept `obs.Warn` log line (observability), incoming side wanted to remove it
  - `tests/fix-sec-l2b/evidence/S3-renew-tampered-generic.md`: Removed (doesn't exist in core)
- Fix: Added missing `context` and `errors` imports to `handler_test.go`
- Gates G1-G3: ALL PASS (compile, 15 packages unit tests, contamination clean)
- Docs updated: architecture.md, api.md, concepts.md, implementation-map.md, getting-started-operator.md
- CHANGELOG.md updated with B5 section

**B5 Status: CHERRY-PICK DONE, G1-G3 PASS — needs Docker gates + acceptance tests + review**

### B5 Status: MERGED (2026-03-30)

Branch: `fix/sec-l2b` → develop
12 commits (4 cherry-picks + 8 docs/test/fix commits), 28 files changed
Gates G1-G6: ALL PASS (G7 skipped — batch-specific not wired in test_batch.sh)
Acceptance tests: 5/5 PASS, 1 SKIP (S5 TLS). 4 regression PASS (R1-R4).
Code review: PASS — no contamination, all 5 planned items satisfied
Contamination: CLEAN
LIVE-TEST-TEMPLATE updated with audience, personas, real-world grounding guidance

Key lessons: acceptance tests are NOT integration scripts. `integration.sh` is CI smoke — real evidence needs individual story files with executive-readable banners per LIVE-TEST-TEMPLATE. Personas must reflect production reality (App vs Developer).

### Action: B6 — SEC-A1 + Gates cherry-pick (2026-03-30)

Branch: `fix/sec-a1` off `develop`
Cherry-picked 2 commits from agentauth: `9422e7c`, `e395a15`

- `9422e7c`: Conflict in `tkn_svc.go` — incoming had `AppID`, `AppName`, `OriginalPrincipal` fields not in core's `IssueReq`. Kept TTL fix, dropped three fields.
- `e395a15`: Applied cleanly.
- Gates G1-G6: ALL PASS
- Regression unit test added: `TestRenew_PreservesTTL` (5 subtests)
- Acceptance tests: 4 stories (S1, S2, S3, R1) — all PASS VPS mode

### Decision: Code comments must explain roles, not restate code (2026-03-30)

During acceptance test authoring, the agent built tests against the admin flow instead of the app flow because no code comments explained which role calls which endpoint. Multiple prior sessions wrote and reviewed this code without flagging the gap. New rule in `.claude/rules/golang.md`: comments tell you what reading the code would NOT tell you — who calls it, why, boundaries.

### Decision: TECH-DEBT.md moved to repo root (2026-03-30)

Was at `.plans/TECH-DEBT.md`. Tech debt is a first-class artifact, not a planning doc. MEMORY.md now points to it instead of duplicating entries.

### Decision: cherrypick-devflow skill updated with Step 3 (Regression Unit Tests) (2026-03-30)

New step between Pick and Verify. If a cherry-pick changes existing behavior, write regression unit tests before running gates — so the new tests are included in G2. Also added code comments requirements to Pick step and verification to Docs step.

### B6 Status: MERGED (2026-03-30)

Branch: `fix/sec-a1` → develop
4 commits (2 cherry-picks + 1 docs/tests/comments + 1 merge), 24 files changed
Gates G1-G6: ALL PASS
Acceptance tests: 4/4 PASS (S1 admin TTL, S2 app TTL, S3 scope boundary, R1 lifecycle)
Code review: 2 findings fixed (unused var, missing S3 in story index)
Contamination: CLEAN
Tech debt: TD-011 through TD-014 added

### Decision: Post-migration repo strategy — DEFERRED (2026-03-30)

Three repos exist: `agentauth-internal` (golden history, 412 commits), `agentauth` (enterprise modules: OIDC, HITL, cloud, federation + migration plans), `agentauth-core` (open-source core, B0-B6 merged).

Questions to resolve:
1. `agentauth-core` needs to become `divineartis/agentauth` on GitHub — current `agentauth` must be renamed/archived first
2. Does enterprise module code stay in one archived repo or get extracted into separate module repos (per open-core model)?
3. Does `agentauth-internal` stay on devonartis or move to divineartis?
4. Migration artifacts (`.plans/modularization/`, cherry-pick guides, feature inventory) — archive or keep?

**Deferred because:** Priority is reviewing all code comments across `internal/` to match the new standard, then updating external documentation to deep-dive on scopes and the role model (TD-012, TD-014). The repo strategy decision comes after the codebase is properly documented.

### Decision: Next work sequence (2026-03-30)

Three phases, in order:

1. **Code comments audit** (TD-014) — Go through ALL code in `internal/` and `cmd/` and update comments to the new standard (`.claude/rules/golang.md`): who calls it, why it exists, boundaries, design history. This is the foundation — you can't write correct docs without understanding the code's intent.

2. **Public documentation update** — After the comments reveal the full picture of roles, scopes, and boundaries, update all public-facing docs (`docs/`) to be compliant with what we found. Write `docs/roles.md` (TD-012) with the Admin/App/Agent role model, scopes, and production flow. Verify every doc against the code comments — not the other way around. Answer the design questions that come up (like TD-013: should admin create agents?).

3. **Repo strategy** — Once the codebase is properly documented and the role model is clear, come back to the three-repo question (archive, rename, extract enterprise modules). The documentation work may surface more design decisions that affect the repo strategy.

Use `devflow` for this work.
