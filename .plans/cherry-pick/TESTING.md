# Cherry-Pick Migration — Testing Tracker

Single source of truth for test status across all batches. Update after every test run.

**Last updated:** 2026-03-29
**Operator:** Divine

---

## Test Gate Definitions

Every batch must pass ALL applicable gates before merging to `develop`. A batch is not done until every gate is green.

| Gate | What | Command | When Required |
|------|------|---------|---------------|
| G1: Compile | Code compiles with zero errors | `go build ./...` | Every batch |
| G2: Unit Tests | All unit tests pass | `go test ./...` | Every batch |
| G3: Contamination | Zero HITL/OIDC/sidecar/cloud/federation references in Go code | `grep -rni "hitl\|approval\|oidc\|federation\|thumbprint\|jwk\|sidecar\|cloud" internal/ cmd/ --include="*.go"` | Every batch |
| G4: Docker Build | Docker image builds successfully | `docker compose build broker` | B0+ (once Dockerfile is clean) |
| G5: Docker Start | Broker starts and responds to health check | `docker compose up -d broker && sleep 2 && curl -sf http://localhost:8080/v1/health && docker compose down` | B0+ |
| G6: Smoke Test | Core API flows work (admin auth, launch token, register, app CRUD) | See [Smoke Test Script](#smoke-test-script) below | B0+ |
| G7: Batch-Specific | Feature-specific verification for the batch | See [Batch-Specific Tests](#batch-specific-tests) below | Varies |
| G8: Regression | Previous batch tests still pass after new batch | Re-run prior batch's G7 tests | B1+ (cumulative) |

---

## Batch Status Matrix

| Batch | Branch | G1 Compile | G2 Unit | G3 Contam | G4 Docker Build | G5 Docker Start | G6 Smoke | G7 Batch | G8 Regress | Status |
|-------|--------|------------|---------|-----------|-----------------|-----------------|----------|----------|------------|--------|
| B0: Sidecar Removal | `fix/sidecar-removal` | PENDING | PENDING | PASS | PENDING | PENDING | PENDING | PENDING | n/a | **cherry-picked, needs local test** |
| B1: P0 | — | — | — | — | — | — | — | — | — | not started |
| B2: P1 | — | — | — | — | — | — | — | — | — | not started |
| B3: SEC-L1 | — | — | — | — | — | — | — | — | — | not started |
| B4: SEC-L2a | — | — | — | — | — | — | — | — | — | not started |
| B5: SEC-L2b | — | — | — | — | — | — | — | — | — | not started |
| B6: SEC-A1 + Gates | — | — | — | — | — | — | — | — | — | not started |

---

## How to Run Each Gate

### G1: Compile

```bash
cd /Users/divineartis/proj/agentauth-core
go build ./...
echo $?  # must be 0
```

**Pass criteria:** Exit code 0, no error output.

### G2: Unit Tests

```bash
go test ./... -v 2>&1 | tee /tmp/test-output.txt
echo $?  # must be 0
```

**Pass criteria:** Exit code 0, all tests pass. Save output for evidence.

### G3: Contamination Check

```bash
# Must return NOTHING
grep -rni "hitl\|approval\|oidc\|federation\|thumbprint\|jwk\|sidecar\|cloud" internal/ cmd/ --include="*.go"
echo $?  # must be 1 (no matches)
```

**Pass criteria:** Exit code 1 (grep found nothing). Any match is a FAIL — investigate and remove before proceeding.

**Known false positives (NOT failures):**
- `IssueReq`, `IssueResp`, `issuer` — these are JWT concepts, not OIDC
- `CHANGELOG.md` — historical entries, not code

### G4: Docker Build

```bash
docker compose build broker
echo $?  # must be 0
```

**Pass criteria:** Image builds successfully. If Dockerfile still references `cmd/sidecar`, this will fail — see TD-S01 in TECH-DEBT.md.

### G5: Docker Start

```bash
docker compose up -d broker
sleep 3
curl -sf http://localhost:8080/v1/health | jq .
# Expected: {"status":"ok"} or similar
docker compose down
```

**Pass criteria:** Health endpoint responds with 200 OK.

### G6: Smoke Test Script

```bash
# Prerequisites
export AA_ADMIN_SECRET="test-secret-minimum-32-characters-long"

# 1. Start broker
docker compose up -d broker
sleep 3

# 2. Health check
echo "=== Health Check ==="
curl -sf http://localhost:8080/v1/health | jq .

# 3. Admin auth
echo "=== Admin Auth ==="
ADMIN_TOKEN=$(curl -sf -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AA_ADMIN_SECRET\"}" | jq -r '.access_token')
echo "Admin token: ${ADMIN_TOKEN:0:20}..."

# 4. Create launch token
echo "=== Create Launch Token ==="
LAUNCH_TOKEN=$(curl -sf -X POST http://localhost:8080/v1/admin/launch-tokens \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"allowed_scope":["read:data","write:data"],"max_uses":1}' | jq -r '.token')
echo "Launch token: ${LAUNCH_TOKEN:0:20}..."

# 5. Register agent
echo "=== Register Agent ==="
AGENT_TOKEN=$(curl -sf -X POST http://localhost:8080/v1/register \
  -H "Authorization: Bearer $LAUNCH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sub":"spiffe://agentauth/agent/test-001","scope":["read:data"],"task_id":"task-1","orch_id":"orch-1"}' | jq -r '.access_token')
echo "Agent token: ${AGENT_TOKEN:0:20}..."

# 6. Verify agent token
echo "=== Verify Token ==="
curl -sf -X POST http://localhost:8080/v1/token/verify \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}" | jq .

# 7. Renew agent token
echo "=== Renew Token ==="
curl -sf -X POST http://localhost:8080/v1/token/renew \
  -H "Authorization: Bearer $AGENT_TOKEN" | jq .

# 8. Cleanup
docker compose down
echo "=== Smoke Test Complete ==="
```

**Pass criteria:** All 6 API calls return 200 and produce expected JSON output. Any non-200 is a FAIL.

---

## Batch-Specific Tests

### B0: Sidecar Removal

Sidecar code is gone. Verify nothing references it and core API still works.

| # | Test | Command | Expected | Status |
|---|------|---------|----------|--------|
| B0-1 | No sidecar Go code | `grep -rni "sidecar" internal/ cmd/ --include="*.go"` | Exit 1 (no matches) | PASS (verified in session) |
| B0-2 | `token_exchange_hdl.go` deleted | `ls internal/handler/token_exchange_hdl.go` | "No such file" | PASS (verified in session) |
| B0-3 | `docker-compose.uds.yml` deleted | `ls docker-compose.uds.yml` | "No such file" | PASS (verified in session) |
| B0-4 | Compile clean | `go build ./...` | Exit 0 | PENDING — run locally |
| B0-5 | Unit tests pass | `go test ./...` | Exit 0, all pass | PENDING — run locally |
| B0-6 | Docker image builds | `docker compose build broker` | Exit 0 | PENDING — run locally |
| B0-7 | Broker starts + health | G5 commands | 200 OK | PENDING — run locally |
| B0-8 | Full smoke test | G6 script | All 200s | PENDING — run locally |

### B1: P0 — Production Foundations

Persistent signing key, graceful shutdown, predecessor revocation on renewal.

| # | Test | Command | Expected | Status |
|---|------|---------|----------|--------|
| B1-1 | Key persists across restart | Start broker with `AA_SIGNING_KEY_PATH=/tmp/test.key`, stop, restart — verify same `kid` in token header | Same `kid` value | — |
| B1-2 | Graceful shutdown | Send SIGTERM to broker, verify in-flight requests complete, process exits 0 | Exit 0, no dropped requests | — |
| B1-3 | Predecessor revocation | Issue token, renew it, verify original token fails validation | 401 on old token | — |
| B1-4 | JTI pruning | Verify expired JTI entries are cleaned up (check store internals or logs) | Memory stable | — |

### B2: P1 — Admin Secret & Config

Config file parser, bcrypt admin auth, aactl init workflow.

| # | Test | Command | Expected | Status |
|---|------|---------|----------|--------|
| B2-1 | `aactl init` generates config | `aactl init --output /tmp/agentauth.yaml` | Config file created with bcrypt hash | — |
| B2-2 | Broker rejects startup without hash | Start broker with no `AA_ADMIN_SECRET_HASH` | Error: missing admin secret | — |
| B2-3 | Bcrypt auth works | Auth with plaintext secret against bcrypt hash | 200 + token | — |
| B2-4 | Old plaintext auth rejected | Auth with `{"secret":"..."}` against bcrypt-configured broker | 401 | — |

### B3: SEC-L1 — Security Foundation

Bind address, TLS enforcement, timeouts, weak secret denylist.

| # | Test | Command | Expected | Status |
|---|------|---------|----------|--------|
| B3-1 | Default bind 127.0.0.1 | Start broker, verify listening on 127.0.0.1 only | `netstat` shows 127.0.0.1:8080 | — |
| B3-2 | Weak secret rejected | Start broker with `AA_ADMIN_SECRET=password` | Startup error: weak secret | — |
| B3-3 | TLS enforcement | Set `AA_TLS_CERT_FILE` + `AA_TLS_KEY_FILE`, verify HTTPS only | HTTPS works, HTTP refused | — |
| B3-4 | Request timeout | Send slow request (drip body), verify timeout | 408 or connection reset | — |

### B4: SEC-L2a — Token Hardening

Token alg/kid validation, MaxTTL, revocation hardening.

| # | Test | Command | Expected | Status |
|---|------|---------|----------|--------|
| B4-1 | Token without `exp` rejected | Craft token with no `exp` claim, send to verify | 401 | — |
| B4-2 | MaxTTL clamp | Register app with TTL > MaxTTL, verify token has MaxTTL | Token TTL = MaxTTL | — |
| B4-3 | `alg:none` rejected | Send token with `alg:none` header | 401 | — |
| B4-4 | Revoked token fails | Revoke a token, then try to use it | 401 | — |

### B5: SEC-L2b — HTTP Hardening

Security headers, MaxBytesBody, error sanitization.

| # | Test | Command | Expected | Status |
|---|------|---------|----------|--------|
| B5-1 | Security headers present | `curl -I http://localhost:8080/v1/health` | HSTS, X-Content-Type-Options, X-Frame-Options | — |
| B5-2 | Body size limit | Send >1MB POST body | 413 Payload Too Large | — |
| B5-3 | Error sanitization | Send malformed JSON, verify no stack trace in response | Clean error, no internals leaked | — |

### B6: SEC-A1 + Gates

TTL bypass fix, gates regression.

| # | Test | Command | Expected | Status |
|---|------|---------|----------|--------|
| B6-1 | Renewal preserves TTL | Issue token with 120s TTL, renew, verify new token has 120s | Same TTL | — |
| B6-2 | Gates pass | `./scripts/gates.sh task` | Exit 0 | — |

---

## Evidence Log

Record test results here after each run. Include date, batch, gate, result, and any notes.

| Date | Batch | Gate | Result | Notes |
|------|-------|------|--------|-------|
| 2026-03-29 | B0 | G3 | PASS | Zero sidecar/hitl/oidc refs in Go code. Verified in Cowork session. |
| 2026-03-29 | B0 | B0-1 | PASS | `grep -rni sidecar internal/ cmd/ --include="*.go"` returns nothing |
| 2026-03-29 | B0 | B0-2 | PASS | `token_exchange_hdl.go` confirmed deleted |
| 2026-03-29 | B0 | B0-3 | PASS | `docker-compose.uds.yml` confirmed deleted |
| | B0 | G1 | PENDING | Needs local run — sandbox has no Go |
| | B0 | G2 | PENDING | Needs local run |
| | B0 | G4 | PENDING | Needs local run |
| | B0 | G5 | PENDING | Needs local run |
| | B0 | G6 | PENDING | Needs local run |

---

## Blocked Items

Track anything that blocks a test from running.

| Batch | Gate | Blocker | Resolution |
|-------|------|---------|------------|
| B0 | G4 | Dockerfile may still reference `cmd/sidecar` build stage — cherry-pick modified it but needs verification | Check `Dockerfile` for `cmd/sidecar` references after cherry-pick |
| B0 | G5/G6 | Docker compose files may need env var updates post-sidecar-removal | Compare to agentauth's clean compose files |
| B0 | G6 | Smoke test script assumes specific env var names — may change per batch | Update smoke script after each batch if env vars change |

---

## Notes for Slash Command Conversion

This doc is designed to be turned into a `/test-migration` slash command. The command should:

1. Accept a batch name (e.g., `/test-migration B0`)
2. Run all applicable gates for that batch
3. Record results in the evidence log
4. Update the batch status matrix
5. Report pass/fail summary

The gate commands are all shell one-liners or short scripts — they translate directly to skill steps.

| 2026-03-29 | B0 | G1:Compile | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G2:UnitTests | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G3:Contamination | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G7:BatchSpecific(B0) | PASS | branch=fix/sidecar-removal |

| 2026-03-29 | B0 | G1:Compile | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G2:UnitTests | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G3:Contamination | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G4:DockerBuild | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G5:DockerStart | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G6:SmokeTest | FAIL | 6 of 7 checks failed |
| 2026-03-29 | B0 | G7:BatchSpecific(B0) | PASS | branch=fix/sidecar-removal |

| 2026-03-29 | B0 | G1:Compile | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G2:UnitTests | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G3:Contamination | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G4:DockerBuild | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G5:DockerStart | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G6:SmokeTest | FAIL | 6 of 7 checks failed |
| 2026-03-29 | B0 | G7:BatchSpecific(B0) | PASS | branch=fix/sidecar-removal |

| 2026-03-29 | B0 | G1:Compile | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G2:UnitTests | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G3:Contamination | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G4:DockerBuild | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G5:DockerStart | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G6:SmokeTest | FAIL | 6 of 7 checks failed |
| 2026-03-29 | B0 | G7:BatchSpecific(B0) | PASS | branch=fix/sidecar-removal |

| 2026-03-29 | B0 | G1:Compile | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G2:UnitTests | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G3:Contamination | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G4:DockerBuild | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G5:DockerStart | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G6:SmokeTest | FAIL | 6 of 7 checks failed |
| 2026-03-29 | B0 | G7:BatchSpecific(B0) | PASS | branch=fix/sidecar-removal |

| 2026-03-29 | B0 | G1:Compile | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G2:UnitTests | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G3:Contamination | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G4:DockerBuild | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G5:DockerStart | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G6:SmokeTest | FAIL | 4 of 7 checks failed |
| 2026-03-29 | B0 | G7:BatchSpecific(B0) | PASS | branch=fix/sidecar-removal |

| 2026-03-29 | B0 | G1:Compile | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G2:UnitTests | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G3:Contamination | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G4:DockerBuild | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G5:DockerStart | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G6:SmokeTest | PASS | branch=fix/sidecar-removal |
| 2026-03-29 | B0 | G7:BatchSpecific(B0) | PASS | branch=fix/sidecar-removal |

| 2026-03-29 | B1 | G1:Compile | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G2:UnitTests | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G3:Contamination | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G4:DockerBuild | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G5:DockerStart | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G6:SmokeTest | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G7:BatchSpecific(B1) | FAIL | 1 checks failed |

| 2026-03-29 | B1 | G1:Compile | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G2:UnitTests | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G3:Contamination | PASS | branch=fix/p0-persistent-key |

| 2026-03-29 | B1 | G1:Compile | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G2:UnitTests | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G3:Contamination | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G4:DockerBuild | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G5:DockerStart | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G6:SmokeTest | PASS | branch=fix/p0-persistent-key |
| 2026-03-29 | B1 | G7:BatchSpecific(B1) | PASS | branch=fix/p0-persistent-key |

| 2026-03-29 | B2 | G1:Compile | PASS | branch=fix/p1-admin-secret |
| 2026-03-29 | B2 | G2:UnitTests | PASS | branch=fix/p1-admin-secret |
| 2026-03-29 | B2 | G3:Contamination | PASS | branch=fix/p1-admin-secret |
| 2026-03-29 | B2 | G4:DockerBuild | PASS | branch=fix/p1-admin-secret |
| 2026-03-29 | B2 | G5:DockerStart | PASS | branch=fix/p1-admin-secret |
| 2026-03-29 | B2 | G6:SmokeTest | PASS | branch=fix/p1-admin-secret |
| 2026-03-29 | B2 | G7:BatchSpecific(B2) | PASS | branch=fix/p1-admin-secret |

| 2026-03-29 | B3 | G1:Compile | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G2:UnitTests | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G3:Contamination | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G4:DockerBuild | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G5:DockerStart | FAIL | health check failed after 15s |
| 2026-03-29 | B3 | G6:SmokeTest | FAIL | broker not reachable at http://127.0.0.1:8080 (did G5 run?) |

| 2026-03-29 | B3 | G1:Compile | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G2:UnitTests | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G3:Contamination | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G4:DockerBuild | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G5:DockerStart | PASS | branch=fix/sec-l1 |
| 2026-03-29 | B3 | G6:SmokeTest | PASS | branch=fix/sec-l1 |

| 2026-03-29 | B4 | G1:Compile | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G2:UnitTests | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G3:Contamination | FAIL | add-on code detected in Go sources |
| 2026-03-29 | B4 | G4:DockerBuild | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G5:DockerStart | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G6:SmokeTest | PASS | branch=fix/sec-l2a |

| 2026-03-29 | B4 | G1:Compile | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G2:UnitTests | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G3:Contamination | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G4:DockerBuild | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G5:DockerStart | PASS | branch=fix/sec-l2a |
| 2026-03-29 | B4 | G6:SmokeTest | PASS | branch=fix/sec-l2a |

| 2026-03-30 | B5 | G1:Compile | PASS | branch=fix/sec-l2b |
| 2026-03-30 | B5 | G2:UnitTests | PASS | branch=fix/sec-l2b |
| 2026-03-30 | B5 | G3:Contamination | PASS | branch=fix/sec-l2b |
