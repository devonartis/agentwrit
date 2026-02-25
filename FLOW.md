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
