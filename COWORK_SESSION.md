# COWORK_SESSION.md — Cowork ↔ Claude Code Coordination

Shared state document so both tools know what the other changed. Update this whenever you make a change that the other tool needs to know about.

**Last updated:** 2026-03-29 (Cowork — B4 instructions)

---

## Who Does What

| Tool | Role | What It Touches |
|------|------|-----------------|
| **Cowork** | Planning, docs, skills, coordination, code review | MEMORY.md, FLOW.md, TECH-DEBT.md, TESTING.md, skills, COWORK_SESSION.md |
| **Claude Code** | Compilation, testing, code fixes, gate execution | Go source, test_batch.sh (execution), gates.sh, Docker |

**Rule:** Before editing a file the other tool recently changed, read this doc to see if there's context you need.

---

## ACTIVE TASK: B4 Cherry-Pick — SEC-L2a: Token Hardening

Use `taskcreate` to create task **"B4 Cherry-Pick — SEC-L2a Token Hardening"** then `addblock` for each block below. Execute blocks in order.

**IMPORTANT: B4 has NO existing acceptance tests. You must write them. Follow `tests/LIVE-TEST-TEMPLATE.md` exactly. Put them in `tests/sec-l2a/` with user-stories.md, env.sh, and evidence/ — same structure as `tests/p1-admin-secret/` and `tests/sec-l1/`.**

### Block 1: Read context files
Read `MEMORY.md`, `FLOW.md`, `.plans/tracker.jsonl`, and `tests/LIVE-TEST-TEMPLATE.md`. B0-B3 are merged to develop. B4 is next. B4 has HIGH conflict risk — cherry-pick one commit at a time.

### Block 2: Seed tracker.jsonl with B4 work items (all pending)
Add ALL B4 entries to `.plans/tracker.jsonl` NOW with status `pending`. This is how the agent always knows what's been done and what hasn't. Mark each one `done` as you complete it throughout the remaining blocks.
```jsonl
{"type":"phase","id":"b4","batch":"B4","name":"SEC-L2a: Token Hardening","status":"pending","branch":"fix/sec-l2a","commits":8}
{"type":"task","phase":"b4","id":"b4-g1","name":"G1 Compile","status":"pending"}
{"type":"task","phase":"b4","id":"b4-g2","name":"G2 Unit Tests","status":"pending"}
{"type":"task","phase":"b4","id":"b4-g3","name":"G3 Contamination","status":"pending"}
{"type":"task","phase":"b4","id":"b4-g4","name":"G4 Docker Build","status":"pending"}
{"type":"task","phase":"b4","id":"b4-g5","name":"G5 Docker Start","status":"pending"}
{"type":"task","phase":"b4","id":"b4-g6","name":"G6 Smoke Test","status":"pending"}
{"type":"task","phase":"b4","id":"b4-g7","name":"G7 Batch-Specific","status":"pending"}
{"type":"story","phase":"b4","id":"b4-s1","name":"L2a-S1: Valid EdDSA token accepted (baseline)","status":"pending"}
{"type":"story","phase":"b4","id":"b4-s2","name":"L2a-S2: MaxTTL clamps token lifetime","status":"pending"}
{"type":"story","phase":"b4","id":"b4-s3","name":"L2a-S3: MaxTTL=0 disables ceiling","status":"pending"}
{"type":"story","phase":"b4","id":"b4-s4","name":"L2a-S4: Revoked token rejected after revocation","status":"pending"}
{"type":"story","phase":"b4","id":"b4-s5","name":"L2a-S5: Token renewal works, old token revoked","status":"pending"}
{"type":"story","phase":"b4","id":"b4-s6","name":"L2a-S6: Broker warns DefaultTTL > MaxTTL","status":"pending"}
{"type":"story","phase":"b4","id":"b4-s7","name":"L2a-S7: Empty kid accepted (backward compat)","status":"pending"}
{"type":"story","phase":"b4","id":"b4-n1","name":"L2a-N1: Token with alg=HS256 rejected","status":"pending"}
{"type":"story","phase":"b4","id":"b4-n2","name":"L2a-N2: Token with mismatched kid rejected","status":"pending"}
{"type":"story","phase":"b4","id":"b4-n3","name":"L2a-N3: Token with exp=0 rejected","status":"pending"}
{"type":"story","phase":"b4","id":"b4-n4","name":"L2a-N4: Wrong admin secret rejected (regression)","status":"pending"}
{"type":"story","phase":"b4","id":"b4-n5","name":"L2a-N5: Renewal failure on revocation error (unit test)","status":"pending"}
{"type":"story","phase":"b4","id":"b4-sec1","name":"L2a-SEC1: Code review — Verify() check order","status":"pending"}
```
**Rule: As you complete each gate or story in later blocks, update its status to `done` in tracker.jsonl immediately.** Don't wait until the end.

