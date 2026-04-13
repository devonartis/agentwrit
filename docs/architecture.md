# Architecture — How AgentWrit Works Inside

Two binaries, one Go module, fourteen internal packages. This page shows how every component connects — from HTTP request to signed JWT to audit record.

**Prerequisites:** [Concepts](concepts.md) helps, but isn't required.

---

## System Overview

AgentWrit sits between AI agents and the resources they need to access, providing ephemeral, scoped credentials through a challenge-response identity flow.

<p align="center">
  <img src="diagrams/architecture-overview.svg" alt="AgentWrit Architecture Overview" width="100%">
</p>

> **More diagrams:** [Token Lifecycle](diagrams/token-lifecycle.svg) · [Security Topology](diagrams/security-topology.svg)

**Broker** (`cmd/broker`) -- The central authority. Loads or generates a persistent Ed25519 signing key (`internal/keystore`), issues EdDSA-signed JWTs, validates challenge-response registrations, manages scope attenuation, delegation, revocation, and maintains a hash-chained audit trail. All endpoints use `application/json` with RFC 7807 error responses.

**App Service** (`internal/app`) -- Manages application registrations and app-level authentication. Operators register apps with `awrit app register`, which generates a client_id and client_secret. Apps authenticate with `POST /v1/app/auth` using those credentials to get scoped tokens.

**awrit** (`cmd/awrit`) -- The operator CLI. Reads `AACTL_BROKER_URL` and `AACTL_ADMIN_SECRET` from environment variables and auto-authenticates. Provides table and JSON output for app management, token revocation, and audit trail queries. Intended for operators who prefer a CLI over hand-crafting curl + JWT.

---

## Component Architecture

```mermaid
flowchart TB
    subgraph Broker["Broker (cmd/broker)"]
        direction TB
        MAIN_B["main.go\nService wiring\nRoute registration"]

        subgraph Foundation["Foundation Layer"]
            CFG["cfg\nEnv var config"]
            OBS["obs\nStructured logging\nPrometheus metrics"]
            STORE["store\nSqlStore\nSQLite + In-memory"]
            KEYSTORE["keystore\nEd25519 key persistence\nPKCS8 PEM"]
        end

        subgraph Security["Security Layer"]
            TOKEN["token\nTknSvc\nEdDSA JWT issue/verify"]
            IDENTITY["identity\nIdSvc\nChallenge-response\nSPIFFE IDs"]
            AUTHZ["authz\nValMw\nScope checking\nRate limiting"]
        end

        subgraph Domain["Domain Layer"]
            ADMIN_PKG["admin\nAdminSvc + AdminHdl\nLaunch tokens\nAdmin auth"]
            APP_PKG["app\nAppSvc + AppHdl\nApp registration\nApp auth"]
            DELEG["deleg\nDelegSvc\nScope-attenuated\ndelegation"]
            REVOKE["revoke\nRevSvc\n4-level revocation"]
            AUDIT["audit\nAuditLog\nHash-chained trail"]
        end

        subgraph Transport["Transport Layer"]
            HANDLER["handler\nHTTP handlers\nChallengeHdl, RegHdl,\nValHdl, RenewHdl, ..."]
            SECHDL["security_hdl\nSecurityHeaders"]
            PD["problemdetails\nRFC 7807 errors\nRequestID"]
        end

    end

```

---

## Directory Layout

