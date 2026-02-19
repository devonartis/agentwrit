# Architecture

> **Document Version:** 2.0 | **Last Updated:** February 2026 | **Status:** Current
>
> **Audience:** Contributors, security reviewers, and operators who want to understand how AgentAuth works internally.
>
> **Prerequisites:** [Concepts](concepts.md) for the security pattern overview.
>
> **Next steps:** [API Reference](api.md) | [Contributing](../CONTRIBUTING.md) | [Getting Started: Operator](getting-started-operator.md)

---

## System Overview

AgentAuth sits between AI agents and the resources they need to access, providing ephemeral, scoped credentials through a challenge-response identity flow.

```mermaid
flowchart TB
    subgraph External Actors
        DEV["Developer App"]
        AGENT["AI Agent / Orchestrator"]
        ADMIN["Operator / Admin"]
        RS["Resource Servers"]
    end

    subgraph AgentAuth["AgentAuth System Boundary"]
        BROKER["Broker\ncmd/broker\nPort 8080"]
        SIDECAR["Sidecar\ncmd/sidecar\nPort 8081"]
    end

    DEV -- "POST /v1/token\n(simplified API)" --> SIDECAR
    SIDECAR -- "challenge, register,\nexchange, renew" --> BROKER
    AGENT -- "challenge-response\nregistration" --> BROKER
    ADMIN -- "admin auth,\nlaunch tokens,\nrevocation" --> BROKER
    AGENT -- "Bearer token" --> RS
    DEV -- "Bearer token" --> RS
```

**Broker** (`cmd/broker`) -- The central authority. Generates Ed25519 key pairs on startup, issues EdDSA-signed JWTs, validates challenge-response registrations, manages scope attenuation, delegation, revocation, and maintains a hash-chained audit trail. All endpoints use `application/json` with RFC 7807 error responses.

**Sidecar** (`cmd/sidecar`) -- A developer-facing proxy. Bootstraps itself with the broker using admin auth and a single-use activation token, then exposes a simplified API. Developers call `POST /v1/token` with an agent name and scope; the sidecar handles key generation, challenge-response registration, and token exchange transparently. Includes circuit breaker resilience and cached token fallback.

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
            STORE["store\nSqlStore\nIn-memory maps"]
        end

        subgraph Security["Security Layer"]
            TOKEN["token\nTknSvc\nEdDSA JWT issue/verify"]
            IDENTITY["identity\nIdSvc\nChallenge-response\nSPIFFE IDs"]
            AUTHZ["authz\nValMw\nScope checking\nRate limiting"]
        end

        subgraph Domain["Domain Layer"]
            ADMIN_PKG["admin\nAdminSvc + AdminHdl\nLaunch tokens\nSidecar activation"]
            DELEG["deleg\nDelegSvc\nScope-attenuated\ndelegation"]
            REVOKE["revoke\nRevSvc\n4-level revocation"]
            AUDIT["audit\nAuditLog\nHash-chained trail"]
        end

        subgraph Transport["Transport Layer"]
            HANDLER["handler\nHTTP handlers\nChallengeHdl, RegHdl,\nValHdl, RenewHdl, ..."]
            PD["problemdetails\nRFC 7807 errors\nRequestID\nMaxBytesBody"]
        end

        subgraph Protocol["Protocol Layer (Go API only)"]
            MUTAUTH["mutauth\nMutAuthHdl\nDiscoveryRegistry\nHeartbeatMgr"]
        end
    end

    subgraph Sidecar["Sidecar (cmd/sidecar)"]
        MAIN_S["main.go\nBootstrap loop\nRoute wiring"]
        SC_HANDLER["handler.go\ntokenHandler\nrenewHandler"]
        SC_BOOT["bootstrap.go\nsidecarState\nwaitForBroker"]
        SC_CLIENT["broker_client.go\nHTTP calls to broker"]
        SC_REG["registry.go\nIn-memory agent store\nToken caching"]
        SC_RENEW["renewal.go\nBackground renewal"]
        SC_CB["circuitbreaker.go\nSliding-window CB"]
        SC_PROBE["probe.go\nHealth probe"]
        SC_BYREG["register_handler.go\nBYOK registration"]
    end

    SIDECAR -- "HTTP" --> Broker