### Block 3: Invoke cherry-pick skill
Run the `cherrypick-devflow` skill. Batch is B4 — 8 commits from the agentauth repo.

### Block 4: Create branch from develop
```bash
cd /Users/divineartis/proj/agentauth-core
git checkout develop
git pull
git checkout -b fix/sec-l2a
```

### Block 5: Cherry-pick commits ONE AT A TIME
Cherry-pick each commit individually. Do NOT batch them — 5 of 8 modify tkn_svc.go.

```bash
git cherry-pick 8e63989
```
**Commit 1: MaxTTL config field.** Adds `AA_MAX_TTL` env var (default 86400 = 24h) to `cfg.go`. Adds `MaxTTL int` to Cfg struct and `envIntOr("AA_MAX_TTL", 86400)` in Load(). If cfg.go conflicts, keep MaxTTL field and env parsing, drop any OIDC/IssuerURL/FederationKeyPath fields.

```bash
git cherry-pick 0526c46
```
**Commit 2: Validate JWT alg and kid headers.** Adds alg=EdDSA validation and kid matching to `Verify()` in `tkn_svc.go`. Rejects tokens with wrong algorithm or mismatched kid. Empty kid is allowed (backward compat). If tkn_svc.go conflicts, keep the entire alg/kid validation block.

```bash
git cherry-pick c24e442
```
**Commit 3: Clamp token TTL to MaxTTL.** In `Issue()`, if `MaxTTL > 0 && ttl > MaxTTL`, clamp ttl to MaxTTL. Small focused change.

```bash
git cherry-pick 67aeda7
```
**Commit 4: Move revocation check into Verify.** Adds `IsRevoked(*TknClaims) bool` to `Revoker` interface in `revoker.go`. Adds revocation check in `Verify()` after claims validation. Adds `ErrTokenRevoked`. The Revoker interface change means all implementations must add `IsRevoked()`.

```bash
git cherry-pick b78edb8
```
**Commit 5: Fail renewal when predecessor revocation fails.** Changes `Renew()` from `_ = s.revoker.RevokeByJTI(...)` (ignoring error) to returning error if revocation fails. Small change.

```bash
git cherry-pick ecb4c86
```
**Commit 6: Require non-nil RevocationStore in NewRevSvc.** `NewRevSvc(nil)` now panics. Updates handler_test.go, release_hdl_test.go, heartbeat_test.go to pass `nopRevStore{}` instead of nil. HIGH CONFLICT RISK in test files — B0-B3 modified these files. Add `nopRevStore` struct implementing `SaveRevocation(_, _ string) error { return nil }` in each test file that needs it.

```bash
git cherry-pick 078a674
```
**Commit 7: Reject tokens with no expiry.** Changes `tkn_claims.go` `Validate()` from `if c.Exp != 0 && now > c.Exp` to `if c.Exp <= 0 { return ErrNoExpiry }` then `if now > c.Exp { return ErrTokenExpired }`. Small, safe change.

```bash
git cherry-pick 8366fa9
```
**Commit 8: Warning logs.** Adds `obs.Warn` in `cfg.go` if `DefaultTTL > MaxTTL`. Adds `obs.Warn` in `tkn_svc.go` on kid mismatch before returning error.

