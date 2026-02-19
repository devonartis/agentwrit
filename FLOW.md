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
