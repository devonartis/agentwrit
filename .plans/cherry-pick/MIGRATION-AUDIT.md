# Migration Audit: What's Broken at the Fork Point

**Date:** 2026-03-29
**Fork point:** `2c5194e` (agentauth-internal, TD-006)
**Problem:** The fork point carries dead sidecar code that was removed in agentauth but not in agentauth-internal.

---

## The Sidecar Problem

The agentauth-internal repo at `2c5194e` still has the full sidecar subsystem. It was removed in the agentauth repo by commits `34bb887` and `909a777`, which happened after the squashed initial release. Since we cloned from agentauth-internal (not agentauth), we inherited all the sidecar code.

### Scope of the Problem

**457 sidecar references in Go code** across these files:

| File | What's There |
|------|-------------|
| `internal/handler/token_exchange_hdl.go` | Entire file is sidecar token exchange — DELETE |
| `internal/admin/admin_hdl.go` | Sidecar activation endpoints, sidecar routes — needs sidecar code stripped |
| `internal/admin/admin_svc.go` | Sidecar activation service logic — needs sidecar code stripped |
| `internal/store/sql_store.go` | Sidecar tables, ceiling CRUD, activation CRUD — needs sidecar code stripped |
| `internal/token/tkn_svc.go` | `SidecarID` field in request/response structs — needs field removed |
| `internal/token/tkn_claims.go` | `SidecarID` field in JWT claims — needs field removed |
| `internal/handler/renew_hdl.go` | Sidecar ceiling lookup on renewal — needs sidecar branch removed |
| `internal/obs/obs.go` | Sidecar metrics counters — needs removed |
| `internal/audit/audit_log.go` | Sidecar audit event types — needs removed |
| Tests (multiple) | Sidecar test cases — need removed |

**Infrastructure files with sidecar references:**

| File | Problem |
|------|---------|
| `Dockerfile` | Builds `cmd/sidecar` binary (Stage 3) — won't build since we deleted `cmd/sidecar/` |
| `docker-compose.uds.yml` | Has sidecar UDS service definition |
| `docker-compose.mtls.yml` | Has sidecar mTLS config |
| `docker-compose.tls.yml` | Has sidecar TLS config |
| `scripts/live_test.sh` | Builds cmd/smoketest (deleted), tests sidecar flows |
| `scripts/live_test_docker.sh` | Tests sidecar activation, token exchange |
| `scripts/gates.sh` | References live_test_sidecar.sh |
| `scripts/gen_test_certs.sh` | Generates sidecar client certs |
| `scripts/stack_down.sh` | References sidecar service |
| `scripts/verify_compose.sh` | Checks for sidecar service in compose |
| `docs/api/openapi.yaml` | 51 sidecar endpoint references |

### The Fix: Cherry-Pick Sidecar Removal FIRST

Commits `34bb887` and `909a777` from the agentauth repo properly remove all sidecar code. These need to be the FIRST cherry-picks, before P0/P1/SEC.

```
34bb887  feat(broker): remove sidecar subsystem entirely     (29 files, -2395 lines)
909a777  fix(cleanup): remove final sidecar references       (2 files, -3 lines)
```

### Updated Batch Order

| Batch | What | Commits |
|-------|------|---------|
| **B0: Sidecar Removal** | Remove sidecar subsystem | `34bb887` `909a777` |
| B1: P0 | Persistent key, graceful shutdown | `9c1d51d` `f96549f` `6d0d77d` `cec8b34` `0fef76b` `e823bea` |
| B2: P1 | Config, bcrypt, aactl init | `313aa41` `869a8f7` `58cbce2` `4978ecd` `866cc78` `3dfada7` `ebc4884` `1c5f293` |
| B3: SEC-L1 | Bind address, TLS, timeouts | `632b224` `6fa0198` `574d3b9` `cd09a34` `5489679` |
| B4: SEC-L2a | Token hardening | `8e63989` `0526c46` `c24e442` `67aeda7` `b78edb8` `ecb4c86` `078a674` `8366fa9` |
| B5: SEC-L2b | HTTP hardening | `daf2995` `e592acc` `2857b3a` `247727c` `c5da6c4` |
| B6: SEC-A1 + Gates | TTL fix, gates | `9422e7c` `e395a15` |

