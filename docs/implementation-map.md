# Implementation Map — Ephemeral Agent Credentialing v1.3

> **Purpose:** Trace every component of the [Ephemeral Agent Credentialing v1.3](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.3.md) pattern to the exact Go file, function, and HTTP endpoint that implements it.
>
> **Audience:** Contributors, security reviewers, and anyone who needs to verify that AgentWrit delivers what the pattern specifies.

---

## Component 1: Ephemeral Identity Issuance

**Pattern requirement:** Each agent instance receives a unique, cryptographically-bound identity through a challenge-response flow.

### Code Path

```
GET /v1/challenge
  → internal/handler/challenge_hdl.go:ServeHTTP()
    → internal/store/sql_store.go:CreateNonce()
      Creates 64-char hex nonce, 30s TTL, single-use
    ← Returns {nonce, expires_in}

POST /v1/register
  → internal/handler/reg_hdl.go:ServeHTTP()
    → internal/identity/id_svc.go:Register()
      Step 1:  Validate required fields (launch_token, nonce, public_key, signature, orch_id, task_id, requested_scope)
      Step 2:  store.GetLaunchToken(token) — look up and validate
      Step 3:  authz.ScopeIsSubset(requested, allowed) — enforce ceiling BEFORE consuming token
      Step 4:  store.ConsumeNonce(nonce) — single-use enforcement
      Step 5:  base64.StdEncoding.DecodeString(req.PublicKey) — decode Ed25519 public key
      Step 6:  ed25519.Verify(pubKey, nonceBytes, sigBytes) — cryptographic proof of key ownership
      Step 7:  store.ConsumeLaunchToken(token) — if single_use
      Step 8:  identity.NewSpiffeId(trustDomain, orchID, taskID, instanceID)
              Format: spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}
      Step 9:  token.TknSvc.Issue() — EdDSA JWT with sub=SPIFFE_ID, scope, exp, jti
      Step 10: store.SaveAgent() — persist agent record
    ← Returns {agent_id, access_token, expires_in}
```

### Key Files

| File | What it does |
|------|-------------|
| `internal/identity/id_svc.go:123` | `Register()` — 10-step registration flow |
| `internal/identity/id_svc.go:80` | `NewSpiffeId()` — SPIFFE URI generation |
| `internal/handler/challenge_hdl.go` | HTTP handler for `GET /v1/challenge` |
| `internal/handler/reg_hdl.go` | HTTP handler for `POST /v1/register` |
| `internal/store/sql_store.go` | Nonce and launch token storage |

### Security Properties

- Nonces are single-use and expire in 30 seconds (replay prevention)
- Scope check happens before launch token consumption (no wasted tokens on policy violations)
- Ed25519 signature proves the agent holds the private key corresponding to the submitted public key
- Each agent gets a unique SPIFFE ID encoding its orchestration context

---

## Component 2: Short-Lived Task-Scoped Tokens

**Pattern requirement:** Tokens are short-lived (minutes, not hours), scoped to specific resources, and carry task context.

### Code Path

```
Token Issuance (called by Register, Renew, admin auth, app auth):
  → internal/token/tkn_svc.go:Issue()
    1. TTL resolution: req.TTL > 0 ? req.TTL : cfg.DefaultTTL (300s)
    2. MaxTTL clamping: if cfg.MaxTTL > 0 && ttl > cfg.MaxTTL → ttl = cfg.MaxTTL
    3. Generate JTI: 16 random bytes → 32-char hex string
    4. Build TknClaims: iss, sub, aud, exp, nbf, iat, jti, scope, task_id, orch_id, sid, delegation_chain, chain_hash
    5. Sign: EdDSA (Ed25519) over base64url(header).base64url(claims)
    6. JWT header includes kid (RFC 7638 JWK Thumbprint of signing public key)
    ← Returns {access_token, expires_in, token_type, claims}

Token Verification (called on every authenticated request):
  → internal/token/tkn_svc.go:Verify()
    1. Format check: split on ".", must have 3 parts
    2. Algorithm check: header.alg must be "EdDSA" (prevents CVE-2015-9235)
    3. Key ID check: if header.kid present, must match broker's kid (prevents cross-broker replay)
    4. Signature check: ed25519.Verify(pubKey, signingInput, sigBytes)
    5. Claims decode and validate: iss="agentwrit", sub non-empty, jti non-empty, exp>0, exp>now, nbf<=now
    6. Revocation check: revoker.IsRevoked(claims) — defense-in-depth
    ← Returns TknClaims or error

Token Renewal:
  → internal/token/tkn_svc.go:Renew()
    1. Verify(tokenStr) — full verification pipeline
    2. revoker.RevokeByJTI(claims.Jti) — revoke predecessor BEFORE issuing new token
    3. Compute originalTTL = claims.Exp - claims.Iat (preserve launch-time TTL)
    4. Issue(TTL: originalTTL) — new token with same sub, scope, task, orch, delegation chain, TTL
       MaxTTL clamp still applies in Issue()
    ← Returns new IssueResp
```

