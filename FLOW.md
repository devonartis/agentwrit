# FLOW.md

Running decision log. Append to this file continuously during every session.

## Rules

- After each brainstorming step, design decision, skill invocation, or meaningful action, append a 2-3 line summary.
- Capture **what** was done and **why** — the reasoning, trade-offs, and user intent behind decisions.
- Future sessions use this to understand thinking, not just output.
- Never overwrite existing entries. Always append under the current session date header.

### Superpowers Skill Logging

When any superpowers skill completes a phase, log it here with:
1. **Skill name** (brainstorming, writing-plans, executing-plans, TDD, etc.)
2. **Summary of decisions made** — what was chosen, what was rejected, and why
3. **Pointer to the artifact** — link to the design doc, plan, or implementation that was produced

Format:
```
### [Skill]: [Topic]
[2-3 line summary of decisions and reasoning]
→ Artifact: `path/to/document.md`
```

---

## 2026-02-26 (Session 16)

### Executing-Plans: Fix 6 — Structured Audit Log Fields

Implementing the last compliance fix. Functional options pattern (`RecordOption`) chosen to keep existing `Record()` callers backward-compatible — variadic `...RecordOption` means zero-option calls compile unchanged. New structured fields (`resource`, `outcome`, `deleg_depth`, `deleg_chain_hash`, `bytes_transferred`) added to `AuditEvent`, included in `computeHash()` for tamper evidence, persisted via SQLite migration, and exposed through query API with `outcome` filtering.

→ Plan: transcript plan from session 16

### Decision: AuditRecorder Interface Update

