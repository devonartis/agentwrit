# MEMORY.md — agentauth-core

## Mission

**Build the open-source core of AgentAuth** — a production-grade, pluggable credential broker for AI agents implementing the **[Ephemeral Agent Credentialing v1.3](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.3.md)** security pattern.

**Core principles:**
- **Pattern-driven:** Every feature, fix, and design decision traces to the v1.3 pattern document. The code implements all 8 core components.
- **Pluggable architecture:** The core is designed so enterprise modules (HITL, OIDC provider, Resource Server, MCP integration, cloud credential exchange, federation bridge) can plug in without modifying core code. Interfaces and extension points over hard-coded integrations.
- **Zero add-on contamination:** No HITL, OIDC, cloud, federation, or sidecar code in this repo. Those are enterprise modules that plug into the core.
- **Minimal dependencies:** Ed25519/JWT/hash-chain/scope/revocation all use Go stdlib. Only 5 direct Go dependencies. Strong supply chain story.

## Origin

This repo was cloned from `agentauth-internal` at commit `2c5194e` (TD-006: Per-App JWT TTL). It contains all 8 v1.3 blueprint core components plus the complete app credential lifecycle. Zero HITL code — verified.

**Fork point:** `2c5194e` — all 8 core components + app registration + app launch tokens + per-app configurable TTL.

## Open-Core Model

AgentAuth uses an open-core model:

- **Core (this repo):** 8 blueprint components + App credential lifecycle. Pluggable extension points. Will become open-source.
- **Enterprise modules (separate repos, future):** HITL approval flow, OIDC provider, Resource Server, MCP integration, cloud credential exchange, federation bridge. Plug into core via interfaces. Stays private/enterprise.

Both the legacy repos are kept as private archives:

- `agentauth-internal` (git@github.com:devonartis/agentauth-internal.git) — 412 incremental commits, real feature-by-feature history
- `agentauth` (git@github.com:divineartis/agentauth.git) — production hardening commits, enterprise add-ons, migration planning docs

## Branching Model

GitFlow: `main` → `develop` → `fix/*` or `feature/*` branches. Cherry-pick batches use `fix/` branches merged to `develop`, then `develop` merged to `main` after verification.

## Current State

**Migration in progress — B5 merged, B6 is next (last batch).** B0-B5 all merged to develop. SecurityHeaders, MaxBytesBody, error sanitization all verified. LIVE-TEST-TEMPLATE improved with executive audience guidance.

**Current branch:** `develop` — clean. Next: `fix/sec-a1` for B6 (2 commits: TTL carry-forward on renewal, gates.sh regression).

Use the `cherrypick-devflow` skill to run the migration. After B6 merge → post-migration cleanup, then switch to `devflow` for new feature development.

## Key Documents (in legacy agentauth repo)

| Document | Path (in agentauth repo) | What |
|----------|-------------------------|------|
| Feature Inventory | `.plans/modularization/Cowork-Feature-Inventory.md` | Master inventory: milestones, cherry-pick list, delete list, execution steps |
| Cherry-Pick Guide | `.plans/modularization/Cherry-Pick-Guide.md` | Batch-by-batch cherry-pick instructions with conflict resolution guidance |
| Repo Directory Map | `.plans/modularization/Repo-Directory-Map.md` | What's in each repo, directory trees, quick reference |
| Feature Inventory (docx) | `.plans/modularization/Cowork-Feature-Inventory.docx` | Word doc version of the inventory |

## Cherry-Pick Batches

| Batch | What | Commits | Status |
|-------|------|---------|--------|
| B0: Sidecar Removal | Remove sidecar subsystem | `34bb887` `909a777` | **done** — merged |
| B1: P0 | Persistent signing key, graceful shutdown | `9c1d51d` `f96549f` `6d0d77d` `cec8b34` `0fef76b` `e823bea` | **done** — merged |
| B2: P1 | Config file parser, bcrypt admin auth, aactl init | `313aa41` `869a8f7` `58cbce2` `4978ecd` `866cc78` `3dfada7` `ebc4884` `1c5f293` | **done** — merged |
| B3: SEC-L1 | Bind address, TLS enforcement, timeouts, weak secret denylist | `632b224` `6fa0198` `574d3b9` `cd09a34` `5489679` | **done** — merged |
| B4: SEC-L2a | Token alg/kid validation, MaxTTL, revocation hardening | `8e63989` `0526c46` `c24e442` `67aeda7` `b78edb8` `ecb4c86` `078a674` `8366fa9` | **done** — 13/13 PASS, merged |
| B5: SEC-L2b | Security headers, MaxBytesBody, error sanitization | `daf2995` `e592acc` `2857b3a` `247727c` `c5da6c4` | **done** — G1-G6 PASS, 5/5 acceptance PASS, 1 SKIP, merged |
| B6: SEC-A1 + Gates | TTL bypass fix, gates regression | `9422e7c` `e395a15` | pending |