```
agentwrit/
|-- cmd/
|   |-- broker/
|   |   +-- main.go              # Service wiring, route registration, startup
|   |-- awrit/                   # Operator CLI (awrit binary)
|   |   |-- main.go              # Cobra root command, env var config
|   |   |-- client.go            # HTTP client with auto-auth
|   |   |-- apps.go              # app register / list
|   |   |-- revoke.go            # revoke --level --target
|   |   |-- audit.go             # audit events with filters
|   |   |-- token.go              # token release command
|   |   +-- output.go            # Table and JSON output helpers
|-- internal/
|   |-- admin/                   # Admin auth, launch tokens
|   |-- app/                     # App registration, app auth
|   |-- audit/                   # Hash-chained audit trail
|   |-- authz/                   # Bearer middleware, scope checking, rate limiter
|   |-- cfg/                     # Broker configuration from AA_* env vars
|   |-- deleg/                   # Scope-attenuated delegation with chain signing
|   |-- handler/                 # HTTP handlers for all broker endpoints + security_hdl.go (SecurityHeaders)
|   |-- identity/                # Challenge-response registration, SPIFFE IDs
|   |-- keystore/                # Ed25519 signing key persistence (PKCS8 PEM)
|   |-- obs/                     # Structured logging and Prometheus metrics
|   |-- problemdetails/          # RFC 7807 errors, request ID, body limits
|   |-- revoke/                  # Four-level token revocation
|   |-- store/                   # SQLite-backed persistence + in-memory maps
|   +-- token/                   # EdDSA JWT issuance, verification, renewal
|-- scripts/                     # Gate checks, Docker helpers, E2E test scripts
|-- docs/                        # Documentation
|-- docker-compose.yml           # Broker on bridge network
+-- Dockerfile                   # Multi-stage build (builder, broker)
```

---

## Package Dependencies

Each service is initialized in `cmd/broker/main.go` with explicit constructor injection — no globals, no `init()`.

| Package | Depends on | What it receives |
|---|---|---|
| `token` | `cfg`, `keystore` | Config (TTLs, issuer), Ed25519 key pair, revoker interface |
| `identity` | `store`, `token`, `audit` | Nonce storage, JWT issuance, audit recording |
| `admin` | `token`, `store`, `audit` | JWT issuance, app/launch-token persistence, audit |
| `app` | `store`, `token`, `audit` | App persistence, JWT issuance, audit |
| `authz` | `token`, `revoke`, `audit` | Token verification, revocation checks, scope violation audit |
| `deleg` | `token`, `store`, `audit`, `keystore` | JWT issuance, chain persistence, audit, signing key |
| `revoke` | `store` | Revocation persistence |
| `audit` | `store` | Event persistence (hash chain rebuilt from disk on startup) |
| `store` | — | Foundation — no dependencies |
| `keystore` | — | Foundation — loads/generates Ed25519 key from disk |
| `cfg` | — | Foundation — parses `AA_*` env vars and config files |
| `obs` | — | Foundation — structured logging, Prometheus counters |

---

## Pattern Components Mapped to Code

The 8-component Ephemeral Agent Credentialing pattern maps directly to Go packages:

| Pattern Component | Go Packages | Key Types | Key Functions |
|---|---|---|---|
| **Foundation** | | | |
| Store | `store` | `SqlStore`, `LaunchTokenRecord`, `AgentRecord`, `AppRecord`, `RevocationEntry` | `CreateNonce()`, `SaveAgent()`, `SaveApp()`, `SaveRevocation()`, `LoadAllAuditEvents()`, `LoadAllRevocations()` |
| Keystore | `keystore` | — | `LoadOrGenerate()` — persistent Ed25519 key management (PKCS8 PEM, 0600 permissions) |
| **Pattern Components** | | | |
| 1. Ephemeral Identity Issuance | `identity`, `store`, `handler` | `IdSvc`, `RegHdl`, `ChallengeHdl`, `SqlStore` | `IdSvc.Register()`, `NewSpiffeId()` |
| 2. Short-Lived Task-Scoped Tokens | `token`, `authz` | `TknSvc`, `TknClaims`, `IssueReq`, `Revoker` | `TknSvc.Issue()`, `TknSvc.Verify()`, `TknSvc.Renew()`, `TknSvc.SetRevoker()` |
| 3. Zero-Trust Enforcement | `authz`, `handler` | `ValMw`, `RateLimiter` | `ValMw.Wrap()`, `ValMw.RequireScope()`, `ValMw.RequireAnyScope()`, `ScopeIsSubset()` |
| 4. Automatic Expiration & Revocation | `revoke`, `token`, `handler` | `RevSvc`, `Revoker`, `RevokeHdl`, `ReleaseHdl` | `RevSvc.Revoke()`, `RevSvc.RevokeByJTI()`, `RevSvc.IsRevoked()`, `RevSvc.LoadFromEntries()` |
| 5. Immutable Audit Logging | `audit`, `handler` | `AuditLog`, `AuditEvent`, `AuditHdl`, `RecordOption` | `AuditLog.Record()`, `AuditLog.Query()`, `WithOutcome()`, `WithResource()` |
| 6. Delegation Chain Verification | `deleg`, `handler` | `DelegSvc`, `DelegHdl`, `DelegRecord` | `DelegSvc.Delegate()` |

