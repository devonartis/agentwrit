# Design Solution + Implementation Plan: Full Compliance and Sidecar Sprawl Fix

**Date:** 2026-02-20
**Authors:** security-architect, system-designer, code-planner, integration-lead, devils-advocate
**Branch:** develop (all code evidence verified directly from develop branch via `git show`)
**Status:** DRAFT — pending devil's advocate approval

---

## Executive Summary

Four independent compliance reviewers (India, Juliet, Kilo, Lima) evaluated the AgentAuth develop branch against the Ephemeral Agent Credentialing Security Pattern v1.2. The codebase achieved 92-96% compliance with zero NOT COMPLIANT findings. This plan addresses every partial compliance item, one additional gap the reviewers all missed (audience validation), and the sidecar port sprawl operational problem.

**Six fixes, all independently implementable:**

| # | Fix | Compliance Gap | Priority | Scope |
|---|-----|---------------|----------|-------|
| 1 | Native TLS/mTLS in broker | 3.3 mTLS (all 4 reviewers) | P0 | Medium |
| 2 | Revocation persistence to SQLite | Revocations lost on restart (real security gap) | P0 | Small |
| 3 | Audience validation | `Aud` field never set or checked (missed by all reviewers) | P1 | Small |
| 4 | Token release endpoint | 4.4 task-completion signal (Juliet) | P1 | Small |
| 5 | Sidecar UDS listen mode | Sidecar port sprawl (N apps = N ports) | P1 | Small |
| 6 | Structured audit log fields | 5.2 free-form Detail field (Kilo) | P2 | Large |

---

## Part 1: Compliance Gap Analysis

### Findings Consolidated Across All Four Reviewers

| Finding | India | Juliet | Kilo | Lima | Consensus |
|---------|-------|--------|------|------|-----------|
| No native TLS/mTLS (3.3) | PARTIAL | PARTIAL | PARTIAL | PARTIAL | All 4 |
| No task-completion signal (4.4) | -- | PARTIAL | -- | -- | Juliet only |
| Anomaly detection heartbeat-only (4.3/4.5) | -- | PARTIAL | -- | PARTIAL | 2 reviewers; pattern says optional |
| Revocation propagation single-instance (4.4) | -- | -- | PARTIAL | -- | Kilo only |
| Audit Detail is free-form (5.2) | -- | -- | PARTIAL | -- | Kilo only |

### Code Evidence for Each Finding (Verified on Develop Branch)

**mTLS gap:** `cmd/broker/main.go:174` — `http.ListenAndServe(addr, rootHandler)`. No TLS config exists anywhere in the broker binary. Confirmed: no `tls.Config`, no `ListenAndServeTLS`, no certificate loading.

**Revocation persistence gap:** `internal/revoke/rev_svc.go` — `RevSvc` holds four pure in-memory maps (`tokens`, `agents`, `tasks`, `chains`). `NewRevSvc()` returns empty maps. No store field, no write-through persistence, no load-on-startup. Broker restart silently clears all revocations. A revoked compromised agent can re-use its token after restart until `exp` elapses. This is a real security gap, not merely an operational one.

**Audit structured fields gap:** `internal/audit/audit_log.go` — `AuditEvent` struct has `Detail string` (free-form). `computeHash()` covers: `prevHash|id|timestamp|eventType|agentID|taskID|orchID|detail`. Pattern log schema requires `resource`, `outcome`, `delegation_depth`, `delegation_chain_hash` as discrete queryable fields.

**Task-completion signal gap:** `cmd/broker/main.go` route table — no `POST /v1/token/release` or equivalent endpoint. Agents can only let tokens expire naturally or use the admin `POST /v1/revoke`.

### Additional Gap Discovered (Missed by All Four Reviewers)

**Audience validation not enforced:**
- `internal/token/tkn_claims.go` — `TknClaims.Validate()` checks iss/sub/jti/exp/nbf but never checks `Aud`
- `Aud []string` field exists in `TknClaims` (tagged `json:"aud,omitempty"`) but is never populated in `Register()`, `Renew()`, or `Delegate()` paths
- Impact: a broker-issued token can be presented to any external service without audience binding; cross-service token replay is possible

### Items Explicitly Deferred

