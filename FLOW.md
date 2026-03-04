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

## Next session: Docker live test for Phase 1B

**Branch:** `feature/phase-1b-launch-tokens` — 7 commits, all unit tests green (15 packages). Code complete, needs Docker live test before merge.
**Action:** Run `./scripts/stack_up.sh`, execute all 11 stories from `tests/phase-1b/user-stories.md`, save evidence to `tests/phase-1b/evidence/`, merge to develop.

---

## 2026-03-04 (Session 27 — Phase 0 Live Tests + Template Rewrite)

### Phase 0 live tests: 12/12 PASS

Executed all 12 stories against Docker stack. Evidence piped directly into files with banners. All sidecar routes return 404, admin auth works with new format, old format rejected with guidance, app registration/login/scope isolation/audit trail all passing.

### Decision: Live test evidence must be one test per file, plain language

Divine's feedback during live testing: evidence files were being written with multiple checks bundled together ("check all 6 routes in one file") and using technical jargon ("new auth shape"). Rules established:
- **One test per evidence file.** Each file has one action, one banner, one result.
- **Plain language.** "The operator tries to log in with..." not "the new auth shape". An executive should understand it.
- **Who, what, why in the banner.** The Context section tells you who is doing the work, what they're doing, where (Docker broker at 127.0.0.1:8080), and why it matters.
- **Operator uses aactl, developer uses curl.** No mixing personas.
- **Banner goes IN the bash call.** Echo the banner, pipe output to file, display the result — all in one call.
→ Template: `tests/LIVE-TEST-TEMPLATE.md` (complete guide with real examples)
→ Phase 0 evidence: `tests/phase-0/evidence/` (12 files, all PASS)

---

## 2026-03-04 (Session 26 — Phase 0 Legacy Cleanup Implementation)

### Decision: Skip brainstorming when design already exists

Brainstorming skill was invoked but stopped — Phase 0 spec (`.plans/phase-0/Phase-0-Legacy-Cleanup.md`) was already a complete, approved design from Session 25. Went straight to executing-plans. Lesson: don't brainstorm what's already designed.

### Decision: Admin auth — detect old format, return migration guidance

For admin auth (Task 0.2), instead of silently failing or returning a generic 400, the broker now detects the old `{"client_id", "client_secret"}` format and returns a specific error: "Use `{"secret": "..."}` instead of client_id/client_secret". This costs a few extra lines but prevents hours of debugging for anyone with old scripts. The detection reads the body once and tries both unmarshals.

### Decision: Keep sidecar handler methods, remove only routes

Task 0.1 removed route registrations from `RegisterRoutes()` and `main.go` but kept the handler methods in `internal/admin/admin_hdl.go`. The methods are tested independently of routes and will be re-wired in Phase 2 with app-scoped activation tokens. Deleted the route-based tests since they'd get 404s. Skipped (not deleted) sidecar integration tests in `cmd/sidecar/`.

---

## 2026-03-04 (Session 24 — Phase 1B Implementation)

### writing-plans: Phase 1B App-Scoped Launch Tokens

Created detailed implementation plan from the Phase 1b spec. 9 tasks across 5 batches. Key architectural decisions: one endpoint two callers (RequireAnyScope instead of separate route), ceiling enforcement at handler level (not service), app JWT scopes stay hard-coded (ceiling enforced at use-time not token-issuance).
→ Artifact: `.plans/phase-1b/Phase-1b-Implementation-Plan.md`

### subagent-driven-development: Phase 1B Execution

Executed all 9 tasks using fresh subagent per task with parallel dispatch for independent work. Batch 1 (3 tasks) ran in parallel — minor git collision on sql_store.go resolved cleanly. Batch 2 was the core task (8 files changed). Batch 3 ran 2 tasks in parallel. Total: 7 commits, 302+ lines of new code, 22 doc comments added.

### Decision: One endpoint, two callers — not separate routes

The spec says "apps use the existing launch token endpoint, just with different auth." Instead of creating `POST /v1/app/launch-tokens`, we added `RequireAnyScope` middleware that accepts either `admin:launch-tokens:*` or `app:launch-tokens:*`. This avoids handler duplication and keeps the API surface minimal. The handler detects caller type from JWT `sub` prefix (`app:` vs `admin`).

