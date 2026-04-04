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

**Migration: B6 acceptance tests PASS — pending code review and merge (last batch).** B0-B5 merged. B6 on `fix/sec-a1` with all gates green and 4/4 acceptance stories PASS.

**Current branch:** `fix/sec-a1` — ready for merge after code review. Then post-migration cleanup (Go module path update, final verification, remote swap), then switch to `devflow` for new feature development.

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
| B6: SEC-A1 + Gates | TTL bypass fix, gates regression | `9422e7c` `e395a15` | **done** — G1-G6 PASS, 4/4 acceptance PASS, pending merge |

## Tech Debt

See `TECH-DEBT.md` at repo root for the full tech debt tracker.

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
| B6 | `tests/sec-a1/` | 4 stories (S1-S2, S3, R1) |

## Standing Rules

- **Live tests require Docker** — `./scripts/stack_up.sh` first. No Docker = not a live test.
- **No add-on code in core** — zero tolerance. `grep -ri "hitl\|approval\|oidc\|federation\|cloud\|sidecar" internal/ cmd/` must return nothing.
- **Cherry-pick one batch at a time** — build + test after each batch before proceeding.
- **Acceptance tests adapted for core** — legacy tests have OIDC/HITL/sidecar code. Always audit and adapt before copying to core.
- **Docs update WITH every code change** — if code changes behavior, the docs update goes in the same commit or the same branch. No "fix docs later." B0-B4 proved that deferred doc updates cause massive drift. The doc files to check: `docs/api.md`, `docs/architecture.md`, `docs/concepts.md`, `docs/implementation-map.md`, `docs/scenarios.md`, `docs/api/openapi.yaml`.
- **Use `cherrypick-devflow` skill** for migration. Use `devflow` for new features after migration.
- **Pluggable architecture** — core code must expose interfaces and extension points. Enterprise modules plug in; they never get baked into core.
- **MEMORY.md lessons learned EVERY session** — before clearing context or ending a session, update MEMORY.md with lessons learned. This is not optional. If you learned something, write it down. If the user corrected you, write down what they said and why.
- **Strong code comments on ALL code** — every function, handler, and type must have comments explaining what it does, who can call it (role/scope), why it exists, and its boundaries. See `.claude/rules/golang.md` for the full standard. Code must be self-documenting — if you have to read three other files to understand who can call a function, the comments are insufficient.
- **Role model document required** — `docs/roles.md` defines who does what: Admin (operator), App (software managing agents), Agent (does work). All code and tests must align with these roles. See `TECH-DEBT.md` TD-012 for the gap. No acceptance test should be written without understanding the role model first.

## Backburner Designs (review after migration is complete)

- **Acceptance test automation + verification** — `.plans/designs/acceptance-test-automation.md`. Born during B5: how to automate story evidence creation while maintaining template compliance, and how to verify the agent followed the template with a deterministic hook. The `integration.sh` script is a CI smoke test — it does NOT produce proper evidence files. Three options captured: review hook, verify-evidence skill, or a runner script that produces template-compliant evidence. Review once B6 is merged.

## Recent Lessons (last 3 sessions — older archived to MEMORY_ARCHIVE.md)

### Python SDK v0.2.0 Session (2026-04-01) — extraction, cleanup, and live verification

**What happened:**
- Extracted Python SDK from monorepo via `git filter-repo`
- Wrote spec and implementation plan for HITL removal + API alignment
- Executed 12-task plan: removed HITLApprovalRequired class, approval_token parameter, HITL error parsing, HITL demo app, HITL docs/tests
- Code review caught HITL contamination in docs/ (4 files) — not covered by original plan. Fixed.
- Expanded contamination guard tests to scan docs/ and README in addition to src/
- Live broker verification: 13 integration tests passed against broker v2.0.0
- All API field names aligned — the known mismatches from MEMORY.md (token vs access_token, etc.) were already fixed during the monorepo phase
- Merged to main as v0.2.0 (14 commits, 2416 lines removed, 164 added)

**What we discovered:**
- `examples/hitl-demo/` was a full FastAPI app with templates — not documented in the design doc. Discovered during implementation and added to the deletion plan.
- API contract was already aligned from code inspection — live broker testing confirmed it. The "known mismatches" from the parent project were stale.
- Code review is essential even for removal work — the plan missed docs/ contamination. The reviewer caught it.
- Comments should explain intent, not restate code. User corrected this multiple times.

**What's NOT done:**
- No demo application (deleted HITL demo, clean replacement needed)
- Not pushed to GitHub yet
- No CI (GitHub Actions)
- Not on PyPI

**This repo (`agentauth-core`) tracks:** strategic decisions about the SDK (release strategy, repo model). The SDK repo (`~/proj/agentauth-python`) tracks its own implementation.

### Release Strategy Session (2026-03-31) — architectural planning

**What happened:**
- Cloned and analyzed `devonartis/agentauth-clients` — monorepo with Python and TypeScript SDKs, built against the OLD broker (`authAgent2`) with HITL/OIDC baked in.
- Researched how real open-source projects handle SDK placement: Model 1 (per-language repos — Stripe, Twilio, HashiCorp), Model 2 (multi-SDK monorepo — AWS), Model 3 (SDKs in server repo — small projects).
- Decision: **Model 1 — separate per-language repos.** Aligns with open-core model, gives clean package identity, independent release cycles.
- Wrote high-level release strategy at `.plans/release-strategy.md` covering 4 phases: repo cleanup/archive → SDK repo setup → SDK core update → future enterprise extensions. Each phase will break into its own devflow cycle.