| Item | Reason |
|------|--------|
| Behavioral anomaly detection | Pattern marks as "Optional but Recommended." Heartbeat liveness auto-revocation is implemented and sufficient. |
| Signing key persistence | Ephemeral keys provide automatic key rotation. Short TTLs (5 min default) bound the re-registration window after restart. Deferred with documentation. |
| Multi-instance revocation propagation | Single-instance deployment target. Fix 2 (SQLite persistence) ensures revocations survive restarts within the same instance. |
| SPIRE Workload API integration | File-based TLS certs (Fix 1) are the immediate path. Config is certificate-source-agnostic for future SPIRE. |

---

## Part 2: Design Solutions

### Fix 1: Native TLS/mTLS in Broker

**Problem:** `cmd/broker/main.go:174` — `http.ListenAndServe(addr, rootHandler)` — plain HTTP only. All four reviewers flagged this as the primary partial compliance finding.

**Design:** Config-driven TLS with three modes controlled by `AA_TLS_MODE`:

| Mode | Required Config | Behavior | Use Case |
|------|----------------|----------|----------|
| `none` (default) | — | Plain HTTP, current behavior | Dev, proxy-terminated deployments |
| `tls` | `AA_TLS_CERT` + `AA_TLS_KEY` | Server-side TLS only | Standard production TLS |
| `mtls` | `AA_TLS_CERT` + `AA_TLS_KEY` + `AA_TLS_CLIENT_CA` | Full mutual TLS | High-security deployments |

- Minimum TLS version: `tls.VersionTLS13`
- mTLS client auth: `tls.RequireAndVerifyClientCert`
- Default `none` preserves backward compatibility; existing deployments behind a reverse proxy are unaffected
- Startup fail-fast if `AA_TLS_MODE=tls` or `mtls` but cert/key files are missing or unreadable

**Implementation touch points:**
- `internal/cfg/cfg.go`: Add `TLSMode`, `TLSCert`, `TLSKey`, `TLSClientCA string` fields
- `cmd/broker/main.go:174`: Replace `http.ListenAndServe` with mode switch:
  - `none` → `http.ListenAndServe(addr, rootHandler)`
  - `tls` → `http.ListenAndServeTLS(addr, cfg.TLSCert, cfg.TLSKey, rootHandler)`
  - `mtls` → load client CA pool, construct `tls.Config`, wrap in `http.Server`, call `srv.ListenAndServeTLS`

**Feature branch:** `feature/broker-tls`
**Estimated lines changed:** ~60

---

### Fix 2: Revocation Persistence to SQLite

**Problem:** `internal/revoke/rev_svc.go` — `RevSvc` uses only in-memory maps. Broker restart clears all revocations. A compromised agent whose credentials were revoked regains access after restart until token `exp`.

**Design:** Write-through persistence on every `Revoke()` call; bulk load on startup.

**SQLite schema** (added to `InitDB()` in `internal/store/sql_store.go`):
```sql
CREATE TABLE IF NOT EXISTS revocations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    level      TEXT NOT NULL,
    target     TEXT NOT NULL,
    revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(level, target)
);
```

No `expires_at` column by design: automatic cleanup before token `exp` could silently re-enable revoked entries. Safe cleanup logic (only remove where `revoked_at + system_max_ttl < now`) is deferred to a future PR requiring explicit safety analysis. SQLite handles millions of rows; table growth is not an operational concern on the current scale.

**RevSvc changes:**
- Add `store RevocationStore` interface field (for testability — SQLite store implements it)
- `Revoke()`: write to SQLite via `store.SaveRevocation(level, target)` after updating in-memory map. Persist-then-update order: if persistence fails, log and return error (no silent data loss)
- New constructor `NewRevSvcWithStore(store RevocationStore) *RevSvc`

**Startup sequence** (in `cmd/broker/main.go`, after `sqlStore.InitDB()`):
```go
entries, err := sqlStore.LoadAllRevocations()
// handle err
revSvc := revoke.NewRevSvcWithStore(sqlStore)
revSvc.LoadEntries(entries)
```

**Signing key interaction:** Broker regenerates its Ed25519 signing key on every startup. Pre-restart tokens fail signature verification before the revocation check runs — persisted JTI-level revocations become dead weight after restart. This is not a security problem (those tokens are already invalid). Agent-level, task-level, and chain-level revocations remain meaningful across restart because new tokens can be issued for the same agent/task identities. Document this interaction in operator runbook.

