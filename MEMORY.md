# MEMORY.md

## 2026-02-24 (Session 7)

Session work:
- Reviewed git log and reconciled MEMORY.md + FLOW.md with actual history
- Confirmed harness removal was intentional — `develop-harness-backup` preserved as reference only, not to be merged
- `develop` is ahead of `origin/develop` by 1 commit (`dcff7ec`)
- Starting implementation of 6 compliance fixes from `plans/implementation-plan.md`

What's next:
1. Implement Fix 1 (mTLS) — `feature/broker-tls`
2. Implement Fix 2 (revocation persistence) — `feature/revocation-persistence`
3. Implement Fix 3 (audience validation) — `feature/audience-validation`
4. Implement Fix 4 (token release) — `feature/token-release`
5. Implement Fix 5 (sidecar UDS) — `feature/sidecar-uds`
6. Implement Fix 6 (structured audit) — `feature/structured-audit`

See `plans/implementation-plan.md` and `plans/design-solution.md` for full spec.

## 2026-02-20 (Session 6)

Session work:
- Cleanup: removed `conductor/` directory, removed `internal_use_docs/` and `misc_docs/` session artifacts (`c7df130`, `dcff7ec`)
- Renamed `compliance_review/` → `plans/` and unignored it (`c8bfcb0`, `39c3c49`)
- Harness work (autonomous coding agent harness) was built then deliberately removed — preserved as `develop-harness-backup` branch for reference
- Ran 4 compliance reviewers (India, Juliet, Kilo, Lima) against develop branch — results in `plans/round2-reviewer-*.md`
- Ran 5-agent design team (security-architect, system-designer, code-planner, integration-lead, devils-advocate) to produce design and plan
- Design approved by devils-advocate — `plans/design-solution.md`
- Implementation plan written — `plans/implementation-plan.md`

Six fixes identified (all independently implementable):
1. Fix 1: Native TLS/mTLS in broker (P0) — `feature/broker-tls`
2. Fix 2: Revocation persistence to SQLite (P0) — `feature/revocation-persistence`
3. Fix 3: Audience validation enforcement (P1) — `feature/audience-validation`
4. Fix 4: Token release endpoint (P1) — `feature/token-release`
5. Fix 5: Sidecar UDS listen mode (P1) — `feature/sidecar-uds`
6. Fix 6: Structured audit log fields (P2) — `feature/structured-audit`

## 2026-02-19 (Session 5)

Session work:
- Merged `feature/list-sidecars-endpoint` to `develop`
- Moved docx files to `misc_docs/`, deleted `docs/plans/`, added docs-only policy to CLAUDE.md
- Created `FLOW.md` as running decision log (pointed from CLAUDE.md)
- Brainstormed `aactl` CLI design — cobra, env var auth (demo only), table+json output, core 5 commands first
- Design approved, moving to implementation planning
- Wrote 9-task implementation plan → `.plans/active/2026-02-19-aactl-impl-plan.md`
- Chose subagent-driven execution (fresh subagent per task) over parallel session — user wants isolated subagents to preserve main context. Each task gets an implementer subagent + spec review + code quality review before moving to next.
- Branch: `feature/aactl-cli` from `develop`
- Implemented `aactl` CLI (`cmd/aactl/`) — Tasks 1-9 complete, all gates pass, E2E verified against Docker stack
  - 5 commands: sidecars list, ceiling get/set, revoke, audit events
  - Godoc comments on all exported symbols
  - Operator docs updated (getting-started-operator.md, common-tasks.md, architecture.md)
  - Branch: `feature/aactl-cli` — ready for review/merge

See `FLOW.md` for full decision rationale.

## 2026-02-19 (Session 4)

Session work:
- Fixed broken `prime` skill — was standalone `prime.md` with wrong frontmatter, restructured to `prime/SKILL.md` directory format with correct fields (`description`, `allowed-tools`)
- Moved `docs/claude-code-subagent-guide.md` to `misc_docs/`
- Committed all outstanding doc changes (impl plan, backlog, roadmap, CLAUDE.md, MEMORY.md) as `bb09ef1`
- Logged insight about Claude Code skill format to daily note + AI-Systems-Building insights log

Branch: feature/list-sidecars-endpoint — NOT merged yet
Remaining untracked: `docs/*.docx` files and a duplicate roadmap copy