### Key Files

| File | What it does |
|------|-------------|
| `internal/token/tkn_svc.go:105` | `Issue()` — token creation with TTL clamping |
| `internal/token/tkn_svc.go:158` | `Verify()` — 6-step verification pipeline |
| `internal/token/tkn_svc.go:218` | `Renew()` — predecessor revocation + reissuance (returns generic `"token renewal failed"` on error) |
| `internal/token/tkn_svc.go:59` | `computeKid()` — RFC 7638 JWK Thumbprint |
| `internal/token/tkn_claims.go:61` | `Validate()` — claim-level checks (exp, nbf, iss, sub, jti) |
| `internal/token/revoker.go` | `Revoker` interface — breaks circular dependency |
| `internal/cfg/cfg.go:63` | `MaxTTL` field — `AA_MAX_TTL` env var (default 86400) |

### Configuration

| Env Var | Default | Effect |
|---------|---------|--------|
| `AA_DEFAULT_TTL` | 300 | Default token lifetime in seconds |
| `AA_MAX_TTL` | 86400 | Hard ceiling on all token lifetimes (0 = no ceiling) |
| `AA_SIGNING_KEY_PATH` | `./signing.key` | Ed25519 key file (persistent across restarts) |

---

## Component 3: Zero-Trust Enforcement

**Pattern requirement:** Every request is validated. No trust based on network position or prior authentication.

### Code Path

```
Every authenticated HTTP request:
  → internal/authz/val_mw.go:Wrap()
    1. Extract "Authorization: Bearer <token>" header
    2. Call TknSvc.Verify(token) — full 6-step pipeline (Component 2)
    3. If revoker wired: revSvc.IsRevoked(claims) — second revocation check
    4. If audience configured: check claims.Aud contains broker audience
    5. Inject claims into request context
    → next handler

Scope enforcement (on endpoints requiring specific scopes):
  → internal/authz/val_mw.go:RequireScope()
    1. Extract claims from context
    2. authz.ScopeIsSubset([]string{required}, claims.Scope)
    3. If insufficient: 403 + audit event (scope_violation)

  → internal/authz/val_mw.go:RequireAnyScope()
    1. Extract claims from context
    2. Check if any required scope matches any token scope
    3. Used on POST /v1/admin/launch-tokens (admin OR app scope)

Rate limiting:
  → internal/authz/rate_mw.go:RateLimiter
    POST /v1/admin/auth — 5 req/s, burst 10, per IP
    POST /v1/app/auth — 10 req/min, burst 3, per client_id
```

### Key Files

