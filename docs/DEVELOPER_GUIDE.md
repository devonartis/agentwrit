# AgentAuth Developer Guide

This guide covers the internals of the AgentAuth broker. After reading it you should be able to navigate the codebase, trace any request end-to-end, write new endpoints, and extend the system with confidence.

If you need a step-by-step app integration walkthrough (Python/TypeScript), start with [Agent Integration Guide](AGENT_INTEGRATION_GUIDE.md) first, then return here for internals.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Package Dependency Graph](#2-package-dependency-graph)
3. [Package Walkthrough](#3-package-walkthrough)
4. [Request Lifecycle](#4-request-lifecycle)
5. [Bootstrap Flow](#5-bootstrap-flow)
6. [Token Lifecycle](#6-token-lifecycle)
7. [Scope System](#7-scope-system)
8. [SPIFFE ID Generation](#8-spiffe-id-generation)
9. [Audit System](#9-audit-system)
10. [Delegation](#10-delegation)
11. [Middleware Pipeline](#11-middleware-pipeline)
12. [Error Handling](#12-error-handling)
13. [Configuration](#13-configuration)
14. [Testing Patterns](#14-testing-patterns)
15. [How to Add a New Endpoint](#15-how-to-add-a-new-endpoint)
16. [How to Add a New Audit Event Type](#16-how-to-add-a-new-audit-event-type)

---

## 1. Architecture Overview

AgentAuth is a broker-based credential issuance service. Four components interact across explicit trust boundaries:

```
                              TRUST BOUNDARY
                    ================================
                    |                              |
                    |         +------------+       |
                    |         |   Broker   |       |
                    |         |    (Go)    |       |
                    |         +-----+------+       |
                    |           ^   ^   ^          |
                    ================================
                    admin/auth  |   |   |  challenge/register
                    launch-tkn  |   |   |  validate/renew/revoke
                                |   |   |
               +----------------+   |   +----------------+
               |                    |                    |
         +-----+------+     +------+-----+     +--------+-----+
         |Orchestrator |     |  Agent(s)  |     |   Resource   |
         |  (Python)   |     |  (Python)  |     |    Server    |
         +-------------+     +-----+------+     |   (FastAPI)  |
                                   |            +--------------+
                                   |                    ^
                                   +---- Bearer JWT ----+
```

| Component | Language | Trust Level |
|-----------|----------|-------------|
| **Broker** | Go | Root of trust. Compromise = total system compromise. |
| **Orchestrator** | Any | Trusted to request appropriate credentials, bounded by policy. |
| **Agent** | Python | NOT trusted beyond issued scope. Every access validated independently. |
| **Resource Server** | Python/FastAPI | Trusts broker signatures, independently validates claims. |

The broker exposes a single HTTP port (default `:8080`). All tokens are EdDSA-signed JWTs. All errors use RFC 7807. All state transitions are audit-logged.

**Note on TLS:** The broker listens on plain HTTP by default. Production deployments MUST use a TLS-terminating reverse proxy (e.g., nginx, envoy, Caddy) or configure a load balancer with TLS termination. Native TLS support (`AA_TLS_CERT`, `AA_TLS_KEY`) is planned for a future release.

---

## 2. Package Dependency Graph

```
cmd/broker/main.go
    |
    +---> cfg           (config: reads AA_* env vars)
    +---> obs           (logging + Prometheus metrics)
    +---> store         (in-memory storage, mutex-protected maps)
    +---> audit         (hash-chain audit trail)
    +---> token         (JWT issue/verify/renew, uses cfg)
    +---> revoke        (4-level revocation, uses token for claims type)
    +---> identity      (registration, SPIFFE IDs; uses store, token, authz, obs, audit)
    +---> authz         (scope parsing + ValMw middleware; uses token, revoke)
    +---> deleg         (delegation; uses token, store, audit, authz)
    +---> admin         (admin auth + launch tokens; uses token, store, audit, authz, obs)
    +---> handler       (HTTP handlers; uses identity, token, authz, revoke, audit, deleg, obs)
```

Foundation packages (`cfg`, `obs`, `store`) have no internal dependencies. Everything else builds upward from them.

```
Layer 3 (HTTP)     handler/   admin/ (AdminHdl)
                       |          |
Layer 2 (Logic)    identity   deleg   admin/ (AdminSvc)   authz/ (ValMw)
                       |         |         |                  |
Layer 1 (Core)     token     store     audit     revoke     authz/ (scope)
                       |
Layer 0 (Infra)      cfg       obs
```

---

## 3. Package Walkthrough

### `cfg` -- Configuration
**File:** `internal/cfg/cfg.go`

Reads `AA_*` environment variables into a `Cfg` struct. Pure data, no side effects.

```go
type Cfg struct {
    Port        string // AA_PORT (default "8080")
    LogLevel    string // AA_LOG_LEVEL (default "verbose")
    TrustDomain string // AA_TRUST_DOMAIN (default "agentauth.local")
    DefaultTTL  int    // AA_DEFAULT_TTL (default 300)
    AdminSecret string // AA_ADMIN_SECRET (required for admin auth)
    SeedTokens  bool   // AA_SEED_TOKENS (dev-only convenience flag)
}
```

The `Load()` function uses `envOr()` and `envIntOr()` helpers that return defaults when env vars are unset.

### `obs` -- Observability
**File:** `internal/obs/obs.go`

Two responsibilities: structured logging and Prometheus metrics.

**Logging** uses four severity functions, all with the same signature `(module, component, msg string, ctx ...string)`:

| Function | Level | Output |
|----------|-------|--------|
| `obs.Ok` | Standard+ | stdout |
| `obs.Warn` | Standard+ | stdout |
| `obs.Fail` | All (except quiet) | stderr |
| `obs.Trace` | Trace only | stdout |

Log format: `[AA:MODULE:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | CONTEXT`

**Prometheus metrics** are declared as package-level vars using `promauto`:

| Metric | Type | Labels |
|--------|------|--------|
| `agentauth_tokens_issued_total` | Counter | scope |
| `agentauth_tokens_revoked_total` | Counter | level |
| `agentauth_registrations_total` | Counter | status |
| `agentauth_admin_auth_total` | Counter | status |
| `agentauth_launch_tokens_created_total` | Counter | -- |
| `agentauth_active_agents` | Gauge | -- |
| `agentauth_request_duration_seconds` | Histogram | endpoint |
| `agentauth_clock_skew_total` | Counter | -- |

### `store` -- Storage
**File:** `internal/store/sql_store.go`

`SqlStore` holds three in-memory maps behind a `sync.RWMutex`:

- `nonces` -- challenge nonces (32 random bytes, hex-encoded, 30s TTL)
- `launchTokens` -- launch token records with policy binding
- `agents` -- registered agent records with public keys

Key types:
- `LaunchTokenRecord` -- token value, allowed scope, max TTL, single-use flag, expiry, consumed-at timestamp
- `AgentRecord` -- SPIFFE ID, Ed25519 public key, orchestration context, granted scope

All write operations acquire the write lock. Read operations use the read lock. Nonces are cleaned up on expiry check (delete from map during `ConsumeNonce`).

### `token` -- Token Service
**Files:** `internal/token/tkn_claims.go`, `internal/token/tkn_svc.go`

`TknClaims` is the JWT payload. It holds standard claims (`iss`, `sub`, `exp`, `nbf`, `iat`, `jti`) plus AgentAuth-specific fields (`scope`, `task_id`, `orch_id`, `delegation_chain`).

`TknSvc` handles three operations:

1. **Issue** -- builds claims, JSON-encodes header+claims, base64url-encodes both, signs with Ed25519, returns `header.claims.signature`.
2. **Verify** -- splits on `.`, decodes signature, calls `ed25519.Verify`, decodes claims JSON, runs `claims.Validate()` (checks issuer, subject, JTI, expiry, nbf).
3. **Renew** -- verifies the existing token, then issues a new one with the same `sub`, `scope`, `task_id`, `orch_id`, and `delegation_chain` but fresh `iat`, `exp`, and `jti`.

Token IDs (JTI) are 16 bytes from `crypto/rand`, hex-encoded (32 characters).

The JWT uses a custom minimal implementation (not a third-party JWT library). The signing algorithm is EdDSA with Ed25519 keys. The header is fixed: `{"alg":"EdDSA","typ":"JWT"}`.

### `identity` -- Registration and SPIFFE
**Files:** `internal/identity/id_svc.go`, `internal/identity/spiffe.go`

`IdSvc` orchestrates the full agent registration flow. The `Register` method:

1. Validates all required fields are present
2. Looks up the launch token in the store
3. **Checks scope BEFORE consuming the launch token** (critical: on scope violation, the token remains available for retry)
4. Consumes the nonce (validates it exists, is not expired, not already used)
5. Decodes the Ed25519 public key from base64
6. Verifies the nonce signature against the public key
7. Consumes the launch token (marks it as used)
8. Generates a SPIFFE ID via `NewSpiffeId`
9. Issues a JWT via `TknSvc.Issue`
10. Saves the agent record to the store
11. Records two audit events: `agent_registered` and `token_issued`
12. Increments Prometheus metrics

SPIFFE IDs use the `go-spiffe/v2` library. Format: `spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}`. The `instanceID` is 8 random bytes, hex-encoded.

### `authz` -- Authorization
**Files:** `internal/authz/scope.go`, `internal/authz/val_mw.go`

Two distinct responsibilities in this package:

**Scope logic** (`scope.go`):
- `ParseScope(s)` splits `"action:resource:identifier"` into three parts
- `scopeCovers(requested, allowed)` checks if one scope entry covers another (wildcard `*` on identifier matches anything)
- `ScopeIsSubset(requested, allowed)` checks that every requested scope is covered by at least one allowed scope

**Validation middleware** (`val_mw.go`):
- `ValMw.Wrap(next)` extracts the `Authorization: Bearer <token>` header, calls `TknSvc.Verify`, checks revocation via `RevSvc.IsRevoked`, stores claims in request context
- `WithRequiredScope(scope, next)` pulls claims from context and checks the token's scope covers the required scope
- `ClaimsFromContext(ctx)` retrieves claims from the context (used by downstream handlers)
- `TokenFromRequest(r)` extracts the raw bearer token string

**Rate limiting** (`rate_mw.go`):
- `RateLimiter.Wrap(next)` is a per-IP token-bucket rate limiter. Used on `POST /v1/admin/auth` (5 req/s, burst 10) to mitigate brute-force attacks. Exceeding the limit returns `429 Too Many Requests` in RFC 7807 format with a `Retry-After: 1` header.
- `clientIP(r)` extracts the client IP, preferring `X-Forwarded-For` when present, falling back to `r.RemoteAddr`. **Trusted proxy assumption:** this function trusts `X-Forwarded-For` unconditionally. Production deployments must place the broker behind a reverse proxy that overwrites this header with the true client IP; direct internet exposure allows rate limit bypass via header spoofing.

These are composable. In the route table, admin endpoints chain both: `valMw.Wrap(authz.WithRequiredScope("admin:revoke:*", revokeHdl))`.

### `revoke` -- Revocation
**File:** `internal/revoke/rev_svc.go`

`RevSvc` maintains four in-memory sets (maps to `bool`), one per revocation level:

| Level | Key | Effect |
|-------|-----|--------|
| `token` | JTI | Revoke a single token |
| `agent` | SPIFFE ID (sub) | Revoke all tokens for an agent |
| `task` | Task ID | Revoke all tokens for a task |
| `chain` | Root delegator's agent ID (SPIFFE ID) | Revoke all tokens in a delegation chain |

`IsRevoked(claims)` checks all four maps in order: JTI, Sub, TaskId, then chain (if delegation chain is present, checks `DelegChain[0].Agent` -- the root delegator's agent ID). This runs inside `ValMw.Wrap` on every authenticated request.

### `audit` -- Audit Trail
**File:** `internal/audit/audit_log.go`

`AuditLog` maintains an append-only slice of `AuditEvent` entries with hash-chain integrity.

Each event's hash is `SHA-256(prevHash|id|timestamp|eventType|agentID|taskID|orchID|detail)`. The genesis event's `prev_hash` is 64 zeros.

PII sanitization runs on the `detail` field before storage. It masks values after keywords like "secret", "password", "token_value", "private_key" with `***REDACTED***`.

`Query(filters)` supports filtering by agent_id, task_id, event_type, since/until timestamps, plus limit/offset pagination (default limit 100, max 1000).

Twelve event type constants are defined (see [Section 9](#9-audit-system)).

### `deleg` -- Delegation
**File:** `internal/deleg/deleg_svc.go`

`DelegSvc.Delegate` performs scope-attenuated delegation:

1. Validates required fields
2. Checks delegation depth (current chain length < `maxDelegDepth` which is 5)
3. Checks scope attenuation: delegated scope MUST be subset of delegator's scope
4. Verifies the target agent exists in the store
5. Appends a new `DelegRecord` to the chain (delegator's SPIFFE ID, scope, timestamp)
6. Issues a new token with the delegate's SPIFFE ID as subject and the narrowed scope
7. Records a `delegation_created` audit event

### `admin` -- Admin Authentication
**Files:** `internal/admin/admin_svc.go`, `internal/admin/admin_hdl.go`

`AdminSvc` has two operations:
- `Authenticate(clientID, clientSecret)` -- constant-time comparison against `AA_ADMIN_SECRET`, issues admin JWT with scope `["admin:launch-tokens:*", "admin:revoke:*", "admin:audit:*"]` and 300s TTL
- `CreateLaunchToken(req, createdBy)` -- generates 32 random bytes (hex-encoded) as the opaque token, stores with policy binding (allowed scope, max TTL, single-use, expiry)

`AdminHdl` registers two routes via `RegisterRoutes(mux)`:
- `POST /v1/admin/auth` -- no auth required (this IS the auth endpoint)
- `POST /v1/admin/launch-tokens` -- wrapped with `valMw.Wrap` + `WithRequiredScope("admin:launch-tokens:*")`

### `handler` -- HTTP Handlers
**Files:** `internal/handler/*.go`

Each handler is a struct implementing `http.Handler`. Pattern: decode JSON, call service, encode response or `WriteProblem`.

| Handler | Route | Auth |
|---------|-------|------|
| `ChallengeHdl` | `GET /v1/challenge` | None |
| `RegHdl` | `POST /v1/register` | Launch token |
| `ValHdl` | `POST /v1/token/validate` | None |
| `RenewHdl` | `POST /v1/token/renew` | Bearer (via ValMw) |
| `RevokeHdl` | `POST /v1/revoke` | Bearer + admin scope |
| `DelegHdl` | `POST /v1/delegate` | Bearer (via ValMw) |
| `AuditHdl` | `GET /v1/audit/events` | Bearer + admin scope |
| `HealthHdl` | `GET /v1/health` | None |
| `MetricsHdl` | `GET /v1/metrics` | None (Prometheus) |

The `WriteProblem` helper in `problem.go` produces RFC 7807 JSON with `Content-Type: application/problem+json`.

The `admin` package defines its own unexported `writeProblem` variant in `internal/admin/admin_hdl.go` with an additional `title` parameter:

```go
func writeProblem(w http.ResponseWriter, status int, errType, title, detail, instance string)
```

Unlike the handler package's `WriteProblem` (which derives `title` from `http.StatusText(status)`), the admin variant accepts an explicit `title` string, allowing admin endpoints to provide custom error titles (e.g., `"Invalid Request"`, `"Unauthorized"`).

---

## 4. Request Lifecycle

Tracing `POST /v1/register` end-to-end:

```
Client                  net/http.ServeMux        RegHdl           IdSvc
  |                           |                    |                |
  |  POST /v1/register        |                    |                |
  |  Content-Type: app/json   |                    |                |
  |-------------------------->|                    |                |
  |                           | route match        |                |
  |                           |  (no middleware)    |                |
  |                           |------------------->|                |
  |                           |                    | json.Decode    |
  |                           |                    | req body       |
  |                           |                    |--------------->|
  |                           |                    |                | 1. Validate fields
  |                           |                    |                | 2. store.GetLaunchToken
  |                           |                    |                | 3. authz.ScopeIsSubset
  |                           |                    |                | 4. store.ConsumeNonce
  |                           |                    |                | 5. ed25519.Verify(sig)
  |                           |                    |                | 6. store.ConsumeLaunchToken
  |                           |                    |                | 7. NewSpiffeId
  |                           |                    |                | 8. tknSvc.Issue (JWT)
  |                           |                    |                | 9. store.SaveAgent
  |                           |                    |                | 10. auditLog.Record x2
  |                           |                    |<---------------|
  |                           |                    | json.Encode    |
  |                           |                    | {agent_id,     |
  |                           |                    |  access_token, |
  |                           |                    |  expires_in}   |
  |<--------------------------------------------------------|      |
```

For authenticated endpoints (e.g., `POST /v1/token/renew`), the middleware intercepts first:

```
Client              ServeMux         ValMw.Wrap       RenewHdl      TknSvc
  |                    |                |                |             |
  |  POST /renew       |                |                |             |
  |  Authorization:    |                |                |             |
  |  Bearer <jwt>      |                |                |             |
  |-------------------->                |                |             |
  |                    |--------------->|                |             |
  |                    |                | Extract Bearer |             |
  |                    |                | tknSvc.Verify  |------------>|
  |                    |                |                |<------------|
  |                    |                | revSvc.IsRevoked             |
  |                    |                | ctx = WithValue(claims)      |
  |                    |                |--------------->|             |
  |                    |                |                | Renew(token)|
  |                    |                |                |------------>|
  |                    |                |                |<------------|
  |<---------------------------------------------------------|       |
```

For admin endpoints, `ValMw.Wrap` runs first, then `WithRequiredScope` checks the scope claim:

```
ValMw.Wrap --> WithRequiredScope("admin:revoke:*") --> RevokeHdl
```

---

## 5. Bootstrap Flow

The complete credential flow from a cold start:

```
Step 1: Platform Admin
    Sets AA_ADMIN_SECRET env var, starts broker

Step 2: Orchestrator authenticates
    POST /v1/admin/auth
    {"client_id":"admin","client_secret":"<AA_ADMIN_SECRET>"}
    --> AdminSvc.Authenticate (constant-time compare)
    --> TknSvc.Issue (admin JWT, scope: admin:launch-tokens:*, admin:revoke:*, admin:audit:*)
    <-- 200 {access_token, expires_in: 300}

Step 3: Orchestrator provisions launch token
    POST /v1/admin/launch-tokens
    Authorization: Bearer <admin-jwt>
    {"agent_name":"data-reader","allowed_scope":["read:Customers:*"],"max_ttl":300}
    --> ValMw verifies admin JWT
    --> WithRequiredScope checks "admin:launch-tokens:*"
    --> AdminSvc.CreateLaunchToken (32 random bytes, hex-encoded)
    <-- 201 {launch_token, expires_at, policy}

Step 4: Agent gets challenge
    GET /v1/challenge
    --> store.CreateNonce (32 random bytes, hex, 30s TTL)
    <-- 200 {nonce, expires_in: 30}

Step 5: Agent registers
    POST /v1/register
    {"launch_token":"...","nonce":"...","public_key":"<b64>","signature":"<b64>",
     "orch_id":"orch-1","task_id":"task-1","requested_scope":["read:Customers:12345"]}
    --> IdSvc.Register (full validation pipeline)
    <-- 200 {agent_id: "spiffe://agentauth.local/agent/orch-1/task-1/<hex>",
             access_token: "<jwt>", expires_in: 300}

Step 6: Agent accesses resources
    GET /resource
    Authorization: Bearer <agent-jwt>
    --> Resource server validates JWT signature, expiry, scope, revocation
    <-- 200 {data}
```

Key invariants:
- The admin JWT can ONLY create launch tokens, revoke, and query audit. It cannot issue agent tokens.
- Launch tokens are single-use with a 30-second default TTL.
- `requested_scope` MUST be a subset of the launch token's `allowed_scope`. Scope can only narrow.

---

## 6. Token Lifecycle

### Issuance

`TknSvc.Issue` (`internal/token/tkn_svc.go`, `Issue` method):

1. Determine TTL: use `req.TTL` if positive, otherwise `cfg.DefaultTTL`
2. Generate JTI: 16 random bytes from `crypto/rand`, hex-encoded
3. Build `TknClaims` with `iss: "agentauth"`, timestamps, scope, metadata
4. Sign: JSON-encode header `{"alg":"EdDSA","typ":"JWT"}` and claims, base64url-encode both, concatenate with `.`, sign with Ed25519 private key, append base64url-encoded signature

### Verification

`TknSvc.Verify` (`internal/token/tkn_svc.go`, `Verify` method):

1. Split token on `.` into 3 parts (header, claims, signature)
2. Base64url-decode the signature
3. Call `ed25519.Verify(pubKey, signingInput, sigBytes)` -- signingInput is `header.claims`
4. Base64url-decode and JSON-unmarshal the claims
5. Run `claims.Validate()`: check `iss == "agentauth"`, `sub` not empty, `jti` not empty, not expired, not before `nbf`

### Renewal

`TknSvc.Renew` (`internal/token/tkn_svc.go`, `Renew` method):

1. Verify the existing token (full pipeline)
2. Issue a new token with same `sub`, `scope`, `task_id`, `orch_id`, `delegation_chain`
3. New token gets fresh `iat`, `exp`, `jti`

### Revocation

Revocation is checked by `ValMw` on every authenticated request. `RevSvc.IsRevoked` checks four maps in order: token JTI, agent SPIFFE ID, task ID, delegation chain root delegator's agent ID (`DelegChain[0].Agent`).

```
Issue --> Verify (on each request) --> Renew (extends lifetime) --> Revoke (kills it)
   |                                                                     |
   +-- JTI assigned                                         RevSvc.Revoke(level, target)
   +-- TTL set (exp = iat + ttl)                            level: token|agent|task|chain
```

---

## 7. Scope System

### Format

```
action:resource:identifier
```

Three colon-separated segments. All three must be non-empty. Examples:
- `read:Customers:12345` -- read customer 12345
- `read:Customers:*` -- read any customer (wildcard)
- `admin:launch-tokens:*` -- manage launch tokens

### Parsing

`authz.ParseScope` (`internal/authz/scope.go`, `ParseScope` function) splits on `:` using `strings.SplitN(s, ":", 3)`. Returns error if fewer than 3 parts or any part is empty.

### Subset Checking

`scopeCovers(requested, allowed)` (`internal/authz/scope.go`, `scopeCovers` function):
- `requested.action == allowed.action` AND
- `requested.resource == allowed.resource` AND
- (`requested.identifier == allowed.identifier` OR `allowed.identifier == "*"`)

The wildcard `*` only applies to the identifier segment. There is no wildcard for action or resource.

`ScopeIsSubset(requested, allowed)` (`internal/authz/scope.go`, `ScopeIsSubset` function) checks that every element in `requested` is covered by at least one element in `allowed`. This is an O(n*m) check.

### Enforcement Points

Scope is enforced at three points in the system:

1. **Agent registration** (`internal/identity/id_svc.go`, `Register` method): `requested_scope` vs launch token's `allowed_scope`
2. **Delegation** (`internal/deleg/deleg_svc.go`, `Delegate` method): delegated scope vs delegator's current scope
3. **Admin endpoints** (`internal/authz/val_mw.go`, `WithRequiredScope` function): checks token scope on every request

---

## 8. SPIFFE ID Generation

**File:** `internal/identity/spiffe.go`

Format: `spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}`

Example: `spiffe://agentauth.local/agent/orch-456/task-789/a1b2c3d4e5f6g7h8`

`NewSpiffeId` uses the `go-spiffe/v2` library:
1. Parse trust domain via `spiffeid.TrustDomainFromString`
2. Create ID via `spiffeid.FromSegments(td, "agent", orchID, taskID, instanceID)`
3. Return `.String()` representation

`ParseSpiffeId` reverses the process:
1. Parse with `spiffeid.FromString`
2. Extract path, split on `/`
3. Validate format: exactly 4 segments, first must be `"agent"`
4. Return `orchID`, `taskID`, `instanceID`

The `instanceID` is generated by `randomInstanceID()` in `id_svc.go`: 8 bytes from `crypto/rand`, hex-encoded (16 characters).

---

## 9. Audit System

**File:** `internal/audit/audit_log.go`

### Hash Chain

Each event's hash is computed as:

```
SHA-256(prevHash | id | timestamp | eventType | agentID | taskID | orchID | detail)
```

Fields are pipe-delimited. The genesis `prevHash` is 64 hex zeros. This creates a tamper-evident chain: modifying any historical event breaks the hash chain for all subsequent events.

### Event Types

| Constant | Value | When |
|----------|-------|------|
| `EventAdminAuth` | `admin_auth` | Successful admin authentication |
| `EventAdminAuthFailed` | `admin_auth_failed` | Failed admin authentication |
| `EventLaunchTokenIssued` | `launch_token_issued` | Launch token created |
| `EventLaunchTokenDenied` | `launch_token_denied` | Launch token denied |
| `EventAgentRegistered` | `agent_registered` | Agent registered |
| `EventRegistrationViolation` | `registration_policy_violation` | Scope violation at registration |
| `EventTokenIssued` | `token_issued` | Token issued |
| `EventTokenRevoked` | `token_revoked` | Token revoked |
| `EventTokenRenewed` | `token_renewed` | Token successfully renewed |
| `EventTokenRenewalFailed` | `token_renewal_failed` | Token renewal attempt failed |
| `EventDelegationCreated` | `delegation_created` | Delegation created |
| `EventResourceAccessed` | `resource_accessed` | Resource access logged |

### PII Sanitization

`sanitizePII` scans the detail string for keywords (`secret`, `password`, `token_value`, `private_key`). If found, `maskSensitiveValues` replaces the value portion after `=` or `: ` with `***REDACTED***`.

### Querying

`Query(filters)` accepts filtering by:
- `AgentID` -- exact match on SPIFFE ID
- `TaskID` -- exact match on task ID
- `EventType` -- exact match on event type string
- `Since` / `Until` -- time range (RFC 3339)
- `Limit` -- max results (default 100, max 1000)
- `Offset` -- pagination offset

### Recording Events

All services that record audit events accept an `AuditRecorder` interface:

```go
type AuditRecorder interface {
    Record(eventType, agentID, taskID, orchID, detail string)
}
```

Nil-check before recording is the standard pattern:

```go
if s.auditLog != nil {
    s.auditLog.Record(audit.EventAgentRegistered, agentID, taskID, orchID, detail)
}
```

---

## 10. Delegation

**File:** `internal/deleg/deleg_svc.go`

### Scope Attenuation

Scope can only narrow at each delegation hop. The check uses the same `authz.ScopeIsSubset` function as registration:

```
delegated_scope MUST be subset of delegator.scope
```

### Delegation Chain

The chain is an array of `DelegRecord` embedded in the JWT claims:

```go
type DelegRecord struct {
    Agent       string    `json:"agent"`        // SPIFFE ID of delegating agent
    Scope       []string  `json:"scope"`        // scope held by this agent at time of delegation
    DelegatedAt time.Time `json:"delegated_at"` // timestamp
    Signature   string    `json:"signature,omitempty"` // Ed25519 signature of "agent|scope_csv|delegated_at"
}
```

Each delegation appends the delegator's entry to the chain. The new token's `sub` is the delegate's SPIFFE ID. The `task_id` and `orch_id` are inherited from the delegator.

### Depth Limit

Maximum 5 hops (`maxDelegDepth = 5`). The check is `len(delegatorClaims.DelegChain) >= maxDelegDepth`.

### Chain Revocation

Revoking at level `"chain"` with the root delegator's agent ID (SPIFFE ID) as target will cause `RevSvc.IsRevoked` to return `true` for any token whose `DelegChain[0].Agent` matches the revoked target. This effectively kills all downstream delegated tokens in that delegation lineage.

---

## 11. Middleware Pipeline

### ValMw.Wrap

`ValMw.Wrap(next)` (`internal/authz/val_mw.go`, `Wrap` method) creates a handler that:

1. Reads `Authorization` header
2. Rejects if missing or not `Bearer` scheme (401)
3. Calls `tknSvc.Verify(token)` (401 on failure)
4. Calls `revSvc.IsRevoked(claims)` (403 if revoked)
5. Stores claims in context via `context.WithValue(r.Context(), claimsKey, claims)`
6. Calls `next.ServeHTTP(w, r.WithContext(ctx))`

### WithRequiredScope

`WithRequiredScope(scope, next)` (`internal/authz/val_mw.go`, `WithRequiredScope` function) is a standalone handler wrapper:

1. Pulls claims from context via `ClaimsFromContext`
2. Checks `ScopeIsSubset([]string{scope}, claims.Scope)` (403 if insufficient)
3. Calls `next.ServeHTTP(w, r)`

### Composition in main.go

Routes are wired in `cmd/broker/main.go`:

```go
// No auth
mux.Handle("GET /v1/challenge", challengeHdl)

// Bearer auth only
mux.Handle("POST /v1/token/renew", valMw.Wrap(renewHdl))

// Bearer + specific scope
mux.Handle("POST /v1/revoke",
    valMw.Wrap(authz.WithRequiredScope("admin:revoke:*", revokeHdl)))

// Admin routes registered separately
adminHdl.RegisterRoutes(mux)
```

Inside `AdminHdl.RegisterRoutes`:
```go
mux.HandleFunc("POST /v1/admin/auth", h.handleAuth)          // no auth (IS the auth endpoint)
mux.Handle("POST /v1/admin/launch-tokens",
    h.valMw.Wrap(authz.WithRequiredScope("admin:launch-tokens:*",
        http.HandlerFunc(h.handleCreateLaunchToken))))
```

### Context Propagation

Claims flow through the request context. Any handler downstream of `ValMw.Wrap` can retrieve them:

```go
claims := authz.ClaimsFromContext(r.Context())
if claims == nil {
    // not authenticated (should not happen if ValMw is in the chain)
}
// Use claims.Sub, claims.Scope, claims.TaskId, etc.
```

---

## 12. Error Handling

All errors use RFC 7807 `application/problem+json`.

### WriteProblem Helper

**File:** `internal/handler/problem.go`

```go
func WriteProblem(w http.ResponseWriter, status int, errType, detail, instance string) {
    w.Header().Set("Content-Type", "application/problem+json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(ProblemDetail{
        Type:     "urn:agentauth:error:" + errType,
        Title:    http.StatusText(status),
        Status:   status,
        Detail:   detail,
        Instance: instance,
    })
}
```

### Error Type Mapping

Handlers map domain errors to HTTP status codes and error types:

| Domain Error | HTTP Status | Error Type |
|-------------|-------------|------------|
| `ErrMissingField` | 400 | `invalid_request` |
| `ErrScopeViolation` | 403 | `scope_violation` |
| `ErrTokenNotFound`, `ErrTokenExpired` | 401 | `unauthorized` |
| `ErrInvalidSignature` | 401 | `unauthorized` |
| `ErrInvalidLevel` | 400 | `invalid_request` |
| `ErrDepthExceeded` | 403 | `scope_violation` |
| `ErrDelegateNotFound` | 404 | `not_found` |
| (any unexpected) | 500 | `internal_error` |

### Error Propagation Pattern

Services return sentinel errors (declared with `errors.New`). Handlers use `errors.Is` to match:

```go
switch {
case errors.Is(err, identity.ErrMissingField):
    WriteProblem(w, http.StatusBadRequest, "invalid_request", err.Error(), r.URL.Path)
case errors.Is(err, identity.ErrScopeViolation):
    WriteProblem(w, http.StatusForbidden, "scope_violation", err.Error(), r.URL.Path)
// ...
}
```

Services that wrap errors use `fmt.Errorf("%w: ...", sentinel)` so `errors.Is` continues to work through the wrapping.

---

## 13. Configuration

All environment variables use the `AA_` prefix:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `AA_PORT` | string | `"8080"` | HTTP port |
| `AA_LOG_LEVEL` | string | `"verbose"` | Log level: `quiet`, `standard`, `verbose`, `trace` |
| `AA_TRUST_DOMAIN` | string | `"agentauth.local"` | SPIFFE trust domain |
| `AA_DEFAULT_TTL` | int | `300` | Default token TTL in seconds |
| `AA_ADMIN_SECRET` | string | (required) | Pre-shared secret for admin authentication |
| `AA_SEED_TOKENS` | bool | `false` | Dev-only: print seed launch and admin tokens to stdout on startup |

Configuration is loaded once at startup by `cfg.Load()` and passed to services that need it (primarily `TknSvc` for `DefaultTTL` and `IdSvc` for `TrustDomain`).

---

## 14. Testing Patterns

### Test Organization

Tests live alongside implementation files with the `_test.go` suffix. They use the same package (not `_test` external package), so they have access to unexported symbols.

### Test Helpers

Common pattern: a setup function that creates a full service stack:

```go
func setupIdSvc(t *testing.T) (*IdSvc, *store.SqlStore, *audit.AuditLog) {
    t.Helper()
    sqlStore := store.NewSqlStore()
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    c := cfg.Cfg{Port: "8080", LogLevel: "quiet", TrustDomain: "agentauth.local", DefaultTTL: 300}
    tknSvc := token.NewTknSvc(priv, pub, c)
    auditLog := audit.NewAuditLog()
    idSvc := NewIdSvc(sqlStore, tknSvc, "agentauth.local", auditLog)
    return idSvc, sqlStore, auditLog
}
```

Note: tests always use `LogLevel: "quiet"` to suppress log output.

### Key Pair Generation

Tests generate fresh Ed25519 keys per test:

```go
func testKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
    t.Helper()
    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        t.Fatalf("generate key pair: %v", err)
    }
    return pub, priv
}
```

### Launch Token and Nonce Helpers

Tests create launch tokens directly via the store (bypassing the admin API):

```go
func createLaunchToken(t *testing.T, s *store.SqlStore, allowedScope []string) string {
    t.Helper()
    tokenVal := make([]byte, 16)
    rand.Read(tokenVal)
    tok := hex.EncodeToString(tokenVal)
    s.SaveLaunchToken(store.LaunchTokenRecord{
        Token: tok, AgentName: "test-agent", AllowedScope: allowedScope,
        MaxTTL: 300, SingleUse: true,
        CreatedAt: time.Now(), ExpiresAt: time.Now().Add(30 * time.Second),
        CreatedBy: "admin:orchestrator",
    })
    return tok
}
```

### Signature Test Pattern

Token signature tests cover three tampering modes:
1. **Tampered signature** -- decode sig bytes, XOR-flip a byte, re-encode
2. **Tampered payload** -- decode payload bytes, XOR-flip a byte, re-encode (signature no longer matches)
3. **Wrong key** -- issue with one key pair, verify with a different public key

Important: when tampering base64url strings, always decode to bytes first, flip, then re-encode. Never tamper the last character directly (it may be a padding artifact with non-significant tail bits).

### Table-Driven Tests

Scope tests use the standard Go table-driven pattern:

```go
func TestScopeIsSubset(t *testing.T) {
    tests := []struct {
        name      string
        requested []string
        allowed   []string
        want      bool
    }{
        {"exact match", []string{"read:Customers:*"}, []string{"read:Customers:*"}, true},
        {"wildcard covers specific", []string{"read:Customers:12345"}, []string{"read:Customers:*"}, true},
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ScopeIsSubset(tt.requested, tt.allowed)
            if got != tt.want {
                t.Errorf("ScopeIsSubset(%v, %v) = %v, want %v", tt.requested, tt.allowed, got, tt.want)
            }
        })
    }
}
```

### Running Tests

```bash
go test ./...                       # all tests
go test ./... -short                # unit tests only (skip integration)
go test ./internal/token/...        # single package
go test ./internal/token/... -v     # verbose output
go test -race ./...                 # race detector
```

---

## 15. How to Add a New Endpoint

Step-by-step guide for adding `POST /v1/example`:

### Step 1: Define the handler

Create `internal/handler/example_hdl.go`:

```go
package handler

import (
    "encoding/json"
    "net/http"
)

type ExampleHdl struct {
    // inject dependencies here
}

func NewExampleHdl(/* deps */) *ExampleHdl {
    return &ExampleHdl{/* ... */}
}

type exampleReq struct {
    Field string `json:"field"`
}

type exampleResp struct {
    Result string `json:"result"`
}

func (h *ExampleHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    var req exampleReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteProblem(w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
        return
    }

    // Business logic here...

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(exampleResp{Result: "ok"})
}
```

### Step 2: Wire into main.go

In `cmd/broker/main.go`, instantiate and register:

```go
exampleHdl := handler.NewExampleHdl(/* deps */)

// No auth:
mux.Handle("POST /v1/example", exampleHdl)

// With Bearer auth:
mux.Handle("POST /v1/example", valMw.Wrap(exampleHdl))

// With Bearer + scope:
mux.Handle("POST /v1/example",
    valMw.Wrap(authz.WithRequiredScope("admin:example:*", exampleHdl)))
```

### Step 3: Add tests

Create `internal/handler/example_hdl_test.go`. Use `httptest.NewRecorder` and `httptest.NewRequest`:

```go
func TestExampleHdl(t *testing.T) {
    hdl := handler.NewExampleHdl(/* test deps */)
    body := `{"field":"value"}`
    req := httptest.NewRequest("POST", "/v1/example", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    hdl.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Errorf("status = %d, want 200", w.Code)
    }
}
```

### Step 4: Update documentation

- Add the endpoint to `docs/API_REFERENCE.md`
- Add to the OpenAPI spec at `docs/api/openapi.yaml`
- Update `CHANGELOG.md` under `[Unreleased]`

### Step 5: Run gates

```bash
./scripts/gates.sh task
```

---

## 16. How to Add a New Audit Event Type

### Step 1: Add the constant

In `internal/audit/audit_log.go`, add a new constant:

```go
const (
    // existing...
    EventMyNewEvent = "my_new_event"
)
```

### Step 2: Record the event

In the service that triggers this event, call `Record`:

```go
if s.auditLog != nil {
    s.auditLog.Record(audit.EventMyNewEvent, agentID, taskID, orchID,
        fmt.Sprintf("descriptive detail about what happened"))
}
```

### Step 3: Update the tech spec

Add the new event type to the event types table in `plans/AgentAuth-Technical-Spec-v2.0.md` Section 3.

### Step 4: Add tests

Verify the event appears in the audit log after the triggering action:

```go
events := auditLog.Events()
found := false
for _, e := range events {
    if e.EventType == "my_new_event" {
        found = true
    }
}
if !found {
    t.Error("missing my_new_event audit event")
}
```

### Step 5: Verify hash chain integrity

The hash chain is automatic. Each new event's hash includes the previous event's hash. No manual work needed -- just verify the event appears in the log.

---

## 17. Known Limitations

The following are known gaps relative to the full [Ephemeral Agent Credentialing](../plans/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md) security pattern. They are documented here for transparency and are planned for future iterations.

### No native TLS / mTLS

The broker listens on plain HTTP (`http.ListenAndServe`). There is no TLS configuration, certificate loading, or `http.ListenAndServeTLS` call anywhere in the codebase. The security pattern's Component 3 specifies mTLS between agent and server as a transport-layer enforcement point.

**Mitigation:** Production deployments MUST use a TLS-terminating reverse proxy (nginx, Caddy, envoy, or a cloud load balancer). See the [Security Hardening](../docs/USER_GUIDE.md#security-hardening) section of the User Guide. Native TLS support (`AA_TLS_CERT`, `AA_TLS_KEY`) is planned for a future release.

### Delegation chain per-step signatures

Each `DelegRecord` is signed by the broker's Ed25519 key at delegation time. The canonical signing input is `agent|scope_csv|delegated_at_rfc3339`, and the hex-encoded signature is stored in the `Signature` field. This provides cryptographic proof that each delegation step was authorized by the broker. External validators can verify individual chain links using the broker's public key.

**Implementation:** `DelegSvc.signRecord()` in `internal/deleg/deleg_svc.go`.

### Delegation chain hash

Delegated tokens include a `chain_hash` claim (JSON key: `chain_hash`) containing the hex-encoded SHA-256 hash of the JSON-serialized delegation chain. This allows quick integrity verification without re-validating each individual signature in the chain.

**Implementation:** `computeChainHash()` in `internal/deleg/deleg_svc.go`. The `ChainHash` field is defined in `TknClaims` (`internal/token/tkn_claims.go`).