**Feature branch:** `feature/revocation-persistence`
**Estimated lines changed:** ~120

---

### Fix 3: Audience Validation

**Problem:** `TknClaims.Aud` field exists but is never populated during issuance (`Register`, `Renew`, `Delegate`) and never checked during validation. A token issued by the broker can be replayed to any external service.

**Design:** Two-part fix — both sides must ship together:

**Part A — Issuance (populate `Aud`):**
- Add `Audience string` to `internal/cfg/cfg.go` loaded from `AA_AUDIENCE` (default `""`)
- When `cfg.Audience != ""`, `TknSvc.Issue()` sets `Aud: []string{cfg.Audience}` on every issued token
- `TknSvc.Renew()` propagates `Aud` from existing claims (preserves audience across renewal)
- `deleg_svc.go` `Delegate()` propagates `Aud` from delegator claims

**Part B — Validation (check `Aud`):**
- Add `ValidateWithAudience(expectedAud string) error` to `TknClaims`; returns `ErrAudienceMismatch` if `Aud` slice does not contain `expectedAud`
- `ValMw.Wrap()`: when `cfg.Audience != ""`, call `claims.ValidateWithAudience(cfg.Audience)` after `claims.Validate()`

**Default behavior — opt-in:** When `AA_AUDIENCE` is empty (default), audience validation is skipped entirely. This is backward compatible: existing deployed tokens have no `Aud` field; making enforcement the default would break all existing tokens on upgrade.

**Rollout path for existing deployments:**
1. Deploy with `AA_AUDIENCE=""` (no change to token validation, new tokens get no `Aud`)
2. After one full TTL window (all old tokens expired), set `AA_AUDIENCE=agentauth`
3. New tokens now carry `Aud: ["agentauth"]` and validation enforces it

**Implementation touch points:**
- `internal/cfg/cfg.go`: Add `Audience string`
- `internal/token/tkn_claims.go`: Add `ValidateWithAudience()`, `ErrAudienceMismatch`
- `internal/token/tkn_svc.go`: Populate `Aud` in `Issue()` when configured; propagate in `Renew()`
- `internal/authz/val_mw.go`: Conditional audience check in `Wrap()`
- `internal/identity/id_svc.go`: Pass audience through `IssueReq` in `Register()`
- `internal/deleg/deleg_svc.go`: Propagate `Aud` in `Delegate()`

**Feature branch:** `feature/audience-validation`
**Estimated lines changed:** ~60

---

### Fix 4: Token Release Endpoint

**Problem:** No explicit task-completion signal. Juliet flagged as partial compliance on requirement 4.4 ("Task-based: Agent signals task completion"). Agents currently rely on token expiry or admin revocation.

**Design:** `POST /v1/token/release` — self-service JTI revocation by the token holder.

- Sits behind `valMw.Wrap()` — the token being released is the Bearer token in the Authorization header
- Extracts `claims.Jti` from context (set by `ValMw`)
- Calls `revSvc.Revoke("token", claims.Jti)`
- Records `EventTokenReleased = "token_released"` audit event
- Returns `200 OK` with empty body on success
- No request body required

**Security properties:** Only the token holder can release their own token. The token must be valid (not expired, not already revoked) to be released — prevents double-release confusion. After release, the JTI is in the revocation list; any subsequent use returns 401.

**Implementation touch points:**
- New `internal/handler/release_hdl.go`
- `internal/audit/audit_log.go`: Add `EventTokenReleased = "token_released"` constant
- `cmd/broker/main.go`: Register route `mux.Handle("POST /v1/token/release", valMw.Wrap(releaseHdl))`

**Feature branch:** `feature/token-release`
**Estimated lines changed:** ~60

---

### Fix 5: Sidecar UDS Listen Mode (Port Sprawl Solution)

**Problem:** `cmd/sidecar/main.go` — `http.ListenAndServe(":"+cfg.Port, mux)`. Every sidecar binds one TCP port. N apps = N sidecars = N distinct TCP ports. Port allocation requires firewall rule management, compose file port mapping, and conflict avoidance.

**Analysis of options:**

