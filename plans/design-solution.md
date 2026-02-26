# Design Solution v2: Compliance Fixes and Sidecar Transport

**Date:** 2026-02-25
**Branch:** develop
**Supersedes:** archive/design-solution-v1.md (2026-02-20)
**Ordering rationale:** 2026-02-25-fix-ordering-analysis.md

---

## Context

Four independent compliance reviewers scored AgentAuth develop at 92-96% against the Ephemeral Agent Credentialing Security Pattern v1.2. Zero NOT COMPLIANT findings. Six fixes address every PARTIAL finding plus one gap all reviewers missed (audience validation).

Session 8 Docker testing revealed the v1 design had an incomplete scope for Fix 1 (broker TLS only, sidecar client missing) and false independence claims between fixes. This v2 design is written from scratch against verified code state on develop.

---

## Fix Ordering

Derived from first principles (see `2026-02-25-fix-ordering-analysis.md`):

```
1. Fix 2  Revocation Persistence     P0  Security gap
2. Fix 3  Audience Validation         P1  Compliance gap
3. Fix 4  Token Release               P1  Compliance (depends on Fix 2)
4. Fix 1  Sidecar TLS Client          P0  Compliance (broker side done)
5. Fix 5  Sidecar UDS Listen Mode     P1  Operations
6. Fix 6  Structured Audit Fields     P2  Compliance (widest change)
```

Each fix merges to develop independently. Each requires `gates.sh` + Docker live test before merge.

---

## Fix 2: Revocation Persistence to SQLite

### Problem

`internal/revoke/rev_svc.go` holds four in-memory maps (`tokens`, `agents`, `tasks`, `chains`). All revocations are lost on broker restart. A token revoked for compromise becomes valid again after restart until its `exp` elapses.

Note: because signing keys are ephemeral (regenerated on startup), pre-restart tokens fail signature verification anyway. But the revocation layer must be correct independently — signing key persistence is a planned future feature, and when it ships, revocation persistence must already be in place.

### Design

**Write-through persistence.** Every `Revoke()` call writes to both the in-memory map and SQLite. On startup, `LoadAllRevocations()` populates the in-memory maps from SQLite.

**Schema:**

```sql
CREATE TABLE IF NOT EXISTS revocations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    level      TEXT NOT NULL,
    target     TEXT NOT NULL,
    revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(level, target)
);
```

No `expires_at` column. Revocation entries are permanent. Rationale: if cleanup runs before a revoked token's `exp` elapses, the token becomes valid again — a security regression. Safe cleanup (only remove entries where `revoked_at + max_system_ttl < now`) is deferred to a future PR.

The `UNIQUE(level, target)` constraint makes `Revoke()` idempotent — re-revoking the same target is a no-op at the DB level.

### Changes

| File | What Changes |
|------|-------------|
| `internal/store/sql_store.go` | Add `revocations` table to `InitDB()`. Add `SaveRevocation(level, target string) error` and `LoadAllRevocations() ([]struct{Level, Target string}, error)` methods. |
| `internal/revoke/rev_svc.go` | Add `store` field to `RevSvc`. `NewRevSvc()` accepts optional store. `Revoke()` calls `store.SaveRevocation()` after in-memory write. Add `LoadFromStore()` method called at startup. |
| `cmd/broker/main.go` | Pass `sqlStore` to `NewRevSvc()`. Call `revSvc.LoadFromStore()` after DB init (between lines 105-114). |

### Docker Live Test

Per user stories in `tests/fix2-revocation-persistence-user-stories.md`:
1. Register agent, revoke token, verify rejection
2. Restart broker container (`docker compose restart broker`)
3. Verify revoked token is still rejected after restart
4. Verify non-revoked tokens are not false-positived

---

## Fix 3: Audience Validation Enforcement

### Problem

