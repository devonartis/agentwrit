# COWORK_SESSION.md — Cowork ↔ Claude Code Coordination

Shared state document so both tools know what the other changed. Update this whenever you make a change that the other tool needs to know about.

**Last updated:** 2026-03-29 (Cowork — B3 instructions)

---

## Who Does What

| Tool | Role | What It Touches |
|------|------|-----------------|
| **Cowork** | Planning, docs, skills, coordination, code review | MEMORY.md, FLOW.md, TECH-DEBT.md, TESTING.md, skills, COWORK_SESSION.md |
| **Claude Code** | Compilation, testing, code fixes, gate execution | Go source, test_batch.sh (execution), gates.sh, Docker |

**Rule:** Before editing a file the other tool recently changed, read this doc to see if there's context you need.

---

## ACTIVE TASK: B3 Cherry-Pick — SEC-L1

Use `taskcreate` to create task **"B3 Cherry-Pick — SEC-L1"** then `addblock` for each block below. Execute blocks in order.

### Block 1: Read context files
Read `MEMORY.md`, `FLOW.md`, and `.plans/tracker.jsonl`. B0, B1, B2 are merged to develop. B3 is next.

### Block 2: Invoke cherry-pick skill
Run the `cherrypick-devflow` skill. Batch is B3 — 5 commits from the agentauth repo:
- `632b224` — .gitignore adds .env
- `6fa0198` — weak secret denylist in cfg.go (rejects "change-me-in-production" and empty)
- `574d3b9` — bind address defaults to 127.0.0.1, warns on 0.0.0.0 without TLS
- `cd09a34` — HTTP timeouts in serve.go (slowloris prevention)
- `5489679` — TLS 1.2 minimum + AEAD-only ciphers in serve.go

### Block 3: Create branch from develop
```bash
cd /Users/divineartis/proj/agentauth-core
git checkout develop
git pull
git checkout -b fix/sec-l1
```

### Block 4: Cherry-pick the 5 commits
```bash
git cherry-pick 632b224 6fa0198 574d3b9 cd09a34 5489679
```
**If cfg.go conflicts:** Keep ONLY SEC-L1 additions (BindAddress field, weak secret denylist logic). Drop any OIDC/IssuerURL/FederationKeyPath/cloud fields. Use current cfg.go on develop as the base.

**If serve.go conflicts:** These are new functions (buildServer, TLS config). Keep all SEC-L1 additions.

### Block 5: Gate G1 — Compile
```bash
go build ./...
```
Must pass with zero errors.

### Block 6: Gate G2 — Unit tests
```bash
go test ./...
```
Must pass. If new tests fail, check if they reference fields/functions from P2 that don't exist in core — fix.

### Block 7: Gate G3 — Contamination check
```bash
grep -ri "hitl\|approval\|human.in.the.loop" internal/ cmd/ --include="*.go"
```
MUST return nothing. Zero tolerance.

### Block 8: Gate G4-G5 — Docker build and start
```bash
./scripts/stack_up.sh
```
Broker must start and show `127.0.0.1` in the bind address log line.

### Block 9: Gate G6 — Smoke test
```bash
./scripts/test_batch.sh B3
```
If test_batch.sh doesn't have B3-specific logic, add a B3 case that checks:
1. Broker startup log shows bind address `127.0.0.1:8080`
2. Broker rejects startup with `AA_ADMIN_SECRET=change-me-in-production` (exit code 1)
3. Admin auth still works with a valid secret

### Block 10: Copy acceptance tests from legacy repo
```bash
cp -r /Users/divineartis/proj/agentauth/tests/fix-sec-l1/ /Users/divineartis/proj/agentauth-core/tests/sec-l1/
```
Review copied files. Remove any references to OIDC endpoints, HITL, or features not in core. There are 13 stories — adapt any that reference features not in agentauth-core.

### Block 11: Run acceptance tests
Run all 13 stories against Docker. Follow `tests/LIVE-TEST-TEMPLATE.md`. Record evidence in `tests/sec-l1/evidence/`. All 13 must PASS.