| Approach | Verdict | Rationale |
|----------|---------|-----------|
| UDS per-app sidecar | **SELECTED** | Eliminates port namespace congestion, preserves per-app isolation, uses kernel filesystem ACL |
| Shared TCP gateway | Rejected | Cross-app blast radius: one compromised app's sidecar token can affect all apps sharing the gateway |
| Dynamic port assignment | Rejected | Solves conflict but not firewall/routing complexity; still N open ports |

**What UDS eliminates:** Port allocation overhead, firewall rule management, port conflict risk, `docker-compose.yml` port mapping for every sidecar. What it does NOT change: process count (N sidecars still means N processes — this is intentional for isolation).

**Config additions to `cmd/sidecar/config.go`:**
- `ListenMode string` — loaded from `AA_SIDECAR_LISTEN_MODE` (default `"tcp"`)
- `SocketPath string` — loaded from `AA_SIDECAR_SOCKET_PATH` (required when `ListenMode=uds`)

**Implementation in `cmd/sidecar/main.go`:**
```go
// When ListenMode == "uds":
os.Remove(cfg.SocketPath) // clean up stale socket from previous run
ln, err := net.Listen("unix", cfg.SocketPath)
os.Chmod(cfg.SocketPath, 0660) // owner+group read/write
go http.Serve(ln, mux)
// Cleanup on graceful shutdown:
defer os.Remove(cfg.SocketPath)
```

**Socket path timing constraint:** The sidecar receives its broker-assigned ID (`state.sidecarID`) only after `bootstrap()` completes — which happens AFTER the HTTP server must start (health probes run pre-bootstrap). The socket path MUST therefore be operator-configured via `AA_SIDECAR_SOCKET_PATH` at deploy time. It cannot be derived from the broker-assigned ID at runtime.

**Kubernetes deployment model:** Mount a shared `emptyDir` volume at `/var/run/agentauth/` in both the sidecar container and the app container. Set `AA_SIDECAR_SOCKET_PATH=/var/run/agentauth/sidecar.sock`. App connects to the socket path. No port mapping needed.

**Docker Compose deployment model:** Each sidecar service has a named volume shared with its app service. `AA_SIDECAR_SOCKET_PATH=/var/run/agentauth/myapp.sock`. Eliminate port mapping from the sidecar service definition entirely.

**Feature branch:** `feature/sidecar-uds`
**Estimated lines changed:** ~50

---

### Fix 6: Structured Audit Log Fields

**Problem:** `AuditEvent.Detail` is a free-form string. Kilo flagged as partial compliance on requirement 5.2. The pattern's log schema specifies `resource`, `outcome`, `delegation_depth`, `delegation_chain_hash` as discrete queryable fields — currently embedded in unstructured Detail text, which cannot be filtered by the query API.

**Design:** Extend `AuditEvent` with five new optional structured fields. Use functional options on `Record()` for backward compatibility — all existing callers compile unchanged.

**New fields on `AuditEvent`:**
```go
Resource         string `json:"resource,omitempty"`
Outcome          string `json:"outcome,omitempty"`
DelegDepth       int    `json:"deleg_depth,omitempty"`
DelegChainHash   string `json:"deleg_chain_hash,omitempty"`
BytesTransferred int64  `json:"bytes_transferred,omitempty"`
```

**Functional options pattern:**
```go
type RecordOption func(*AuditEvent)

func WithResource(r string) RecordOption      { return func(e *AuditEvent) { e.Resource = r } }
func WithOutcome(o string) RecordOption       { return func(e *AuditEvent) { e.Outcome = o } }
func WithDelegDepth(d int) RecordOption       { return func(e *AuditEvent) { e.DelegDepth = d } }
func WithDelegChainHash(h string) RecordOption { return func(e *AuditEvent) { e.DelegChainHash = h } }
```

`Record(eventType, agentID, taskID, orchID, detail string, opts ...RecordOption)` — existing callers pass no opts.

**Hash chain continuity:** New fields MUST be included in `computeHash()`. Omitting them means a tampered `Resource` or `Outcome` field would not break the chain — defeating tamper evidence for the new fields. Updated format string:
```go
fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d|%s|%d",
    evt.PrevHash, evt.ID, evt.Timestamp.Format(time.RFC3339Nano),
    evt.EventType, evt.AgentID, evt.TaskID, evt.OrchID, evt.Detail,
    evt.Resource, evt.Outcome, evt.DelegDepth, evt.DelegChainHash, evt.BytesTransferred)
```