### Decision: Ceiling enforcement at handler level

The handler extracts `app_id` from JWT `sub`, looks up `AppRecord.ScopeCeiling`, and runs `ScopeIsSubset` before calling the service. This keeps the service layer pure — `CreateLaunchToken` doesn't need to know about auth context. The handler is the right place because it already has claims in context.

### Decision: App JWT scopes stay hard-coded

Phase 1a issues all app JWTs with `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]` regardless of app ceiling. This is correct — those are API-level permissions ("you can call this endpoint"), not delegation permissions. The ceiling is enforced when the app *uses* those permissions (creating launch tokens). Changing `AuthenticateApp` was explicitly out of scope.

### Decision: AppID as optional empty string, not pointer

`AppID string` (not `*string`) on both `LaunchTokenRecord` and `AgentRecord`. Empty string means admin-created. Simpler than nil-pointer handling, consistent with how `CreatedBy` works. No migration needed for in-memory records.

### Process lesson: User stories before code

The plan had user stories as Task 8, but CLAUDE.md standing rule says "Write user stories FIRST, save to tests/, before writing test code." We wrote code first, then created user stories. Future sessions: extract user stories as Task 1 regardless of what the plan says. The standing rule overrides the plan.

### Audit: Doc comment quality gate

22 Go doc comments added across 5 files (Priority 1-3 from audit). All exported Phase 1a and 1b types, methods, and struct fields now have professional doc comments. Key improvements: `CreateLaunchToken` appID param documented, `LaunchTokenRecord` and `AgentRecord` fields fully documented, `RegisterAppResp.ClientSecret` plaintext-once rule documented.

---

## 2026-03-03 (Session 23 — Phase 1A Live Test & Infrastructure Fixes)

### Decision: Sidecar removed from docker-compose.yml

Sidecar was starting unconditionally with every stack. Architecture (Session 20) says it's "optional" but the PRD never defines what it's optional for. No Phase 1A story uses it. Removed entirely from `docker-compose.yml` and `stack_up.sh`. Code stays in `cmd/sidecar/` — only infrastructure removed. Can be re-added when a concrete use case is documented.

### Decision: Two personas, two tools for acceptance testing

Operator stories (0-5, 11) use `aactl` — the built binary, not `go run`. Developer stories (6-7) use `curl` / REST API — the developer has no CLI, no admin key, only `client_id` + `client_secret` from the operator. Security stories (8-10) use both. Testing with the wrong persona's tool is cutting corners.

### Decision: Acceptance tests are operator experience, not scripts

The live test is an operator running commands at a terminal — not a bash script wrapping everything. If the commands don't work without a wrapper, the tooling is broken. Build `./bin/aactl`, source `tests/phase-1a/env.sh`, run commands. Same as a VPS.

### Live test results: 10 PASS, 2 PARTIAL

All 12 stories (0-11) executed against Docker stack (broker only). Two issues found:
1. `app_rate_limited` audit event not emitted — rate limiter middleware fires before handler reaches audit. P0 fix needed.
2. `sk_live_` prefix removed from criteria — decided plain 64-char hex is sufficient for now.

→ Artifact: `tests/phase-1a/evidence/` (per-story evidence files), `.plans/phase-1a/ADR-Phase-1a-Tech-Debt.md`

### Decision: Phase test artifacts organized per-phase

All Phase 1A test materials moved to `tests/phase-1a/` (user-stories.md, env.sh, lessons-learned.md, evidence/). Future phases get same structure. Phase 1A regression stories carry forward into Phase 1B.

---

## 2026-03-03 (Session 22b — Phase 1a Tasks 4-5 Completion)

### Skill: executing-plans — Phase 1a Tasks 4-5

Executed remaining Phase 1a tasks: wiring `AppHdl` into `main.go`, fixing missing `scopes` field in app auth response, adding `WrapWithKeyExtractor` to the rate limiter with per-client_id body-peeking, updating `NewAppHdl` to use 10 req/min burst-3 rate, creating `cmd/aactl/apps.go` with all 5 CLI commands, and adding `doDelete` to `client.go`. 4 new rate limiter tests added. `go test ./...` clean with zero regressions across all 15 packages.

