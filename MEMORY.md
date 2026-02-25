# MEMORY.md

## Standing Rules

**Live tests require Docker — the app must be running in containers.** (established 2026-02-24)
- Self-hosted binary tests are NOT live tests — they are quick local checks only
- Real live tests run against the Docker stack (`./scripts/stack_up.sh` first)
- Every fix/feature must have a Docker live test before merge
- User stories go in `tests/<name>-user-stories.md` before writing any test code
- `docker-compose.yml` must be updated when a fix adds new env vars

**MEMORY.md records git operations and user reasoning.** (established 2026-02-25)
- Log every branch create/delete, merge, push
- Capture user feedback, rants, and reasoning — paraphrased or quoted — so future sessions know *what the user was thinking* and *why* decisions were made, not just what changed
- Keep it short: links to commits/files instead of full descriptions where possible

**Before any branch delete, merge, or push — verify the feature's Docker live test passed.** (established 2026-02-25)
- Check standing rules first, act second
- Don't delete branches or push until testing is confirmed

**Docker live test process — every fix/feature.** (established 2026-02-25, Session 10)
1. `./scripts/stack_up.sh` — bring up the stack
2. `curl http://127.0.0.1:8080/v1/health` — verify broker is healthy
3. Run user story commands against the running stack (admin auth, the fix-specific operations, restarts, SQLite checks, etc.)
4. Verify each story passes on the running stack
5. `docker compose down -v` — tear down
- Do NOT use `live_test_docker.sh` for manual testing — it creates its own stack and conflicts
- Design the test BEFORE implementation: read user stories, understand constraints, then code
- The test is part of the fix, not a separate task to defer

## 2026-02-25 (Session 8)

### Git operations
- Deleted merged `fix/broker-tls` branch (was `ea1c936`)
- Pushed `develop` to origin (`dcff7ec..829172b`)
- Created `fix/broker-tls-docker-test` branch off develop
- Commits on that branch: `056e164` (design doc), `9055430` (impl plan), `3c9b9d0` (TLS Docker test infrastructure), `cd12501` (WIP sidecar TLS client)
- Branch NOT merged — pending redesign decision

### What happened
Fix 1 (broker TLS) was merged to develop in Session 7 without a Docker live test. Session 8 discovered this, created `fix/broker-tls-docker-test` to build Docker TLS test infrastructure.

Docker test results:
- **HTTP mode: 9/9 PASSED** — baseline, no TLS
- **TLS mode: 10/10 PASSED** — one-way TLS with self-signed certs
- **mTLS mode: NOT RUNNABLE** — sidecar has no TLS client support (see critical finding below)

### Critical finding: Fix 1 design was incomplete
Fix 1 only implemented the broker's TLS server side. For mTLS to work end-to-end, the sidecar must also be an mTLS client — it needs to present a client cert and verify the broker's cert. The sidecar's `brokerClient` uses a plain `http.Client` with zero TLS config. This was missed in the original design (`plans/design-solution.md` line 90: "Files: `internal/cfg/cfg.go`, `cmd/broker/main.go`" — sidecar not mentioned).

Additionally, when TLS is enabled on the broker:
- Sidecar's `AA_BROKER_URL` must change from `http://` to `https://`
- Sidecar needs CA cert access to verify broker (via `SSL_CERT_FILE` env var for system trust, or custom config)
- For mTLS, sidecar needs `AA_SIDECAR_TLS_CERT`, `AA_SIDECAR_TLS_KEY`, `AA_SIDECAR_TLS_CA` env vars

The operator docs already noted this at `getting-started-operator.md:204` but the implementation plan didn't account for it.

### Decision: go back to design
All 6 fixes need re-evaluation. The original plan said they were "all independently implementable" but Fix 1 (TLS) actually depends on sidecar client TLS support, and Fix 5 (UDS) also touches the sidecar. Dependencies between fixes need to be mapped properly before implementation continues.

Branch `fix/broker-tls-docker-test` has working Docker TLS test infrastructure that can be reused. The compose overlay pattern (docker-compose.tls.yml, docker-compose.mtls.yml) and test script changes are solid.

