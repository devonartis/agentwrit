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

### Decision: Release strategy — Model 1 (separate per-language repos) (2026-03-31)

Analyzed three models for SDK placement: per-language repos (Stripe/Twilio/HashiCorp), multi-SDK monorepo (AWS), SDKs inside server repo. Chose Model 1 — separate per-language repos.

**Reasons:** Aligns with open-core model (core SDK open, enterprise HITL/OIDC extensions separate). Clean package identity (`pip install agentauth` from `agentauth-python`). Independent release cycles. Language-specific contributor experience.

**Trade-off accepted:** API contract drift across N repos, mitigated by `docs/api.md` as single source of truth.

### Decision: Release strategy is 4 phases (2026-03-31)

High-level plan at `.plans/release-strategy.md`. Each phase breaks into its own devflow cycle (brainstorm → spec → plan → code → review → test → merge).

1. **Phase 1: Repo cleanup & archive** — Archive old `divineartis/agentauth` as #2 archive. Rename `agentauth-core` → `divineartis/agentauth`. Clean artifacts per TD-017.
2. **Phase 2: SDK repo setup** — Extract Python and TS from `agentauth-clients` monorepo into `divineartis/agentauth-python` and `divineartis/agentauth-ts`. Archive monorepo as #3 archive.
3. **Phase 3: SDK core update** — Audit SDK calls against core API contract. Remove HITL/enterprise features. Integration test against core broker. Update SDK docs.
4. **Phase 4: Enterprise SDK extensions** — Future. Not in scope now.

**Analyzed `devonartis/agentauth-clients`:** Monorepo with Python SDK (6 modules, 122 unit tests, HITL baked in) and TypeScript SDK (6 modules, mirrored). Both built against old broker with HITL/OIDC. 7/8 endpoint calls exist in core — only HITL retry missing. Update is surgical, not a rewrite.

### Status: Next work (2026-03-31)

Remaining from previous decision (2026-03-30):
1. ~~Code comments audit~~ — chunks 1-5 done per commit history
2. **Public documentation update** — still pending (TD-012 roles doc, TD-014 code comments verification)
3. **Release strategy** — high-level plan written, needs user review before breaking into specs

**Next session:** Review `.plans/release-strategy.md`, decide sequencing (docs first vs repo cleanup first), then start Phase 1 of release strategy via devflow.

### Decision: Python SDK repo first, then TypeScript (2026-04-01)

Reviewed `.plans/release-strategy.md`. User chose to focus on SDK work first. Starting with Python SDK (`divineartis/agentauth-python`), TypeScript follows the same pattern after.

**Approach:** `git filter-repo` extraction from `devonartis/agentauth-clients` monorepo — preserves commit history for the Python subdirectory. Fresh repo, not a copy.

**Design approved:** `.plans/designs/2026-04-01-python-sdk-repo-design.md`

Key decisions:
- Separate per-language repos (Stripe/Twilio model)
- `uv` as package manager (mandatory)
- Strict type safety — every variable annotated, `mypy --strict` enforced
- HITL contamination removal (same pattern as B0 sidecar removal)
- API contract audit with live broker verification — not just doc review
- Version starts at `v0.2.0` (continues from monorepo `v0.1.0`)

### Status: Python SDK v0.2.0 COMPLETE (2026-04-01)

**Done:** Python SDK extracted, HITL removed, API aligned, live-tested, merged to main.

Details:
- Repo: `~/proj/agentauth-python` (not yet pushed to GitHub as `divineartis/agentauth-python`)
- 14 commits on `feature/hitl-removal`, merged to main
- 2416 lines removed (HITL class, approval_token, demo app, docs, tests)
- 119 unit tests + 13 integration tests passing against live broker v2.0.0
- Contamination guard tests scan src/, tests/, docs/ — zero HITL/approval/OIDC
- `/broker` slash command created for managing test broker from SDK repo
- No demo application yet — deleted HITL demo, clean replacement needed

