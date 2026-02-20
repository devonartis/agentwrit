# Design Solution: Full Compliance and Sidecar Sprawl Fix

**Date:** 2026-02-20
**Authors:** integration-lead, security-architect, system-designer, code-planner, devils-advocate
**Branch:** develop (all code references are to develop branch)
**Status:** APPROVED (devils-advocate reviewed and signed off)

---

## Executive Summary

Four independent compliance reviewers (India, Juliet, Kilo, Lima) evaluated the AgentAuth develop branch against the Ephemeral Agent Credentialing Security Pattern v1.2. The codebase achieved 92-96% compliance across reviewers with zero NOT COMPLIANT findings. This design addresses every partial compliance item plus one gap the reviewers missed (audience validation), and solves the sidecar port sprawl problem (N apps = N sidecars = N ports).

**Six fixes, all independently implementable:**

| # | Fix | Compliance Gap | Priority | Scope |
|---|-----|---------------|----------|-------|
| 1 | Native TLS/mTLS in broker | 3.3 mTLS (all 4 reviewers) | P0 | Medium |
| 2 | Revocation persistence to SQLite | Revocations lost on restart (real security gap) | P0 | Small |
| 3 | Audience validation enforcement | Token aud field never set or checked (all paths) | P1 | Small |
| 4 | Token release endpoint | 4.4 task-completion signal (Juliet) | P1 | Small |
| 5 | Sidecar UDS listen mode | Sidecar port sprawl (N ports) | P1 | Small |
| 6 | Structured audit log fields | 5.2 free-form Detail field (Kilo) | P2 | Large |

---

## Compliance Gap Analysis

### Findings Consolidated Across All Four Reviewers

| Finding | India | Juliet | Kilo | Lima | Consensus |
|---------|-------|--------|------|------|-----------|
| No native TLS/mTLS (3.3) | PARTIAL | PARTIAL | PARTIAL | PARTIAL | All 4 |
| No task-completion signal (4.4) | -- | PARTIAL | -- | -- | 1 reviewer |
| Anomaly detection heartbeat-only (4.5) | -- | PARTIAL | -- | PARTIAL | 2 reviewers (pattern says optional) |
| Revocation single-instance only (4.4) | -- | -- | PARTIAL | -- | 1 reviewer |
| Audit log Detail is free-form (5.2) | -- | -- | PARTIAL | -- | 1 reviewer |

### Additional Gaps Discovered During Team Analysis

**Audience validation not enforced (missed by all 4 reviewers):**
- `internal/token/tkn_claims.go`: `Validate()` checks `iss`, `sub`, `jti`, `exp`, `nbf` but never checks `aud`
- `IssueReq.Aud` exists in the struct but is never populated by `Register()`, `Renew()`, or `Delegate()` — all three issue tokens with empty audience
- Impact: a token intended for one resource server can be presented to any other; broker-internal tokens have no audience binding

**Revocation loss on restart is a real security gap, not just operational inconvenience:**
- `internal/revoke/rev_svc.go`: `RevSvc` holds four in-memory maps only — all revocations are lost when the broker process exits
- Impact: a token revoked for compromise becomes valid again after broker restart, until its `exp` elapses
- Fix 2 addresses this directly

### Known Limitation: Ephemeral Signing Key

AgentAuth v2.x generates a fresh Ed25519 signing key pair on every broker startup (`cmd/broker/main.go`). Outstanding tokens issued before a broker restart will fail signature verification after restart, even if their `exp` has not elapsed.

**Impact:** This does NOT satisfy the security pattern's stated guarantee that "existing valid credentials continue to work during broker outage." Agents must implement retry logic — a 401 after a previously valid token should trigger re-registration.

**Rationale for deferral:** Short TTLs (default 5 min) bound the operational impact window. Key persistence requires a dedicated design (file permissions, rotation procedures). Deferred to a future release (`AA_SIGNING_KEY_PATH`).

**Required mitigation:** Document in SECURITY.md and docs/architecture.md. Agents implement exponential backoff and re-register on persistent 401 responses.

### Items Explicitly Deferred

| Item | Reason |
|------|--------|
| Behavioral anomaly detection | Pattern marks as "Optional but Recommended." Heartbeat monitoring sufficient. |
| Multi-instance revocation propagation | Single-instance deployment. Fix 2 ensures restart survival. |
| SPIRE Workload API integration | File-based TLS certs (Fix 1) are the immediate path. |
| Signing key persistence | Deferred with explicit documentation of the limitation above. |

---

## Design Solutions

### Fix 1: Native TLS/mTLS in Broker

**Problem:** Broker uses `http.ListenAndServe()` (plain HTTP) at `cmd/broker/main.go:174`. All four reviewers flagged this.