### B0 Conflict Expectations

HIGH. `34bb887` was written against the agentauth repo's squashed initial release (`22fb430`), not against agentauth-internal's `2c5194e`. The files will differ in structure. Key conflicts expected in:

- `internal/admin/admin_hdl.go` — agentauth-internal may have different function ordering or additional code from Phase 1A/1B
- `internal/admin/admin_svc.go` — same issue
- `internal/store/sql_store.go` — internal has app tables from Phase 1A/1B/TD-006 that agentauth's squashed release also had
- `cmd/broker/main.go` — route wiring may differ

The resolution approach: read the agentauth repo's CLEAN versions of these files (post-sidecar-removal) and use them as the reference for what the final state should look like. Then manually ensure the agentauth-internal's extra code (Phase 1A/1B/TD-006 additions) is preserved while sidecar code is removed.

### Test Infrastructure That Needs Updating

After sidecar removal, these scripts/files also need to be updated or rewritten:

| File | Action |
|------|--------|
| `Dockerfile` | Remove Stage 3 (sidecar image), remove sidecar build step |
| `docker-compose.uds.yml` | Remove sidecar service, keep broker-only UDS config |
| `docker-compose.mtls.yml` | Remove sidecar config |
| `docker-compose.tls.yml` | Remove sidecar config |
| `scripts/live_test.sh` | Rewrite — remove smoketest dependency, test broker directly with curl |
| `scripts/live_test_docker.sh` | Rewrite — remove all sidecar tests, test broker API directly |
| `scripts/verify_compose.sh` | Remove sidecar check |
| `scripts/gen_test_certs.sh` | Remove sidecar cert generation |
| `scripts/stack_down.sh` | Remove sidecar reference |
| `docs/api/openapi.yaml` | Remove sidecar endpoints |

**The agentauth repo already has clean versions of most of these.** After cherry-picking the sidecar removal, compare each file to its agentauth counterpart and bring over the clean version.

---

## How to Verify Each Batch Actually Works

After each batch, beyond `go build` and `go test`:

### Docker Build Test

```bash
docker compose build broker
```

This is the minimum — if the Docker image won't build, nothing else matters. Must pass after B0.

### Docker Start Test

```bash
docker compose up -d broker
sleep 2
curl -s http://localhost:8080/v1/health | jq .
docker compose down
```

Broker should start and respond to health checks. Must pass after B0.

### API Smoke Test (post-B0)

```bash
# Start broker
docker compose up -d broker
sleep 2

# Health check
curl -s http://localhost:8080/v1/health

# Register an agent
curl -s -X POST http://localhost:8080/v1/register \
  -H "Authorization: Bearer $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"orchestrator_id":"orch-1","task_id":"task-1","scope":["read:data"]}'

# Register an app
curl -s -X POST http://localhost:8080/v1/apps \
  -H "Authorization: Bearer $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-app","scope_ceiling":["read:data"]}'

docker compose down
```

### Batch-Specific Tests

| After | Test |
|-------|------|
| B0 (sidecar removal) | Docker builds, broker starts, health responds, basic register + app create work |
| B1 (P0) | Broker starts with `AA_SIGNING_KEY_PATH`, survives SIGTERM gracefully, key persists across restart |
| B2 (P1) | `aactl init` generates config, broker rejects startup without AdminSecretHash, bcrypt auth works |
| B3 (SEC-L1) | Broker binds 127.0.0.1 by default, rejects weak secrets, TLS enforcement when configured |
| B4 (SEC-L2a) | Token without expiry rejected, MaxTTL clamp works, revoked token cannot validate |
| B5 (SEC-L2b) | Response headers include HSTS/X-Content-Type-Options, 1MB body limit enforced, error responses don't leak internals |
| B6 (SEC-A1) | Renewal preserves original TTL, gates.sh regression runs |
