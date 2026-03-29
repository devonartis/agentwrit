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

**Migration in progress — B0 complete.** Sidecar subsystem removed (cherry-picked `34bb887` and `909a777` from agentauth). Go code is zero-sidecar. Docs still have some sidecar user guide references (will rewrite later).

**Current branch:** `fix/sidecar-removal` — needs `go build ./...` and `go test ./...` on local machine (sandbox has no Go), then merge to `develop`.

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
| B1: P0 | Persistent signing key, graceful shutdown | `9c1d51d` `f96549f` `6d0d77d` `cec8b34` `0fef76b` `e823bea` | pending |
| B2: P1 | Config file parser, bcrypt admin auth, aactl init | `313aa41` `869a8f7` `58cbce2` `4978ecd` `866cc78` `3dfada7` `ebc4884` `1c5f293` | pending |
| B3: SEC-L1 | Bind address, TLS enforcement, timeouts, weak secret denylist | `632b224` `6fa0198` `574d3b9` `cd09a34` `5489679` | pending |
| B4: SEC-L2a | Token alg/kid validation, MaxTTL, revocation hardening | `8e63989` `0526c46` `c24e442` `67aeda7` `b78edb8` `ecb4c86` `078a674` `8366fa9` | pending |
| B5: SEC-L2b | Security headers, MaxBytesBody, error sanitization | `daf2995` `e592acc` `2857b3a` `247727c` `c5da6c4` | pending |
| B6: SEC-A1 + Gates | TTL bypass fix, gates regression | `9422e7c` `e395a15` | pending |

## Tech Debt (carried forward from internal — relevant to core only)

| ID | What | Severity |
|----|------|----------|
| TD-001 | `app_rate_limited` audit event not emitted (rate limiter fires before handler) | Low |
| TD-007 | Resilient logging — audit writes inline, no fallback on store failure | Medium |
| TD-008 | Token predecessor not invalidated on renewal — two valid tokens exist | Medium |
| TD-009 | JTI blocklist never pruned — memory grows indefinitely | Medium |
| TD-010 | Admin TTL hardcoded — should be operator-configurable | Low |

## Standing Rules

- **Live tests require Docker** — `./scripts/stack_up.sh` first. No Docker = not a live test.
- **No HITL in core** — zero tolerance. `grep -ri "hitl\|approval" internal/ cmd/` must return nothing.
- **Cherry-pick one batch at a time** — build + test after each batch before proceeding.
- **Use `cherrypick-devflow` skill** for migration. Use `devflow` for new features after migration.

## Lessons Learned (from internal development)

- The original agentauth repo was a file COPY (not clone) of agentauth-internal — that's why it had no history. This time we cloned properly so all 412 commits are preserved.
- Phase 1C-alpha (`3f9639f`) looks clean but has `hitl_scopes` baked into the app data model in 4 source files. Fork point must be `2c5194e` (TD-006) to get truly zero HITL.
- SEC-L1/L2a/L2b commits are on the P2 branch which also has OIDC code. Cherry-picks from these commits may have OIDC context in conflict markers — always check for IssuerURL, federation, thumbprint, jwk references and drop them.
- `cfg.go` is the most conflict-prone file — it gets modified by P1, SEC-L1, and SEC-L2a. Each batch adds fields to the same struct.
- B0 sidecar removal cherry-pick (`34bb887`) conflicted in 5 files (MEMORY.md, tkn_svc.go, renew_hdl.go, sql_store.go x3, admin_hdl_test.go). Key resolution: remove SidecarID from IssueReq/claims, remove ScopeCeiling from renewResp, remove sidecar CRUD/tables from store, remove stale sidecar comment from routes. Keep app-level code intact.
- Cherry-pick brings stale files from agentauth (flow.md, .vscode/, .plans/production-gap-analysis.md) — always unstage and discard these before committing.