→ Artifact: `internal/authz/rate_mw.go` (WrapWithKeyExtractor), `internal/app/app_hdl.go` (scopes, per-client_id rate limit, clientIDFromBody), `cmd/broker/main.go` (AppHdl wired), `cmd/aactl/apps.go` (new), `cmd/aactl/client.go` (doDelete added)

### Decision: Retry-After: 60 for per-client_id (vs 1 for IP-based)

IP-based rate limiting uses `Retry-After: 1` (short — mainly protects against bursts). Per-client_id app auth uses `Retry-After: 60` because the bucket refills over a 60-second window (10 req/min). Matching the retry hint to the actual window avoids clients hammering immediately after a 429.

### What's next: Task 6 — Docker live test against all 11 user stories

All unit tests pass. The gate before merging to `develop` is the Docker live test: `./scripts/stack_up.sh` + validate all 11 user stories from `tests/phase-1a-user-stories.md` against the running stack.

---

## 2026-03-03 (Session 22 — Claude Code)

### Skill: TDD — Phase 1a Implementation (Tasks 1–3)

TDD was invoked before any implementation. Three complete RED → GREEN cycles:
1. Store tests (12) → AppRecord + 6 CRUD methods
2. Service tests (15) → AppSvc with bcrypt, validation, audit events
3. Handler tests (17) → AppHdl with all 6 endpoints, RFC 7807 errors

All 44 new tests written first, watched fail, then minimal implementation to pass. Zero regressions across full suite after each cycle. Bcrypt cost 12 adds ~260ms per auth test — acceptable for security, expected in test output.

→ Artifact: `internal/app/` (new package), `internal/store/sql_store.go` (AppRecord additions), `internal/audit/audit_log.go` (6 event constants)

### Decision: unit tests ≠ acceptance tests

User corrected an important conflation: unit tests (RED/GREEN TDD cycles) test code logic in isolation. Acceptance tests are the user stories in `tests/phase-1a-user-stories.md`, verified manually against the running Docker stack. Task 6 is not "write more tests" — it is "run the Docker stack and prove each story's acceptance criteria passes." These serve completely different purposes and neither substitutes for the other.

### Decision: tests/phase-1a-user-stories.md as the gate

Created before any code was written. 11 stories. This is the standing rule: user stories file in `tests/` must exist before test code. The stories are the acceptance test — the checklist run against Docker before merge. Not a formality.

### Lesson: handler test mux needs both handlers

AppHdl's admin endpoints require `admin:launch-tokens:*` scope. Tests need a real admin Bearer token. Solution: register both `AdminHdl` and `AppHdl` on the same test mux. `getAdminToken()` helper calls `POST /v1/admin/auth` on the same mux to get a valid token before calling protected app endpoints. This is the same pattern used in `admin_hdl_test.go`.

---

## 2026-03-02 (Session 20 — Claude CoWork, continued)

### Architecture: Full Pattern Flow Added to Design Doc

The architecture doc (`.plans/CoWork-Architecture-Direct-Broker.md`) originally only answered "do we need the sidecar?" Divine's feedback: "you also missed the full flow of everything" and "we can't fix one thing and break other things." Added a complete 9-stage walkthrough proving every capability is preserved, enhanced, or explicitly new. Result: 8 preserved, 5 enhanced, 4 new, 0 removed. All 10 security invariants maintained.

**Why this matters:** An architecture change that only talks about what it fixes — without proving what it preserves — is incomplete. The Full Pattern Flow is the proof that the Ephemeral Agent Credentialing Pattern still works end-to-end after making the sidecar optional.

→ Artifact: `.plans/CoWork-Architecture-Direct-Broker.md` (Section: "Full Pattern Flow")
→ Artifact: `.plans/CoWork-Diagram-FullLifecycle.svg`

### Decision: Named the Pre-Coding Process — "Context-First Development" (CFD)

Divine asked: "how do we include this level of process before developing in the future? what would we call it and how would we do it with coding agents?"

Every mid-session correction in Sessions 18-20 traces to insufficient pre-build context. The agent didn't know about SPIRE opt-outs, didn't inventory all capabilities before designing, didn't prove preserved capabilities, didn't get design reviewed before task breakdown. **Context-First Development** formalizes the fix: 4 phases, 13 steps, with explicit "build the map before you build the code" sequencing.