`TknClaims.Aud` field exists (`tkn_claims.go:33`) but is never populated or validated. `Register()` in `id_svc.go` issues tokens with empty `Aud`. `Renew()` in `tkn_svc.go` drops `Aud` when building the renewal `IssueReq` (line 175 — `Aud` not included). `Delegate()` in `deleg_svc.go` also issues with empty `Aud` (line 151). Impact: a token has no audience binding — it can be presented to any service.

### Design

**Two-part fix:**

**Part A — Broker sets and validates audience on its own tokens:**
- `Register()` sets `Aud: []string{cfg.Audience}` on every `IssueReq` (when `cfg.Audience` is non-empty)
- `Renew()` propagates `Aud` from existing claims: add `Aud: claims.Aud` to the `IssueReq`
- `Delegate()` propagates `Aud` from delegator claims: add `Aud: delegatorClaims.Aud` to the `IssueReq`
- `ValMw.Wrap()` checks that token `Aud` contains `cfg.Audience` after `Verify()` succeeds. When `cfg.Audience` is empty, the check is skipped entirely (not fail-closed).

**Part B — Config:**
- New env var: `AA_AUDIENCE` (default `"agentauth"`)
- Migration: set `AA_AUDIENCE=""` during rollout until all old tokens cycle (max one TTL window), then set to `"agentauth"`

### Changes

| File | What Changes |
|------|-------------|
| `internal/cfg/cfg.go` | Add `Audience string` field. Load from `AA_AUDIENCE` env var, default `"agentauth"`. |
| `internal/token/tkn_svc.go` | `Renew()`: add `Aud: claims.Aud` to `IssueReq` (line ~175). |
| `internal/authz/val_mw.go` | Add `audience string` field to `ValMw`. `NewValMw()` accepts audience param. In `Wrap()`, after `Verify()` succeeds: if `audience != ""`, check `claims.Aud` contains it; reject with 401 if not. |
| `internal/identity/id_svc.go` | `Register()`: set `Aud: []string{cfg.Audience}` in `IssueReq` when audience is configured. |
| `internal/deleg/deleg_svc.go` | `Delegate()`: set `Aud: delegatorClaims.Aud` in `IssueReq`. |
| `cmd/broker/main.go` | Pass `c.Audience` to `NewValMw()`. |

### Docker Live Test

Per user stories in `tests/fix3-audience-validation-user-stories.md`:
1. Start broker with `AA_AUDIENCE=broker-production`, verify wrong-audience tokens are rejected
2. Start broker with `AA_AUDIENCE` unset, verify all tokens accepted (backward compatible)
3. Start broker with `AA_AUDIENCE=broker-production`, verify correct-audience tokens pass

---

## Fix 4: Token Release Endpoint

### Problem

No explicit task-completion signal. Pattern v1.2 section 4.4 expects agents to surrender tokens when done.

### Design

`POST /v1/token/release` — agent presents its Bearer token, broker revokes the token's JTI via `revSvc.Revoke("token", jti)`, records a `token_released` audit event.

This is effectively self-revocation. Unlike admin `POST /v1/revoke` (which requires `admin:revoke:*` scope), the release endpoint requires only valid Bearer auth — the agent can only release its own token.

With Fix 2 already in place, the released token's JTI is persisted to SQLite automatically through the existing write-through path. No additional persistence work needed.

Double-release is idempotent: the `UNIQUE(level, target)` constraint in the revocations table handles re-revocation silently, and `IsRevoked()` returns true on the second call before `Revoke()` is even reached.

### Changes

| File | What Changes |
|------|-------------|
| `internal/audit/audit_log.go` | Add `EventTokenReleased = "token_released"` constant. |
| `internal/handler/release_hdl.go` | **New file.** `ReleaseHdl` struct with `tknSvc`, `revSvc`, `auditLog` fields. `ServeHTTP()` extracts claims from context, calls `revSvc.Revoke("token", claims.Jti)`, records audit event. Returns 204. |
| `cmd/broker/main.go` | Wire `ReleaseHdl`. Add route: `POST /v1/token/release` behind `valMw.Wrap()` (Bearer auth, no scope requirement). |