---

## Request Lifecycle

Every HTTP request passes through the same middleware stack before reaching a handler:

```
Request → RequestID → Logging → MaxBytesBody → SecurityHeaders → [Route Match]

  Public routes:      → Handler (health, challenge, metrics, validate)
  Auth routes:        → ValMw.Wrap → Handler (renew, delegate, release)
  Admin scope routes: → ValMw.Wrap → RequireScope → Handler (revoke, audit, apps, launch-tokens)
  Rate-limited:       → RateLimiter → Handler (admin/auth, app/auth)
  Registration:       → Handler (launch token validated in body, not Bearer)
```

Not all routes use every middleware. Public endpoints (health, challenge, metrics) skip `ValMw` and `ValMw.RequireScope`. `SecurityHeaders` (`internal/handler/security_hdl.go`) and `MaxBytesBody` (`internal/problemdetails/problemdetails.go`) are global middleware applied to ALL requests. Execution order on an incoming request: `RequestID → Logging → MaxBytesBody → SecurityHeaders → route handler`. `SecurityHeaders` sets `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, and `X-Frame-Options: DENY` on every response, and adds `Strict-Transport-Security` (HSTS) when `AA_TLS_MODE` is `tls` or `mtls`. `MaxBytesBody` enforces a 1 MB request body limit — oversized requests get HTTP 413 before reaching any handler.

---

## Data Flow Diagrams

### Agent Registration Flow

The 10-step registration is the core identity issuance flow:

```mermaid
sequenceDiagram
    participant A as Agent
    participant CH as ChallengeHdl
    participant S as SqlStore
    participant RH as RegHdl
    participant ID as IdSvc
    participant TK as TknSvc

    A->>CH: GET /v1/challenge
    CH->>S: CreateNonce()
    S-->>CH: 64-char hex nonce (30s TTL)
    CH-->>A: {"nonce": "abc...", "expires_in": 30}

    Note over A: Sign hex-decoded nonce bytes<br/>with Ed25519 private key

    A->>RH: POST /v1/register {launch_token, nonce,<br/>public_key, signature, orch_id, task_id, requested_scope}
    RH->>ID: Register(req)

    ID->>ID: 1. Validate required fields
    ID->>S: 2. GetLaunchToken(token)
    S-->>ID: LaunchTokenRecord (scope ceiling, policy)
    ID->>ID: 3. ScopeIsSubset(requested, allowed)
    Note over ID: Scope check BEFORE token consumption
    ID->>S: 4. ConsumeNonce(nonce)
    ID->>ID: 5. Decode base64 public key (32 bytes)
    ID->>ID: 6. ed25519.Verify(pubKey, nonce, sig)
    ID->>S: 7. ConsumeLaunchToken() if single-use
    ID->>ID: 8. NewSpiffeId(domain, orch, task, instance)
    ID->>TK: 9. Issue(sub=spiffeID, scope, ttl)
    TK-->>ID: {access_token, expires_in}
    ID->>S: 10. SaveAgent(record)

    ID-->>RH: RegisterResp
    RH-->>A: {"agent_id": "spiffe://...",<br/>"access_token": "eyJ...", "expires_in": 300}
```

### Delegation Flow

Agent A delegates a narrower-scoped token to Agent B:

```mermaid
sequenceDiagram
    participant A as Agent A (delegator)
    participant DH as DelegHdl
    participant DS as DelegSvc
    participant S as SqlStore
    participant TK as TknSvc
    participant B as Agent B (delegate)

    A->>DH: POST /v1/delegate<br/>Bearer: agent-a-token<br/>{"delegate_to": "spiffe://...agentB",<br/>"scope": ["read:data:*"], "ttl": 60}

    Note over DH: ValMw verifies Bearer token

    DH->>DS: Delegate(delegatorClaims, req)
    DS->>DS: Check chain depth < 5
    DS->>DS: ScopeIsSubset(requested, delegator.scope)
    DS->>S: GetAgent(delegate_to)
    S-->>DS: Agent B record exists

    DS->>DS: Build DelegRecord:<br/>agent, scope, timestamp
    DS->>DS: Sign record with Ed25519 broker key
    DS->>DS: Append to delegation_chain
    DS->>DS: chain_hash = SHA-256(JSON(chain))

    DS->>TK: Issue(sub=agentB, scope=subset,<br/>delegation_chain, chain_hash, ttl=60)
    TK-->>DS: {access_token, expires_in}
    DS-->>DH: DelegResp
    DH-->>A: {"access_token", "expires_in",<br/>"delegation_chain": [...]}

    Note over A: Deliver token to Agent B
    A->>B: delegated token