## Tech Debt (carried forward from internal — relevant to core only)

| ID | What | Severity |
|----|------|----------|
| TD-001 | `app_rate_limited` audit event not emitted (rate limiter fires before handler) | Low |
| TD-007 | Resilient logging — audit writes inline, no fallback on store failure | Medium |
| TD-008 | Token predecessor not invalidated on renewal — two valid tokens exist | Medium |
| TD-009 | JTI blocklist never pruned — memory grows indefinitely | Medium |
| TD-010 | Admin TTL hardcoded — should be operator-configurable | Low |

## Cowork ↔ Claude Code Coordination

When both Cowork and Claude Code are active, read `COWORK_SESSION.md` for shared state. It tracks who changed what and what's uncommitted.

## Docker Lifecycle & Admin Secret

**Standard test secret:** `live-test-secret-32bytes-long-ok` — used by `live_test.sh`, `live_test_docker.sh`, `test_batch.sh`, and the `broker-up` skill. Do NOT use any other secret for testing.

**Secret flow:** `AA_ADMIN_SECRET` env var → `docker-compose.yml` passes via `${AA_ADMIN_SECRET:-change-me-in-production}` → container env → `cfg.Load()` reads `os.Getenv("AA_ADMIN_SECRET")` → `main.go` fatals if empty. See `internal/cfg/cfg.go` and `cmd/broker/main.go`.

**Docker lifecycle scripts:** Use `scripts/stack_up.sh` (build + start) and `scripts/stack_down.sh` (teardown with `-v --remove-orphans`) for Docker operations. Raw `docker compose build` is OK for build-only (G4 gate). Do NOT use raw `docker compose down` — always use `stack_down.sh`.

## Acceptance Tests

Each cherry-pick batch has acceptance tests in `tests/<batch-name>/`:
- `user-stories.md` — stories with Who/What/Why/How/Expected
- `integration.sh` — automated script that runs all stories + regression tests against a live broker
- `evidence/` — terminal output from test runs

**Pattern:** Legacy tests in `agentauth/tests/` must be adapted for core before use. Remove all OIDC/HITL/cloud/sidecar/federation references. Update ports (8443), registration flow (launch tokens), and endpoint paths.

| Batch | Tests | Stories |
|-------|-------|---------|
| B0 | `tests/p0-production-foundations/` | 7 (K1-K5, S1-S2) |
| B1 | `tests/p0-production-foundations/` | Same as B0 |
| B2 | `tests/p1-admin-secret/` | 9 stories + 3 security reviews |
| B3 | `tests/sec-l1/` | 12 stories |
| B4 | `tests/sec-l2a/` | 13 stories (S1-S7, N1-N5, SEC1) |
| B5 | `tests/sec-l2b/` | 6 stories (S1-S4,S6 + S5 skip) + 4 regression (R1-R4) |
| B6 | TBD — must write before merge | TBD |

## Standing Rules

- **Live tests require Docker** — `./scripts/stack_up.sh` first. No Docker = not a live test.
- **No add-on code in core** — zero tolerance. `grep -ri "hitl\|approval\|oidc\|federation\|cloud\|sidecar" internal/ cmd/` must return nothing.
- **Cherry-pick one batch at a time** — build + test after each batch before proceeding.
- **Acceptance tests adapted for core** — legacy tests have OIDC/HITL/sidecar code. Always audit and adapt before copying to core.
- **Docs update WITH every code change** — if code changes behavior, the docs update goes in the same commit or the same branch. No "fix docs later." B0-B4 proved that deferred doc updates cause massive drift. The doc files to check: `docs/api.md`, `docs/architecture.md`, `docs/concepts.md`, `docs/implementation-map.md`, `docs/scenarios.md`, `docs/api/openapi.yaml`.
- **Use `cherrypick-devflow` skill** for migration. Use `devflow` for new features after migration.
- **Pluggable architecture** — core code must expose interfaces and extension points. Enterprise modules plug in; they never get baked into core.
- **MEMORY.md lessons learned EVERY session** — before clearing context or ending a session, update MEMORY.md with lessons learned. This is not optional. If you learned something, write it down. If the user corrected you, write down what they said and why.