**Sequence — what's next:**
1. **Demo application** for Python SDK — runnable example showing core flow
2. Push `agentauth-python` to GitHub as `divineartis/agentauth-python`
3. TypeScript SDK — same extraction + cleanup process → `divineartis/agentauth-ts`
4. Archive `devonartis/agentauth-clients` monorepo
5. Phase 1 repo cleanup — archive old `divineartis/agentauth`, rename `agentauth-core` → `divineartis/agentauth`
6. Public documentation update (TD-012, TD-014) — still pending from previous sessions

### Decision: Focus on agentauth-core, defer SDK work (2026-04-04)

User decided to focus on this repo (agentauth-core) before returning to SDK repos. Items 1-4 above (agentauth-python demo, GitHub push, TypeScript extraction, monorepo archive) are deferred. Next work is items 5-6: Phase 1 repo cleanup and public documentation update (TD-012, TD-014).

### Decision: Repo rename strategy and enterprise code preservation (2026-04-04)

**Plan:**
1. Archive `divineartis/agentauth` on GitHub (rename to `agentauth-enterprise-archive` or similar)
2. Rename `agentauth-core` → `divineartis/agentauth` so the open-source core gets the canonical name

**Critical note — enterprise code in the old agentauth repo:**
The archived `divineartis/agentauth` contains HITL approval flow and OIDC provider code that is NOT in agentauth-core. This code needs to be:
- Extracted from the archive
- Tested and built as the paid/enterprise binary
- Plugged into core via the existing interface/extension points

This is future work — the archive is NOT throwaway. It's the source for the enterprise modules (HITL, OIDC, cloud federation) that will become the paid product. Document what's in there before archiving so we know exactly what to extract later.

**Next:** Review what cleanup this repo needs before the rename can happen (Go module path, docs, references, tech debt). Then brainstorm the full plan.

### Decision: Cleanup plan finalized — CC v4 (2026-04-04)

After 4 iterations (CC v1-v4) and 3 PI versions, converged on `CC-2026-04-04-repo-cleanup-plan-v4.md`. Key decisions:
- Rename `divineartis/agentauth` → `agentauth-ENT` (private), rename `agentauth-core` → `agentauth` (private until review)
- Development files (MEMORY.md, FLOW.md, etc.) live on `develop`, stripped on merge to `main`
- `.gitignore` only blocks OS/tool junk, NOT development files
- `scripts/strip_for_main.sh` handles develop → main cleanup
- No enterprise extraction map (scope creep — code is in agentauth-ENT, catalog when needed)
- Human review gate after every batch
- Multi-agent review before going public

### Action: Starting CC v4 execution — Phase 1 (2026-04-04)

**Discovery:** GitHub org is `devonartis`, not `divineartis`. Plan had the wrong org. go.mod says `github.com/divineartis/agentauth` — needs updating to `github.com/devonartis/agentauth` (154 occurrences across 46 Go files). CC v4 plan updated with this fix.

### Step 1.1: Rename enterprise repo — DONE (2026-04-04)

- `devonartis/agentauth` → `devonartis/agentauth-ENT` via `gh api` ✓
- Local folder: `~/proj/agentauth` → `~/proj/agentauth-ENT` (user renamed) ✓
- Remote updated: `git remote set-url origin git@github.com:devonartis/agentauth-ENT.git` ✓
- Discovered `fix/pre-modularize-security` branch with 184 unpushed commits (enterprise module boundary, SEC-A2/A3, cloud credentials, modularization). Pushed to remote. ✓

**Next: Step 1.2 — rename `devonartis/agentauth-core` → `devonartis/agentauth`**

### Action: CC v4 cleanup complete + develop → main merge (2026-04-04)

CC v4 plan fully executed. Develop → main merge fast-forwarded, strip_for_main.sh ran, main clean at `df9b496`. Both branches pushed to `devonartis/agentauth` (private).

**Next:** Phase 3 — multi-agent review before going public.

