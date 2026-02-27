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

**Every endpoint must have aactl operator tooling — no raw curl in tests.** (established 2026-02-25, Session 12)
- If an endpoint is operator-facing, add an `aactl` command for it as part of the fix
- Docker live tests should use `aactl` commands, not hand-crafted curl chains
- Raw curl is only acceptable for truly public/unauthenticated endpoints (health, challenge)
- An endpoint without tooling is not shippable (same lesson as Session 3 with list-sidecars)

**Docker live test process — every fix/feature.** (established 2026-02-25, Session 10)
1. `./scripts/stack_up.sh` — bring up the stack
2. `curl http://127.0.0.1:8080/v1/health` — verify broker is healthy
3. Run user story commands against the running stack (admin auth, the fix-specific operations, restarts, SQLite checks, etc.)
4. Verify each story passes on the running stack
5. `docker compose down -v` — tear down
- Do NOT use `live_test_docker.sh` for manual testing — it creates its own stack and conflicts
- Design the test BEFORE implementation: read user stories, understand constraints, then code
- The test is part of the fix, not a separate task to defer

**Docker live test evidence — save to `tests/<fix-name>-evidence/`.** (established 2026-02-26, Session 16)
- Every Docker live test must produce a `tests/<fix-name>-evidence/` folder
- Folder contains: `README.md` (overview + story table), `story-N-<name>.md` per story (plain English, reproduction steps, raw output, what to look for, verdict), `smoketest-output.txt`
- Anyone should be able to open the evidence folder and understand what was tested and whether it passed without running anything
- This is not optional — a live test without saved evidence is incomplete

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

### What's next
- All 6 P1 compliance fixes are on `develop`. Ready for release preparation.

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