### Docker Live Test

Per user stories in `tests/fix4-token-release-user-stories.md`:
1. Register, release, verify token rejected afterward
2. Verify `token_released` audit event recorded
3. Double-release returns success (idempotent)

---

## Fix 1: Complete Sidecar TLS Client

### Problem

Broker-side TLS is complete (`cmd/broker/serve.go`). The broker can serve in `none`, `tls`, or `mtls` modes. But the sidecar's `brokerClient` in `cmd/sidecar/broker_client.go` uses a plain `http.Client` (line 38: `&http.Client{Timeout: 10 * time.Second}`). The sidecar cannot:
- Connect to a broker serving TLS (no CA trust)
- Present client certificates for mTLS

This was the design gap discovered in Session 8. The v1 design listed only broker files and missed the sidecar entirely.

### Design

**Sidecar TLS client config** — mirror the broker's three modes:

| Mode | Sidecar Config | Behavior |
|------|---------------|----------|
| `none` (default) | No TLS env vars | Plain HTTP client (current behavior) |
| `tls` | `AA_SIDECAR_CA_CERT` | HTTPS client, verifies broker cert against CA |
| `mtls` | `AA_SIDECAR_CA_CERT` + `AA_SIDECAR_TLS_CERT` + `AA_SIDECAR_TLS_KEY` | HTTPS client with client cert presentation |

When `AA_SIDECAR_CA_CERT` is set, the sidecar builds a custom `tls.Config` with the CA pool and (optionally) client certificate, then constructs `http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}`.

The sidecar must also switch its `BrokerURL` from `http://` to `https://` when TLS is active. This is operator-configured via `AA_BROKER_URL` — no auto-detection.

### Changes

| File | What Changes |
|------|-------------|
| `cmd/sidecar/config.go` | Add fields to `sidecarConfig`: `CACert string` (`AA_SIDECAR_CA_CERT`), `TLSCert string` (`AA_SIDECAR_TLS_CERT`), `TLSKey string` (`AA_SIDECAR_TLS_KEY`). Load in `loadConfig()`. |
| `cmd/sidecar/broker_client.go` | `newBrokerClient()` accepts TLS config. When CA cert is provided: load CA pool, optionally load client cert pair, build `tls.Config`, create `http.Transport` with it, use as `http.Client.Transport`. |
| `cmd/sidecar/main.go` | Pass TLS config from `sidecarConfig` to `newBrokerClient()`. |

### Docker Live Test

Extends the Docker TLS test infrastructure from Session 8 (not merged but reusable):
1. HTTP mode: sidecar connects to broker over plain HTTP — all operations work
2. TLS mode: broker serves TLS, sidecar trusts broker CA — all operations work over HTTPS
3. mTLS mode: broker requires client certs, sidecar presents them — all operations work
4. mTLS rejection: sidecar without client cert cannot connect to mTLS broker

---

## Fix 5: Sidecar UDS Listen Mode

### Problem

N applications = N sidecars = N TCP ports. Port assignment, firewall rules, and collision risk scale linearly.

### Design

Move from port namespace to filesystem namespace. The sidecar listens on a Unix domain socket instead of TCP when configured.

| Mode | Config | Behavior |
|------|--------|----------|
| TCP (default) | `AA_SIDECAR_PORT=8081` | Listen on TCP port (current) |
| UDS | `AA_SOCKET_PATH=/var/run/agentauth/myapp.sock` | Listen on Unix domain socket |

When `AA_SOCKET_PATH` is set, the sidecar:
1. Calls `net.Listen("unix", socketPath)` instead of the TCP listener
2. Sets socket permissions to `0660` (owner + group)
3. Defers `os.Remove(socketPath)` for cleanup on shutdown
4. Logs a startup message with the socket path
5. Does NOT open any TCP port