### Block 6: Gate G1 — Compile (mark b4-g1 done when pass)
```bash
go build ./...
```
Must pass with zero errors. If `IsRevoked` is missing from the RevSvc implementation, add it to `internal/revoke/rev_svc.go`:
```go
func (s *RevSvc) IsRevoked(claims *token.TknClaims) bool {
    // Check if this token's JTI has been revoked
    return s.store.IsRevoked(claims.Jti)
}
```
Also check if the `RevocationStore` interface needs `IsRevoked(jti string) bool` added.

### Block 7: Gate G2 — Unit tests (mark b4-g2 done when pass)
```bash
go test ./...
```
Must pass. Common issues:
- Test files calling `NewRevSvc(nil)` — change to `NewRevSvc(nopRevStore{})`
- Missing `nopRevStore` struct — add it with `SaveRevocation` and `IsRevoked` methods
- Missing mock methods on `mockRevoker` — add `IsRevoked` method

### Block 8: Gate G3 — Contamination check (mark b4-g3 done when pass)
```bash
grep -ri "hitl\|approval\|human.in.the.loop\|oidc\|federation\|issuer.url" internal/ cmd/ --include="*.go"
```
MUST return nothing. Zero tolerance.

### Block 9: Gate G4-G5 — Docker build and start (mark b4-g4 and b4-g5 done when pass)
```bash
./scripts/stack_up.sh
```
Broker must start successfully.

### Block 10: Gate G6 — Smoke test (mark b4-g6 done when pass)
```bash
./scripts/test_batch.sh B4
```
If test_batch.sh doesn't have B4-specific logic, add a B4 case that checks:
1. Admin auth still works (POST /v1/admin/auth)
2. Broker starts with AA_MAX_TTL=3600 without error
3. Warning log appears if AA_DEFAULT_TTL > AA_MAX_TTL

### Block 11: Gate G7 — Batch-specific checks (mark b4-g7 done when pass)
Verify these in the code:
1. `tkn_svc.go` `Verify()` has alg validation (checks `hdr.Alg == "EdDSA"`)
2. `tkn_svc.go` `Verify()` has kid validation (checks `hdr.Kid == "" || hdr.Kid == s.kid`)
3. `tkn_svc.go` `Verify()` has revocation check (`s.revoker.IsRevoked`)
4. `tkn_svc.go` `Issue()` has MaxTTL clamp
5. `tkn_svc.go` `Renew()` returns error on revocation failure
6. `tkn_claims.go` `Validate()` rejects `exp <= 0`
7. `rev_svc.go` `NewRevSvc(nil)` panics
8. `cfg.go` has MaxTTL field with 86400 default

### Block 12: Write acceptance tests
Create `tests/sec-l2a/` (user-stories.md, env.sh, evidence/) following `tests/LIVE-TEST-TEMPLATE.md` and the same structure as `tests/sec-l1/` and `tests/p1-admin-secret/`. Write BOTH positive and negative stories for each of the 7 hardenings in B4. Include a security review story for the Verify() check order. Use JWT tampering (base64url decode/modify/re-encode) for alg, kid, and exp negative tests.

### Block 13: Run acceptance tests (mark each story done in tracker as it passes)
Run all 13 stories (7 positive + 5 negative + 1 security review) one at a time against VPS first, then Container. Follow `tests/LIVE-TEST-TEMPLATE.md` exactly — banner first, command pipes output to evidence file, then verdict. Record evidence in `tests/sec-l2a/evidence/`.

All 13 must PASS. If a story cannot be tested at the live level (like L2a-N5 revocation failure), verify the unit test passes and document it in the evidence.

For stories requiring JWT tampering (S1, S2, S4), use this approach:
1. Get a valid token from admin auth
2. Split the JWT on dots: `header.payload.signature`
3. Base64url decode the header, modify alg/kid/exp, base64url re-encode
4. Reconstruct the JWT with the tampered header
5. Present to an authenticated endpoint