```

---

## Directory Layout

```
agentauth/
|-- cmd/
|   |-- broker/
|   |   +-- main.go              # Service wiring, route registration, startup
|   |-- sidecar/
|   |   |-- main.go              # Bootstrap loop, route registration, shutdown
|   |   |-- config.go            # Environment variable parsing
|   |   |-- handler.go           # tokenHandler, renewHandler, healthHandler
|   |   |-- bootstrap.go         # sidecarState, bootstrap(), waitForBroker()
|   |   |-- broker_client.go     # HTTP client for all broker interactions
|   |   |-- registry.go          # In-memory agent store with per-agent locking
|   |   |-- renewal.go           # Background token renewal goroutine
|   |   |-- register_handler.go  # BYOK challenge proxy and registration
|   |   |-- circuitbreaker.go    # Sliding-window circuit breaker
|   |   |-- probe.go             # Background health probe for circuit recovery
|   |   +-- metrics.go           # 9 Prometheus metrics
|   +-- smoketest/               # Container smoke test binary
|-- internal/
|   |-- admin/                   # Admin auth, launch tokens, sidecar activation
|   |-- audit/                   # Hash-chained audit trail
|   |-- authz/                   # Bearer middleware, scope checking, rate limiter
|   |-- cfg/                     # Broker configuration from AA_* env vars
|   |-- deleg/                   # Scope-attenuated delegation with chain signing
|   |-- handler/                 # HTTP handlers for all broker endpoints
|   |-- identity/                # Challenge-response registration, SPIFFE IDs
|   |-- mutauth/                 # Mutual authentication (Go API only)
|   |-- obs/                     # Structured logging and Prometheus metrics
|   |-- problemdetails/          # RFC 7807 errors, request ID, body limits
|   |-- revoke/                  # Four-level token revocation
|   |-- store/                   # In-memory storage (nonces, agents, launch tokens)
|   +-- token/                   # EdDSA JWT issuance, verification, renewal
|-- scripts/                     # Gate checks, Docker helpers, E2E test scripts
|-- docs/                        # Documentation
|-- docker-compose.yml           # Broker + Sidecar on bridge network
+-- Dockerfile                   # Multi-stage build (builder, broker, sidecar)
```

---

## Package Dependency Graph

```mermaid
flowchart TD
    MAIN_B["cmd/broker/main.go"] --> CFG
    MAIN_B --> OBS
    MAIN_B --> STORE
    MAIN_B --> TOKEN
    MAIN_B --> IDENTITY
    MAIN_B --> AUTHZ
    MAIN_B --> ADMIN
    MAIN_B --> DELEG
    MAIN_B --> REVOKE
    MAIN_B --> AUDIT
    MAIN_B --> HANDLER
    MAIN_B --> PD

    MAIN_S["cmd/sidecar/main.go"] --> OBS

    HANDLER["handler"] --> IDENTITY
    HANDLER --> TOKEN
    HANDLER --> REVOKE
    HANDLER --> DELEG
    HANDLER --> AUDIT
    HANDLER --> STORE
    HANDLER --> PD
    HANDLER --> OBS
    HANDLER --> AUTHZ

    ADMIN["admin"] --> TOKEN
    ADMIN --> STORE
    ADMIN --> AUDIT
    ADMIN --> AUTHZ
    ADMIN --> OBS
    ADMIN --> PD

    IDENTITY["identity"] --> TOKEN
    IDENTITY --> STORE
    IDENTITY --> AUTHZ
    IDENTITY --> OBS

    AUTHZ["authz"] --> TOKEN
    AUTHZ --> REVOKE
    AUTHZ --> AUDIT
    AUTHZ --> OBS
    AUTHZ --> PD

    DELEG["deleg"] --> TOKEN
    DELEG --> STORE
    DELEG --> AUTHZ
    DELEG --> AUDIT
    DELEG --> OBS

    REVOKE["revoke"] --> TOKEN

    TOKEN["token"] --> CFG
    TOKEN --> OBS

    MUTAUTH["mutauth"] --> TOKEN
    MUTAUTH --> STORE
    MUTAUTH --> REVOKE

    PD["problemdetails"]
    CFG["cfg"]
    OBS["obs"]
    STORE["store"]
    AUDIT["audit"]

    classDef foundation fill:#e8f5e9
    classDef security fill:#e3f2fd
    classDef domain fill:#fff3e0
    classDef transport fill:#fce4ec
    classDef protocol fill:#f3e5f5

    class CFG,OBS,STORE foundation
    class TOKEN,IDENTITY,AUTHZ security
    class ADMIN,DELEG,REVOKE,AUDIT domain
    class HANDLER,PD transport
    class MUTAUTH protocol