## Backburner Designs (review after migration is complete)

- **Acceptance test automation + verification** — `.plans/designs/acceptance-test-automation.md`. Born during B5: how to automate story evidence creation while maintaining template compliance, and how to verify the agent followed the template with a deterministic hook. The `integration.sh` script is a CI smoke test — it does NOT produce proper evidence files. Three options captured: review hook, verify-evidence skill, or a runner script that produces template-compliant evidence. Review once B6 is merged.

## Recent Lessons (last 3 sessions — older archived to MEMORY_ARCHIVE.md)

### B5 Acceptance Testing (2026-03-30) — CRITICAL lessons

- **Acceptance tests are NOT integration scripts.** `integration.sh` runs PASS/FAIL checks but cuts corners: no individual story files, no executive-readable banners, no proper personas. It's a CI smoke test. Real acceptance tests produce individual `story-*.md` files per the `LIVE-TEST-TEMPLATE.md`.
- **Executives and QA testers read acceptance evidence.** Every banner (Who/What/Why/How/Expected) must make sense to a non-technical reader. Write for the executive, not the engineer.
- **Personas must reflect production reality.** "Developer (curl)" is wrong when the real actor is an automated App. Ask: "Who does this in production?" App = automated software. Developer = human exploring. Operator = human managing. Security Reviewer = verifying controls.
- **Ground every story in reality.** If using curl to emulate an app, say so: "We emulate what the app does in production." Don't describe testing mechanics — describe the real-world scenario.
- **Legacy acceptance tests need deep adaptation.** The legacy `integration.sh` had: wrong response field names (`token` vs `access_token`), wrong request field names (`allowed_scopes` vs `allowed_scope`, missing `agent_name`), wrong registration flow (simple name+scopes vs challenge-response with Ed25519), wrong nonce encoding (base64 vs hex), OIDC endpoints that don't exist in core. Every field must be verified against actual handler structs.
- **One story at a time, verdict earned.** Don't pre-write PASS. Run the story, see the output, then write the verdict based on what you actually observed.
- **LIVE-TEST-TEMPLATE updated** with: "Who Reads These Tests?" section, App persona, "Ground Every Story in Reality" guidance, Bad/Good banner examples.

### B5 Cherry-Pick (2026-03-30) — technical lessons

- B5: Commit `247727c` was empty after conflict resolution — content already present from `e592acc`. Skipped safely.
- B5: `e592acc` conflict in `main.go` contained OIDC routes and cloud handler. All dropped — add-on code.
- B5: Missing `context` and `errors` imports in `handler_test.go` after cherry-pick. LSP diagnostics caught it.
- B5: `curl -sI -X POST` returns empty headers for POST endpoints — use `curl -s -D - -o /dev/null` instead to dump headers on POST requests.
- jcodemunch indexes code symbols only — not markdown docs. Use context-mode for doc analysis.
- `settings.json` (project, committed) vs `settings.local.json` (personal, gitignored). Broad tool permissions go in project-level.
- Post-merge doc verification caught 2 critical inaccuracies: middleware ordering was backwards in architecture.md (19 route rows + prose), MaxBytesBody attributed to wrong source file in implementation-map.md. Fixed. Always verify docs against actual code after sub-agent updates.
- `cherrypick-devflow` skill updated: added Step 4 (Application Docs) and Step 5 (Acceptance Tests). Skill now has `references/acceptance-examples.md` with real bash examples showing how to create story evidence files.
- Skills use `references/` directory for companion docs that get loaded on demand. Keeps SKILL.md lean (<500 lines) while providing examples and detailed guidance.
- Next: B6 (SEC-A1 + Gates) — 2 commits, last batch. Then post-migration cleanup.
