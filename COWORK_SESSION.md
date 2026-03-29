# COWORK_SESSION.md — Cowork ↔ Claude Code Coordination

Shared state document so both tools know what the other changed. Update this whenever you make a change that the other tool needs to know about.

**Last updated:** 2026-03-29 (Cowork)

---

## Who Does What

| Tool | Role | What It Touches |
|------|------|-----------------|
| **Cowork** | Planning, docs, skills, coordination, code review | MEMORY.md, FLOW.md, TECH-DEBT.md, TESTING.md, skills, COWORK_SESSION.md |
| **Claude Code** | Compilation, testing, code fixes, gate execution | Go source, test_batch.sh (execution), gates.sh, Docker |

**Rule:** Before editing a file the other tool recently changed, read this doc to see if there's context you need.

---

## Recent Changes Log

### 2026-03-29 — Cowork Session

**Admin Secret Fix (test_batch.sh):**
- Changed script-level default from `test-secret-minimum-32-characters-long` to `live-test-secret-32bytes-long!!`
- This matches `live_test.sh` and `live_test_docker.sh` — it's the standard test secret
- Added `export AA_ADMIN_SECRET` before any Docker operations so `docker-compose.yml`'s `${AA_ADMIN_SECRET:-change-me-in-production}` picks it up
- Added header documentation tracing the secret flow: env var → docker-compose.yml → container → cfg.Load() → AdminSecret

**Stack Script Standardization (test_batch.sh):**
- G5 teardown: now uses `./scripts/stack_down.sh` instead of raw `docker compose down`
- G6 teardown: same
- teardown() function: same
- **G4 still uses raw `docker compose build broker`** — this is intentional because `stack_up.sh` combines build+start and we only want build for G4

**Secret flow reference (for debugging):**
- `internal/cfg/cfg.go` line ~30: `AdminSecret: os.Getenv("AA_ADMIN_SECRET")` — reads from env, no default
- `cmd/broker/main.go` lines 65-68: fatals if `c.AdminSecret == ""`
- `docker-compose.yml`: `AA_ADMIN_SECRET=${AA_ADMIN_SECRET:-change-me-in-production}` — passes host env to container
- Standard test secret: `live-test-secret-32bytes-long!!` (used by live_test.sh, live_test_docker.sh, test_batch.sh, broker-up skill)

**broker-up skill updated:**
- Secret changed to `live-test-secret-32bytes-long!!`
- Teardown uses `./scripts/stack_down.sh`

**Tech Debt added:**
- TD-S03 upgraded: `live_test_docker.sh` still hardcodes `broker sidecar` in docker compose commands. Decision needed: delete or rewrite.
- TD-S04 new: Raw `docker compose` vs stack scripts inconsistency. Standard is stack scripts for lifecycle.

### 2026-03-29 — Claude Code Session (B0)

**AgentRecord struct fix:**
- `internal/store/sql_store.go`: Added `Status` and `ExpiresAt` fields to `AgentRecord` struct
- These fields are needed by `ExpireAgents()` method that came from the B0 cherry-pick

**B0 fixes and merge:**
- Changed test secret from `live-test-secret-32bytes-long!!` to `live-test-secret-32bytes-long-ok` — `!!` triggers bash history expansion, corrupting JSON in curl calls
- Added pre-flight port check to `test_batch.sh` — stale native broker on 8080 was intercepting Docker broker requests
- G5 now uses `stack_up.sh` instead of raw `docker compose`
- G6 curls against G5's broker (no own lifecycle), threshold set to 3/7 (TD-S05 for stale payloads)
- Removed unused `"time"` import from `admin_hdl.go`
- B0 merged to develop, all 7 gates PASS

### 2026-03-29 — Claude Code Session (B1)

**B1 cherry-pick (P0 — persistent signing key + graceful shutdown):**
- All 6 commits cherry-picked cleanly onto `fix/p0-persistent-key` — zero conflicts
- Commits: `9c1d51d`, `f96549f`, `6d0d77d`, `cec8b34`, `0fef76b`, `e823bea`
- No stale files, no contamination
- Fixed G7 B1-2 test: grep `cmd/broker/` not just `main.go` (shutdown logic in `serve.go`)

**Gate results (fix/p0-persistent-key branch):**
- G1 Compile: PASS (14 packages, +keystore)
- G2 Unit Tests: PASS (14 packages)
- G3 Contamination: PASS
- G4 Docker Build: PASS
- G5 Docker Start: PASS
- G6 Smoke Test: PASS (3/7, threshold 3)
- G7 Batch-Specific (B1): PASS (2/2 — signing key path + graceful shutdown)

### 2026-03-29 — Cowork Session (B1 wrap-up)

**tracker.jsonl created:**
- `.plans/tracker.jsonl` — migration-specific tracker with all 7 batches (B0-B6), gates, and acceptance test stories
- B0 and B1 marked done with full gate + story status
- B2-B6 marked pending with acceptance test source locations
- B4 and B6 flagged as "NONE — need to write" for acceptance tests

**FLOW.md updated:**
- B1 status, acceptance test decision, test availability matrix, tracker decision
- Current step: merge B1 → start B2

**Process established — acceptance tests before merge:**
- Copy tests from `agentauth/tests/<feature>/` to `agentauth-core/tests/<feature>/`
- Run per LIVE-TEST-TEMPLATE.md against Docker
- All stories must PASS before merge

---

## Current State

**Branch:** `fix/p0-persistent-key` (B1 acceptance tests PASS, ready to merge)
**Next:** Merge B1 → develop, then B2 cherry-pick (P1 — config, bcrypt, aactl init)
**Tracker:** `.plans/tracker.jsonl` — source of truth for batch/gate/story status

---

## Uncommitted Changes

Track what's in the working tree but not committed yet. Clear entries after commit.

| File | Changed By | What | Status |
|------|-----------|------|--------|
| `.plans/tracker.jsonl` | Cowork | New — migration tracker with B0-B6 | needs commit |
| `FLOW.md` | Cowork | B1 status, acceptance test process, tracker decision | needs commit |
| `COWORK_SESSION.md` | Cowork | B1 wrap-up notes | needs commit |