### Block 12: Update tracker.jsonl
Add B3 entries to `.plans/tracker.jsonl`:
```jsonl
{"type":"phase","id":"b3","batch":"B3","name":"SEC-L1: Bind Address + TLS + Timeouts + Weak Secret Denylist","status":"done","branch":"fix/sec-l1","commits":5,"files_changed":4}
{"type":"task","phase":"b3","id":"b3-g1","name":"G1 Compile","status":"done"}
{"type":"task","phase":"b3","id":"b3-g2","name":"G2 Unit Tests","status":"done"}
{"type":"task","phase":"b3","id":"b3-g3","name":"G3 Contamination","status":"done"}
{"type":"task","phase":"b3","id":"b3-g4","name":"G4 Docker Build","status":"done"}
{"type":"task","phase":"b3","id":"b3-g5","name":"G5 Docker Start","status":"done"}
{"type":"task","phase":"b3","id":"b3-g6","name":"G6 Smoke Test","status":"done"}
{"type":"task","phase":"b3","id":"b3-g7","name":"G7 Batch-Specific","status":"done"}
```
Then add one story entry per acceptance test story (13 total), all status `done`. Use IDs `b3-s1` through `b3-s13` with names matching the story titles from the copied test files.

### Block 13: Commit all changes
```bash
git add -A
git commit -m "feat(B3): SEC-L1 cherry-pick — bind address, TLS, timeouts, weak secret denylist

Gates G1-G7: PASS
Acceptance tests: 13/13 PASS
Contamination: CLEAN"
```

### Block 14: Update MEMORY.md
Add under the session log:
```markdown
## B3 Complete (2026-03-29)
- SEC-L1 cherry-pick: 5 commits, 4 files changed
- Bind address now defaults to 127.0.0.1 (was 0.0.0.0)
- Weak secret denylist rejects "change-me-in-production" at startup
- HTTP timeouts: Read 15s, ReadHeader 5s, Write 30s, Idle 120s
- TLS 1.2 minimum with AEAD-only cipher suites
- Gates G1-G7: ALL PASS
- Acceptance tests: 13/13 PASS
- Contamination: CLEAN
- Conflicts resolved: [list any cfg.go or serve.go conflicts and how resolved]
- Next: STOP — notify Cowork for merge review before merging to develop
```

### Block 15: Update FLOW.md
Add B3 section:
```markdown
## B3 — SEC-L1: Bind Address + TLS + Timeouts + Weak Secret Denylist — READY FOR MERGE REVIEW
- Branch: fix/sec-l1
- 5 commits, 4 files changed
- Gates G1-G7: ALL PASS
- Acceptance tests: 13/13 PASS
- Contamination: CLEAN
- Waiting on Cowork merge review
- Next: B4 (SEC-L2a: Token Hardening)
```

### Block 16: Commit MEMORY.md and FLOW.md
```bash
git add MEMORY.md FLOW.md
git commit -m "docs: B3 complete — update FLOW.md and MEMORY.md, awaiting merge review"
```

### Block 17: STOP — Notify Cowork
Do NOT merge to develop. Tell the user: "B3 is complete on fix/sec-l1. All gates pass, 13/13 acceptance tests pass. Ready for Cowork merge review."

---

## Recent Changes Log

### 2026-03-29 — Cowork Session (B3 setup)
- B2 merge review completed: PASS — 9/9 stories, 8/10 security findings addressed, 2 deferred as tech debt (TD-S06, TD-S07)
- B3 instructions written to COWORK_SESSION.md using taskcreate/addblock format
- Next: Claude Code executes B3, Cowork reviews before merge

### 2026-03-29 — Claude Code Session (B2 merge)
- B2 merged to develop: fix/p1-admin-secret → develop
- 10 commits, 33 files changed
- Gates G1-G7: ALL PASS
- Acceptance tests: 9/9 PASS + 3 security reviews
- tracker.jsonl, MEMORY.md, FLOW.md updated
- Tech debt added: TD-S06 (rate limiting), TD-S07 (post-migration doc refresh)

### 2026-03-29 — Claude Code Session (B1)
- B1 cherry-pick (P0 — persistent signing key + graceful shutdown)
- All 6 commits cherry-picked cleanly — zero conflicts
- Gates G1-G7: ALL PASS
- B1 merged to develop

### 2026-03-29 — Claude Code Session (B0)
- B0 fixes and merge (sidecar removal)
- Secret changed to `live-test-secret-32bytes-long-ok`
- Port pre-flight check added
- B0 merged to develop

---

## Current State

**Branch:** `develop` (B0+B1+B2 merged)
**Active task:** B3 cherry-pick on `fix/sec-l1` (Claude Code executing)
**Tracker:** `.plans/tracker.jsonl` — source of truth for batch/gate/story status

---

## Uncommitted Changes

Track what's in the working tree but not committed yet. Clear entries after commit.

| File | Changed By | What | Status |
|------|-----------|------|--------|
| `COWORK_SESSION.md` | Cowork | B3 instructions | needs commit by Claude Code |