```

---

## Middleware Stack

The broker applies two layers of middleware: global middleware on all requests, and per-route middleware on specific endpoints.

```mermaid
flowchart LR
    REQ["HTTP\nRequest"] --> RID["RequestID\nMiddleware"]
    RID --> LOG["Logging\nMiddleware"]
    LOG --> MUX["http.ServeMux\nRoute Match"]
    MUX --> SEC["SecurityHeaders\n(global)"]
    SEC --> MB["MaxBytesBody\n1 MB (global)"]

    MB --> PUB["Public Handlers\n(health, challenge,\nmetrics, validate)"]

    MB --> AUTH_ONLY["ValMw.Wrap"] --> AUTH_H["Auth Handlers\n(renew, delegate,\nrelease)"]

    MB --> AUDIT_W["ValMw.Wrap"] --> AUDIT_C["ValMw\n.RequireScope"] --> AUDIT_H["Audit Handler\n(audit/events)"]

    MB --> SCOPE_W["ValMw.Wrap"] --> SCOPE_C["ValMw\n.RequireScope /\n.RequireAnyScope"] --> ADMIN_H["Scoped POST Handlers\n(revoke, launch-tokens,\nadmin/apps)"]

    MB --> RL["RateLimiter\n.Wrap"] --> RL_H["Rate-Limited\n(admin/auth,\napp/auth)"]

    MB --> REG_H["Register\n(launch token\nin body)"]
