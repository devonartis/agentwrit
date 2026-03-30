# MEMORY.md — agentauth-core

## Source Pattern

**[Ephemeral Agent Credentialing v1.2](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.2.md)** — the security pattern AgentAuth implements. Every feature, fix, and design decision traces to this document.

## Origin

This repo was cloned from `agentauth-internal` at commit `2c5194e` (TD-006: Per-App JWT TTL). It contains all 8 v1.3 blueprint core components plus the complete app credential lifecycle. Zero HITL code — verified.

**Fork point:** `2c5194e` — all 8 core components + app registration + app launch tokens + per-app configurable TTL.

## Open-Core Model

AgentAuth uses an open-core model:

- **Core (this repo):** 8 blueprint components + App credential lifecycle. Will become open-source.
- **Add-ons (separate repo, future):** HITL approval flow, OIDC provider, cloud credential exchange, federation bridge. Stays private/enterprise.

Both the legacy repos are kept as private archives:

- `agentauth-internal` (git@github.com:devonartis/agentauth-internal.git) — 412 incremental commits, real feature-by-feature history
- `agentauth` (git@github.com:divineartis/agentauth.git) — production hardening commits, enterprise add-ons, migration planning docs

## Branching Model

GitFlow: `main` → `develop` → `fix/*` or `feature/*` branches. Cherry-pick batches use `fix/` branches merged to `develop`, then `develop` merged to `main` after verification.

## Current State

**Migration in progress — B5 cherry-picked, gates G1-G3 PASS.** SecurityHeaders middleware, global MaxBytesBody, error sanitization (val_hdl, renew_hdl, ValMw) all landed. Docs updated. Needs Docker gates (G4-G7), acceptance tests, review, then merge to develop.

**Current branch:** `fix/sec-l2b` — 4 cherry-pick commits (1 skipped as empty). G1-G3 PASS. Next: Docker gates, acceptance tests from `agentauth/tests/fix-sec-l2b/`, code review, merge.

Use the `cherrypick-devflow` skill to run the migration. After migration is complete, switch to `devflow` for new feature development.

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
| B0: Sidecar Removal | Remove sidecar subsystem | `34bb887` `909a777` | **done** — merged to fix/sidecar-removal, needs go test + merge to develop |
| B1: P0 | Persistent signing key, graceful shutdown | `9c1d51d` `f96549f` `6d0d77d` `cec8b34` `0fef76b` `e823bea` | **done** — merged |
| B2: P1 | Config file parser, bcrypt admin auth, aactl init | `313aa41` `869a8f7` `58cbce2` `4978ecd` `866cc78` `3dfada7` `ebc4884` `1c5f293` | **done** — merged |
| B3: SEC-L1 | Bind address, TLS enforcement, timeouts, weak secret denylist | `632b224` `6fa0198` `574d3b9` `cd09a34` `5489679` | **done** — merged |
| B4: SEC-L2a | Token alg/kid validation, MaxTTL, revocation hardening | `8e63989` `0526c46` `c24e442` `67aeda7` `b78edb8` `ecb4c86` `078a674` `8366fa9` | **done** — 13/13 PASS, merged |
| B5: SEC-L2b | Security headers, MaxBytesBody, error sanitization | `daf2995` `e592acc` `2857b3a` `247727c` `c5da6c4` | **in progress** — G1-G3 PASS, needs Docker gates + acceptance tests |
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

## Standing Rules

- **Live tests require Docker** — `./scripts/stack_up.sh` first. No Docker = not a live test.
- **No HITL in core** — zero tolerance. `grep -ri "hitl\|approval" internal/ cmd/` must return nothing.
- **Cherry-pick one batch at a time** — build + test after each batch before proceeding.
- **Docs update WITH every code change** — if code changes behavior, the docs update goes in the same commit or the same branch. No "fix docs later." B0-B4 proved that deferred doc updates cause massive drift. The doc files to check: `docs/api.md`, `docs/architecture.md`, `docs/concepts.md`, `docs/implementation-map.md`, `docs/scenarios.md`, `docs/api/openapi.yaml`.
- **Use `cherrypick-devflow` skill** for migration. Use `devflow` for new features after migration.

## Recent Lessons (last 3 sessions — older archived to MEMORY_ARCHIVE.md)

- B5 (SEC-L2b): Commit `247727c` (renew_hdl sanitization) was empty after conflict resolution — the sanitized `WriteProblem` call and tests were already present from `e592acc`. Skipped safely.
- B5: Commit `c5da6c4` had a modify/delete conflict on `tests/fix-sec-l2b/evidence/S3-renew-tampered-generic.md` — evidence file doesn't exist in core (deleted during internal cleanup). Removed and continued.
- B5: `e592acc` conflict in `main.go` contained OIDC routes (`/v1/jwks`, `/.well-known/openid-configuration`) and cloud handler (`/v1/cloud/credentials`). All dropped — add-on code.
- B5: `handler_test.go` already had `newTestBroker` wired with SecurityHeaders + MaxBytesBody from prior batches. Only the new test functions at the bottom needed merging.
- B5: Missing `context` and `errors` imports in `handler_test.go` after cherry-pick — needed by `TestRenew_DirectErrorMessageIsGeneric` which uses `context.Background()` and `errors.New()`. LSP diagnostics caught it.
- Doc overhaul (2026-03-30): B0-B4 docs were never updated with code changes. Result: 54 findings (8 CRIT, 22 HIGH). Fixed all on `fix/docs-overhaul`. Standing rule: docs update WITH every code change.
- jcodemunch indexes code symbols only — not markdown docs. Use context-mode (`ctx_execute_file`, `ctx_search`) for doc analysis to save context window.
- `settings.json` (project, committed) vs `settings.local.json` (personal, gitignored). Broad tool permissions go in project-level; machine-specific Bash patterns go in local.
- Next: B5 needs Docker gates (G4-G7), acceptance tests from `agentauth/tests/fix-sec-l2b/`, code review, then merge. After B5: B6 (SEC-A1 + Gates) — 2 commits, last batch.