| File | What it does |
|------|-------------|
| `internal/authz/val_mw.go:62` | `Wrap()` — token verification middleware (returns generic `"token verification failed"` on error) |
| `internal/authz/val_mw.go:130` | `RequireScope()` — single scope enforcement |
| `internal/authz/val_mw.go:158` | `RequireAnyScope()` — multi-scope enforcement |
| `internal/authz/scope.go` | `ScopeIsSubset()` — scope comparison logic |
| `internal/authz/rate_mw.go` | `RateLimiter` — per-IP and per-key rate limiting |
| `internal/handler/security_hdl.go` | `SecurityHeaders` — global middleware: `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `X-Frame-Options: DENY`, HSTS when TLS enabled. |
| `internal/problemdetails/problemdetails.go` | `MaxBytesBody` — global middleware: 1 MB request body limit on all endpoints. Returns HTTP 413 for oversized requests. |

### Scope Format

```
action:resource:identifier
```

Examples: `read:data:*`, `admin:revoke:*`, `app:launch-tokens:*`

Wildcard `*` in identifier covers any specific value. Scopes can only narrow, never expand.

---

## Component 4: Automatic Expiration & Revocation

**Pattern requirement:** Credentials automatically expire and can be revoked at multiple granularity levels.

### Code Path

```
Four-level revocation:
  → internal/revoke/rev_svc.go:Revoke(level, target)
    level="token" → revoke single JTI
    level="agent" → revoke all tokens for a SPIFFE ID
    level="task"  → revoke all tokens for a task_id
    level="chain" → revoke all tokens in a delegation tree (root delegator)

  → internal/revoke/rev_svc.go:IsRevoked(claims)
    Check order: JTI → agent (sub) → task (task_id) → chain (delegation_chain[0].agent)

Revocation inside Verify() (defense-in-depth):
  → internal/token/tkn_svc.go:Verify() step 6
    revoker.IsRevoked(claims) — catches revoked tokens even if middleware is bypassed

Predecessor revocation on renewal:
  → internal/token/tkn_svc.go:Renew()
    revoker.RevokeByJTI(oldJTI) BEFORE Issue(newToken)
    If revocation fails, renewal fails — no orphaned valid tokens

Agent self-revocation:
  → internal/handler/release_hdl.go:ServeHTTP()
    POST /v1/token/release — agent revokes its own JTI (task completion signal)

JTI pruning (background):
  → cmd/broker/main.go (goroutine, 60s ticker)
    store.PruneExpiredJTIs() — removes expired JTIs from memory
    store.ExpireAgents() — removes expired agent records
```

### Key Files

| File | What it does |
|------|-------------|
| `internal/revoke/rev_svc.go:104` | `Revoke()` — 4-level revocation with SQLite persistence |
| `internal/revoke/rev_svc.go:67` | `IsRevoked()` — checks all 4 levels |
| `internal/revoke/rev_svc.go:133` | `RevokeByJTI()` — implements `token.Revoker` interface |
| `internal/token/revoker.go` | `Revoker` interface — `RevokeByJTI()` + `IsRevoked()` |
| `internal/handler/release_hdl.go` | `POST /v1/token/release` — self-revocation |
| `internal/handler/revoke_hdl.go` | `POST /v1/revoke` — admin revocation |

---

## Component 5: Immutable Audit Logging

**Pattern requirement:** Every security-relevant action is recorded in a tamper-evident, append-only audit trail.

### Code Path

```
Recording an event:
  → internal/audit/audit_log.go:Record()
    1. Create AuditEvent with all fields (type, agent_id, task_id, orch_id, detail, outcome, resource, etc.)
    2. PII sanitization: redact values matching "secret", "password", "token_value", "private_key"
    3. Compute SHA-256 hash: H(prev_hash || event_type || agent_id || timestamp || detail)
    4. Link to previous event's hash (chain integrity)
    5. Persist to SQLite via store.SaveAuditEvent()

Querying events:
  → internal/handler/audit_hdl.go:ServeHTTP()
    GET /v1/audit/events?agent_id=X&task_id=Y&event_type=Z&outcome=success&since=T&until=T&limit=100&offset=0
    → audit.AuditLog.Query(filters)
    ← Returns paginated AuditEvent array