**Phase 1: DISCOVER** — map what exists (codebase archaeology), extract pattern requirements, check constraints and compliance
**Phase 2: DESIGN** — PRD with "MUST NOT change" section, impact analysis (PRESERVED/ENHANCED/NEW/REMOVED per capability), implementation details with DO NOT TOUCH lists, peer review gate
**Phase 3: BUILD** — compile TASK-BRIEF.md (the one doc the agent reads first), break into tasks, write user stories + negative cases, agent implements
**Phase 4: VERIFY** — verification tasks (forward), regression verification (backward), post-build review (design vs actual)

→ Artifact: `.plans/CoWork-Context-First-Development.md` (compact reference for repo)
→ Artifact: `.plans/CoWork-Agent-Development-Lifecycle.html` (full visual guide with case study)

### Decision: 5 New Skills to Fill Superpowers Gaps

Divine pointed to [obra/superpowers](https://github.com/obra/superpowers/tree/main/skills) as the execution framework. Mapped all 14 existing Superpowers skills against the CFD process. Finding: strong on BUILD + VERIFY, gaps in DISCOVER + early DESIGN.

**Existing skills that cover CFD steps:** brainstorming (Step 1.2), writing-plans (Step 3.1), executing-plans (Step 3.3), subagent-driven-development (Step 3.3), test-driven-development (Step 3.2), verification-before-completion (Step 4.1), requesting-code-review (Step 2.4 partial), finishing-a-dev-branch (Step 4.3 partial)

**5 new skills to create (in build order):**
1. **codebase-archaeology** — map every capability before designing. Would have prevented "you missed the full flow."
2. **impact-analysis** — capability preservation matrix. Would have prevented "we can't fix one thing and break other things."
3. **design-review** — architecture review BEFORE coding (vs code review AFTER). Would have caught the 7 peer review gaps earlier.
4. **task-briefing** — compile TASK-BRIEF.md from all Phase 1-2 outputs. The bridge between human design and agent execution.
5. **regression-verification** — prove OLD capabilities still work after changes. Superpowers' verification-before-completion only proves NEW work passes.

**5 enhancements to existing skills:**
- brainstorming: add ADR format + "MUST NOT change" section
- writing-plans: add DO NOT TOUCH file lists per task
- requesting-code-review: add design-review prompt template
- verification-before-completion: add regression category
- test-driven-development: add negative test cases + user story format

→ Artifact: `.plans/CoWork-Superpowers-Gap-Analysis.html`

### Lesson: Impact Analysis Is the Missing Gate

The 3-step process from Session 18 (Architecture Challenge → Gap Analysis → Process Maps) was correct but incomplete. It answers "should this exist?" and "what's broken?" but not "what breaks if we change this?" The impact analysis step — the capability preservation matrix — is the gate that prevents the "fix one thing, break another" failure mode. This is now Step 2.2 in CFD.

### Lesson: TASK-BRIEF.md Is the Universal Agent Interface

No matter which coding agent you use (Claude Code, Cursor, Copilot, etc.), they all read files. TASK-BRIEF.md is a standard markdown file that any agent can follow. The only difference is HOW each agent discovers it: CLAUDE.md for Claude, .cursorrules for Cursor, etc. This means the process is agent-agnostic — the human writes one briefing doc, any agent can execute from it.

---

## 2026-02-28 (Session 18 — continued)

### Architecture Challenge: ADR-002 Invalidated

ADR-002 (Session 15) kept sidecars mandatory based on code analysis. CoWork Architecture doc (`.plans/CoWork-Architecture-Direct-Broker.md`) correctly challenged this from the pattern and user needs. The Ephemeral Agent Credentialing v1.2 pattern does NOT require sidecars. 3rd-party developers should not need to deploy sidecars. Apps need client_id + client_secret, not activation tokens that expire on first use.

ADR-002 is now invalidated. The agreed architecture is: apps as first-class entities, sidecar optional (3 paths: SDK/Proxy/HTTP), client_id+client_secret for persistent app credentials.

→ Artifact: `.plans/CoWork-Architecture-Direct-Broker.md`

### Gap Analysis: 24 Gaps Identified Against Current Implementation

Full inventory of current system against production needs. 6 BLOCKERS (all app registration — apps don't exist as entities), 7 MAJOR (security/operational), 11 MINOR (polish). All 6 blockers trace to one root cause: no App entity in the system.

→ Artifact: `.plans/01-gap-analysis-process-map.md`

### Process Maps: 4 Personas in Plain Language

Documented production flows for Operator (Building Manager), 3rd Party Developer (Tenant), Running App (Software), AI Agent (Guest). Used hotel/building analogies throughout — JWT → "key card", Sidecar → "helper", Broker → "security desk". SVG visual diagrams created for VS Code viewing.

→ Artifact: `.plans/02-production-process-maps.md`, `.plans/diagrams/01-06.svg`

### Decision: Named the Three-Step Development Process

Established taxonomy: (1) Architecture Challenge — question fundamentals, (2) Gap Analysis — inventory what's broken, (3) Process Maps — document flows per persona. Order matters — Step 2 without Step 1 means analyzing the wrong architecture. This is what happened: gap analysis ran before architecture was settled.

→ Artifact: `PROCESS-LESSON.md`

### Lesson: Pushback Priority

When reviewing architecture proposals, push back on architecture decisions (high value), not on time estimates (trivial). User quote: "so to push back on size you should have pushed back on the sidecar now to agree with it."

### Lesson: "Why does this exist?" Has a Specific Answer Source

When someone asks "should X exist?" — the answer comes from (1) user's question, (2) the pattern, (3) production needs. NOT from reading the current code. Code says what IS. The question is about what SHOULD BE. Standing rule added to MEMORY.md.

---

## 2026-03-02 (Session 19 — Claude CoWork)

### Shift: Engineering → Product Strategy & Thought Leadership

Session pivoted from code to product positioning. All work done in Claude CoWork desktop app (not Claude Code). Triggered by user question about whether AgentAuth should be commercial vs open-source given zero community engagement on the published pattern.

### Decision: Open-Core Business Model

Analyzed competitive landscape and chose open-core: free self-hosted core + paid hosted tiers. Rationale: captures both thought leadership credibility (open pattern/code) and revenue (hosted service removes deployment friction for SMBs). Bootstrap-viable — no funding needed to start. Key insight: no competitor actually solves credential *issuance* for AI agents. Enterprise vendors do governance/discovery. Developer platforms extend OAuth. AgentAuth fills the gap.

→ Artifact: `.plans/gtm-strategy/AgentAuth-Competitive-Brief.docx`, `.plans/gtm-strategy/AgentAuth-GTM-Strategy.docx`

### Decision: NIST Public Comment as Thought Leadership Entry Point

NIST NCCoE published "Accelerating the Adoption of Software and AI Agent Identity and Authorization" (Feb 5, 2026) with public comment period ending April 2, 2026. This is the highest-ROI immediate action for establishing authority — the pattern maps directly to NIST's 4 focus areas. Submission based on the pattern and architecture design (not dependent on code completion). User can position as independent practitioner who has both the theory (pattern) and working implementation (AgentAuth).

→ Artifact: `.plans/nist-submission/NIST-NCCoE-Public-Comment-AgentAuth.docx`

### Decision: DeepSecure is Not a Real Competitor

DeepSecure/DeepTrail (Python, 44 GitHub stars, LangChain integration) validates market existence but has architectural weaknesses: Python for security infrastructure (wrong language for performance/reliability), framework coupling (LangChain dependency limits adoption). AgentAuth's Go implementation and framework-agnostic design are strategic advantages, not just preferences.

---

## 2026-02-28 (Session 18)

### Cleanup: Moved `.plans/` to backup

`.plans/` was missed during the Session 16 pre-release cleanup (`5626f13`). That commit removed `plans/` but not `.plans/` — two different directories. Contents were all stale: roadmap, backlog, list-sidecars design docs, completed P0 plans, and an old `USER-STORIES-PLAN.md` from Session 1. Moved to `/Users/divineartis/agentAuth_Backup_docs/dot-plans/`. Nothing in `.plans/` is needed for current work — the active demo stories are in `agentAuthDemoApps/app_ideas_stories/` (confirmed via MEMORY.md Session 17 artifacts).

### Agent team attempt — failed, agents couldn't collaborate

Spawned 5 agents (pattern-analyst, code-auditor, operator-analyst, developer-analyst, security-analyst) on a TeamCreate team but they couldn't message each other — "Not in a team context" error. Agents ran independently and sent solo reports instead of debating. The security analyst found useful things (admin secret used in 3 runtime locations, two security gates on token exchange, dependency order for changes) but it worked alone, not as a council. User wanted a real debate — agents sharing, challenging, arriving at ONE answer together. Failed to deliver that. Need to understand CLAUDE_CODE_TEAM_NAME setup before trying again.

### BLOCKER FOUND: App registration workflow is broken — blocks demo and merge

While testing agentauth-app, discovered the sidecar bootstrap is fundamentally flawed. Every sidecar self-provisions with the admin secret. Apps don't exist as entities in the broker. Onboarding an app means editing docker-compose — not using an API. This goes deeper than KI-001 (admin secret blast radius) — this is about the identity model itself. User's key point: "Docker is infrastructure. This should run on someone's virtual server. If it can't, it's wrong." Must fix before demo.

→ Artifact: `BIG_BAD_GAP.md`

### Decision: Demo app validation gates the develop → main merge

AgentAuth `develop` has all 6 compliance fixes but hasn't been merged to `main` yet. The gate: get `agentauth-app` working against the current broker, test the demo end-to-end. Once the demo app validates the broker works correctly, merge `develop` to `main`. The demo is the acceptance test for the release.

### Feature catalog saved to agentauth-app

Created `new-features-agentauth.md` at the root of `agentauth-app` — full catalog of broker endpoints, sidecar endpoints, aactl commands, env vars, audit events, token claims, and all 6 fix summaries. Lesson: should have used `git log -30` instead of an Explore agent — the commit messages already had everything needed.

### Decision: Finish agentauth-app demo before new scenario demos

Two demo app directories exist. `agentauth-app` is the original showcase (Python, SDK, dashboard, agents) but it's pinned to a stale broker (~50 commits behind). `agentAuthDemoApps` has the three scenario design docs from Session 17 but no code. User wants to finish the original demo first — pull the current broker with all 6 fixes, update the SDK, get it working with the latest features. Then build the three scenario demos. Open question deferred: whether to merge everything into one repo, keep separate, or restructure.

### Lesson: MEMORY.md is the source of truth, not git log

When asked "where are the demo user stories?", grepped git log and FLOW.md instead of reading MEMORY.md first. MEMORY.md Session 17 lines 65-68 had the exact answer: `agentAuthDemoApps/app_ideas_stories/`. The standing rule in CLAUDE.md says "Read MEMORY.md and FLOW.md first every session" — that applies to mid-session lookups too, not just session start.

---

## 2026-02-27 (Session 17)

### Brainstorming: Demo Application Stories

Designed three demo apps to prove AgentAuth solves the agent identity problem. Started wrong — explored the codebase instead of the pattern. Divine corrected: the Ephemeral Agent Credentialing Pattern v1.2's Problem section already defines the three inadequate approaches. Stories map 1:1 to those rows, not independently brainstormed from code.

→ Artifact: `agentAuthDemoApps/app_ideas_stories/` (3 scenario docs + design-doc.md)

### Decision: One App, Two Modes — Not Separate Codebases

Each demo is a single Python app with `--mode vulnerable` and `--mode secure`. Same business logic, same task, different credential model. This makes the comparison visceral — toggle it and watch. Rejected the original design of separate `vulnerable/` and `secure/` directories. The audience needs to see the same app behave differently, not two different apps.

### Decision: Pull AgentAuth from GitHub, Not Local

Demo apps pull AgentAuth from https://github.com/devonartis/agentAuth/tree/develop. No dependency on having the authAgent2 repo locally. The demo repo is self-contained — clone it, `docker compose up`, everything runs. This is how a real developer would encounter AgentAuth.

### Decision: Real Live Stack, No Mocks

Demos run against the real AgentAuth broker and sidecar. No mock resource servers, no simulated responses. The denials, audit trails, scope violations, and revocations are all real. The audience sees actual API responses, actual JWT claims, actual hash-chained audit entries.

### Decision: Pin Pattern URL in Session Context

The pattern URL was not in CLAUDE.md or MEMORY.md — the two files read at session start. Pinned it in both. Standing rule: critical reference documents must be linked where the AI reads them on startup, not just known to the human.

---

## 2026-02-26 (Session 16)

### Executing-Plans: Fix 6 — Structured Audit Log Fields

Implementing the last compliance fix. Functional options pattern (`RecordOption`) chosen to keep existing `Record()` callers backward-compatible — variadic `...RecordOption` means zero-option calls compile unchanged. New structured fields (`resource`, `outcome`, `deleg_depth`, `deleg_chain_hash`, `bytes_transferred`) added to `AuditEvent`, included in `computeHash()` for tamper evidence, persisted via SQLite migration, and exposed through query API with `outcome` filtering.

→ Plan: transcript plan from session 16

### Decision: AuditRecorder Interface Update

Changing `Record()` signature to `Record(..., opts ...RecordOption)` broke the `AuditRecorder` interfaces in `authz` and `identity` packages. Go structural typing means variadic params change the function signature — concrete types compile fine but interfaces with the old exact signature break. Fixed by updating both interfaces. Added `audit` import to `identity/id_svc.go` (verified no circular dependency: `audit` doesn't import `identity`).

### Decision: SQLite Nullable Columns for Backward Compat

New audit columns use `DEFAULT NULL` + `sql.NullString`/`sql.NullInt64` scan types so existing rows with NULL values don't break `LoadAllAuditEvents`. Helper funcs `nullableString()` and `nullableInt()` map zero-values to NULL on write (zero-value fields → NULL in DB, non-zero → real value).

### Executing-Plans: Fix 6 — All 10 tasks complete

Tasks 1-10 done. All ~20 `Record()` callers annotated with `WithOutcome`. Lint gate required fixing errcheck on two `SaveAuditEvent` calls in test code. Gates: build PASS, lint PASS, unit tests PASS, security WARN (pre-existing). Docker live test passed all 3 stories. Evidence saved to `tests/fix6-structured-audit-evidence/`.

### Decision: Demo-Ready — Known Issues Don't Block

Reviewed KI-001 through KI-004. All are production hardening items (admin secret narrowing, TCP default, sidecar distinguishability, ephemeral registry). None affect a controlled demo environment. The codebase is demo-ready with all 6 compliance fixes merged, smoketest passing 12/12, and structured audit trail working end-to-end.

### Decision: Python SDK First for Demo

First demo audience is Python developers. They need a client SDK to interact with AgentAuth — the broker and sidecar expose HTTP APIs, but nobody should be hand-rolling curl in their agent code. Python SDK is the critical missing piece. TypeScript SDK is also needed but deferred to after the first demo. The SDK should cover: agent registration (Ed25519 challenge-response), token requests, token renewal, token release. The sidecar handles most of the complexity (lazy registration, renewal), so the SDK's primary interface is the sidecar's API, not the broker's.

### Decision: Pre-Release Cleanup — Remove Internal Artifacts

Moved `plans/`, `docs/plans/`, `generate-presentation.js`, and `generate-roadmap.js` out of the repo to `/Users/divineartis/agentAuth_Backup_docs/`. These are session planning artifacts (architecture decisions, reviewer reports, roadmap presentations, cost basis slides, one-off generation scripts) — internal working documents, not application code or user-facing docs. They were cluttering the repo and would confuse anyone pulling the release. The `docs/` folder now contains only application documentation (API, architecture, getting-started guides, troubleshooting). Backup location is outside the repo so nothing is lost.

### Decision: Test Evidence Structure

Every fix/feature Docker live test now produces a `tests/<fix-name>-evidence/` folder containing:
1. `README.md` — overview of what was tested, table of stories, how events were generated
2. `story-N-<name>.md` — per-story evidence: plain English explanation, reproduction steps, raw JSON output, what to look for, pass/fail verdict
3. `smoketest-output.txt` — raw smoketest output

The goal: anyone can open the evidence folder and understand what was tested and whether it passed without running anything. This is the introspection record — not just "it passed" but "here's exactly what the API returned and why that proves the story."

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