**Design:** Config-driven TLS with three modes:

| Mode | Config | Behavior |
|------|--------|----------|
| `none` (default) | No TLS env vars set | Plain HTTP (dev, proxy-terminated) |
| `tls` | `AA_TLS_CERT` + `AA_TLS_KEY` | Server-side TLS only |
| `mtls` | `AA_TLS_CERT` + `AA_TLS_KEY` + `AA_TLS_CLIENT_CA` | Full mutual TLS |

- Minimum TLS version: 1.3
- Client auth for mTLS: `tls.RequireAndVerifyClientCert`
- Backward compatible: default `none`

**Files:** `internal/cfg/cfg.go`, `cmd/broker/main.go`
**Dependencies:** None.

---

### Fix 2: Revocation Persistence to SQLite

**Problem:** `internal/revoke/rev_svc.go` uses in-memory maps only. Broker restart clears revocations. Real security gap.

**Design:** SQLite persistence (write-through on `Revoke()`, bulk load on startup).

**Schema (no expires_at):**
```sql
CREATE TABLE IF NOT EXISTS revocations (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    level     TEXT NOT NULL,
    target    TEXT NOT NULL,
    revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(level, target)
);
```

The `expires_at` column is intentionally omitted. Revocation entries are permanent and never cleaned up automatically. Rationale: if cleanup runs before a token's `exp` elapses, the revoked token becomes valid again — a security regression. The table is small; even at high volume SQLite handles millions of rows without issue. Safe cleanup logic (only remove entries where `revoked_at + max_system_ttl < now`) is deferred to a future PR requiring explicit safety analysis.

The `UNIQUE(level, target)` constraint makes `Revoke()` idempotent.

**Files:** `internal/revoke/rev_svc.go`, `internal/store/sql_store.go`, `cmd/broker/main.go`
**Dependencies:** None.

---

### Fix 3: Audience Validation Enforcement

**Problem:** `TknClaims.Validate()` never checks `Aud`. `IssueReq.Aud` exists but is never populated in `Register()`, `Renew()`, or `Delegate()`. All four reviewers missed this. Impact: tokens have no audience binding — a broker-internal token can be replayed to any external service and vice versa.

**Design: Two-part fix**

**Part A — Broker populates and validates audience for its own tokens:**
- `Register()` in `id_svc.go` sets `Aud: []string{"agentauth"}` on every issued token
- `TknSvc.Renew()` propagates `Aud` from existing claims so it is preserved across renewals
- `Delegate()` in `deleg_svc.go` propagates `Aud` from delegator claims
- `ValMw.Wrap()` validates that token `Aud` contains the configured `AA_AUDIENCE` string; skip validation when `AA_AUDIENCE` is empty (opt-in for backward compatibility during rollout)
- New cfg field: `AA_AUDIENCE` (default `"agentauth"`)

**Part B — Agents may specify audience at registration (optional):**
- `RegisterReq` accepts an optional `Aud []string` field. If present, it is passed through to `IssueReq.Aud`, overriding the default. This allows agents to pre-bind tokens to specific external resource servers.
- If not provided, the default `["agentauth"]` from Part A applies.

**Migration note:** Existing deployed tokens have no `aud` claim. Set `AA_AUDIENCE=""` during rollout to skip validation until all tokens have cycled (max one TTL window). Then set `AA_AUDIENCE="agentauth"` to enforce.

**Files:** `internal/cfg/cfg.go` (add `Audience string`), `internal/token/tkn_claims.go` (add `ValidateWithAudience()`), `internal/token/tkn_svc.go` (propagate Aud in Renew), `internal/authz/val_mw.go` (call audience check), `internal/identity/id_svc.go` (set default Aud), `internal/deleg/deleg_svc.go` (propagate Aud)
**Dependencies:** None.

---

### Fix 4: Token Release Endpoint

**Problem:** No explicit task-completion signal endpoint.

**Design:** `POST /v1/token/release` -- agent presents Bearer token, broker revokes JTI, records `token_released` audit event.

**Files:** New `internal/handler/release_hdl.go`, `internal/audit/audit_log.go`, `cmd/broker/main.go`
**Dependencies:** Uses existing `revSvc` and `valMw`.

---

### Fix 5: Sidecar UDS Listen Mode (Port Sprawl Solution)

**Problem:** N apps = N sidecars = N TCP ports.

**Decision: UDS per-app sidecars (not shared gateway)**

| Approach | Verdict | Rationale |
|----------|---------|-----------|
| UDS per-app sidecar | SELECTED | Eliminates port sprawl, preserves isolation, kernel ACL |
| Shared gateway | Rejected v1 | Cross-app blast radius violates isolation principles |