### User feedback (Session 8)
- Frustrated that Fix 1 was merged without Docker test, then the branch was deleted and pushed — making it worse
- "THIS IS BULLSHIT YOU ARE REALLY OVER ENGINEERING SHIT" — too much ceremony (brainstorming skill → design doc → impl plan → subagent-driven-development) for what should have been straightforward
- "no randomize port that is not good" — test should use fixed ports like production
- "isnt that what the pattern says we need to have mtls" — mTLS is the recommended production mode per the security pattern, so Fix 1 not supporting it is a real gap
- "so maybe you design the whole thing wrong with the initial fix" — Fix 1 needs redesign to include sidecar client side
- "lets commit what we have and go back to design to ensure we design and review everything over first"

### What's next: REDESIGN ALL 6 FIXES

The next session must redesign before writing any code. Here is everything needed.

**The 6 fixes** (from `plans/design-solution.md` and `plans/implementation-plan.md`):

| # | Fix | What it does | Current state |
|---|-----|-------------|---------------|
| 1 | Native TLS/mTLS | Encrypt broker ↔ sidecar traffic, require client certs | Broker server side done on develop (`ea1c936`). Sidecar client side NOT done. mTLS is broken. |
| 2 | Revocation persistence | Persist revocations to SQLite so they survive restart | Not started. Design in `plans/design-solution.md`. User stories in `tests/fix2-*`. |
| 3 | Audience validation | Set and check `aud` field on all JWTs | Not started. Design in `plans/design-solution.md`. User stories in `tests/fix3-*`. |
| 4 | Token release | `POST /v1/token/release` for task completion signal | Not started. Design in `plans/design-solution.md`. User stories in `tests/fix4-*`. |
| 5 | Sidecar UDS | Unix domain socket listen mode to eliminate port sprawl | Not started. Design in `plans/design-solution.md`. User stories in `tests/fix5-*`. |
| 6 | Structured audit | Typed fields instead of free-form Detail string | Not started. Design in `plans/design-solution.md`. User stories in `tests/fix6-*`. |

**Known dependency issues with the old plan:**
1. Fix 1 and Fix 5 both modify sidecar transport — Fix 1 changes how sidecar talks TO the broker (outbound HTTP client → HTTPS/mTLS client), Fix 5 changes how apps talk TO the sidecar (inbound TCP listener → UDS listener). They touch different sides but both modify `cmd/sidecar/`.
2. Fix 1 is incomplete — the broker server TLS is on develop but the sidecar client TLS is missing. Need to decide: complete Fix 1 (add sidecar TLS client) or redesign Fix 1 scope.
3. The old plan claimed all fixes were independent. That's wrong. Need to map which fixes touch which files and identify real conflicts.

**Files each fix touches** (from design + what we learned):
- Fix 1: `internal/cfg/cfg.go`, `cmd/broker/main.go` (done), `cmd/sidecar/broker_client.go`, `cmd/sidecar/config.go`, `cmd/sidecar/main.go` (not done), `docker-compose.yml`, `docker-compose.tls.yml`, `docker-compose.mtls.yml`
- Fix 2: `internal/revoke/rev_svc.go`, `internal/store/sql_store.go`, `cmd/broker/main.go`
- Fix 3: `internal/cfg/cfg.go`, `internal/token/tkn_claims.go`, `internal/token/tkn_svc.go`, `internal/authz/val_mw.go`, `internal/identity/id_svc.go`, `internal/deleg/deleg_svc.go`
- Fix 4: new `internal/handler/release_hdl.go`, `internal/audit/audit_log.go`, `cmd/broker/main.go`
- Fix 5: `cmd/sidecar/config.go`, `cmd/sidecar/main.go`
- Fix 6: `internal/audit/audit_log.go`, `internal/store/sql_store.go`, ~6 callers

**What the redesign must produce:**
1. Correct dependency graph showing which fixes must come before which
2. Identify fixes that truly conflict vs. ones that can be parallel
3. New phase ordering (the old Phase 1/2/3 was wrong)
4. Updated scope for Fix 1 — must include sidecar TLS client
5. Each fix must have a Docker live test defined as part of its spec
6. New implementation plan replacing `plans/implementation-plan.md`

