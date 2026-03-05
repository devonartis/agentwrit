# MEMORY.md

## Source Pattern

**[Ephemeral Agent Credentialing v1.2](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.2.md)** — the security pattern AgentAuth implements. Every feature, fix, and design decision traces to this document. Read it before making architectural choices.

## Tech Debt

Active tech debt. Append here when new debt is taken. Full details in `.plans/PRD.md` Tech Debt table.

| ID | What | Severity | When to fix |
|----|------|----------|-------------|
| TD-001 | `app_rate_limited` audit event not emitted (rate limiter fires before handler) | Low | Before Phase 1C |
| TD-002 | No operator onboarding (`aactl init`, admin secret generation) | Low | Future |
| TD-003 | Sidecar has no defined use case — removed from infra, code still exists | Medium | When PRD defines a use case |
| ~~TD-004~~ | ~~Admin auth uses legacy client_id/client_secret shape~~ | ~~High~~ | ~~RESOLVED Session 26~~ |
| ~~TD-005~~ | ~~6 sidecar routes still wired in broker~~ | ~~High~~ | ~~RESOLVED Session 26~~ |
| TD-006 | App JWT TTL hardcoded to 5 min — should be 30 min default, per-app configurable by operator | Medium | Before Phase 1C |

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

**Every endpoint must have aactl operator tooling — no raw curl in tests.** (established 2026-02-25, Session 12; updated Session 23)
- If an endpoint is operator-facing, add an `aactl` command for it as part of the fix
- Docker live tests should use `aactl` commands, not hand-crafted curl chains
- Raw curl is acceptable for developer-facing endpoints (`/v1/app/auth`) — the developer has no CLI
- An endpoint without tooling is not shippable (same lesson as Session 3 with list-sidecars)

**No cutting corners in live tests.** (established 2026-03-03, Session 23)
- Build `aactl` to `./bin/aactl` — not `/tmp/`, not `go run`
- Source `tests/<phase>/env.sh` once — don't inline env vars on every command
- Operator stories use `aactl`. Developer stories use `curl`/REST API. Don't mix personas.
- If you wouldn't do it on a VPS, don't do it in the test.
- Full lessons: `tests/phase-1a/lessons-learned.md`

**Test artifacts organized per-phase.** (established 2026-03-03, Session 23)
- Each phase gets `tests/phase-Xn/` with: `user-stories.md`, `env.sh`, `evidence/`, `lessons-learned.md`
- Phase N+1 carries forward regression stories from Phase N
- Evidence folder has one file per story plus README with verdict table

**Docker live test process — every fix/feature.** (established 2026-02-25, Session 10)
1. `./scripts/stack_up.sh` — bring up the stack
2. `curl http://127.0.0.1:8080/v1/health` — verify broker is healthy
3. Run user story commands against the running stack (admin auth, the fix-specific operations, restarts, SQLite checks, etc.)
4. Verify each story passes on the running stack
5. `docker compose down -v` — tear down
- Do NOT use `live_test_docker.sh` for manual testing — it creates its own stack and conflicts
- Design the test BEFORE implementation: read user stories, understand constraints, then code
- The test is part of the fix, not a separate task to defer

**When `/obsidian:daily` runs, analyze — don't just execute steps.** (established 2026-02-27)
- Read what Divine wrote in the note — morning entries, rants, reflections, carried-over todos
- Respond to what's there as a person: what's stuck, what's moving, what he's feeling, what's being avoided
- If the same todos have been carried over for a week, say something — push, don't just copy
- If medications/supplements are already logged, don't re-ask
- The skill steps are mechanics. The analysis is the point. No checklist running.

**Docker live test evidence — save to `tests/<fix-name>-evidence/`.** (established 2026-02-26, Session 16)
- Every Docker live test must produce a `tests/<fix-name>-evidence/` folder
- Folder contains: `README.md` (overview + story table), `story-N-<name>.md` per story (plain English, reproduction steps, raw output, what to look for, verdict), `smoketest-output.txt`
- Anyone should be able to open the evidence folder and understand what was tested and whether it passed without running anything
- This is not optional — a live test without saved evidence is incomplete


**"Why does this exist?" questions must be answered from the pattern and user needs, NOT from the code.** (established 2026-02-28, Session 18)
- When anyone asks "should X be required?" or "why does X exist?" — that's an Architecture Challenge
- The answer source is: (1) the user's actual question, (2) the pattern (v1.2), (3) production deployment needs
- Code tells you what IS. The user is asking what SHOULD BE. Different questions, different sources.
- ADR-002 (Session 15) got this wrong — defended sidecars by reading how they work instead of whether they're required
- Full history: `PROCESS-LESSON.md`

**Development process has three named steps — run them in order.** (established 2026-02-28, Session 18)
1. **Architecture Challenge** — question fundamentals against user needs and the pattern. "Should this exist?"
2. **Gap Analysis** — inventory what's broken against the agreed architecture. "What's missing?"
3. **Process Maps** — document flows for every persona. "How does each user experience this?"
- Step 2 without Step 1 = analyzing the wrong architecture
- Full rationale: `PROCESS-LESSON.md`