Pre-migration events loaded from SQLite have NULL new columns → Go zero values (`""`, `0`) → deterministic hash output. Chain rebuilds correctly on startup because zero-value substitution is stable.

**SQLite migration:** `ALTER TABLE audit_events ADD COLUMN resource TEXT`, one per new field, each with `DEFAULT NULL`. SQLite executes `ADD COLUMN` online. No data backfill. No downtime.

**Query API extension:** Add `Resource` and `Outcome` as optional filter parameters to `QueryFilters` and `GET /v1/audit/events`.

**Implementation touch points:**
- `internal/audit/audit_log.go`: New fields, functional options, updated `computeHash()`, updated `Record()` signature
- `internal/store/sql_store.go`: `InitDB()` migration, updated `SaveAuditEvent()`, updated `QueryAuditEvents()` SELECT
- ~6 caller sites in delegation, token issuance, revocation, registration handlers

**Feature branch:** `feature/structured-audit`
**Estimated lines changed:** ~200 (high touch count — do last)

---

## Part 3: Implementation Plan

### Phase Ordering

```
Phase 1 (Security — P0)         Phase 2 (Compliance — P1)        Phase 3 (Operations — P2)
+---------------------------+   +---------------------------+    +------------------------+
| Fix 1: mTLS in broker     |   | Fix 3: Audience validation|    | Fix 5: Sidecar UDS     |
| Fix 2: Revocation persist |   | Fix 4: Token release      |    | Fix 6: Structured audit|
+---------------------------+   +---------------------------+    +------------------------+
```

Phase 2 and Phase 3 fixes are independent of Phase 1 (no blocking dependency). P0 fixes should be merged first to keep main in the strongest security state, but Phase 2/3 PRs can be authored in parallel.

### Per-Fix Summary

| Fix | Branch | Est. Lines | Key Test |
|-----|--------|-----------|---------|
| 1 | `feature/broker-tls` | ~60 | Start broker in each TLS mode; verify TLS handshake; verify plain HTTP default |
| 2 | `feature/revocation-persistence` | ~120 | Revoke, restart broker, confirm revocation still active; agent+task level across restart |
| 3 | `feature/audience-validation` | ~60 | Token with wrong aud rejected when AA_AUDIENCE set; empty AA_AUDIENCE skips check |
| 4 | `feature/token-release` | ~60 | Release token, verify subsequent use returns 401 |
| 5 | `feature/sidecar-uds` | ~50 | Start sidecar in UDS mode; app connects via socket; health probe works pre-bootstrap |
| 6 | `feature/structured-audit` | ~200 | Hash chain intact after SQLite migration; new fields queryable via API |

Each fix must pass `./scripts/gates.sh task` before PR to develop.

---

## Part 4: Post-Implementation Compliance

| Requirement | Before This Plan | After All Fixes | Fix |
|------------|-----------------|-----------------|-----|
| 3.3 mTLS transport | PARTIAL (all 4 reviewers) | COMPLIANT | Fix 1 |
| Revocation survives restart | NOT ADDRESSED | COMPLIANT | Fix 2 |
| Token audience binding | NOT CHECKED | COMPLIANT (opt-in) | Fix 3 |
| 4.4 Task-completion signal | PARTIAL (Juliet) | COMPLIANT | Fix 4 |
| Sidecar port sprawl | N/A (operational) | RESOLVED | Fix 5 |
| 5.2 Structured audit schema | PARTIAL (Kilo) | COMPLIANT | Fix 6 |

---

## Part 5: Design Decisions

### Decision 1: `AA_TLS_MODE` defaults to `none`
Backward compatible. Deployments behind TLS-terminating reverse proxies are unaffected. Operators who want native TLS explicitly opt in. Alternative (fail-fast without TLS) would break all existing dev and proxy-terminated deployments.

### Decision 2: Revocation table has no `expires_at`
Automatic cleanup of revocation entries is dangerous: if cleanup runs before a revoked token's `exp` elapses, the entry disappears and the token becomes valid again. Until explicit safe-cleanup logic is designed and tested, revocation entries are permanent. SQLite handles the table size without issue. Operators can monitor `SELECT COUNT(*) FROM revocations` and manually prune entries older than `system_max_ttl` if needed.