When `AA_SOCKET_PATH` is unset, the sidecar falls back to TCP on `AA_SIDECAR_PORT` and logs a `WARN` that agent-to-sidecar traffic is network-exposed.

Socket path is static — set by operator at deploy time via env var. Not derived from sidecar ID (which is only available after bootstrap).

### Changes

| File | What Changes |
|------|-------------|
| `cmd/sidecar/config.go` | Add `SocketPath string` field (`AA_SOCKET_PATH`). Load in `loadConfig()`. |
| `cmd/sidecar/main.go` | Replace `http.ListenAndServe(addr, mux)` with conditional: if `cfg.SocketPath != ""`, use `net.Listen("unix", cfg.SocketPath)` + `http.Serve(ln, mux)` with cleanup. Otherwise TCP as today. Log warning on TCP fallback. |

### Interaction with Fix 1

Fix 1 modifies the sidecar's **outbound** connection (broker client TLS). Fix 5 modifies the sidecar's **inbound** listener (app-facing transport). These are different code paths:
- Fix 1: `broker_client.go` → `newBrokerClient()` → `http.Client.Transport`
- Fix 5: `main.go` → `http.ListenAndServe()` → `net.Listen()`

No function-level overlap. Doing them back-to-back (steps 4 and 5) minimizes the window where both are in-flight on the same files.

### Docker Live Test

Per user stories in `tests/fix5-sidecar-uds-user-stories.md`:
1. Start sidecar with `AA_SOCKET_PATH`, verify socket file exists, `curl --unix-socket` returns 200
2. Request token via Unix socket — works
3. Start sidecar without `AA_SOCKET_PATH`, verify TCP works and WARN is logged

---

## Fix 6: Structured Audit Log Fields

### Problem

`AuditEvent.Detail` is a free-form string (`audit_log.go:66`). Compliance queries require parsing unstructured text.

### Design

Add structured fields to `AuditEvent` alongside (not replacing) `Detail`:

```go
type AuditEvent struct {
    // ... existing fields ...
    Detail    string `json:"detail"`
    // New structured fields
    Resource        string `json:"resource,omitempty"`
    Outcome         string `json:"outcome,omitempty"`         // "success" or "denied"
    DelegDepth      int    `json:"deleg_depth,omitempty"`
    DelegChainHash  string `json:"deleg_chain_hash,omitempty"`
    BytesTransferred int64 `json:"bytes_transferred,omitempty"`
    // ... Hash, PrevHash ...
}
```

**Functional options** for backward compatibility — existing `Record()` callers continue to work. New callers use options:

```go
func WithResource(r string) RecordOption { ... }
func WithOutcome(o string) RecordOption { ... }
func WithDelegDepth(d int) RecordOption { ... }
```

**Hash coverage:** All new fields MUST be included in `computeHash()` input. Omitting them means a tampered value wouldn't break the chain.

**SQLite migration:** `ALTER TABLE audit_events ADD COLUMN resource TEXT DEFAULT NULL` (etc.) for each new field. No backfill — existing events keep NULL.

**Query support:** Add `outcome` filter to `QueryFilters` and the query path.

### Changes

| File | What Changes |
|------|-------------|
| `internal/audit/audit_log.go` | Add fields to `AuditEvent`. Add `RecordOption` type and option functions. Update `Record()` to accept variadic options. Update `computeHash()` to include new fields. Add `outcome` to `QueryFilters`. |
| `internal/store/sql_store.go` | ALTER TABLE migration for new columns. Update `SaveAuditEvent()`, `LoadAllAuditEvents()`, `QueryAuditEvents()` to handle new fields. |
| ~9 caller files | Update `Record()` calls to pass structured options where the data is available. Callers that don't have structured data keep working as-is (no options = empty fields). |

Caller files: `val_mw.go` (5 calls), `id_svc.go` (3), `deleg_svc.go` (2), `revoke_hdl.go` (1), `renew_hdl.go` (2), `token_exchange_hdl.go` (2), `admin_svc.go` (4+), `admin_hdl.go` (4+), plus the new `release_hdl.go` from Fix 4.