```

### 25 Event Types

| Category | Events |
|----------|--------|
| Admin | `admin_auth`, `admin_auth_failed` |
| Launch tokens | `launch_token_issued`, `launch_token_denied` |
| Registration | `agent_registered`, `registration_policy_violation` |
| Token lifecycle | `token_issued`, `token_revoked`, `token_renewed`, `token_released`, `token_renewal_failed` |
| Enforcement | `token_auth_failed`, `token_revoked_access`, `scope_violation`, `scope_ceiling_exceeded`, `delegation_attenuation_violation` |
| Delegation | `delegation_created` |
| Resource access | `resource_accessed` |
| App lifecycle | `app_registered`, `app_authenticated`, `app_auth_failed`, `app_updated`, `app_deregistered`, `app_rate_limited` |
| Config | `scopes_ceiling_updated` |

### Key Files

| File | What it does |
|------|-------------|
| `internal/audit/audit_log.go:164` | `Record()` — append event with hash chain |
| `internal/audit/audit_log.go:29-56` | 25 event type constants |
| `internal/audit/audit_log.go:62` | `AuditEvent` struct — 14 fields including hash chain |
| `internal/handler/audit_hdl.go` | `GET /v1/audit/events` — query with filters |

### Tamper Evidence

Each event contains `hash` (SHA-256 of current event) and `prev_hash` (hash of previous event). Breaking any event in the chain is detectable by recomputing hashes from the beginning.

---

## Component 6: Agent-to-Agent Mutual Authentication

**Pattern requirement:** Agents can verify each other's identity before sharing data or delegating work.

### Code Path

```
3-step handshake (Go API only — not HTTP-exposed):

Step 1 — Initiate:
  → internal/mutauth/mut_auth_hdl.go:InitiateHandshake(initiatorToken, targetAgentID)
    1. Verify initiator's token
    2. Look up target agent in DiscoveryRegistry
    3. Generate handshake nonce
    ← Returns HandshakeReq (nonce, initiator identity)

Step 2 — Respond:
  → internal/mutauth/mut_auth_hdl.go:RespondToHandshake(responderToken, handshakeReq)
    1. Verify responder's token
    2. Verify handshake nonce
    3. Sign nonce with responder's identity
    ← Returns HandshakeResp (signed nonce, responder identity)

Step 3 — Complete:
  → internal/mutauth/mut_auth_hdl.go:CompleteHandshake(handshakeResp)
    1. Verify responder's signature
    2. Confirm identities match
    ← Returns MutualAuthResult (both identities verified)