```

**Route-to-middleware mapping from `cmd/broker/main.go`:**

| Route | Middleware Chain |
|---|---|
| `GET /v1/challenge` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> Handler |
| `GET /v1/health` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> Handler |
| `GET /v1/metrics` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> Handler |
| `POST /v1/token/validate` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> Handler |
| `POST /v1/register` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> Handler |
| `POST /v1/token/renew` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> Handler |
| `POST /v1/delegate` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> Handler |
| `POST /v1/token/release` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> Handler |
| `POST /v1/revoke` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:revoke:*`) -> Handler |
| `GET /v1/audit/events` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:audit:*`) -> Handler |
| `POST /v1/admin/auth` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> RateLimiter(5/s, burst 10) -> Handler |
| `POST /v1/admin/launch-tokens` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `POST /v1/app/launch-tokens` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`app:launch-tokens:*`) -> Handler |
| `POST /v1/admin/apps` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `GET /v1/admin/apps` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `GET /v1/admin/apps/{id}` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `PUT /v1/admin/apps/{id}` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `DELETE /v1/admin/apps/{id}` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `POST /v1/app/auth` | RequestID -> Logging -> MaxBytesBody -> SecurityHeaders -> RateLimiter(10/min per client_id, burst 3) -> Handler |
---

## Key Design Decisions

1. **Hybrid persistence.** Critical state (audit events, revocations, app registrations) is persisted to SQLite via write-through from in-memory structures. Transient state (nonces, agent records, launch tokens) lives in memory behind `sync.RWMutex` and is cleared on restart. This design ensures the audit trail and revocation list survive broker restarts while keeping the hot path fast.

2. **Persistent Ed25519 signing key.** On first startup, the broker generates an Ed25519 key pair via `crypto/rand` and writes it to `AA_SIGNING_KEY_PATH` (default `./signing.key`) in PKCS8 PEM format with `0600` permissions. On subsequent startups, the existing key is loaded from disk. Tokens remain valid across restarts as long as the key file persists. Delete the key file to force key rotation (all previously issued tokens become unverifiable). See `internal/keystore` for implementation.

3. **Scope attenuation is one-way.** Scopes can never expand — same or narrower, never wider. Enforced at registration (requested scope vs. launch token ceiling), delegation (delegated vs. delegator scope), and app-authenticated launch-token creation (requested `allowed_scope` vs. app ceiling).

4. **Scope check before launch token consumption.** At registration, `ScopeIsSubset` is called before `ConsumeLaunchToken`. A scope violation returns an error without wasting a single-use token.

5. **Bcrypt secret comparison.** Admin authentication uses `bcrypt.CompareHashAndPassword` (cost 12) to prevent timing attacks on `AA_ADMIN_SECRET`. The plaintext secret is bcrypt-hashed at startup in `cfg.Load()` and the hash is stored in `Cfg.AdminSecretHash`. The plaintext is wiped from memory after hashing.

6. **Apps as first-class entities.** Developer applications are registered with the broker via `awrit` and authenticate directly using client credentials (`POST /v1/app/auth`). Each app has its own scope ceiling and configurable JWT TTL (bounded by broker-wide min/max).

---

## Security Assumptions

These are explicit trust boundaries and limitations of the current implementation:

- **X-Forwarded-For trusted unconditionally.** The `clientIP()` function in `internal/authz/rate_mw.go` trusts the first entry in `X-Forwarded-For` without validation. In production, the broker must sit behind a trusted reverse proxy that sets this header correctly. Without a trusted proxy, rate limiting can be bypassed via header spoofing.

- **Persistent and transient state split.** Audit events, revocations, and app registrations are persisted to SQLite and reloaded on startup. Nonces, agent records, and launch tokens are transient (memory only). All previously issued tokens become unverifiable after restart (new signing keys). The split is intentional — audit and revocation are security-critical; nonces and agent records are ephemeral by design.

- **Single broker instance.** There is no replication, consensus, or shared state mechanism. The broker is a single process. Running multiple instances would result in split-brain token verification (each instance has its own signing key).

- **Nonce window is 30 seconds.** Nonces expire after 30 seconds. Agents must complete the challenge-response flow within this window. Clock skew between agent and broker can cause failures.

- **Admin secret is the root of trust.** `AA_ADMIN_SECRET` is the single shared secret that bootstraps the entire system. If compromised, an attacker can issue arbitrary launch tokens and app credentials.

---

## External Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics exposition |
| `github.com/prometheus/client_model` | v0.6.2 | Prometheus data model |
| `github.com/spiffe/go-spiffe/v2` | v2.6.0 | SPIFFE ID validation |
| `modernc.org/sqlite` | v1.35.0 | Pure-Go SQLite driver for audit event persistence (zero CGo) |
| Go stdlib `crypto/ed25519` | -- | Token signing and nonce signature verification |
| Go stdlib `crypto/sha256` | -- | Audit hash chain, delegation chain hash |
| Go stdlib `net/http` | -- | HTTP server (Go 1.22+ method routing) |
| `golang.org/x/crypto/bcrypt` | v0.48.0 | Admin secret hash verification |

---

## Background Maintenance

The broker runs two background tasks on a 60-second ticker:

**JTI Pruning** (`store.PruneExpiredJTIs`) — Removes expired JWT ID entries from the database. JTIs are stored to prevent token replay; once a token expires, its JTI is no longer needed. The pruner keeps the database from growing unbounded.

**Agent Expiration** (`store.ExpireAgents`) — Marks agents as expired when they exceed inactivity thresholds. This catches agents that crashed without calling `POST /v1/token/release` to self-revoke.

Both tasks log their results at the `Ok` level with the operation name and count of affected records.

---

## What's Next?

You've seen how the pieces fit together. Pick your next step:

**[API Reference →](api.md)**
Every endpoint — request formats, response shapes, error codes, and rate limits.

Or explore related topics:

| If you want to... | Read this |
|-------------------|-----------|
| Find where a feature lives in the code | [Implementation Map](implementation-map.md) |
| Use the operator CLI | [CLI Reference (awrit)](awrit-reference.md) |
| Deploy in production | [Getting Started: Operator](getting-started-operator.md) |
| Understand the security pattern | [Concepts Deep Dive](concepts.md) |

---

*Previous: [Concepts](concepts.md) · Next: [API Reference](api.md)*