```

**Legend:** Green = Foundation, Blue = Security, Orange = Domain, Pink = Transport, Purple = Protocol (Go API only)

---

## Pattern Components Mapped to Code

The 7-component Ephemeral Agent Credentialing pattern maps directly to Go packages:

| Pattern Component | Go Packages | Key Types | Key Functions |
|---|---|---|---|
| 1. Ephemeral Identity Issuance | `identity`, `store`, `handler` | `IdSvc`, `RegHdl`, `ChallengeHdl`, `SqlStore` | `IdSvc.Register()`, `NewSpiffeId()` |
| 2. Short-Lived Task-Scoped Tokens | `token`, `authz` | `TknSvc`, `TknClaims`, `IssueReq` | `TknSvc.Issue()`, `TknSvc.Renew()` |
| 3. Zero-Trust Enforcement | `authz`, `handler` | `ValMw`, `RateLimiter` | `ValMw.Wrap()`, `ValMw.RequireScope()`, `ScopeIsSubset()` |
| 4. Automatic Expiration & Revocation | `revoke`, `handler` | `RevSvc`, `RevokeHdl` | `RevSvc.Revoke()`, `RevSvc.IsRevoked()` |
| 5. Immutable Audit Logging | `audit`, `handler` | `AuditLog`, `AuditEvent`, `AuditHdl` | `AuditLog.Record()`, `AuditLog.Query()` |
| 6. Agent-to-Agent Mutual Auth | `mutauth` | `MutAuthHdl`, `DiscoveryRegistry`, `HeartbeatMgr` | `InitiateHandshake()`, `RespondToHandshake()`, `CompleteHandshake()` |
| 7. Delegation Chain Verification | `deleg`, `handler` | `DelegSvc`, `DelegHdl`, `DelegRecord` | `DelegSvc.Delegate()` |

---

## Request Lifecycle

Every HTTP request passes through the same middleware stack before reaching a handler:

```mermaid
sequenceDiagram
    participant C as Client
    participant RID as RequestIDMiddleware
    participant LOG as LoggingMiddleware
    participant MUX as http.ServeMux
    participant MB as MaxBytesBody
    participant VAL as ValMw.Wrap
    participant SC as ValMw.RequireScope
    participant H as Handler

    C->>RID: HTTP Request
    RID->>RID: Generate/propagate X-Request-ID
    RID->>LOG: Request + context
    LOG->>LOG: Record start time
    LOG->>MUX: Route to handler
    MUX->>MB: Match route (Go 1.22+ method routing)
    MB->>MB: Limit body to 1 MB
    MB->>VAL: (if auth required)
    VAL->>VAL: Extract Bearer token
    VAL->>VAL: TknSvc.Verify(token)
    VAL->>VAL: RevSvc.IsRevoked(claims)
    VAL->>VAL: Inject claims into context
    VAL->>SC: (if scope required)
    SC->>SC: ScopeIsSubset check
    SC->>H: Authenticated + authorized request
    H->>H: Business logic
    H-->>C: JSON response
    LOG-->>LOG: Log method, path, status, latency, request_id