**Docker test infrastructure to rebuild** (was on deleted branch):
- Compose overlay pattern: `docker-compose.tls.yml` (one-way TLS), `docker-compose.mtls.yml` (mutual TLS)
- `live_test_docker.sh` needs `--tls` and `--mtls` flags
- Runtime cert generation via openssl (no certs checked into repo)
- Key configs: sidecar needs `AA_BROKER_URL=https://...`, `SSL_CERT_FILE` for CA trust, cert volume mounts
- TLS-specific assertions: plain HTTP returns 400 (not connection refused) from Go's TLS server

### Uncommitted on develop
- `agentauth.db` (runtime artifact)
- 5 user story files (`tests/fix2-*` through `tests/fix6-*`)

### Local branches
- `develop` (current)
- `main`
- `develop-harness-backup` (dead/reference only)

## 2026-02-25 (Session 10)

### Git operations
- Created `fix/revocation-persistence` off `develop`
- Commits: `e457274`, `9d4dc3d`, `37b6d1e`, `eadd8b1`, `ac14850`
- Branch NOT yet merged — ready for merge

### What happened
Implemented Fix 2 (revocation persistence) from `docs/plans/2026-02-25-fix2-revocation-persistence.md`. TDD throughout.

- `internal/store/sql_store.go`: `revocations` table, `SaveRevocation()`, `LoadAllRevocations()`
- `internal/revoke/rev_svc.go`: `RevocationStore` interface, write-through in `Revoke()`, `LoadFromEntries()`
- `cmd/broker/main.go`: loads revocations on startup, passes `sqlStore` to `NewRevSvc()`
- `Dockerfile`: added `sqlite` to broker image for DB inspection
- `scripts/live_test_docker.sh`: extended with Fix 2 persistence tests
- Updated 4 test files for new `NewRevSvc(nil)` signature

Gates: 3 PASS, 0 FAIL, 1 WARN (gosec, non-blocking).

### Docker live test — PASSED

**Steps to run live test manually:**
1. `./scripts/stack_up.sh` — bring up Docker stack, wait for healthy
2. `curl http://127.0.0.1:8080/v1/health` — verify broker is up
3. Admin auth: `POST /v1/admin/auth` with `change-me-in-production`
4. Revoke: `POST /v1/revoke` with `{"level":"token","target":"..."}` and `{"level":"agent","target":"..."}`
5. Check SQLite: `docker compose exec broker sqlite3 /data/agentauth.db "SELECT * FROM revocations"`
6. Restart broker: `docker compose restart broker`
7. Wait for healthy, check logs: `docker compose logs broker --tail=20 | grep revocat`
8. Verify SQLite still has entries after restart
9. Admin auth again (new keys after restart), validate fresh token — should be `valid:true` (no false positive)
10. `docker compose down -v` — tear down

**Results:**
- Story 1: 2 revocations persisted to SQLite, broker logged `revocations loaded count=2` after restart
- Story 2: Fresh post-restart token validated `valid:true` — no false positives
- Story 3: SQLite entries visible before and after restart

### Testing constraint: ephemeral signing keys
Signing keys are regenerated on every startup. After restart, ALL pre-restart tokens fail signature verification before the revocation check runs. You cannot distinguish "revoked" from "bad signature" on a pre-restart token via the validate endpoint. The test works around this by checking SQLite directly + broker logs for persistence proof, and using fresh tokens for false-positive testing.

### Process lessons
1. **Understand the test before you code.** Should have read user stories, test infrastructure, and figured out the signing key constraint before writing implementation. Instead discovered it at test time.
2. **Don't punt Docker tests.** Tried to defer live test to "next session" — that's wrong. The live test is part of the fix.
3. **`live_test_docker.sh` creates its own stack.** It spins up an isolated project with random ports, which conflicts with a stack from `stack_up.sh`. The manual test steps above are how to properly test against a running stack.
4. **`stack_up.sh` first, then test.** The correct process: bring up stack, verify healthy, run commands against it. Not a single script that does everything.

### What's next: IMPLEMENT FIX 3
- Branch: `fix/audience-validation` off `develop`
- Plan: `docs/plans/2026-02-25-fix3-audience-validation.md`
- User stories: `tests/fix3-audience-validation-user-stories.md`
- **Read user stories and test infrastructure FIRST, design Docker test, then implement**
- After Fix 3 → Fix 4 → Fix 1 → Fix 5 → Fix 6

### Local branches
- `develop` (current)
- `main`
- `develop-harness-backup` (dead/reference only)

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