**What UDS solves:** Moves from port namespace to filesystem namespace. N sidecars still means N processes — process count is unchanged. The sprawl eliminated is port assignment overhead, firewall rules, and port conflict risk.

**Config:**

| Mode | Config | Behavior |
|------|--------|----------|
| `tcp` (default) | `AA_SIDECAR_PORT=8081` | Listen on TCP port (current behavior) |
| `uds` | `AA_SOCKET_PATH=/var/run/agentauth/myapp.sock` | Listen on Unix domain socket |

**Socket ownership model:**
- The sidecar creates the socket file at `AA_SOCKET_PATH` on startup, owned by the sidecar process UID/GID
- Default permissions: `0660` (owner + group read/write)
- The connecting app must run as the same user or same group
- In Kubernetes: mount a shared `emptyDir` volume at `/var/run/agentauth/` in both sidecar and app containers; set matching `securityContext.runAsGroup` in the pod spec

**Socket path is operator-set at deploy time:**
- `AA_SOCKET_PATH` is a static env var — no runtime derivation from sidecar ID needed
- Example: `AA_SOCKET_PATH=/var/run/agentauth/myapp.sock` (operator chooses a name per app)
- The app is configured with the same path to know where to connect — same pattern as configuring a TCP port

**Sidecar cleans up socket on shutdown** via `os.Remove(cfg.SocketPath)` in a deferred call or signal handler.

**Files:** `cmd/sidecar/config.go` (add `ListenMode`, `SocketPath`), `cmd/sidecar/main.go` (switch to `net.Listen("unix", path)` when mode is `uds`)
**Dependencies:** None. Backward compatible (default `tcp`).

---

### Application Onboarding: How a New App Authenticates With the Broker

**The core question:** An application starts with zero credentials. How does it prove its identity and get its first Bearer token?

**Answer:** The app receives a one-time **launch token** from an operator or orchestrator. The launch token is the "secret zero" -- it is NOT the Bearer credential itself, but a one-time authorization code that the app exchanges for a JWT through a cryptographic challenge-response protocol.

#### Broker-Direct Onboarding (No Sidecar)

Fully implemented on develop (`internal/identity/id_svc.go:Register()`). Three phases:

**Phase A -- Operator provisions the app (before app starts):**

1. Operator authenticates with broker: `POST /v1/admin/auth` with `AA_ADMIN_SECRET`
2. Operator creates a launch token for the app:
   ```
   POST /v1/admin/launch-tokens
   {
     "agent_name": "my-data-processor",
     "allowed_scope": ["read:customers:*", "write:reports:*"],
     "max_ttl": 600,
     "single_use": true,
     "ttl": 3600
   }
   ```
   Response: `{ "launch_token": "a1b2c3d4..." }` (64 hex chars, cryptographically random)
3. Operator delivers launch token to app via secure channel:
   - Kubernetes: `Secret` resource mounted as env var `AA_LAUNCH_TOKEN`
   - Cloud: Secrets Manager (AWS SSM, Azure Key Vault, GCP Secret Manager)
   - CI/CD: Pipeline secret injected at deploy time
   - This is the ONLY secret the app needs at startup

**Phase B -- App registers with the broker (first startup):**

4. App generates a fresh Ed25519 keypair locally (in-memory, private key never leaves process)
5. App gets a one-time nonce: `GET /v1/challenge` (30-second TTL, single-use)
6. App signs the nonce with its Ed25519 private key
7. App calls `POST /v1/register` with:
   ```json
   {
     "launch_token": "a1b2c3d4...",
     "nonce": "f8e7d6c5...",
     "public_key": "<base64 Ed25519 public key>",
     "signature": "<base64 signature of nonce>",
     "orch_id": "data-pipeline-v2",
     "task_id": "daily-report-20260220",
     "requested_scope": ["read:customers:active"]
   }
   ```
8. Broker validates (10-step process in `id_svc.go:Register()`):
   - Validates required fields
   - Looks up launch token, checks not expired/consumed
   - Enforces scope attenuation: `requested_scope` must be subset of `allowed_scope`
   - Consumes nonce (one-time use)
   - Validates Ed25519 public key (32 bytes)
   - Verifies nonce signature against public key
   - Consumes launch token (if single-use)
   - Generates SPIFFE ID: `spiffe://{domain}/agent/{orch_id}/{task_id}/{instance_id}`
   - Issues JWT with granted scope, SPIFFE subject, configured TTL
   - Persists agent record to SQLite
9. Broker returns: `{agent_id: "spiffe://...", access_token: "eyJ...", expires_in: 300}`

**Phase C -- App operates (ongoing):**

10. App uses `access_token` as Bearer token for all authenticated API calls
11. App renews before expiry: `POST /v1/token/renew` with current Bearer token
12. App releases when done: `POST /v1/token/release` (Fix 4)