### Docker Live Test

Per user stories in `tests/fix6-structured-audit-user-stories.md`:
1. Trigger operations, verify structured fields in audit event JSON
2. Filter by `outcome=denied`, verify only denied events returned
3. Verify hash chain integrity covers new fields

---

## Design Decisions

### Decision 1: Dependency-driven ordering over thematic grouping
v1 grouped by theme (Security / Compliance / Operations). This hid the real dependency: Fix 4 needs Fix 2's persistence. The new ordering (2 → 3 → 4 → 1 → 5 → 6) follows code dependencies.

### Decision 2: Fix 1 scope includes sidecar client
v1 listed only broker files. The sidecar's `brokerClient` uses plain HTTP. mTLS requires both sides. Fix 1 now covers `cmd/sidecar/broker_client.go` and `cmd/sidecar/config.go`.

### Decision 3: UDS per-app sidecars over shared gateway
Shared gateway introduces cross-app blast radius. UDS eliminates port sprawl without new trust boundaries. (Unchanged from v1.)

### Decision 4: File-based TLS over SPIRE integration
SPIRE adds infrastructure dependency. File-based TLS closes compliance immediately. Config is cert-source-agnostic for future SPIRE. (Unchanged from v1.)

### Decision 5: No expires_at in revocations table
Security correctness over table size. Cleanup that runs too early silently un-revokes tokens. (Unchanged from v1.)

### Decision 6: Functional options for audit Record()
Backward compatible. Existing callers unchanged. New callers pass structured data. Idiomatic Go. (Unchanged from v1.)

### Decision 7: New audit fields included in hash computation
Tamper evidence requires all fields covered. Excluding new fields creates integrity gaps. (Unchanged from v1.)

### Decision 8: Audience validation opt-in via AA_AUDIENCE
Default `"agentauth"` for new deployments. Empty string skips validation for backward compatibility during rollover. (Unchanged from v1.)

### Decision 9: Sidecar TLS config is separate from broker TLS config
Sidecar env vars use `AA_SIDECAR_*` prefix. Broker uses `AA_TLS_*`. They're different binaries with different roles — the sidecar is a TLS client, the broker is a TLS server. Sharing config would conflate client and server concerns.

---

## Known Limitation: Ephemeral Signing Key

AgentAuth generates a fresh Ed25519 key pair on every broker startup. Pre-restart tokens fail signature verification. This does NOT satisfy the pattern's guarantee that credentials survive broker outage. Agents must retry on 401.

Deferred to future release (`AA_SIGNING_KEY_PATH`). Short TTLs (default 5 min) bound the impact window.

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Revocation table growth | Low | Low | No expires_at by design; safe cleanup deferred |
| Sidecar TLS misconfiguration | Medium | High | Docker test matrix covers all 3 modes |
| Audience migration breaks tokens | Medium | Medium | AA_AUDIENCE="" during rollout window |
| UDS permissions wrong | Low | Medium | Default 0660; document K8s securityContext |
| Audit hash schema boundary | Low | Medium | New fields in hash; migration timestamp in runbook |
| Ephemeral key on restart | Medium | Medium | Agents retry on 401; short TTL bounds window |

---

## Compliance Mapping

| Requirement | Before | After | Fix |
|------------|--------|-------|-----|
| Revocation persistence | PARTIAL | COMPLIANT | 2 |
| Token audience binding | NOT CHECKED | COMPLIANT | 3 |
| Task-completion signal (4.4) | PARTIAL | COMPLIANT | 4 |
| mTLS transport (3.3) | PARTIAL | COMPLIANT | 1 |
| Sidecar port sprawl | N/A (ops) | RESOLVED | 5 |
| Structured audit schema (5.2) | PARTIAL | COMPLIANT | 6 |