**Context-First Development (CFD) — the full pre-coding process for working with AI coding agents.** (established 2026-03-02, Session 20)
- Before coding, the human builds context; the agent builds from that context. The agent is the builder, not the architect.
- Uses [Superpowers skills](https://github.com/obra/superpowers/tree/main/skills) as the execution framework (brainstorming, writing-plans, executing-plans, TDD, verification-before-completion, etc.)
- 5 new skills fill the gaps Superpowers doesn't cover: **codebase-archaeology**, **impact-analysis**, **design-review**, **task-briefing**, **regression-verification**
- Every feature gets a `TASK-BRIEF.md` — the one doc the agent reads before writing code. Contains: what exists, what to build, what NOT to touch, how to verify.
- Impact analysis is mandatory before task breakdown: for every existing capability, state PRESERVED / ENHANCED / NEW / REMOVED. If you can't fill the table, you don't understand the change well enough to build it.
- Full process: `.plans/CoWork-Context-First-Development.md`
- Gap analysis against Superpowers: `.plans/CoWork-Superpowers-Gap-Analysis.html`

**Architecture plan for apps as first-class entities is the agreed design.** (established 2026-03-02, Session 20)
- Sidecar is optional, NOT mandatory. Three paths to broker: SDK, optional proxy, raw HTTP.
- Apps register with `client_id` + `client_secret` (scoped, per-app, revocable). Master key stays with operator only.
- Ed25519 challenge-response identity (NOT SPIRE — opted out due to heavy infrastructure). Uses SPIFFE ID format but not SPIRE infrastructure.
- Full Pattern Flow proves nothing breaks: 8 capabilities preserved, 5 enhanced, 4 new, zero removed. All 10 security invariants maintained.
- Revised phases: 1a (app model + auth + scopes + rate limiting) → 1b (app-scoped launch tokens) → 1c (app revocation + audit + secret rotation) → 2 (activation token bootstrap) → 3 (Python SDK) → 4 (JWKS) → 5 (key persistence)
- Peer review incorporated: 7 implementation gaps identified by 3rd-party developer, all given decisions.
- Full architecture doc: `.plans/CoWork-Architecture-Direct-Broker.md` (also `.html` and `.pdf` versions)
- Lifecycle diagram: `.plans/CoWork-Diagram-FullLifecycle.svg`

## 2026-03-04 (Session 26 — Phase 0 Legacy Cleanup Implementation)

### What happened

Started Phase 0 implementation on branch `fix/phase-0-legacy-cleanup` off `develop`. Completed Tasks 0.1 and 0.2 (code changes). Created live test template and Phase 0 test plan. Docker stack is up, ready to execute tests.

### Branch

- Created `fix/phase-0-legacy-cleanup` off `develop` (stashed Phase 1B WIP)

### Code changes (Tasks 0.1 + 0.2)

**Task 0.1 — Remove sidecar routes from broker:**
- Removed 5 sidecar route registrations from `AdminHdl.RegisterRoutes` in `internal/admin/admin_hdl.go`
- Removed `/v1/token/exchange` route + `tokenExchangeHdl` init from `cmd/broker/main.go`
- Removed sidecar ceiling loading from broker startup in `cmd/broker/main.go`
- Updated doc comment route table in `main.go`
- Removed sidecar route tests from `internal/admin/admin_hdl_test.go` and `internal/handler/handler_test.go`
- Skipped 3 sidecar integration tests in `cmd/sidecar/integration_test.go` (Phase 2)
- Handler methods kept in source for Phase 2

**Task 0.2 — Fix admin auth endpoint:**
- Changed `authReq` struct from `{client_id, client_secret}` to `{secret}` in `internal/admin/admin_hdl.go`
- Added `legacyAuthReq` detection — old format returns 400 with migration message
- Changed `AdminSvc.Authenticate()` from `(clientID, secret)` to `(secret)` in `internal/admin/admin_svc.go`
- Updated `aactl` client (`cmd/aactl/client.go`) to send `{"secret": "..."}`
- Updated sidecar broker client (`cmd/sidecar/broker_client.go`) to send new format
- Updated tests: `admin_hdl_test.go`, `admin_svc_test.go`, `handler_test.go`, `app_hdl_test.go`, `broker_client_test.go`, `integration_test.go`
- All 15 packages pass (`go test ./...`)

### Live test process formalized

Divine's feedback: "we need to come up with a documented process on how to test... it should be a clear story where it says so the QA team can read what was the test and what is expected and the output evidence." Key points:
- Each evidence file = ONE test, not multiple checks bundled together
- Banner must be plain language — "The operator tries to log in with..." not "the new auth shape"
- Who is doing the work, why, where, and what they are doing
- An executive should be able to read it and understand
- Operator stories use `aactl`, developer stories use `curl`

Created:
- `tests/LIVE-TEST-TEMPLATE.md` — reusable template for all future phases
- `tests/phase-0/user-stories.md` — 12 stories (6 sidecar removal, 2 admin auth, 4 regression)
- `tests/phase-0/env.sh` — test environment
- `tests/phase-0/evidence/` — 12 evidence plan files (Pass 1 complete, actuals pending)

### Tech debt updates

- TD-004 (admin auth legacy shape) — RESOLVED by Task 0.2
- TD-005 (sidecar routes on broker) — RESOLVED by Task 0.1

### Phase 0 live tests executed — 12/12 PASS

All 12 stories run against Docker stack, evidence piped directly into files:
- S1-S6: All six removed sidecar routes return 404 — confirmed gone
- S7: Operator login with new admin format works (aactl app list returned empty list, no errors)
- S8: Old admin login format returns 400 with clear migration message
- R1: App registration works (cleanup-test app created, credentials returned)
- R2: Developer app login works (JWT returned with app-level scopes)
- R3: App JWT correctly rejected at admin endpoint (403 Forbidden)
- R4: Audit trail complete — app_registered, app_authenticated, scope_violation all recorded, no secrets leaked

Evidence files: `tests/phase-0/evidence/` — README verdict table updated, all 12 stories PASS.

### Live test template rewritten as full guide

Divine's feedback: the template was a skeleton — nobody could follow it. Rewrote `tests/LIVE-TEST-TEMPLATE.md` as a complete step-by-step guide with:
- Real bash examples showing how the coding agent must make calls (banner + output piped into evidence file in one shot)
- Real completed evidence file from Phase 0 (R4 audit trail) as reference
- Banner format broken down (who/what/why/how/expected) with good vs bad language examples
- 11 rules including "one story at a time," "output goes in the file," "verdict is earned"

### Phase 0 merged to develop, develop merged into Phase 1B

- `a83466d` — committed Phase 0 on `fix/phase-0-legacy-cleanup`
- `882b39c` — merged `fix/phase-0-legacy-cleanup` → `develop` (no conflicts)
- `52c6b7d` — merged `develop` → `feature/phase-1b-launch-tokens` (two conflicts resolved)

**Conflict resolution on merge into 1B:**
- `FLOW.md` — kept both session entries (1B's Session 24 + Phase 0's Sessions 26-27), ordered chronologically
- `internal/admin/admin_hdl_test.go` — two conflicts:
  1. Imports: kept `path/filepath` and `strings` (needed by 1B's app ceiling tests), added `audit` import (needed by `newAppTestMux`), dropped `revoke` (only used by deleted sidecar tests)
  2. Test body: removed old sidecar route tests (lines 278-733, deleted by Phase 0), kept Phase 1B's app ceiling enforcement tests (`TestCreateLaunchToken_*`), kept Phase 0's removal comment
- All 15 packages pass after resolution

**Current state:** on `feature/phase-1b-launch-tokens`, up to date with develop. Ready for Phase 1B Docker live tests.

### Concept paper alignment checklist added

Created `.plans/CONCEPT-PAPER-ALIGNMENT.md` — maps NCCoE concept paper recommendations to AgentAuth features. Categorized as DONE, QUICK-ADD (before release), and FUTURE. Key QUICK-ADDs: full SPIFFE ID in audit (L-2), task context in audit (L-3), resource accessed in all events (L-5), single-agent workflow reconstruction test (V-1). Biggest gap: delegation chain support (D-1 through D-8) — everything we recommend but don't have yet.

### What's next

1. **Run Phase 1B Docker live tests** — 11 stories from `tests/phase-1b/user-stories.md`
2. **Read `tests/LIVE-TEST-TEMPLATE.md` first** — the complete guide with real examples. Banner in the call, one story at a time, output piped to evidence file, verdict after seeing result.
3. Save evidence to `tests/phase-1b/evidence/`
4. Merge Phase 1B → develop
5. Then: QUICK-ADD audit enhancements from concept paper checklist before release

---

## 2026-03-03 (Session 23 — Phase 1A Live Test & Lessons Learned)

### What happened

Ran Phase 1A Docker live test (Task 6). Found infrastructure and process problems. Fixed them. Ran all 12 stories (0-11). 10 PASS, 2 PARTIAL.

### Key decisions

- **Sidecar removed from docker-compose.yml** — no defined use case. PRD says "optional" but never says for what. Removed until a real reason exists.
- **Two personas in acceptance testing** — operator uses `aactl`, developer uses `curl`/REST API. Testing with the wrong tool is cutting corners.
- **Acceptance tests = operator experience** — not scripts. Run the same commands an operator would on a VPS.
- **`sk_live_` prefix removed from criteria** — plain 64-char hex for now. Revisit in Phase 3 (SDK).
- **Test artifacts organized per-phase** — `tests/phase-1a/` contains user-stories.md, env.sh, lessons-learned.md, evidence/

### Open issues

- **`app_rate_limited` audit event missing** — P0. Rate limiter middleware fires before handler audit call. Needs fix branch.
- **No operator onboarding** — no `aactl init`, admin secret origin undocumented. P2 for later.

### Artifacts

- `tests/phase-1a/evidence/` — per-story evidence files with README
- `tests/phase-1a/lessons-learned.md` — corner-cutting incident, infrastructure gaps
- `.plans/phase-1a/ADR-Phase-1a-Tech-Debt.md` — all tech debt documented

### What's next

1. Docker live test for Phase 1B — `./scripts/stack_up.sh` + run 11 stories from `tests/phase-1b/user-stories.md`
2. Save evidence to `tests/phase-1b/evidence/` with per-story files + README verdict table
3. Merge `feature/phase-1b-launch-tokens` → `develop`
4. Begin Phase 1C (app revocation + audit + secret rotation)
5. TD-001 (`app_rate_limited` audit event) — fix before Phase 1C

---

## 2026-03-04 (Session 24 — Phase 1B Implementation)

### What happened

Phase 1B (app-scoped launch tokens) implemented via subagent-driven development. Apps can now create launch tokens within their scope ceiling. The traceability chain App → Launch Token → Agent is established.

### Git operations

- Created branch: `feature/phase-1b-launch-tokens` (from `develop`)
- 7 commits on branch, all unit tests green (15 packages)

### Work completed

**Task 1 — RequireAnyScope middleware** (`internal/authz/val_mw.go`)
- New `RequireAnyScope(scopes []string, next http.Handler)` — accepts if token carries ANY of the listed scopes
- 3 tests: app passes, admin passes, neither rejected

**Tasks 2-3 — AppID fields** (`internal/store/sql_store.go`)
- `LaunchTokenRecord.AppID` — empty for admin-created tokens
- `AgentRecord.AppID` — inherited from launch token at registration

**Task 4 — Core ceiling enforcement** (`internal/admin/admin_hdl.go`, `admin_svc.go`, `cmd/broker/main.go`)
- Route changed to `RequireAnyScope(["admin:launch-tokens:*", "app:launch-tokens:*"])`
- Handler detects app caller via `strings.HasPrefix(claims.Sub, "app:")`, looks up AppRecord, enforces `ScopeIsSubset(requested, ceiling)`
- `CreateLaunchToken` signature: added `appID string` param
- `AdminHdl` receives `*store.SqlStore` for app lookups
- 6 new tests: within ceiling, exceeds ceiling, carries AppID, admin no ceiling, admin regression, audit on ceiling exceeded
- All 8 call sites updated across codebase

**Task 5 — AppID flows to agent** (`internal/identity/id_svc.go`)
- `SaveAgent` now sets `AppID: ltRec.AppID`
- 2 new tests: inherits from app token, empty from admin token

**Task 6 — Audit attribution** (`internal/admin/admin_svc.go`, `internal/identity/id_svc.go`)
- `launch_token_issued` detail includes `app_id=` when app-created
- `agent_registered` and `token_issued` detail includes `app_id=` when app-traced
- 4 new tests

**Task 7 — User stories** (`tests/phase-1b/`)
- 8 stories (developer, operator, security) + 3 Phase 1A regression stories
- `tests/phase-1b/env.sh` created

**Task 8 — Go doc comments** (5 files)
- 22 professional doc comments added across Phase 1a and 1b types
- All exported types, methods, and struct fields documented

### Commits

```
f8c74cb docs: add professional Go doc comments to Phase 1a and 1b types
fb78aa6 test: add Phase 1b user stories and test env
6bb603d feat(audit): include app_id in launch token and agent registration events
cb0057f feat(identity): agent inherits AppID from launch token on registration
aa3a1cd feat(admin): apps can create launch tokens within scope ceiling
f37404d feat(store): add AppID field to LaunchTokenRecord
33f4461 feat(authz): add RequireAnyScope middleware for multi-caller endpoints
```

### Process lesson

User stories should be Task 1, not Task 8. CLAUDE.md standing rule: "Write user stories FIRST." We wrote code first this session. Fixed mid-session. Future sessions: always extract user stories before touching code, regardless of plan ordering.

---

## 2026-03-03 (Session 23 — Phase 1A Live Test & Merge)

### What happened

Picked up Phase 1a where Session 22 left off. Tasks 4 and 5 done. `go test ./...` clean across all 15 packages.

### Work completed

**Pre-task fix: AppHdl wasn't wired into main.go**
- Added `app` import, `appSvc`/`appHdl` initialization, `appHdl.RegisterRoutes(mux)` to `cmd/broker/main.go`
- Fixed missing `Scopes []string` field in `appAuthResp` (spec required it, it was absent)

**Task 4 — Per-client_id rate limiting**
- `WrapWithKeyExtractor` added to `internal/authz/rate_mw.go`
- `clientIDFromBody` helper in `app_hdl.go` — reads body, resets with `io.NopCloser(bytes.NewReader(data))`
- Rate: 10 req/min, burst 3 (changed from IP-based 5/s, burst 10)
- 4 new tests: key used, keys independent, IP fallback, body readable after extract

**Task 5 — aactl app commands**
- `cmd/aactl/apps.go` — `register`, `list`, `get`, `update`, `remove`
- `doDelete` added to `cmd/aactl/client.go`
- `register` warns "Save the client_secret — it cannot be retrieved again"

### Next steps (gate before merge)
- **Task 6**: Docker live test — `./scripts/stack_up.sh` + validate all 11 user stories from `tests/phase-1a-user-stories.md`
- After Task 6 passes: merge `feature/phase-1a-app-registration` → `develop`, then begin Phase 1b

---

## 2026-03-03 (Session 22 — Claude Code)

### What happened

Phase 1a implementation started. Branch created, user stories written, Tasks 1–3 complete via TDD.

### Git operations
- Created branch: `feature/phase-1a-app-registration` (from `develop`)
- No commits yet — all work is uncommitted on the branch

### Work completed

**Gate satisfied first:**
- Created `tests/phase-1a-user-stories.md` — 11 stories (operator, developer, security reviewer, regression)

**Task 1 — AppRecord store (RED → GREEN)**
- `internal/store/sql_store_app_test.go` — 12 tests
- `internal/store/sql_store.go` — `ErrAppNotFound`, `AppRecord`, `createAppsTable` DDL, 6 CRUD methods, `InitDB()` hook

**Task 2 — AppSvc service (RED → GREEN)**
- `internal/app/app_svc_test.go` — 15 tests
- `internal/app/app_svc.go` — new package; bcrypt cost 12, name regex validation, scope validation, audit events
- `internal/audit/audit_log.go` — 6 app event constants
- Added `golang.org/x/crypto/bcrypt`

**Task 3 — AppHdl handler (RED → GREEN)**
- `internal/app/app_hdl_test.go` — 17 tests
- `internal/app/app_hdl.go` — 6 endpoints, RFC 7807 errors, `client_secret_hash` never returned, rate limiter on POST /v1/app/auth

All 14 packages green, zero regressions after each task.

### Important user correction
Unit tests ≠ acceptance tests. Acceptance tests = user stories in `tests/phase-1a-user-stories.md`, verified against Docker stack. Unit tests prove code logic. Both required, different purposes, different timing.

### Next steps
- ~~Task 4: Per-client-id rate limiting (`internal/authz/rate_mw.go`)~~ ✓ Done
- ~~Task 5: aactl app commands (`cmd/aactl/apps.go`)~~ ✓ Done
- ~~Task 6: Wire AppHdl into `cmd/broker/main.go`~~ ✓ Done
- Task 6: Docker live test against all 11 user stories

---

## 2026-03-02 (Session 20 — Claude CoWork, continued)

### What happened
Continued architecture design work from Session 19. Three major deliverables this session:

1. **Full Pattern Flow section** added to the architecture doc — proves the entire Ephemeral Agent Credentialing Pattern works end-to-end with the new design, not just the sidecar fix
2. **Context-First Development (CFD)** — a named development process for working with AI coding agents, born from the gaps we hit during this session
3. **Superpowers gap analysis** — mapped all 14 Superpowers skills against the CFD process, identified 5 missing skills and 5 enhancements

### Why the Full Pattern Flow exists

Divine's feedback mid-session: "you also missed the full flow of everything — we should be showing how things presently work when it comes to agents and all of what the app can do to ensure it is done properly with your new design." And: "not just sidecar because the true reason for the app is the pattern being able to work." And: "we cant fix one thing and break other things."

The architecture doc originally only answered "do we need the sidecar?" (answer: no, make it optional). But it didn't prove the rest of the system still works. The Full Pattern Flow walks through all 9 stages of the agent lifecycle (app registration → app auth → launch token → Ed25519 challenge-response → token issuance → scope enforcement → delegation → revocation → audit) and for EACH stage shows: what happens today, what changes in the new design, and what the code path is.

Result: 8 capabilities preserved with zero changes, 5 enhanced backward-compatibly, 4 new, zero removed. All 10 security invariants maintained. This is the proof that the new design doesn't break anything.

→ Artifact: `.plans/CoWork-Architecture-Direct-Broker.md` (Section: "Full Pattern Flow: How Every Capability Works End-to-End")
→ Artifact: `.plans/CoWork-Diagram-FullLifecycle.svg` (8-stage lifecycle diagram)

### Why Context-First Development exists

Every correction during Sessions 18-20 traces to the same root cause: the AI agent didn't have enough context BEFORE it started working. Specific examples:

- "We are not using SPIRE" → agent didn't know about opt-out decisions (would be caught by **codebase-archaeology** skill, Step 1.1)
- "You missed the full flow" → agent didn't inventory all capabilities first (would be caught by **codebase-archaeology** skill, Step 1.1)
- "We can't fix one thing and break other things" → no impact analysis proving preserved capabilities (would be caught by **impact-analysis** skill, Step 2.2)
- 3rd-party developer found 7 gaps after design was "done" → no design review gate (would be caught by **design-review** skill, Step 2.4)

Divine asked: "how do we include this level of process before developing in the future?" This led to naming the process and mapping it against the Superpowers skill framework we use.

The process is 4 phases, 13 steps:
- **DISCOVER** (3 steps): current state inventory, brainstorm/pattern input, constraint & compliance check
- **DESIGN** (4 steps): PRD + ADRs, impact analysis, implementation details with DO NOT TOUCH lists, peer review gate
- **BUILD** (4 steps): agent context briefing (TASK-BRIEF.md), task breakdown, user stories + negative cases, implementation
- **VERIFY** (3 steps): verification tasks, regression verification, post-build review

→ Artifact: `.plans/CoWork-Context-First-Development.md` (compact reference — goes in repo)
→ Artifact: `.plans/CoWork-Agent-Development-Lifecycle.html` (full visual guide — goes in docs)

### Why the Superpowers Gap Analysis exists

Divine pointed out we use Superpowers skills (`https://github.com/obra/superpowers/tree/main/skills`). The CFD process needed to be mapped against what Superpowers already provides so we don't duplicate effort. Analysis found:

- **4 steps fully covered** by existing Superpowers skills (brainstorming, writing-plans, executing-plans, verification-before-completion)
- **5 steps partially covered** (brainstorming needs ADRs, writing-plans needs DO NOT TOUCH lists, TDD needs negative cases, etc.)
- **5 steps missing entirely** — these are the new skills to create:
  1. **codebase-archaeology** (map what exists before designing) — Phase 1
  2. **impact-analysis** (prove changes don't break existing capabilities) — Phase 2
  3. **design-review** (architecture review before coding, not just code review after) — Phase 2
  4. **task-briefing** (compile TASK-BRIEF.md from all Phase 1-2 outputs) — Phase 3
  5. **regression-verification** (prove old capabilities still work after changes) — Phase 4

Build order recommendation: codebase-archaeology first (feeds everything else), then impact-analysis (biggest gap), then the rest.

→ Artifact: `.plans/CoWork-Superpowers-Gap-Analysis.html` (full gap analysis with skill chain flow)

### All artifacts from Sessions 19-20 (CoWork)

| File | What | Why |
|------|------|-----|
| `.plans/CoWork-Architecture-Direct-Broker.md` | Architecture design: apps as first-class, sidecar optional | Answers "do we need the sidecar?" + proves full pattern still works |
| `.plans/CoWork-Architecture-Direct-Broker.html` | Visual HTML version with SaaS palette | Engaging format for stakeholder review |
| `.plans/CoWork-Architecture-Direct-Broker.pdf` | 15-page PDF version | Offline/print format |
| `.plans/CoWork-Diagram-DirectBroker.svg` | Side-by-side: today vs proposed architecture | Visual comparison of mandatory sidecar vs 3 paths |
| `.plans/CoWork-Diagram-FullLifecycle.svg` | 8-stage agent lifecycle diagram | Shows NEW vs PRESERVED at each stage |
| `.plans/CoWork-Process-Map.md` | 9 processes evaluated across 4 personas | What works, what's broken, what's missing |
| `.plans/CoWork-Gap-Analysis.md` | 8 failures, 7 enhancements, 12 new features | Comprehensive gap inventory |
| `.plans/CoWork-Flow-Diagrams.html` | All SVG diagrams embedded in one HTML page | Single-page visual reference (SVGs inline, not external) |
| `.plans/CoWork-Context-First-Development.md` | Context-First Development process (compact) | The pre-coding process for AI coding agents |
| `.plans/CoWork-Agent-Development-Lifecycle.html` | CFD visual guide with case study | Full guide showing 4 phases, 13 steps, role assignments |
| `.plans/CoWork-Superpowers-Gap-Analysis.html` | Superpowers vs CFD gap analysis | Maps existing skills, identifies 5 new skills needed |

### User feedback (Session 20)
- "why you only did one what happen to the other ones" — only linked 2 files instead of all deliverables
- "the html diagram is not working" — browser couldn't resolve external SVG `<img>` references; fixed by embedding inline
- "well we are not using SPIRE Agent we opted out because of the heavy infrastructure" — critical correction, updated all docs
- "you also missed the full flow of everything" — architecture doc only covered sidecar, not the complete pattern
- "not just sidecar because the true reason for the app is the pattern being able to work" — the security story IS the pattern, not just one component
- "we cant fix one thing and break other things" — led to the capability preservation matrix
- "how do we include this level of process before developing in the future" — led to naming Context-First Development
- "WE USE THE SUPERPOWERS" — pointed to obra/superpowers as the execution framework; led to gap analysis

### What's next
1. **Build the 5 new Superpowers-compatible skills** (codebase-archaeology, impact-analysis, design-review, task-briefing, regression-verification) — use `writing-skills` skill from Superpowers
2. **Start Phase 1a implementation** — AppRecord data model + `POST /v1/admin/apps` + `POST /v1/app/auth` + app JWT scopes + per-app rate limiting
3. **Write TASK-BRIEF.md for Phase 1a** using the new CFD process — this will be the first real test of the process

## 2026-02-28 (Session 18)

### What happened
Cleanup: moved `.plans/` directory out of the repo to `/Users/divineartis/agentAuth_Backup_docs/dot-plans/`. This was missed during the Session 16 pre-release cleanup — that commit (`5626f13`) only removed `plans/` (no dot), leaving `.plans/` behind. Contents were all stale session artifacts from early sessions (roadmap, backlog, list-sidecars design docs, completed P0 plans, and the old `USER-STORIES-PLAN.md`).

Also confirmed: the demo application user stories live in `agentAuthDemoApps/app_ideas_stories/` (external repo), not in this repo. The `USER-STORIES-PLAN.md` in `.plans/active/` was old material from Session 1, not the current demo work from Session 17.

### Git operations
- No branch changes (cleanup only, `.plans/` was untracked after the restore in Session 1)

### User feedback (Session 18)
- "I specifically said move .plans/" — the Session 16 cleanup missed it, should have been moved with `plans/`
- "Look through memory" — corrected me for grepping git log instead of reading MEMORY.md first. MEMORY.md is the source of truth for session context, not git log.

### BLOCKER: App registration workflow is fundamentally broken — must fix before demo

Sidecar self-provisions with the admin secret. No app identity in the broker. No way to onboard an app without sharing the admin secret and editing docker-compose. This is deeper than KI-001 — apps are invisible to the system. Must be fixed before demo. Docker is infrastructure, not the product — this should run on anyone's virtual server.

Full analysis: `BIG_BAD_GAP.md`

### Created feature catalog for agentauth-app
Cataloged all AgentAuth broker features (endpoints, env vars, aactl commands, audit events, token claims, all 6 fixes) and saved to `/Users/divineartis/proj/agentauth-app/new-features-agentauth.md` at the root. This gives the app repo full knowledge of what the current broker exposes. Should have used `git log --oneline -30 develop` instead of sending an Explore agent — commit messages had everything. Use git history when commits are well-structured.

### Plan: finish agentauth-app demo, then merge develop to main
Working on the original demo app (`agentauth-app`) next. Once the demo app is tested and working against the current broker (all 6 fixes), merge `develop` to `main` in authAgent2. The demo app validation is the gate for the release merge.

### Two demo app directories — context and plan
There are two separate directories for demo/consumer apps:

1. **`/Users/divineartis/proj/agentauth-app`** — the original showcase app (Python, FastAPI, SDK, CLI, dashboard, demo agents). Customer service triage demo. Has SDK in `app/sdk/`, tests, docs. Branch `feature/p0-audit-app-integration` with an unexecuted 8-task impl plan for dual-broker testing. Pinned broker is ~50 commits behind current `develop`. Root is cluttered with `.docx`, `.jsx`, `.png`, `.patch` files and a stale `agentauth-ctl` binary. Only 1 session logged (Feb 26).

2. **`/Users/divineartis/proj/agentAuthDemoApps`** — the three scenario demos from Session 17 (stolen key, rogue delegate, who did it). Design docs only, no code yet.

**Plan:** Work on `agentauth-app` first — pull the current broker repo so it has the freshest content (all 6 fixes), finish that demo. Then come back to `agentAuthDemoApps` for the three scenario demos. Open question for later: should we merge `agentAuthDemoApps` into `agentauth-app`, make it a new standalone repo, or keep demo apps in a separate directory from the auth agent?

### Agent team attempt — FAILED, needs proper setup next session

Tried to set up a 6-agent council (pattern-analyst, code-auditor, operator-analyst, developer-analyst, security-analyst, devil's advocate) to deep dive into all features AgentAuth needs. Failed three ways:
1. First attempt: spawned parallel task agents — not a team, can't talk to each other
2. Second attempt: used TeamCreate + Task tool with team_name — agents registered in config but got "Not in a team context" error on broadcast. Agents ran independently, sent reports to team lead instead of debating with each other.
3. The agents did individual research but never collaborated. Security analyst sent 3 solo reports. No debate, no challenging each other, no shared answer.

**What went wrong:** Did not properly set CLAUDE_CODE_TEAM_NAME on spawned agents. Without that, they're not in a team context and can't message each other. Need to figure out the correct setup before trying again.

**What the team should do (not what it did):**
- Read BIG_BAD_GAP.md — that's the problem statement. Don't redo the analysis.
- Understand the deployment model: virtual server with Docker containers. Not Kubernetes, not cloud-native. A person deploys this on a VM.
- Come up with the complete set of features so: operator can register an app without editing docker-compose, developer can get credentials without needing a sidecar, the app either spawns its own sidecar or doesn't use one, everything works end-to-end.
- Debate and challenge each other. Arrive at ONE shared answer.
- DO NOT do a security review — the app is secure, the 6 compliance fixes are done. Don't give findings on things already agreed upon.
- DO NOT audit the code — we know what exists. Read BIG_BAD_GAP.md.

**User feedback:** "That's not an agent team." "If you didn't use the team create, it's not a fucking team." "It's not solid, nothing, because that's not what we agreed to." "They weren't useful — I didn't ask for a security analyst. This app is secure. What I want is them to review the big bad gap and come up with a fix."

### Gap Analysis + Process Maps + Architecture Comparison (Session 18 continued)

Created full gap analysis (24 gaps: 6 blockers, 7 major, 11 minor) and production process maps for all 4 personas (Operator, 3rd Party Developer, Running App, AI Agent) in plain language with hotel/building analogies. SVG visual diagrams created for VS Code viewing.

Compared gap analysis with CoWork Architecture doc (`.plans/CoWork-Architecture-Direct-Broker.md`). Both identify the same root problem (apps don't exist as entities) but CoWork goes further — sidecar becomes optional, 3 paths (SDK/Proxy/HTTP), client_id+client_secret model. Agreed with CoWork. This invalidates ADR-002 from Session 15.

**Process lesson learned:** Session 15 agents answered "do sidecars work?" (from code) when user asked "should sidecars be required?" (from pattern + needs). User was right in Session 14 — the correct answer required checking the pattern, not the code. Documented fully in `PROCESS-LESSON.md`.

**User feedback:**
- "this is not clear at all" — first process maps were too technical, complete rewrite needed
- "why would you make html" — HTML doesn't render in VS Code, switched to SVG
- "so to push back on size you should have pushed back on the sidecar now to agree with it" — pushing back on time estimates is trivial; pushing back on architecture decisions is the high-value feedback
- "This is crazy ... not really sure on how to fix this so it wont happen again" — led to naming the 3-step development process and creating standing rules

### Artifacts (Session 18 continued)
- `.plans/01-gap-analysis-process-map.md` — 24 gaps, sorted by severity
- `.plans/02-production-process-maps.md` — plain-language process maps, 4 personas
- `.plans/diagrams/01-system-overview.svg` through `06-gap-summary.svg` — visual diagrams
- `PROCESS-LESSON.md` — full history of the architecture lesson (Sessions 14-18)

### What's next
1. **Implement CoWork architecture** — apps as first-class entities, client_id+client_secret, sidecar optional, 3 paths
2. **Update process maps** to reflect the agreed CoWork architecture (current maps still assume sidecar-mandatory)
3. **Then: finish agentauth-app demo** and merge develop to main

## 2026-02-27 (Session 17)

### What happened
Designed demo application stories for AgentAuth. Three scenarios that show the danger of treating agent identity like traditional IAM, each mapping to one of the pattern's "Current Inadequate Approaches":
1. "The Stolen Key" — credential exfiltration (LangGrinch CVE, Pattern Problem Row 2)
2. "The Rogue Delegate" — privilege escalation via delegation (Pattern Problem Row 3)
3. "Who Did It?" — forensics failure with shared service accounts (Pattern Problem Row 1)

Each demo is one app with a `--mode vulnerable | --mode secure` toggle — same business logic, different credential model. Apps pull AgentAuth from https://github.com/devonartis/agentAuth/tree/develop and run against the real live stack. No mocks.

### Artifacts produced
- `agentAuthDemoApps/app_ideas_stories/scenario-1-stolen-key.md`
- `agentAuthDemoApps/app_ideas_stories/scenario-2-rogue-delegate.md`
- `agentAuthDemoApps/app_ideas_stories/scenario-3-who-did-it.md`
- `agentAuthDemoApps/app_ideas_stories/design-doc.md`

### Changes to this repo
- Pinned pattern URL in CLAUDE.md (under project description)
- Pinned pattern URL in MEMORY.md (top, before standing rules)
- Added standing rule: when `/obsidian:daily` runs, analyze the note — don't just execute steps

### User feedback (Session 17)
- "You should always start with the pattern" — I explored the codebase instead of starting from the pattern's Problem section. Wrong direction. The pattern defines the stories, not the code.
- "These docs should not go into my design docs folder" — demo stories belong in the consumer repo (`agentAuthDemoApps/`), not the product repo
- "We need to be able to turn it on and off so people can see" — one app, two modes, not two separate apps
- "This is not mock either we are doing really live app" — demos run against the real AgentAuth stack, no fake resource servers
- Frustrated that I didn't read MEMORY.md or FLOW.md at session start, didn't follow CORE identity/voice, ran daily skill as a checklist, wrote insights in documentation voice instead of journal voice

### Git operations
- No branch changes (design-only session, artifacts in external repo)
- Edited CLAUDE.md and MEMORY.md on develop

### What's next
1. **Implementation plan** for Scenario 1 — invoke writing-plans skill
2. **Python SDK** — the secure mode of the demos drives the SDK interface
3. Build order: Scenario 1 → Scenario 2 → Scenario 3

## 2026-02-26 (Session 16)

### Git operations
- Created `fix/structured-audit` off `develop`
- `05efc0f` — docs: session 16 kickoff
- `c7e07d1` — feat(audit): add structured fields and RecordOption to AuditEvent
- `253dbea` — feat(audit): include structured fields in computeHash for tamper evidence
- `55e837f` — feat(audit): add outcome filter to QueryFilters
- `06b76a0` — feat(store): SQLite migration for structured audit fields + outcome filter
- `f35635c` — feat(handler): add outcome query param to audit events endpoint (+ interface fixes)

### What happened
Implementing Fix 6 (structured audit log fields). Completed 6 of 10 tasks:
1. Branch + docs — done
2. Added 5 structured fields to `AuditEvent`, created `RecordOption` functional options type, updated `Record()` with variadic options — 22 tests pass
3. Updated `computeHash` to include all structured fields for tamper evidence
4. Added `Outcome` field to `QueryFilters` with filter clause in `Query()`
5. SQLite migration — 5 new columns, updated `SaveAuditEvent`/`LoadAllAuditEvents`/`QueryAuditEvents` with nullable types, outcome index
6. Added `outcome` query param to `audit_hdl.go`, updated `AuditRecorder` interfaces in `authz` and `identity` packages (variadic `...RecordOption` broke the old interface signatures)

**Key blocker hit:** Changing `Record()` to accept `...RecordOption` broke `AuditRecorder` interfaces in `authz/val_mw.go` and `identity/id_svc.go` — Go structural typing means every interface that declared the old exact signature needed updating. Fixed by adding `...audit.RecordOption` to both interfaces. Also added `audit` import to `identity/id_svc.go` (no circular dependency).

### Additional commits (continued session)
- `0e02ca9` — feat(audit): annotate all Record() callers with structured options (9 files, 52 insertions)
- `d014d69` — feat(aactl): add --outcome flag to audit events command
- Gates: build PASS, lint PASS (fixed errcheck in test), unit tests PASS, security WARN (pre-existing gosec findings)

### Docker live test results
- Smoketest: 12/12 PASS (full sidecar lifecycle including exchange + scope escalation denial)
- Story 1: PASS — all 18 audit events have `outcome` field populated (`success`/`denied`), `resource` on auth failures
- Story 2: PASS — `--outcome denied` filter returns exactly 3 events, all denied
- Story 3: PASS — hash chain intact across all 20 events, every `prev_hash` links correctly

### Fix 6 merge + pre-release cleanup
- Merged `fix/structured-audit` into `develop` (no-ff merge)
- Deleted `fix/structured-audit` branch
- **Pre-release cleanup:** Moved internal-use-only folders and files out of the repo to `/Users/divineartis/agentAuth_Backup_docs/`:
  - `plans/` → `agentAuth_Backup_docs/plans` (session plans, architecture decisions, reviewer reports, archive)
  - `docs/plans/` → `agentAuth_Backup_docs/docs-plans` (ROADMAP presentations, cost basis docs, slide images)
  - `generate-presentation.js` → backup (one-off script for generating roadmap slides)
  - `generate-roadmap.js` → backup (one-off script for generating roadmap docs)
- These are session artifacts, internal planning docs, and one-off scripts — not application code or user-facing documentation. They don't belong in a release.

### Known issues review — demo readiness assessment
Reviewed all 4 known issues (KI-001 through KI-004). **None block the demo:**
- **KI-001 (admin secret blast radius):** Production hardening. Demo environment is controlled — no risk of sidecar compromise during a presentation. Fix requires new broker endpoint + credential narrowing — post-demo work.
- **KI-002 (TCP default):** TCP is actually easier to demo (visible curl commands). UDS mode exists and works. Mention "UDS available for production" during demo, show TCP for clarity.
- **KI-003 (sidecar indistinguishability):** Blocked by KI-001 anyway. The `sid` field in tokens + Fix 6 structured audit gives sidecar-level traceability. With 1-2 sidecars in a demo, this is a non-issue.
- **KI-004 (ephemeral registry):** By design. A few hundred ms re-registration latency after restart. Nobody notices in a demo.

**Decision: Codebase is demo-ready.** Team can start building the demo flow. All 6 compliance fixes merged. Smoketest 12/12. Audit trail has structured fields with outcome filtering.

### What's next
1. **Documentation team review** — docs team reviews all code changes, updates docs to be professional-grade, polishes README.md. This happens before any SDK work.
2. **Python SDK for developers** — first demo audience is Python developers, they need a client SDK to interact with AgentAuth via the sidecar API (token request, renewal, release). This is the next development task after docs are polished.
3. **TypeScript SDK** — needed but lower priority, after first demo
- Operator tooling already exists via `aactl` CLI
- All 6 P1 compliance fixes are on `develop`

## 2026-03-02 (Session 19 — Claude CoWork)

### Product strategy & GTM work (done in Claude CoWork, not Claude Code)

This session shifted from engineering to product strategy and thought leadership. All work done in Claude CoWork desktop app.

#### Competitive Brief
Mapped competitive landscape: CyberArk ($15B), Okta/Auth0 ($14B), Aembit ($25M), Astrix ($85M), Entro ($18M), Oasis ($75M), SPIFFE/SPIRE (CNCF), DeepSecure/DeepTrail (44 stars, Python). Key finding: no competitor solves the full ephemeral agent credential lifecycle — enterprise vendors focus on governance/discovery, developer platforms extend OAuth. AgentAuth fills a genuine architectural gap.

→ Artifact: `.plans/gtm-strategy/AgentAuth-Competitive-Brief.docx`

#### Business Model Decision: Open-Core
Recommended open-core over pure open-source or pure commercial. Free self-hosted core + AgentAuth Cloud ($99-499/mo SMB) + Enterprise tier ($2K-10K/mo) + advisory engagements ($5K-20K). User leaning toward thought leadership as primary angle.

#### GTM Strategy & Thought Leadership Playbook
3-phase playbook: (1) establish authority via NIST submission + pattern publication, (2) build distribution via conference talks + blog series, (3) monetize via hosted service + enterprise. Identified NIST NCCoE concept paper (Feb 5, 2026) as highest-ROI immediate action — deadline April 2, 2026.

→ Artifact: `.plans/gtm-strategy/AgentAuth-GTM-Strategy.docx`

#### DeepSecure/DeepTrail Assessment
Found as competitor during research (Python-based, 44 GitHub stars, pip install + LangChain integration). User assessment: "not a real competitor — built in Python, application like this should not be built in Python, and they lean on frameworks we should not include frameworks." Go is the correct language for security infrastructure. Framework coupling (LangChain etc.) is an anti-pattern for credential infrastructure.

#### NIST NCCoE Public Comment Submission
Drafted public comment for NIST's "Accelerating the Adoption of Software and AI Agent Identity and Authorization" concept paper. Maps Ephemeral Agent Credentialing Pattern v1.2's 7 components to NIST's 4 focus areas (Identification, Authorization, Access Delegation, Logging/Transparency). Includes 7 specific recommendations and standards mapping. Submission to: AI-Identity@nist.gov by April 2, 2026.

→ Artifact: `.plans/nist-submission/NIST-NCCoE-Public-Comment-AgentAuth.docx`

### User reasoning (Session 19)
- "this is a lot of work to open source it when people are building SaaS on lesser idea" — frustrated that no community materialized around the published pattern
- Leaning toward thought leadership + business (both, not one or the other)
- Wants Go, not Python, for security infrastructure
- Opposes framework coupling (no LangChain, no framework dependencies)
- Wants recommendations, not just options — "I am asking for recommendations since you mentioned it"
- Documents must go in subfolders under `.plans/`, not at root

### What's next
1. **NIST submission review** — user reviews the public comment draft, adds contact info (email, LinkedIn), submits to AI-Identity@nist.gov before April 2, 2026
2. **Continue engineering work** — architecture redesign (apps as first-class entities), develop → main merge
3. **Thought leadership execution** — blog series, conference submissions per GTM playbook

## 2026-02-25 (Session 15)

### Git operations
- On `fix/sidecar-uds` branch (no new code commits — this session was architecture decision + docs)
- Created `plans/2026-02-25-sidecar-architecture-decision.md` (ADR-002)
- Archived original to `plans/archive/2026-02-25-sidecar-architecture-decision-original.md`
- Created `KNOWN-ISSUES.md` (4 known issues: KI-001 through KI-004)
- Created `plans/2026-02-25-post-merge-roadmap.md` (post-merge TODO)
- Removed stale `docs/plans/2026-02-25-sidecar-architecture-decision.md` (agents wrote to wrong dir)
- Merging `fix/sidecar-uds` to `develop` (this session)

### What happened
Ran a 4-agent collaborative debate to answer the 6 architecture questions from Session 14. Three iterations of team orchestration before getting it right (see FLOW.md for team lessons). Final team: 3 neutral analysts + 1 devil's advocate with veto power, shared prompt, broadcast messaging.

**Architecture decision (ADR-002): Keep sidecars as primary and only model.**
- Admin secret blast radius is unbounded (KI-001) — highest priority security fix
- Scope ceiling enforcement is real security (dual enforcement, cryptographically bound)
- Direct broker access blocked by code (`sidecarAllowedScopes()` requires `sidecar:scope:X` claims) — future work
- One sidecar per trust boundary as hard architectural rule
- All sidecars indistinguishable in audit (KI-003) — needs per-sidecar credentials
- TCP default is a security gap (KI-002) — UDS should be production default

**Rejected alternatives:**
- Direct broker access (`client_id`/`client_secret`) — broker code doesn't support it yet
- Hybrid (both sidecar + direct) — "complexity of both models with clean guarantees of neither"
- Remove sidecars entirely — loses DX, resilience, UDS access control, scope siloing

### User feedback (Session 15)
- Frustrated with team orchestration failures: "they should not have picked a side from the beginning" (pre-assigned positions), "aint no way they have even talked to each other" (agents working in isolation), "why arent they talking to each other and collaborating" (DMs vs broadcast)
- Approved the final decision document
- "why would you put stuff in the application docs WTF" — agents wrote to `docs/plans/` instead of `plans/`
- Directed: convert decision to ADR, create KNOWN-ISSUES.md, merge to develop, create post-merge TODO for docs + SDK

### Compliance fix status — 5 of 6 done, Fix 6 NOT started

| Fix | Status | Session |
|-----|--------|---------|
| Fix 2 — Revocation Persistence | DONE | Session 10 |
| Fix 3 — Audience Validation | DONE | Session 11 |
| Fix 4 — Token Release | DONE | Session 12 |
| Fix 1 — Sidecar TLS Client | DONE | Session 13 |
| Fix 5 — Sidecar UDS | DONE | Session 14 |
| **Fix 6 — Structured Audit** | **NOT STARTED** | Preempted by architecture brainstorm (Sessions 14-15) |

Fix 6 was always "next" after the current fix but never got picked up. Session 14 raised the sidecar architecture questions which blocked Fix 5's merge and consumed Session 15 entirely. Fix 6 is the only compliance fix remaining. Design is in `plans/design-solution.md` (lines 246-300), user stories at `tests/fix6-structured-audit-user-stories.md`.

### What's next
1. **Fix 6** (structured audit) — last compliance fix, design ready, never started
2. **Documentation deep dive** — operator guide, developer guide, architecture FAQ (see `plans/2026-02-25-post-merge-roadmap.md`)
3. **Admin secret narrowing** (KI-001 fix) — new broker endpoint
4. **SDK development** — Python + TypeScript

### Local branches
- `fix/sidecar-uds` (current, merging to develop)
- `develop`
- `main`
- `develop-harness-backup` (dead/reference only)

## 2026-02-26 (Session 14)

### Git operations
- Created `fix/sidecar-uds` off `develop`
- Commits: `f272ab4` (config field), `a5daba7` (UDS listener), `286aaf2` (multi-sidecar integration test), `4113f47` (docs + changelog + lint), `d9b3c18` (Docker live test infra)
- Branch NOT merged — **blocked on sidecar architecture brainstorm** (see below)

### What happened
Implemented Fix 5 (sidecar UDS listen mode) from `docs/plans/2026-02-25-fix5-sidecar-uds.md`. TDD throughout.

- `cmd/sidecar/config.go`: new `SocketPath` field, loaded from `AA_SOCKET_PATH`
- `cmd/sidecar/listener.go`: `startListener()` — UDS or TCP based on config, `0660` permissions, stale socket cleanup
- `cmd/sidecar/main.go`: replaced `http.ListenAndServe` with `startListener()` + `http.Serve`
- `docker-compose.yml`: `AA_SOCKET_PATH` env var
- `docker-compose.uds.yml`: compose overlay — 2 sidecars on different UDS paths + test-client container
- `docs/getting-started-operator.md`: `AA_SOCKET_PATH` in config table, new "Unix domain socket (UDS) mode" section

### Docker live test — PASSED (all 4 stories)

1. **Two sidecars healthy via UDS**: `app1.sock` (scopes: `read:data:*,write:data:*`) and `app2.sock` (scopes: `read:logs:*`) — unique sidecar IDs, both responding via `curl --unix-socket`
2. **Client token requests via UDS**: `data-agent` got token from app1.sock, `log-reader` got token from app2.sock — different `sid` fields confirm isolation
3. **`aactl sidecars list`**: shows both sidecars (Total: 2) with correct scopes and status
4. **TCP fallback**: sidecar without `AA_SOCKET_PATH` works on TCP, logs `WARN: listening on TCP — consider AA_SOCKET_PATH for production deployments`

### Debugging notes
- First run: both sidecars started simultaneously → `SQLITE_BUSY` on concurrent `SaveSidecar` writes. One sidecar missing from `ListSidecars` (SQLite), but present in memory (issued tokens fine). Pre-existing concurrency bug in store, not Fix 5. Workaround: stagger sidecar startups. Future fix needed: write mutex or WAL mode in SqlStore.
- `curlimages/curl` runs as uid 101 — can't access `0660` root-owned sockets. Set `user: "0:0"` on test-client container.

### BLOCKED: Sidecar architecture brainstorm required before merge

User raised fundamental questions about the sidecar model that must be answered and documented before this branch (and the project overall) merges to main. These are not Fix 5 bugs — they're architecture-level questions about *why sidecars exist* and *what alternatives operators have*.

**Questions to brainstorm:**
1. **How do operators create new sidecars?** Step-by-step for deploying a sidecar for a new app?
2. **How do 3rd-party SDK consumers register apps to use sidecars?** What's the developer workflow?
3. **FAQ: Why sidecars?** Rationale vs. direct broker access? What does the sidecar buy you?
4. **Can we remove sidecars entirely?** Could we have a mode where operators create an "app" with client_id/client_secret that talks directly to the broker?
5. **How would we silo scopes without sidecars?** If apps talk directly to the broker, how do we enforce per-app scope ceilings?
6. **How do operators configure sidecars for specific applications?** One per app? Per team? Per trust boundary?

### User feedback (Session 14)
- "we need to figure out really professionally documentation to understand how to use the sidecars and how to register application to ensure sidecars"
- "why we cant register application without using sidecars why cant we remove the sidecars totally"
- "how would we silo scopes for apps if we dont use it"
- Code is done but the *why* and *how* for operators/developers needs to be clear before merge

### What's next
1. **Brainstorm sidecar architecture questions** — resolve the 6 questions above
2. **If we keep sidecars: comprehensive documentation required before merge** — operator guide (how to deploy sidecars for new apps, sidecar-per-app vs per-team guidance), developer guide (SDK consumer onboarding, connecting to sidecar, UDS vs TCP), FAQ (why sidecars exist, what they buy you, alternatives considered), architecture doc updates (sidecar role in the security model). Current docs explain *what* the sidecar does but not *why* it exists or *how* operators/developers are supposed to use it end-to-end.
3. **If we remove sidecars: design the alternative** — app registration model, client_id/client_secret, scope siloing without sidecar ceilings
4. **Then merge** `fix/sidecar-uds` to `develop`
5. **Then Fix 6** (structured audit) — last fix

### Local branches
- `fix/sidecar-uds` (current, NOT merged)
- `develop`
- `main`
- `develop-harness-backup` (dead/reference only)

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

## 2026-02-25 (Session 13)

### Git operations
- Merged `fix/token-release` into `develop` (was already merged from Session 12, branch cleaned up)
- Created `fix/sidecar-tls-client` off `develop`
- Commits: `9d7c8e8` (TLS/mTLS client support), `3ba4ede` (Docker test infra), `a82068e` (docs + changelog)
- Merged `fix/sidecar-tls-client` into `develop` (fast-forward, `3512eb7..a82068e`), deleted branch

### What happened
Implemented Fix 1 (sidecar TLS client) from `docs/plans/2026-02-25-fix1-sidecar-tls-client.md`.

- `cmd/sidecar/config.go`: 3 new fields — `CACert`, `TLSCert`, `TLSKey`
- `cmd/sidecar/broker_client.go`: `newBrokerClient()` takes TLS params, new `buildTLSConfig()` with TLS 1.3 min
- `cmd/sidecar/main.go`: passes TLS config to broker client
- `docker-compose.yml`: 3 new sidecar TLS env vars
- `docker-compose.tls.yml`: compose overlay for one-way TLS testing
- `docker-compose.mtls.yml`: compose overlay for mutual TLS testing
- `scripts/gen_test_certs.sh`: generates CA + broker + sidecar certs (ECDSA P-256, SHA-256)
- `Dockerfile`: added `curl` to broker image for mTLS healthcheck
- `docs/getting-started-operator.md`: sidecar TLS env vars in config table, new "Sidecar TLS client" section
- 40 test call sites updated for new `newBrokerClient` signature, 8 new unit tests for `buildTLSConfig`

### Docker live test — PASSED (all 4 stories)

1. **HTTP baseline**: `./scripts/stack_up.sh` → broker + sidecar healthy, no regression
2. **TLS (one-way)**: `docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d` → sidecar bootstraps over HTTPS, broker health responds, plain HTTP rejected with "Client sent an HTTP request to an HTTPS server"
3. **mTLS (mutual)**: `docker compose -f docker-compose.yml -f docker-compose.mtls.yml up -d` → sidecar presents client cert, broker verifies, full bootstrap succeeds
4. **mTLS rejection**: `curl --cacert ca.pem https://localhost:8080/v1/health` (no client cert) → TLS handshake fails
5. `docker compose down -v` after each mode

### Debugging notes
- First cert generation used SHA-1 (openssl default) — TLS 1.3 rejects SHA-1 signed certs with "CA signature digest algorithm too weak". Fixed by adding `-sha256` to all `openssl` commands.
- mTLS healthcheck: Alpine's BusyBox `wget` doesn't support `--certificate`/`--private-key`. Added `curl` to broker Dockerfile and switched healthcheck to `curl` with `--cert`/`--key`.

### What's next: IMPLEMENT FIX 5
- Branch: `fix/sidecar-uds` off `develop`
- Plan: `docs/plans/2026-02-25-fix5-sidecar-uds.md`
- After Fix 5 → Fix 6

## 2026-02-25 (Session 12)

### Git operations
- Merged `fix/audience-validation` into `develop` (fast-forward, `457c81d..f1212a9`), deleted branch
- Created `fix/token-release` off `develop`
- Commits: `2a61b84` (handler + test + wiring), `7fa20e0` (aactl tooling + changelog)
- Branch NOT yet merged — ready for merge

### What happened
Implemented Fix 4 (token release) from `docs/plans/2026-02-25-fix4-token-release.md`.

- `internal/audit/audit_log.go`: `EventTokenReleased` constant
- `internal/authz/val_mw.go`: `ContextWithClaims()` test helper
- `internal/handler/release_hdl.go`: new handler — extract claims, revoke JTI, audit, 204
- `cmd/broker/main.go`: wired `POST /v1/token/release` through `valMw.Wrap()`
- `cmd/aactl/token.go`: `aactl token release --token <jwt>` operator command
- `cmd/aactl/client.go`: `doPostWithToken()` for agent-facing endpoints

### Docker live test — PASSED (via aactl)

1. `./scripts/stack_up.sh` — stack up, broker healthy
2. Story 1: admin auth → `aactl token release --token <jwt>` → token validate shows `valid: False`
3. Story 2: `aactl audit events --json` → `token_released` event with correct agent_id
4. Story 3: second `aactl token release --token <same>` → "Token already released (revoked)" (idempotent)
5. `docker compose down -v`

### User feedback (Session 12)
- "are you hacking the systems" — called out manual curl chaining as hacky and unrealistic
- "we should have the admin tooling" — every endpoint needs aactl tooling, not curl hacks
- "we should have made that part of the fix build the tooling" — tooling is part of the fix, not separate
- Standing rule added: no endpoint without aactl tooling, no curl in tests

### What's next: IMPLEMENT FIX 1
- Branch: `fix/broker-tls` off `develop`
- Plan: `docs/plans/2026-02-25-fix1-native-tls-mtls.md`
- After Fix 1 → Fix 5 → Fix 6

## 2026-02-25 (Session 11)

### Git operations
- Created `fix/audience-validation` off `develop`
- Commits: `9c5a139` (cfg), `d3e1a93` (authz+token), `7abeb86` (admin fix), `4e03188` (changelog)
- Branch NOT yet merged — ready for merge

### What happened
Implemented Fix 3 (audience validation) from `docs/plans/2026-02-25-fix3-audience-validation.md`. TDD throughout.

- `internal/cfg/cfg.go`: `AA_AUDIENCE` via `LookupEnv` — unset = "agentauth", empty = skip
- `internal/authz/val_mw.go`: audience field, check in `Wrap()` after revocation
- `internal/identity/id_svc.go`: audience field, populates `Aud` on registration tokens
- `internal/token/tkn_svc.go`: `Renew()` preserves `Aud`
- `internal/deleg/deleg_svc.go`: `Delegate()` propagates `Aud` from delegator
- `internal/admin/admin_svc.go`: audience field, populates `Aud` in `Authenticate()` and `ActivateSidecar()`
- `internal/handler/token_exchange_hdl.go`: propagates `Aud` from sidecar caller
- `docker-compose.yml`: `AA_AUDIENCE` env var

### Docker live test finding: missed issuance path
Plan covered 3 of 4 token issuance paths (IdSvc, Renew, Delegate). AdminSvc.Authenticate and ActivateSidecar were missed — admin tokens got "audience mismatch" 401. Docker live test caught this immediately. Also added token exchange handler propagation.

### Docker live test — PASSED

**Steps:**
1. `AA_AUDIENCE=broker-production ./scripts/stack_up.sh`
2. Story 3: admin auth → launch token → register agent → verify `aud: ["broker-production"]` in token → validate → renew → verify audience preserved
3. Story 1: correct-audience tokens accepted on authenticated endpoints (audit, renew)
4. Story 2: `AA_AUDIENCE="" ./scripts/stack_up.sh` → no audience in tokens → all endpoints work → backward compatible
5. `docker compose down -v`

All 3 user stories pass.

### What's next: IMPLEMENT FIX 4
- Branch: `fix/token-release` off `develop`
- Plan: `docs/plans/2026-02-25-fix4-token-release.md`
- User stories: `tests/fix4-token-release-user-stories.md`
- After Fix 4 → Fix 1 → Fix 5 → Fix 6

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

## 2026-03-04 (Session 21 — Claude CoWork)

### What happened

Two-agent devil's advocate review of all PRD + phase specs, followed by targeted fixes based on confirmed findings.

### Agent findings summary

**Validator agent** checked 10 of the original 41 devil's advocate gaps against the actual codebase:
- 7 were FALSE POSITIVES (already handled): mTLS in `serve.go`, bcrypt 72-byte limit safe (64 < 72), `ScopeIsSubset()` exists, `ConsumeNonce()` has mutex, activation endpoint exists, timing-safe auth, launch tokens are opaque hex (correct pattern)
- 2 confirmed real: RevSvc O(n) for app revocation (needs index), SQLite migration for app_id columns

**Second devil's advocate** found 16 fresh gaps. Critical ones addressed:
- SQLite migration: `ALTER TABLE` must use `TEXT DEFAULT NULL` for new `app_id` columns on existing tables
- RevSvc index: `appAgents map[string][]string` in-memory index required in Phase 1c
- Phase 4 + 5 production trap: shipping JWKS without key persistence invalidates cached keys on every restart
- Phase 3 priority: P1 is correct (sidecar covers DX gap post-Phase 2); note added for teams without sidecar
- Dual-secret Non-Goal contradiction: Phase 1c needed dual-secret storage for rotation grace period — fixed

### Changes made this session

1. **`.plans/phase-1a/Phase-1a-App-Registration-Auth.md`** — fixed `initTables()` → `InitDB()`, fixed handler path `internal/handler/` → `internal/app/`, added SQLite migration strategy section
2. **`.plans/phase-1c/Phase-1c-Revocation-Audit-SecretRotation.md`** — fixed Non-Goals contradiction, added `appAgents` index spec, added dual-secret storage schema for rotation grace period
3. **`.plans/phase-4/Phase-4-JWKS-Endpoint.md`** — added production warning at top: Phase 4 without Phase 5 is a stability trap
4. **`.plans/PRD.md`** — added Phase 3 priority rationale, Phase 4+5 bundling recommendation, 3 new rows in risks table

### Decisions made

- **Phase 3 stays P1:** Post-Phase 2, the sidecar covers the DX gap. Teams without sidecar can treat it as personal P0. This is documented.
- **Phase 4+5 ship together in production:** Documented as a risk and recommendation in PRD and Phase 4 spec.
- **SQLite migration = ALTER TABLE + NULL default:** No migration framework needed. `InitDB()` handles it with error-tolerant logic.
- **Dual-secret for rotation is Phase 1c (not P2):** The grace period design requires it. "Permanent dual-active" is still P2.

**Spec update (same session):** All phase specs (1b through 5) now have a `## Testing Workflow` section at the bottom explicitly telling the implementing agent: extract user stories from the spec into `tests/phase-Xn-user-stories.md` before writing any test code. Phase 1a already had this in Task 6. User stories remain IN the specs — the rule just makes the `tests/` step explicit so the agent doesn't skip it.