**Security properties:** Single-use launch tokens consumed on registration (no reuse). Ed25519 challenge-response proves key possession (broker never sees private key). Scope attenuation enforced. Nonce one-time-use with 30s TTL prevents replay. JWT has short TTL (default 5 min) bound to SPIFFE identity.

#### Sidecar-Mediated Onboarding (Recommended for Production)

The sidecar automates the entire flow. The app never sees any secrets:

1. Operator deploys sidecar with `AA_ADMIN_SECRET` and `AA_SIDECAR_SCOPE_CEILING`
2. Sidecar auto-bootstraps: admin auth -> activation token -> activate -> sidecar credential
3. App requests tokens from sidecar (local HTTP or UDS) -> sidecar exchanges with broker

With Fix 5 (UDS), App-to-Sidecar uses Unix domain socket. Bootstrap logic unchanged.

#### Orchestrator-Managed Onboarding (Multi-Agent)

1. Orchestrator (admin-scoped) creates per-agent single-use launch tokens dynamically
2. Spawns agent with launch token injected
3. Agent executes broker-direct flow above

#### What This Design Changes About Onboarding

No new code for onboarding flows. All three work on develop today. Fixes enhance them: Fix 1 (mTLS) secures registration transport, Fix 3 (audience validation) audience-binds tokens, Fix 4 (token release) adds completion signal, Fix 5 (UDS) changes only App-to-Sidecar transport.

---

### Fix 6: Structured Audit Log Fields

**Problem:** `AuditEvent.Detail` is a free-form string. Kilo flagged.

**Design:** Extend `AuditEvent` with `Resource`, `Outcome`, `DelegDepth`, `DelegChainHash`, `BytesTransferred`. Use functional options for backward compatibility.

**SQLite migration:** `ALTER TABLE ... ADD COLUMN ... DEFAULT NULL` per field. No backfill.

**Files:** `internal/audit/audit_log.go`, `internal/store/sql_store.go`, ~6 callers
**Dependencies:** High touch count. Do last.

---

## Design Decisions

### Decision 1: UDS per-app sidecars over shared gateway
Shared gateway introduces cross-app blast radius. UDS eliminates port sprawl without new trust boundaries.

### Decision 2: File-based TLS over SPIRE integration
SPIRE adds infrastructure dependency. File-based TLS closes compliance immediately. Config is certificate-source-agnostic for future SPIRE.

### Decision 3: Token-level release over task-level release
Finer-grained control. Task-level revocation available via existing `POST /v1/revoke`.

### Decision 4: Functional options for audit Record()
Backward compatible. Existing callers unchanged. Idiomatic Go.

### Decision 5: No expires_at in revocations table
Security correctness over table size. An expires_at column creates a gap: if cleanup runs before the revoked token's exp elapses, the token becomes valid again. Revocation entries are permanent until a future PR implements safe TTL-aware cleanup.

### Decision 6: New audit fields included in hash computation
Tamper evidence requires all structured fields to be covered by the hash. Excluding new fields would create gaps that bypass chain integrity. The schema version boundary (pre/post migration) is documented in the operator runbook.

### Decision 7: Audience validation opt-in via AA_AUDIENCE
Default AA_AUDIENCE="agentauth" for new deployments. Upgrading deployments set AA_AUDIENCE="" during the token rollover window, then enable enforcement. Ensures backward compatibility without a flag day.

### Decision 8: Ephemeral signing key deferred, not hidden
The key restart limitation is explicitly documented (not silently accepted) so operators understand the guarantee gap and design agents accordingly.

---

## Appendix A: Compliance Review Cross-Reference

| Reviewer | Total | COMPLIANT | PARTIAL | NOT COMPLIANT |
|----------|-------|-----------|---------|---------------|
| India | 25 | 24 | 1 | 0 |
| Juliet | 29 | 26 | 3 | 0 |
| Kilo | 28 | 25 | 3 | 0 |
| Lima | 25 | 23 | 2 | 0 |

## Appendix B: Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| TLS config errors | Medium | High | Docker stack testing; setup guide |
| Audit hash schema boundary | Low | Medium | New fields included in hash; document migration timestamp in runbook |
| UDS permissions wrong | Low | Medium | Default 0660; document K8s securityContext.runAsGroup |
| Revocation table growth | Low | Low | No expires_at by design; safe cleanup deferred to future PR |
| Audience migration breaks existing tokens | Medium | Medium | Set AA_AUDIENCE="" during rollout window; enable after tokens cycle |
| Ephemeral key on broker restart | Medium | Medium | Agents retry on 401; short TTL bounds impact window |

---

**END OF DESIGN SOLUTION**