What's next:
1. Merge `feature/list-sidecars-endpoint` to develop (feature code done, tests passing)
2. Build `cmd/cli/` — Go CLI for admin endpoints (Backlog #16, P1). This is the blocker that makes admin endpoints shippable. Start with `agentauth-cli sidecars list` to exercise the endpoint we just built.
3. Clean up untracked `.docx` files in `docs/`

## 2026-02-19 (Session 3)

Session work:
- Continued from Session 2 (context compaction)
- Updated MEMORY.md, BACKLOG.md, and Roadmap with CLI gap finding

Critical finding — No CLI in Go repo:
- Built list sidecars endpoint (GET /v1/admin/sidecars) but there's no CLI to access it
- Operators can't use admin endpoints without manually crafting curl + JWT
- CLI does NOT belong in agentauth-app (that's a Python demo app that can change)
- CLI must live in this Go repo as `cmd/cli/` — third binary alongside broker and sidecar
- Added as Backlog #16 (P1) and Roadmap 5.3a
- Docker live test confirmed endpoint works (HTTP 200 with correct JSON, HTTP 401 for unauthed)

User feedback:
- "there is no cli for this in that repo and it should not be in that repo"
- "why would we write this without a cli to access it, can you explain how else is this used realistically"
- Endpoints without operator tooling are not shippable

Branch: feature/list-sidecars-endpoint (from develop) — NOT merged yet
Docker containers still running on ports 8080/8081

## 2026-02-19 (Session 2)

Session work:
- Implemented list sidecars endpoint (Backlog #5) — GET /v1/admin/sidecars
- SQLite sidecar persistence with dual-write pattern (same as audit persistence)
- Store methods: SaveSidecar, ListSidecars, UpdateSidecarCeiling, UpdateSidecarStatus, LoadAllSidecars
- Prometheus metrics: agentauth_sidecars_total gauge, agentauth_sidecar_list_duration_seconds histogram
- Wired SaveSidecar into ActivateSidecar, UpdateSidecarCeiling syncs to SQLite
- Startup loading: LoadAllSidecars populates ceiling map from SQLite on broker start
- Integration test: full end-to-end through HTTP (admin auth → activate sidecar → list sidecars)
- 10-task subagent-driven TDD implementation with spec reviews after each task

Branch: feature/list-sidecars-endpoint (from develop)

## 2026-02-19

Session work:
- Recovered uncommitted doc changes from previous session (doc reorg, CONTRIBUTING.md, SECURITY.md, godoc comments) — committed as `c67f7c9` and `571203f`, merged to develop (`9a6e13c`)
- Restored `.plans/` directory to repo root from `internal_use_docs/dot_plans/` (`c9f2d29`)
- Deleted 33 stale branches (all feature/*, backup-*, codex/*, docs/*, planning/*) — only `develop` and `main` remain
- Removed git worktree at `.worktrees/pattern-components-6-7`
- Created git-mapped roadmap (`.plans/active/AgentAuth-Project-Roadmap-GitMapped.md`) tracking commits from both agentAuth and agentauth-app repos (`92f0c53`)
- Moved completed P0 plans to `.plans/completed/` (design + implementation)
- Updated BACKLOG.md — marked #0 (audit persistence), #1 (sidecar ID), #3 (operator docs) as DONE; #2 (CLI auto-discover) needs verification in agentauth-app

Key findings:
- Previous session left significant uncommitted work in the working tree
- Python showcase (Phase 2) code originated in this repo (M11-M14 milestones) but was extracted to `agentauth-app` — roadmap now tracks both repos
- agentauth-app has `upstream` remote pointing to this repo

Branch state: `develop` only (all feature branches deleted, code already on develop)

## 2026-02-18

Built P0 audit persistence — SQLite-backed so audit events survive broker restarts. Merged to `develop` (`9290e9d`). Branch `docs/coWork-EnhanceDocs` is active for doc improvements.

User feedback this session:
- "you are just doing a terrible job when it comes to testing and docs for new features" — led to adding 9 missing tests and 9 CHANGELOG entries
- "not just unit tests we need real user tests like someone using it" — must do Docker E2E, not just mocks
- "always show evidence when you run" — terminal output required
- "dont merge i am going to have another team test" — separate review team validates before merge
- Docker stack is currently running on ports 8080/8081 (admin secret: `change-me-in-production`)

Added PostToolUse hook (`.claude/hooks/go-quality-check.sh`) — runs gofmt, go vet, golangci-lint, and godoc checks after every Edit/Write on `.go` files.

## Notes

- CLAUDE.md is checked into the repo while it's private. Remove it before going public.