```

Not all routes use every middleware. Public endpoints (health, challenge, metrics) skip `ValMw` and `ValMw.RequireScope`. The `MaxBytesBody` wrapper is applied per-route to POST endpoints only.

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

    A->>RH: POST /v1/register {launch_token, nonce,<br/>public_key, signature, orch_id, task_id, scope}
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

### Sidecar Bootstrap Flow

The sidecar bootstraps with exponential backoff (1s to 60s cap):

```mermaid
sequenceDiagram
    participant SC as Sidecar
    participant HTTP as HTTP Server
    participant BR as Broker

    Note over SC: Start HTTP server immediately<br/>(health + metrics only)
    SC->>HTTP: ListenAndServe(:8081)

    Note over SC: Bootstrap loop begins

    loop Until success (backoff 1s->60s)
        SC->>BR: GET /v1/health
        BR-->>SC: {"status": "ok"}

        SC->>BR: POST /v1/admin/auth<br/>{"client_id", "client_secret"}
        BR-->>SC: {"access_token": "admin-jwt"}

        SC->>BR: POST /v1/admin/sidecar-activations<br/>{"allowed_scopes": [...]}
        BR-->>SC: {"activation_token": "jwt"}

        SC->>BR: POST /v1/sidecar/activate<br/>{"sidecar_activation_token": "jwt"}
        BR-->>SC: {"access_token": "sidecar-bearer",<br/>"sidecar_id": "..."}
    end

    Note over SC: Bootstrap succeeded

    SC->>HTTP: Register routes:<br/>/v1/token, /v1/token/renew,<br/>/v1/challenge, /v1/register

    SC->>SC: Start renewal goroutine<br/>(renew at 80% TTL)
    SC->>SC: Start health probe goroutine<br/>(circuit breaker recovery)

    Note over SC: Ready to serve agents
```

### Token Exchange Flow

When a developer requests a token via the sidecar:

```mermaid
sequenceDiagram
    participant D as Developer App
    participant SC as Sidecar
    participant REG as Agent Registry
    participant BR as Broker

    D->>SC: POST /v1/token<br/>{"agent_name", "scope", "task_id"}
    SC->>SC: scopeIsSubset(requested, ceiling)

    SC->>REG: Lookup agent by name+task
    alt Agent not registered
        SC->>BR: POST /v1/admin/auth
        BR-->>SC: admin JWT
        SC->>BR: POST /v1/admin/launch-tokens<br/>{agent_name, scope ceiling}
        BR-->>SC: {launch_token}
        SC->>SC: Generate Ed25519 key pair
        SC->>BR: GET /v1/challenge
        BR-->>SC: {nonce}
        SC->>SC: Sign nonce with private key
        SC->>BR: POST /v1/register<br/>{launch_token, nonce, pub_key, sig, ...}
        BR-->>SC: {agent_id, access_token}
        SC->>REG: Store agent (keys, SPIFFE ID)
    end

    SC->>BR: POST /v1/token/exchange<br/>Bearer: sidecar-token<br/>{"agent_id", "scope", "ttl"}
    BR-->>SC: {"access_token", "expires_in"}

    SC->>REG: Cache token for failsafe
    SC-->>D: {"access_token", "expires_in",<br/>"scope", "agent_id"}
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

    MUX --> PUB["Public Handlers\n(health, challenge,\nmetrics, validate)"]

    MUX --> MB1["MaxBytesBody"] --> AUTH_ONLY["ValMw.Wrap"] --> AUTH_H["Auth Handlers\n(renew, delegate)"]

    MUX --> MB2["MaxBytesBody"] --> SCOPE_W["ValMw.Wrap"] --> SCOPE_C["ValMw\n.RequireScope"] --> ADMIN_H["Scoped Handlers\n(exchange, revoke,\nlaunch-tokens,\nsidecar-activations)"]

    MUX --> RL["RateLimiter\n.Wrap"] --> RL_H["Rate-Limited\n(admin/auth,\nsidecar/activate)"]

    MUX --> MB3["MaxBytesBody"] --> REG_H["Register\n(launch token\nin body)"]