### Decision 3: Audience validation is opt-in (`AA_AUDIENCE` default `""`)
Existing deployed tokens have no `Aud` claim. Enforcing audience by default on upgrade would invalidate all live tokens. Opt-in preserves backward compatibility. The limitation (no cross-service replay protection when `AA_AUDIENCE` is empty) is documented explicitly.

### Decision 4: UDS per-app sidecars, not shared gateway
A shared gateway eliminates per-app sidecars but introduces cross-app blast radius. One compromised app's sidecar token affects all apps sharing the gateway. Per-app isolation is a core design principle of AgentAuth; UDS preserves it while eliminating port sprawl.

### Decision 5: Functional options for `Record()`
All existing callers compile unchanged. New structured fields are opt-in at call sites. Idiomatic Go pattern.

### Decision 6: New audit fields included in `computeHash()`
Tamper evidence must cover all event data. Excluding new fields from the hash would allow undetected tampering of `Resource` and `Outcome` values. The hash function uses Go zero values (`""`, `0`) for absent fields in pre-migration events, ensuring deterministic output and chain continuity.

---

## Appendix A: Compliance Review Cross-Reference

| Reviewer | Requirements | COMPLIANT | PARTIAL | NOT COMPLIANT |
|----------|-------------|-----------|---------|---------------|
| India | 25 | 24 | 1 | 0 |
| Juliet | 29 | 26 | 3 | 0 |
| Kilo | 28 | 25 | 3 | 0 |
| Lima | 25 | 23 | 2 | 0 |

---

## Appendix B: Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| TLS misconfiguration (wrong cert path, expired cert) | Medium | High | Fail-fast at startup if cert/key unreadable; `TestBrokerTLSModes` integration test |
| Audit hash chain break after Fix 6 migration | Low | High | `TestAuditHashChainAfterMigration`; zero-value substitution for NULL fields is deterministic |
| UDS socket stale from crashed previous run | Low | Medium | `os.Remove(cfg.SocketPath)` before `net.Listen` at startup; `IsNotExist` check |
| Audience validation breaks existing tokens on upgrade | Low | High | Opt-in semantics (Decision 3); empty `AA_AUDIENCE` = no change |
| Revocation table growth over time | Low | Low | Deferred safe-cleanup; table size negligible on current scale |

---

## Appendix C: Code Location Reference

All line numbers are approximate; verify before implementing as the develop branch is live.

| Fix | File | Key Location |
|-----|------|-------------|
| 1 | `cmd/broker/main.go` | Line ~174: replace `ListenAndServe` |
| 1 | `internal/cfg/cfg.go` | Add `TLSMode`, `TLSCert`, `TLSKey`, `TLSClientCA` |
| 2 | `internal/revoke/rev_svc.go` | Add `store` field; wrap `Revoke()` with persist |
| 2 | `internal/store/sql_store.go` | Add `revocations` table to `InitDB()`; add Save/Load methods |
| 2 | `cmd/broker/main.go` | Load revocations after `InitDB()`, before serving |
| 3 | `internal/token/tkn_claims.go` | Add `ValidateWithAudience()`, `ErrAudienceMismatch` |
| 3 | `internal/token/tkn_svc.go` | Populate `Aud` in `Issue()`; propagate in `Renew()` |
| 3 | `internal/authz/val_mw.go` | Conditional audience check in `Wrap()` |
| 3 | `internal/cfg/cfg.go` | Add `Audience string` |
| 4 | `internal/handler/release_hdl.go` | New file |
| 4 | `internal/audit/audit_log.go` | Add `EventTokenReleased` constant |
| 4 | `cmd/broker/main.go` | Register `POST /v1/token/release` route |
| 5 | `cmd/sidecar/config.go` | Add `ListenMode`, `SocketPath` |
| 5 | `cmd/sidecar/main.go` | Conditional `net.Listen("unix", ...)` |
| 6 | `internal/audit/audit_log.go` | New fields, functional options, `computeHash()` update |
| 6 | `internal/store/sql_store.go` | `InitDB()` migration, Save/Load updates |

---

**END OF DESIGN SOLUTION + IMPLEMENTATION PLAN**