### Block 14: Create evidence README
Create `tests/sec-l2a/evidence/README.md` with the summary table of all 13 stories (7 positive, 5 negative, 1 security review), verdicts, and open issues.

### Block 15: Verify tracker — all B4 items should be done
Check `.plans/tracker.jsonl`. Every b4-* entry should be `done`. If any are still `pending`, something was missed — go back and complete it.

### Block 16: Commit all changes
```bash
git add -A
git commit -m "feat(B4): SEC-L2a cherry-pick — token hardening

8 commits: alg/kid validation, MaxTTL ceiling, revocation in Verify,
renewal failure on revocation error, non-nil RevSvc, no-expiry rejection,
warning logs.

Gates G1-G7: PASS
Acceptance tests: 13/13 PASS
Contamination: CLEAN"
```

### Block 17: Update MEMORY.md
Add under the session log:
```markdown
## B4 Complete (2026-03-29)
- SEC-L2a token hardening: 8 commits cherry-picked
- Verify() now validates: alg=EdDSA, kid match, revocation status, exp > 0
- Issue() clamps TTL to MaxTTL ceiling (AA_MAX_TTL, default 86400)
- Renew() fails if predecessor revocation fails (was silent skip)
- NewRevSvc(nil) now panics — all callers must pass valid store
- ErrTokenRevoked and ErrNoExpiry added
- Acceptance tests: 7/7 PASS (wrote from scratch — no tests existed in agentauth)
- Contamination: CLEAN
- Conflicts resolved: [list any conflicts and how resolved]
- Next: STOP — notify Cowork for merge review before merging to develop
```

### Block 18: Update FLOW.md
Add B4 section:
```markdown
## B4 — SEC-L2a: Token Hardening — READY FOR MERGE REVIEW
- Branch: fix/sec-l2a
- 8 commits
- Gates G1-G7: ALL PASS
- Acceptance tests: 7/7 PASS (NEW — written from scratch)
- Contamination: CLEAN
- Waiting on Cowork merge review
- Next: B5 (SEC-L2b: HTTP Hardening)
```

### Block 19: Commit MEMORY.md and FLOW.md
```bash
git add MEMORY.md FLOW.md
git commit -m "docs: B4 complete — update FLOW.md and MEMORY.md, awaiting merge review"
```

### Block 20: STOP — Notify Cowork
Do NOT merge to develop. Tell the user: "B4 is complete on fix/sec-l2a. All gates pass, 7/7 acceptance tests pass. Ready for Cowork merge review."

---

## Recent Changes Log

### 2026-03-29 — Cowork Session (B4 setup)
- B3 merge review completed: PASS — 12/12 stories, contamination clean
- B4 instructions written with 7 NEW acceptance test stories (none existed in agentauth)
- B4 has HIGH conflict risk — cherry-pick one commit at a time
- Tech debt TD-S08 through TD-S15 reviewed and acknowledged

### 2026-03-29 — Claude Code Session (B3 merge)
- B3 merged to develop: fix/sec-l1 → develop
- 5 commits cherry-picked, 12/12 acceptance tests PASS
- Docker bind address fix: AA_BIND_ADDRESS=0.0.0.0 in docker-compose.yml
- Tech debt TD-S08 through TD-S15 added

### 2026-03-29 — Claude Code Session (B2 merge)
- B2 merged to develop: fix/p1-admin-secret → develop
- 10 commits, 9/9 PASS + 3 security reviews
- Tech debt: TD-S06 (rate limiting), TD-S07 (post-migration doc refresh)

---

## Current State

**Branch:** `develop` (B0+B1+B2+B3 merged)
**Active task:** B4 cherry-pick on `fix/sec-l2a` (Claude Code executing)
**Tracker:** `.plans/tracker.jsonl` — source of truth for batch/gate/story status

---

## Uncommitted Changes

| File | Changed By | What | Status |
|------|-----------|------|--------|
| `COWORK_SESSION.md` | Cowork | B4 instructions | needs commit by Claude Code |