**What we discovered — golden information:**
- **SDK placement is one of the most consequential repo-architecture decisions in open-source.** It determines release cadence coupling, contributor experience, and how consumers discover and trust your SDKs. Getting this wrong creates friction that compounds over time.
- **The SDKs have enterprise contamination.** Both Python and TS SDKs have HITL baked in: `HITLApprovalRequired` exception, HITL retry logic in `get_token`, HITL demo app, HITL implementation guides, HITL integration tests. This mirrors the sidecar contamination we cleaned from the broker in B0 — same pattern, different layer.
- **Most of the SDK endpoint calls DO exist in core.** 7/8 endpoints the SDKs call are in `agentauth-core`. Only the HITL retry with `approval_token` is missing. The update is surgical, not a rewrite.
- **Three archives will exist:** `agentauth-internal` (golden history), `agentauth` (enterprise/HITL — becomes archive #2), `agentauth-clients` (current monorepo — becomes archive #3). Three active repos: `agentauth` (core broker), `agentauth-python`, `agentauth-ts`.
- **The rename is the natural moment to restructure.** `agentauth-core` → `divineartis/agentauth` triggers Go module path changes anyway — might as well set up SDK repos at the same time.

**Session thoughts:**
- The SDK work is phases of work, each of which would break into its own brainstorm → spec → plan cycle via devflow. Phase 1-2 are repo operations (git/GitHub). Phase 3 is real development work that needs the full devflow treatment.
- User was clear: high-level plan first, details later. Don't over-specify. Each phase becomes its own devflow cycle when we get to it.

### B6 Session (2026-03-30) — CRITICAL lessons learned

**What went wrong — user corrections:**

1. **Agent kept skipping banners on acceptance tests.** User had to stop me THREE times because I jumped straight to running curl commands without writing the Who/What/Why/How/Expected banner first. The template is non-negotiable. Banner goes IN the bash call, not as a separate step. Verdict comes AFTER seeing output, never pre-written.

2. **Agent built the first acceptance test against the admin flow instead of the app flow.** User caught it: "why are we using launch-token from admin to check agents?" In production, APPS create launch tokens for agents, not admin. Admin registers apps, apps manage agents. The agent didn't know this because nothing in the code or docs explained the role model.

3. **Agent called the handler ownership issue a "code smell" when it was actually a missing foundational document.** User walked me through why `admin:launch-tokens:*` exists (admin needs authority over launch tokens for revocation/oversight) and why admin creating agents is the wrong use of that scope. The agent kept downgrading the severity because it didn't understand the system's intent. User escalated: "you are writing code that you are not properly documenting the code nor giving app documentation."

4. **Agent tried to fix test failures inline instead of running all tests first.** User corrected: "why are you not running acceptance tests all of them then we search on we fix afterwards it is a loop." Run everything, see what fails, then fix. Don't stop to debug after every failure.

5. **Agent put tech debt in MEMORY.md.** User: "that is stupid we should have a TECH-DEBT.md." Then agent put it in `.plans/TECH-DEBT.md`. User: "that should be on the root not in the .plans folder." TECH-DEBT.md already existed at `.plans/` — agent didn't check first before trying to create a new file.

6. **Agent wrote code comments that restated what the code does.** User corrected: "a person or agent can read the code by itself to know what it does." Comments must tell you what reading the code alone would NOT tell you: who calls it, why it exists, security boundaries, design history. If you have to read three other files to understand who can call a function, the comments are insufficient.

**What we discovered — golden information:**

- **Code comments are the interface between human intent and agent execution.** Multiple agent sessions wrote and reviewed code without flagging that the role model was undocumented. Each agent looked at the code, made assumptions, and moved on. Comments that explain roles and boundaries would have prevented every mistake in this session. Without them, agents compound wrong assumptions across sessions.
- **If comments are strong, you can generate missing docs FROM the comments.** If comments are weak, you can't build docs, you can't build correct tests, and agents keep making the same mistakes. Strong comments → correct tests → correct docs. Weak comments → compounding errors.
- **The three roles are: Admin (operator — manages apps, revokes, audits), App (software — manages its own agents within scope ceiling), Agent (does work with short-lived scoped tokens).** This was nowhere in the code or docs. Now in TECH-DEBT.md as TD-012 (CRITICAL) and partially in code comments on `tkn_svc.go`.
- **`admin:launch-tokens:*` scope makes sense for oversight (list, inspect, revoke launch tokens) but the code lets admin CREATE launch tokens with no scope ceiling.** That's a design issue (TD-013), not a code smell. Admin-created agents have no AppID, no scope ceiling, no traceability.
- **Regression unit tests belong BEFORE the gate suite**, not after. New Step 3 in cherrypick-devflow. The tests get included in G2 (unit tests gate), catching regressions before spending time on Docker builds and acceptance tests.
- **Think through the test plan BEFORE writing code.** The agent kept jumping to curl commands, hitting wrong field names, wrong endpoints, wrong flows — all because it didn't verify the API contract first. Banner-first forces you to think about WHO does WHAT before typing a single command.