Changing `Record()` signature to `Record(..., opts ...RecordOption)` broke the `AuditRecorder` interfaces in `authz` and `identity` packages. Go structural typing means variadic params change the function signature — concrete types compile fine but interfaces with the old exact signature break. Fixed by updating both interfaces. Added `audit` import to `identity/id_svc.go` (verified no circular dependency: `audit` doesn't import `identity`).

### Decision: SQLite Nullable Columns for Backward Compat

New audit columns use `DEFAULT NULL` + `sql.NullString`/`sql.NullInt64` scan types so existing rows with NULL values don't break `LoadAllAuditEvents`. Helper funcs `nullableString()` and `nullableInt()` map zero-values to NULL on write (zero-value fields → NULL in DB, non-zero → real value).

---

## 2026-02-25 (Session 15)

### Architecture Decision: Keep Sidecar Model (ADR-002)

4-agent collaborative debate resolved the 6 architecture questions from Session 14. Decision: keep sidecars as the primary and only current model.

Key findings:
1. **Admin secret blast radius is unbounded** — every sidecar holds `AA_ADMIN_SECRET` which grants full admin scope. Scope ceiling enforcement does NOT bound admin credentials. This is a genuine security weakness, not theater.
2. **Scope ceiling enforcement is real** — dual enforcement at sidecar (`handler.go:78`) and broker (`token_exchange_hdl.go:131`) with cryptographically bound JWT claims. Anti-spoof protection on `sid` field.
3. **Direct broker access requires code changes** — `sidecarAllowedScopes()` specifically reads `sidecar:scope:X` prefix. App credentials don't have these. Not a config change — broker code must be extended.
4. **One sidecar per trust boundary** — the scaling unit is trust boundaries, not applications. This answers "N apps = N sidecars = N ports."

Rejected: direct broker access (requires broker changes, no use case today), hybrid model (doubles maintenance, "complexity of both with guarantees of neither"), remove sidecars entirely (loses DX, resilience, UDS, scope siloing).

→ Artifact: `plans/2026-02-25-sidecar-architecture-decision.md` (ADR-002)
→ Known issues: `KNOWN-ISSUES.md` (KI-001 through KI-004)

### Multi-Agent Team Orchestration Lessons

Three iterations to get team orchestration right:
1. **Team 1 (biased):** Pre-assigned FOR/AGAINST positions forced confirmation bias. Agents advocated rather than analyzed.
2. **Team 2 (isolated):** Separate prompts, separate files. Agents worked in silos, never communicated.
3. **Team 3 (collaborative):** Shared prompt, broadcast messaging, devil's advocate veto, one output file. Followed `plans/archive/agent-team-prompt.md` pattern exactly. Worked.

Key lessons: neutral positions > assigned positions, broadcast > DMs, shared prompt > separate prompts, DA veto is the quality gate. Agent shutdown is unreliable (multiple nudges needed).

→ Artifact: Team pattern documented in Obsidian insights (AI-Systems-Building)

### Fix 6 (Structured Audit) — preempted, not forgotten

Fix 6 was always the last fix in the sequence (2→3→4→1→5→6). Every session from 10 onward listed "then Fix 6" as the next step. Session 14 preempted it — user raised fundamental sidecar architecture questions that blocked Fix 5's merge. Session 15 was entirely consumed by the 4-agent architecture debate. Fix 6 was never claimed as done — it was deferred twice and is the only compliance fix remaining.

Design is ready (`plans/design-solution.md` lines 246-300). User stories exist (`tests/fix6-structured-audit-user-stories.md`). No code written, no branch created.

### Post-Merge Roadmap Created

Priorities after merging fix/sidecar-uds:
1. Fix 6 (structured audit) — last compliance fix, part of the original 6-fix plan, should be completed before docs/SDK work
2. Documentation deep dive (operator guide, developer guide, architecture FAQ)
3. Admin secret narrowing (KI-001) — new broker endpoint
4. SDK development (Python + TypeScript) — for operators and developers
5. Merge develop to main (release)

→ Artifact: `plans/2026-02-25-post-merge-roadmap.md`

---

## 2026-02-26 (Session 14)

### Executing-Plans: Fix 5 — Sidecar UDS Listen Mode

Executed `docs/plans/2026-02-25-fix5-sidecar-uds.md`. Three tasks: config field, UDS listener, Docker live test.

Key decisions:
1. **`startListener()` abstraction** — single function returns `net.Listener` + cleanup func. `http.Serve(ln, mux)` works identically for UDS and TCP — transport is decoupled from protocol.
2. **Socket permissions `0660`** — owner + group read/write. Restricts token requests to processes sharing the socket's group. Tighter than TCP's any-process-on-host model.
3. **Stale socket cleanup** — `os.Remove(socketPath)` before `net.Listen("unix", ...)`. Handles crashed sidecars that left socket files behind.
4. **TCP WARN log** — when `AA_SOCKET_PATH` unset, sidecar logs a warning about network exposure. Operator is never silently in a less-secure config.

→ Artifact: `fix/sidecar-uds` branch (5 commits, NOT merged — blocked on brainstorm)

### Finding: SQLITE_BUSY on concurrent sidecar activation

When two sidecars bootstrap at the exact same moment, concurrent `SaveSidecar` SQLite writes cause `SQLITE_BUSY`. One sidecar's persist fails (in-memory ceiling still works, tokens still issue). Pre-existing bug, not caused by Fix 5. Staggering startup avoids it. Proper fix: write mutex or WAL mode in SqlStore.

### Decision: HOLD merge — sidecar architecture brainstorm needed

User raised fundamental questions that go beyond Fix 5 implementation:
- Why do sidecars exist? What's the alternative?
- Can apps talk directly to the broker with client_id/client_secret instead?
- How would scope siloing work without sidecars?
- How do operators deploy sidecars for new apps?
- How do 3rd-party SDK consumers onboard?

These are architecture-level questions about the entire sidecar model, not Fix 5 bugs. The code works — the model needs to be justified and documented before it ships. Brainstorm must happen before merge.

**If sidecars stay:** comprehensive docs are a hard requirement before merge — operator guide (deploying sidecars for new apps), developer guide (SDK consumer onboarding), FAQ (why sidecars, alternatives), architecture updates. Current docs explain the *what* but not the *why* or the end-to-end *how*.

→ Artifact: questions captured in MEMORY.md Session 14

---

## 2026-02-25 (Session 13)

### Executing-Plans: Fix 1 — Sidecar TLS Client

Executed `docs/plans/2026-02-25-fix1-sidecar-tls-client.md`. Three tasks: config fields, broker client TLS, Docker live test.

Key decisions:
1. **TLS 1.3 minimum** — `buildTLSConfig` sets `MinVersion: tls.VersionTLS13`. Broker doesn't explicitly set MinVersion (defaults to TLS 1.2+), but negotiation works since TLS 1.3 clients can connect to TLS 1.2+ servers.
2. **Graceful fallback** — invalid CA cert logs a warning and falls back to plain HTTP rather than crashing. Bootstrap health check will fail and retry with backoff.
3. **SHA-256 certs required** — openssl defaults to SHA-1 for `x509 -req` signing, which TLS 1.3 rejects. Added `-sha256` to all signing commands in `gen_test_certs.sh`.
4. **curl in broker Dockerfile** — Alpine BusyBox wget doesn't support client cert flags. Added curl for mTLS healthcheck.

→ Artifact: `fix/sidecar-tls-client` branch (3 commits, merged to develop)

---

## 2026-02-19 (Session 5)

- Merged `feature/list-sidecars-endpoint` to `develop` — feature was complete, tests passing, no blockers.
- Moved 3 untracked docs (2 `.docx` roadmap exports + 1 duplicate `.md`) to `misc_docs/` instead of deleting — user wants to keep them until repo goes public.
- Deleted `docs/plans/` and added policy note to CLAUDE.md — `docs/` is strictly for application documentation. Plans, roadmaps, and session artifacts go elsewhere.
- Created FLOW.md — user wants a running decision log so brainstorming rationale and design choices persist across sessions.
- Starting CLI design (`cmd/cli/`) — Backlog #16, P1. Operators need CLI tooling to use admin endpoints without hand-crafting curl + JWT.

### Brainstorming: aactl CLI

**Binary name:** Chose `aactl` over `agentauth` and `agentauthctl` — short, fast to type, follows `*ctl` convention (kubectl, istioctl).

**Auth strategy:** Env vars only (`AACTL_BROKER_URL`, `AACTL_ADMIN_SECRET`). Rejected login-command + token-cache (over-engineered for demo) and per-call flags (terrible UX). User's key insight: in production the operator is remote and the shared secret model will be replaced entirely with real auth (mTLS/OIDC/API keys). So don't invest in demo auth plumbing — keep it simple and replaceable.

**CLI framework:** Cobra — industry standard, auto-generated help, shell completions. Rejected stdlib-only (too much manual parsing for 5+ subcommands).

**Output:** Table default + `--json` flag. Covers both interactive operators and CI/scripting pipelines.

**Scope:** Core 5 commands first (sidecars list, ceiling get/set, revoke, audit events). Deferred launch-token create and sidecar-activation create — less common operator flows, ship when needed.

→ Artifact: `.plans/active/2026-02-19-aactl-design.md`

### Writing-Plans: aactl CLI Implementation

9-task TDD plan: scaffold cobra root → HTTP client with auto-auth → output helpers → sidecars list → ceiling get/set → revoke → audit events → Docker E2E test → docs/changelog update. Each task is a single commit. Client auto-authenticates via env vars on every call — simple, stateless, easy to rip out when real auth lands.

→ Artifact: `.plans/active/2026-02-19-aactl-impl-plan.md`

**Standing rule added:** All Go files in this project must include godoc comments on every exported and package-level symbol (functions, types, variables). Subagents left this out in Tasks 1-5 — must be retrofitted and enforced for all remaining tasks.

### Subagent-Driven-Development: aactl CLI Implementation

9-task TDD plan executed via fresh subagents. All tasks complete: scaffold → HTTP client → output helpers → sidecars list → ceiling get/set → revoke → audit events → E2E Docker test → docs/changelog. Godoc retrofitted after Tasks 1-5 (standing rule added). Operator docs updated across 3 docs files. All gates pass (3 PASS, 1 WARN non-blocking). E2E confirmed all 5 command types against live Docker stack.

→ Artifact: `cmd/aactl/`

### Finishing-a-Development-Branch: feature/aactl-cli

Chose Option 1 (merge locally) over creating a PR — branch is feature-complete, gates pass, and no separate review team was requested. Merged `feature/aactl-cli` into `develop` via fast-forward (clean history, no merge commit). Verified gates on merged `develop` (3 PASS, 1 WARN non-blocking). Deleted feature branch. Pushed `develop` to origin at `1cb28e2`.

→ Artifact: `develop` branch, origin `1cb28e2`

---

## 2026-02-20 (Session 6)

- Harness work (autonomous coding agent harness) was built then deliberately removed — not needed, kept as `develop-harness-backup` branch for reference only.
- Cleaned up session artifacts: removed `conductor/`, `internal_use_docs/`, `misc_docs/` — these were never application content.
- Renamed `compliance_review/` → `plans/` and unignored so the directory is tracked in git going forward.

### Compliance Review: Round 2 (India, Juliet, Kilo, Lima)

Four independent reviewers evaluated develop against the Ephemeral Agent Credentialing Security Pattern v1.2. Codebase scored 92-96% compliance with zero NOT COMPLIANT findings. Key partial findings: no native TLS/mTLS (all 4), no task-completion signal (Juliet), revocation lost on restart (Kilo), audit Detail field is free-form (Kilo).

→ Artifacts: `plans/round2-reviewer-*.md`

### Design: Compliance Fix + Sidecar Sprawl

5-agent team (security-architect, system-designer, code-planner, integration-lead, devils-advocate) produced a single approved design. Six independently implementable fixes. Devils-advocate signed off. Key additional gap found by team (missed by all 4 reviewers): audience field is never set or validated — tokens can be presented to any resource server. Harness-based autonomous execution approach was explored and discarded; fixes will be implemented as standard feature branches.

→ Artifact: `plans/design-solution.md`, `plans/implementation-plan.md`

---

## 2026-02-24 (Session 7)

- Reconciled MEMORY.md and FLOW.md with actual git history — previous logs were incomplete.
- Confirmed `develop-harness-backup` is intentionally orphaned (no merge planned).
- `develop` is clean, ahead of `origin/develop` by 1 commit (`dcff7ec`).
- Ready to begin implementing the 6 compliance fixes from `plans/implementation-plan.md`.
- **Standing rule established:** Every fix/feature MUST include a Docker live test. Self-hosted binary tests are quick checks only. Docker is mandatory before merge. User stories go in `tests/<name>-user-stories.md` first. `docker-compose.yml` must be updated when new env vars are added. Added to CLAUDE.md.

### Fix 1 (broker TLS/mTLS) — In Progress

TDD RED confirmed: 3 cfg tests + 3 loadCA tests all failing before any production code written. GREEN: added `TLSMode`, `TLSCert`, `TLSKey`, `TLSClientCA` fields to `internal/cfg/cfg.go`, added `serve.go` + `loadCA()` to `cmd/broker/`, wired `serve()` into `main.go`. All 8 unit tests pass. Live test (`--tls`, `--mtls`) added to `live_test.sh`. User stories saved to `tests/fix1-broker-tls-user-stories.md`. Docker live test still needed — `docker-compose.yml` update pending.

---

## 2026-02-25 (Session 8)

### Docker TLS live test — revealed Fix 1 design gap

Built Docker TLS test infrastructure on `fix/broker-tls-docker-test` branch. Compose overlay pattern: `docker-compose.tls.yml` and `docker-compose.mtls.yml` layer TLS config on top of base compose file. Runtime cert generation via openssl (no certs in repo). Test script `live_test_docker.sh` extended with `--tls` and `--mtls` flags.

**TLS test (one-way) passed 10/10.** Key learnings during debugging:
- Sidecar needs `AA_BROKER_URL=https://broker:8080` when broker has TLS (was `http://`)
- Sidecar needs `SSL_CERT_FILE=/certs/cert.pem` for Go's crypto/tls to trust self-signed certs
- Go's TLS server returns HTTP 400 (not connection refused) when receiving plain HTTP — test assertion updated
- Certs must be mounted into sidecar container too (not just broker) since test curl runs inside sidecar

**mTLS test not runnable — design gap found.** The sidecar's `brokerClient` (`cmd/sidecar/broker_client.go`) uses a plain `http.Client` with no TLS configuration. It cannot present a client certificate. mTLS requires both sides to present certs. Fix 1 only implemented the broker server side.

### Decision: Fix 1 design was incomplete — redesign needed

The original design (`plans/design-solution.md`, Fix 1) scoped the work as broker-only: "Files: `internal/cfg/cfg.go`, `cmd/broker/main.go`". This was wrong. For mTLS to work in production:
1. Broker presents server cert + requires client certs (done)
2. Sidecar presents client cert + verifies broker cert (not done)
3. Sidecar's `AA_BROKER_URL` must switch to HTTPS (config, not code)

The implementation plan also claimed all 6 fixes were "independently implementable." This is incorrect — Fix 1 (TLS) requires sidecar client TLS, and Fix 5 (UDS) also modifies sidecar transport. They share the sidecar as a dependency and should be coordinated.

### Decision: go back to design before continuing implementation

User directed: commit what we have, go back to develop, redesign all 6 fixes with correct dependency ordering. The original phase ordering was:
```
Phase 1: Fix 1 (mTLS) + Fix 2 (revocation)
Phase 2: Fix 3 (audience) + Fix 4 (token release)
Phase 3: Fix 5 (UDS) + Fix 6 (audit)
```

This needs revision. Fix 1 and Fix 5 both touch sidecar transport and should be considered together. New design must map real dependencies.

### Lesson: over-engineering ceremony vs. just doing the work

User frustrated with brainstorming skill → design doc → implementation plan → subagent-driven-development chain for what was essentially "write 3 files and run tests." The ceremony added significant overhead without proportional value. For tactical work (test infrastructure, config fixes), just do the work. Reserve the full skill chain for genuinely complex design decisions.

### Lesson: Docker live tests catch real integration issues

The TLS Docker test caught two categories of bugs that unit tests cannot:
1. **Configuration gaps**: sidecar `AA_BROKER_URL` not switching to HTTPS, cert mounting
2. **Design gaps**: sidecar missing TLS client support entirely

This validates the standing rule. The Docker test should have been part of Fix 1 from the start.

→ Artifacts: `fix/broker-tls-docker-test` branch (compose overlays, test script, WIP sidecar TLS)

---

## 2026-02-25 (Session 9)

### Brainstorming: Redesign all 6 compliance fixes

Old design claimed all fixes were "independently implementable" — Session 8 Docker testing proved this wrong. Redesigned from scratch using first-principles ordering:

1. **Security gaps before compliance gaps** — Fix 2 (revocations lost on restart) is exploitable; Fix 1 (TLS) is a spec checkbox
2. **Foundations before dependents** — Fix 4 calls `revSvc.Revoke()`, needs Fix 2 for persistence
3. **Widest change last** — Fix 6 touches ~9 files, goes last to pick up all new callers
4. **Same-binary changes adjacent** — Fix 1 and Fix 5 both modify sidecar, done back-to-back
5. **Small fast wins early** — Fix 2, 3, 4 are all under 120 lines

Fix 1 scope corrected to include sidecar TLS client (broker_client.go, config.go) — the gap Session 8 found.

→ Artifact: `plans/design-solution.md` (v2, from scratch)

### Writing-Plans: 6 implementation plans

One plan file per fix. Each follows TDD pattern: write failing test → implement → verify → commit. Each plan ends with gates + Docker live test. Plans are in execution order: fix2 → fix3 → fix4 → fix1 → fix5 → fix6.

User feedback drove key decisions: don't patch old broken designs (rewrite from scratch), separate files per fix, recommend with reasoning instead of asking which approach.

→ Artifacts: `docs/plans/2026-02-25-fix{1-6}-*.md`

---

## 2026-02-25 (Session 10)

### Executing-Plans: Fix 2 — Revocation Persistence

Executed `docs/plans/2026-02-25-fix2-revocation-persistence.md` via TDD. Plan was accurate — no deviations needed.

Key implementation decisions:
1. **Write-through pattern** matches existing audit/sidecar persistence
2. **`INSERT OR IGNORE`** with `UNIQUE(level, target)` — idempotent by design
3. **Store is optional** (`nil`) — existing code works unchanged
4. **Persistence failure is non-blocking** — warns but doesn't fail the revocation
5. **`LoadFromEntries` uses anonymous struct** — keeps revoke package decoupled from store

→ Artifact: `fix/revocation-persistence` branch (5 commits)

### Process established: Docker live test for every fix

**The correct process for Docker live testing:**
1. `./scripts/stack_up.sh` — bring up the stack
2. Verify healthy: `curl http://127.0.0.1:8080/v1/health`
3. Run the user story commands against the running stack (admin auth, revoke, validate, restart, check SQLite, etc.)
4. Verify each story passes
5. `docker compose down -v` — tear down

**Do NOT use `live_test_docker.sh` for manual testing** — it creates its own isolated stack and conflicts with `stack_up.sh`. The automated script is for CI. Manual testing runs commands directly against the stack.

**The test must be designed BEFORE implementation.** Read user stories, understand the test infrastructure, identify constraints (like ephemeral signing keys), then code. Not the other way around.

### Lesson: ephemeral signing keys affect revocation testing

Signing keys are regenerated on every startup. After restart, ALL pre-restart tokens fail signature verification before the revocation check runs. Cannot distinguish "revoked" from "bad signature" on pre-restart tokens via validate. Must test persistence via: SQLite inspection, broker startup logs (`revocations loaded count=N`), and fresh tokens for false-positive testing.

This should have been understood before coding — it shapes the entire test design.

→ Artifact: Docker live test steps documented in MEMORY.md Session 10

---

## 2026-02-25 (Session 11)

### Executing-Plans: Fix 3 — Audience Validation

Executed `docs/plans/2026-02-25-fix3-audience-validation.md` via TDD. Plan was mostly accurate but missed 2 of 5 token issuance paths.

Key decisions:
1. **`LookupEnv` over `envOr`** — empty string means "disable validation", unset means "use default agentauth"
2. **Audience check placement** — after revocation check in ValMw, before context storage
3. **Propagation model** — set once at registration, preserved through renewal/delegation/exchange
4. **AdminSvc needs audience too** — plan missed this; Docker live test caught it immediately

### Lesson: every token issuance path must be audited

The plan identified 3 issuance paths (IdSvc.Register, TknSvc.Renew, DelegSvc.Delegate). Reality had 5 more: AdminSvc.Authenticate, AdminSvc.ActivateSidecar, handler.TokenExchange, plus seedAdmin and CreateSidecarActivationToken (last two use special-purpose audiences). When adding a claim to all tokens, grep for `tknSvc.Issue(` to find every path.

→ Artifact: `fix/audience-validation` branch (5 commits)

---

## 2026-02-25 (Session 12)

### Executing-Plans: Fix 4 — Token Release

Executed `docs/plans/2026-02-25-fix4-token-release.md`. Implementation was straightforward — ~45 lines of handler code, 4 unit tests, route wiring.

Key decisions:
1. **No scope gate** — release is self-revocation, any authenticated agent can release its own token. No admin scope needed.
2. **aactl tooling as part of the fix** — user called out manual curl testing as unshippable. Added `aactl token release --token <jwt>`.
3. **Double-release idempotency** — middleware rejects already-revoked tokens with 403. aactl treats "token has been revoked" 403 as idempotent success.
4. **`ContextWithClaims` test helper** — exported for handler tests that need to inject claims without going through full middleware.

### Lesson: every endpoint needs operator tooling

User feedback: "are you hacking the systems" when seeing manual curl chains for Docker live testing. Endpoints without aactl commands are not shippable — same lesson as Session 3 (list-sidecars). Standing rule added: no endpoint ships without aactl tooling, no raw curl in tests (except public/unauthed endpoints).

→ Artifact: `fix/token-release` branch (2 commits)