```

### Key Files

| File | What it does |
|------|-------------|
| `internal/mutauth/mut_auth_hdl.go:91` | `InitiateHandshake()` — step 1 |
| `internal/mutauth/mut_auth_hdl.go` | `RespondToHandshake()` — step 2 |
| `internal/mutauth/mut_auth_hdl.go` | `CompleteHandshake()` — step 3 |
| `internal/mutauth/discovery.go` | `DiscoveryRegistry` — agent lookup |
| `internal/mutauth/heartbeat.go` | `HeartbeatMgr` — liveness monitoring |

### Status

Implemented as a Go API. Not HTTP-exposed. Intended for future HTTP endpoint registration. Fully tested in unit tests.

---

## Component 7: Delegation Chain Verification

**Pattern requirement:** Agents can delegate scoped access to other agents with cryptographic proof of the authorization lineage.

### Code Path

```
POST /v1/delegate
  → internal/handler/deleg_hdl.go:ServeHTTP()
    → internal/deleg/deleg_svc.go:Delegate()
      1. Verify caller's token
      2. Check delegation depth (max 5 hops)
      3. Enforce scope attenuation: ScopeIsSubset(requested, caller's scope)
      4. Build DelegRecord: {agent, scope, delegated_at, signature}
         Signed with broker's Ed25519 key
      5. Append to delegation_chain
      6. Compute chain_hash: SHA-256 of JSON-serialized chain
      7. Issue new token with narrowed scope + updated chain + chain_hash
      ← Returns {access_token, expires_in, delegation_chain}
```

### Key Files

| File | What it does |
|------|-------------|
| `internal/deleg/deleg_svc.go:82` | `Delegate()` — scope attenuation + chain building |
| `internal/token/tkn_claims.go:47` | `DelegRecord` struct — chain entry |
| `internal/handler/deleg_hdl.go` | `POST /v1/delegate` — HTTP handler |

### Security Properties

- Scope can only narrow (attenuation is one-way)
- Max delegation depth: 5 hops
- Each chain entry is signed with the broker's Ed25519 key
- Chain hash embedded in JWT prevents chain tampering
- Chain-level revocation (`POST /v1/revoke level=chain`) invalidates all tokens in a delegation tree

---

## Component 8: Operational Observability

**Pattern requirement:** The credential system's operational state must be visible to monitoring systems.

### Code Path

```
Health check:
  GET /v1/health
  → internal/handler/health_hdl.go:ServeHTTP()
    ← Returns {status:"ok", version:"2.0.0", uptime:N, db_connected:bool, audit_events_count:N}

Prometheus metrics:
  GET /v1/metrics
  → internal/handler/metrics_hdl.go (promhttp.Handler)

Structured logging:
  → internal/obs/obs.go
    Ok(module, component, message, ...fields)
    Warn(module, component, message, ...fields)
    Fail(module, component, message, ...fields)
    Trace(module, component, message, ...fields)

Error responses:
  → internal/problemdetails/problemdetails.go
    RFC 7807 application/problem+json on every error
    Includes request_id for correlation with broker logs
```

### 12 Prometheus Metrics

| Metric | Type | Labels | What it measures |
|--------|------|--------|-----------------|
| `agentwrit_tokens_issued_total` | Counter | `scope` | Token issuance rate by primary scope |
| `agentwrit_tokens_revoked_total` | Counter | `level` | Revocation rate by level |
| `agentwrit_registrations_total` | Counter | `status` | Agent registration success/failure |
| `agentwrit_admin_auth_total` | Counter | `status` | Admin login success/failure |
| `agentwrit_launch_tokens_created_total` | Counter | — | Launch token creation rate |
| `agentwrit_active_agents` | Gauge | — | Currently registered agents |
| `agentwrit_request_duration_seconds` | Histogram | `endpoint` | Request latency per endpoint |
| `agentwrit_clock_skew_total` | Counter | — | Clock drift events |
| `agentwrit_audit_events_total` | Counter | — | Audit trail growth |
| `agentwrit_audit_write_duration_seconds` | Histogram | — | Audit write latency |
| `agentwrit_db_errors_total` | Counter | — | Database error rate |
| `agentwrit_audit_events_loaded` | Gauge | — | Events loaded from SQLite on startup |

### Key Files

| File | What it does |
|------|-------------|
| `internal/obs/obs.go` | Structured logging + all 12 Prometheus metric definitions |
| `internal/handler/health_hdl.go` | `GET /v1/health` — broker health with DB and audit status |
| `internal/handler/metrics_hdl.go` | `GET /v1/metrics` — Prometheus scrape endpoint |
| `internal/problemdetails/problemdetails.go` | RFC 7807 errors with request IDs |

---

## End-to-End: All 8 Components in a Single Request

Here's what happens when an agent makes an authenticated API call, showing every component that fires:

```
Agent → POST /v1/delegate (with Bearer token, requesting scope narrowing)

Component 8: LoggingMiddleware records request start (obs)
Component 8: RequestIDMiddleware assigns X-Request-ID (problemdetails)
Component 3: SecurityHeaders sets security response headers (handler/security_hdl.go)
Component 3: MaxBytesBody enforces 1 MB body limit (problemdetails/problemdetails.go)
Component 3: ValMw.Wrap() extracts Bearer token
Component 2: TknSvc.Verify() runs 6-step pipeline (format, alg, kid, sig, claims, revocation)
Component 4: Revoker.IsRevoked() checks 4 levels (inside Verify)
Component 3: ValMw checks audience
Component 3: Claims injected into context
Component 7: DelegSvc.Delegate() checks depth, enforces scope attenuation, builds chain
Component 2: TknSvc.Issue() creates new token with narrowed scope
Component 5: AuditLog.Record("delegation_created", ...) with hash chain
Component 8: LoggingMiddleware records response (status, latency)
Component 8: Prometheus metrics updated (request_duration, tokens_issued)

← 200 OK {access_token, expires_in, delegation_chain}
```

Every component participates in every authenticated request. The pattern is not 8 separate features — it's 8 layers that compose on every operation.