```

**Route-to-middleware mapping from `cmd/broker/main.go`:**

| Route | Middleware Chain |
|---|---|
| `GET /v1/challenge` | RequestID -> Logging -> Handler |
| `GET /v1/health` | RequestID -> Logging -> Handler |
| `GET /v1/metrics` | RequestID -> Logging -> Handler |
| `POST /v1/token/validate` | RequestID -> Logging -> MaxBytesBody -> Handler |
| `POST /v1/register` | RequestID -> Logging -> MaxBytesBody -> Handler |
| `POST /v1/token/renew` | RequestID -> Logging -> MaxBytesBody -> ValMw -> Handler |
| `POST /v1/delegate` | RequestID -> Logging -> MaxBytesBody -> ValMw -> Handler |
| `POST /v1/token/exchange` | RequestID -> Logging -> MaxBytesBody -> ValMw -> ValMw.RequireScope(`sidecar:manage:*`) -> Handler |
| `POST /v1/revoke` | RequestID -> Logging -> MaxBytesBody -> ValMw -> ValMw.RequireScope(`admin:revoke:*`) -> Handler |
| `GET /v1/audit/events` | RequestID -> Logging -> ValMw -> ValMw.RequireScope(`admin:audit:*`) -> Handler |
| `POST /v1/admin/auth` | RequestID -> Logging -> RateLimiter(5/s, burst 10) -> Handler |
| `POST /v1/admin/launch-tokens` | RequestID -> Logging -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `POST /v1/admin/sidecar-activations` | RequestID -> Logging -> ValMw -> ValMw.RequireScope(`admin:launch-tokens:*`) -> Handler |
| `POST /v1/sidecar/activate` | RequestID -> Logging -> RateLimiter(5/s, burst 10) -> Handler |

---

## Key Design Decisions

1. **In-memory storage.** All state (nonces, agents, launch tokens, revocations, audit events) lives in memory behind `sync.RWMutex`. The type is named `SqlStore` as a placeholder for a planned SQL migration. Restarting the broker clears all state.

2. **Fresh Ed25519 keys every startup.** The broker generates a new signing key pair on each start via `crypto/rand`. All previously issued tokens become unverifiable. This is intentional -- long-lived tokens are an anti-pattern for ephemeral credentialing.

3. **Scope attenuation is one-way.** Scopes can only narrow, never expand. Enforced at registration (requested vs. launch token ceiling), delegation (delegated vs. delegator scope), and token exchange (requested vs. sidecar ceiling entries).

4. **Scope check before launch token consumption.** At registration, `ScopeIsSubset` is called before `ConsumeLaunchToken`. A scope violation returns an error without wasting a single-use token.

5. **Constant-time secret comparison.** Admin authentication uses `subtle.ConstantTimeCompare` to prevent timing attacks on `AA_ADMIN_SECRET`.

6. **Sidecar anti-spoof.** The `sidecar_id` field in token exchange is always derived from the authenticated caller token's `sid` claim. Client-supplied values are ignored.

7. **Mutual auth not HTTP-exposed.** `MutAuthHdl` in `internal/mutauth` provides a 3-step mutual authentication handshake, but it is not registered on any HTTP mux. It exists as a Go API only, tested in unit tests, intended for future HTTP exposure.

8. **Circuit breaker pattern.** The sidecar implements a sliding-window circuit breaker with three states (Closed -> Open -> Probing) for broker connectivity. Failure rate threshold and window duration are configurable via `AA_SIDECAR_CB_*` env vars.

9. **Token caching for failsafe fallback.** The sidecar caches the last-issued token per agent. When the circuit breaker is open and the broker is unreachable, cached tokens are served with an `X-AgentAuth-Cached: true` response header.

10. **BYOK support.** The sidecar supports "Bring Your Own Key" registration where developers provide their own Ed25519 key pairs through `POST /v1/register`, instead of relying on sidecar-managed keys.

---

## Security Assumptions

These are explicit trust boundaries and limitations of the current implementation:

- **X-Forwarded-For trusted unconditionally.** The `clientIP()` function in `internal/authz/rate_mw.go` trusts the first entry in `X-Forwarded-For` without validation. In production, the broker must sit behind a trusted reverse proxy that sets this header correctly. Without a trusted proxy, rate limiting can be bypassed via header spoofing.

- **In-memory state is mostly not persistent.** A broker restart clears nonces, agent records, launch tokens, and revocation entries. All previously issued tokens become unverifiable (new signing keys). **Exception:** Audit events are now persisted to SQLite when `AA_DB_PATH` is configured. On startup, the broker reloads all audit events from SQLite and rebuilds the in-memory hash chain. This means the audit trail survives restarts, but all other operational state is lost.

- **Single broker instance.** There is no replication, consensus, or shared state mechanism. The broker is a single process. Running multiple instances would result in split-brain token verification (each instance has its own signing key).

- **Nonce window is 30 seconds.** Nonces expire after 30 seconds. Agents must complete the challenge-response flow within this window. Clock skew between agent and broker can cause failures.

- **Admin secret is the root of trust.** `AA_ADMIN_SECRET` is the single shared secret that bootstraps the entire system. If compromised, an attacker can issue arbitrary launch tokens and sidecar activations.

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
| Go stdlib `crypto/subtle` | -- | Constant-time admin secret comparison |