---

## 2026-04-08 — Public release readiness (documentation + license posture)

### Decision: License switched from Apache 2.0 to AGPL-3.0 + CLA + enterprise (2026-04-08)

**Choice:** AGPL-3.0 for open-source core. CLA grants maintainer commercial/proprietary rights for enterprise. ENTERPRISE_LICENSE.md summarizes commercial terms (non-binding until final contract).

**Why AGPL-3.0:** Section 13 (“Remote Network Interaction”) requires anyone offering the software as a SaaS to release source code — this prevents unauthorized commercial hosting. Users can self-host and modify freely; commercial embedding or managed service requires a separate enterprise license.

**Files changed:** `LICENSE`, `CLA.md` (new), `ENTERPRISE_LICENSE.md` (new), `CONTRIBUTING.md` (simplified + CLA reference), `README.md` (badge + license section). TD-PUB-001 and TD-018 resolved.

**Remaining:** `docs/api/openapi.yaml` still says Apache 2.0 (fix with TD-S14 OpenAPI rewrite). Domain name needed for contact emails (TD-019). Source file AGPL headers (future).

### Action: Core README updated with SDK + demo sections (2026-04-08)

**Branch:** `docs/readme-sdk-demo` off `develop` — NOT merged, awaiting user review.

**What's on the branch:**
- "SDKs" section after Quick Start — table (Python ready, TypeScript planned), code example
- "See It In Action — MedAssist AI Demo" section — capabilities table, links to beginner + presenter guides
- SDK table added to Documentation section

**Depends on:** Python SDK `docs/readme-license-cleanup` branch (also pending review). Both branches should be reviewed together — core README links to SDK repo content.

**Still needs doing:** `docs/getting-started-developer.md` should link to Python SDK. Core README links point to `main` of SDK repo — SDK branch must merge first.

### Action: Public-facing contributor docs aligned (2026-04-08)

- `CONTRIBUTING.md`: canonical clone URL `github.com/devonartis/agentauth`, accurate module import paths, project tree without obsolete `smoketest`, new **Branch model** note for develop vs `main` + strip script.
- `SECURITY.md`: limitations corrected (persistent signing key, rate limiting on admin auth); removed broken `KNOWN-ISSUES.md` link.
- `CODE_OF_CONDUCT.md`: added (Contributor Covenant 2.1) so CONTRIBUTING’s CoC link resolves.

---

### What’s Next (2026-04-08)

**Two branches awaiting review — review these first, everything else depends on them:**

1. **Python SDK** `docs/readme-license-cleanup` (`~/proj/agentauth-python`)
   - MIT LICENSE file (was missing), pyproject.toml fix (Apache-2.0 → MIT), full README rewrite with MedAssist demo section
   - Review → merge to develop → merge to main (so core README links resolve)

2. **Core** `docs/readme-sdk-demo` (`~/proj/agentauth-core`)
   - SDK section + MedAssist demo showcase + SDK docs table entry in README
   - Review → merge to develop → merge to main (strip dev files as usual)
   - **Must merge AFTER SDK branch** — core README links to SDK repo `main`

**After both branches merge:**

3. **Domain name decision (TD-019, HIGH)** — pick a domain, register it, update all placeholder emails:
   - `CLA.md` — `devon@agentauthdev._`
   - `ENTERPRISE_LICENSE.md` — `devon@agentauthdev._`
   - `SECURITY.md` — `security@agentauth.dev`
   - `CODE_OF_CONDUCT.md` — `conduct@agentauth.dev`

4. **`docs/getting-started-developer.md`** — add link to Python SDK (not done yet)

5. **`docs/api/openapi.yaml`** — still says Apache 2.0 in `info.license.name` (fix with TD-S14 OpenAPI rewrite)

6. **`demo/.env.example`** in SDK repo — has hardcoded vLLM URL (`spark-3171`), needs generic placeholder

7. **GitHub public flip** — after domain, after all docs clean, after external security audit
