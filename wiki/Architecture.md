# Architecture

How AgentAuth is built internally. Useful for contributors, advanced operators, and anyone who wants to understand the system deeply.

---

## System Overview

```
┌──────────────────────────────────────────────────────────┐
│                    Clients                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │ AI Agent │  │ Operator │  │ Resource │               │
│  │          │  │ (aactl)  │  │ Server   │               │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘               │
│       │              │              │                     │
└───────┼──────────────┼──────────────┼─────────────────────┘
        │              │              │
        ▼              ▼              ▼
┌──────────────────────────────────────────────────────────┐
│                    Sidecar (optional)                     │
│  ┌────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ Token Mgmt │  │ Key Manager  │  │ Circuit      │     │
│  │ (acquire,  │  │ (Ed25519 gen,│  │ Breaker      │     │
│  │  renew)    │  │  sign, reg)  │  │              │     │
│  └────────────┘  └──────────────┘  └──────────────┘     │
└──────────────────────────┬───────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────┐
│                    Broker                                 │
│                                                           │
│  ┌─── Protocol Layer ─────────────────────────────────┐  │
│  │  HTTP Router → Middleware → Handlers                │  │
│  └───────────────────────────────────────────────────┘  │
│  ┌─── Domain Layer ───────────────────────────────────┐  │
│  │  AgentSvc │ TokenSvc │ DelegSvc │ RevokeSvc       │  │
│  └───────────────────────────────────────────────────┘  │
│  ┌─── Security Layer ────────────────────────────────┐  │
│  │  Ed25519 Signer │ JWT Codec │ Scope Engine        │  │
│  └───────────────────────────────────────────────────┘  │
│  ┌─── Foundation Layer ──────────────────────────────┐  │
│  │  SQLite Store │ Audit Trail │ Config              │  │
│  └───────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
agentauth-core/
├── cmd/
│   ├── broker/          # Broker entry point (main.go)
│   ├── sidecar/         # Sidecar entry point (main.go)
│   └── aactl/           # CLI tool entry point (main.go)
├── internal/
│   ├── broker/          # Broker business logic
│   │   ├── handler/     # HTTP handlers for each endpoint
│   │   ├── service/     # Domain services (agent, token, delegation, revocation)
│   │   ├── store/       # Data access layer (SQLite)
│   │   ├── middleware/  # HTTP middleware (auth, logging, rate limiting)
│   │   └── config/      # Configuration parsing
│   ├── sidecar/         # Sidecar business logic
│   │   ├── handler/     # Sidecar HTTP handlers
│   │   ├── bootstrap/   # Auto-activation sequence
│   │   └── circuit/     # Circuit breaker implementation
│   ├── crypto/          # Shared cryptography (Ed25519, JWT)
│   ├── audit/           # Audit trail implementation
│   ├── scope/           # Scope parsing, matching, attenuation
│   └── spiffe/          # SPIFFE ID generation and validation
├── docs/                # Documentation source
├── scripts/             # Utility scripts (cert generation, etc.)
├── Dockerfile           # Multi-stage build (broker + sidecar targets)
├── docker-compose.yml   # Default development stack
├── docker-compose.tls.yml    # TLS overlay
├── docker-compose.mtls.yml   # mTLS overlay
├── docker-compose.uds.yml    # UDS overlay
├── go.mod               # Go module definition
└── go.sum               # Dependency checksums
```

---

## Component Architecture

### 5 Layers

| Layer | Purpose | Key Packages |
|-------|---------|-------------|
| **Foundation** | Storage, audit, configuration | `store/`, `audit/`, `config/` |
| **Security** | Cryptography, JWT, scope logic | `crypto/`, `scope/` |
| **Domain** | Business logic (agents, tokens, delegation) | `service/` |
| **Transport** | HTTP routing, middleware | `handler/`, `middleware/` |
| **Protocol** | Wire format, error encoding | RFC 7807 errors, JSON encoding |

### Data Flow: Token Acquisition (via Sidecar)

```
Developer App                  Sidecar                        Broker
     │                           │                              │
     ├─ POST /v1/token ─────────▶│                              │
     │  {agent_name, scope}      │                              │
     │                           ├─ Generate Ed25519 key pair   │
     │                           ├─ GET /v1/challenge ─────────▶│
     │                           │◀─ {nonce} ───────────────────│
     │                           ├─ Sign nonce with private key │
     │                           ├─ POST /v1/register ─────────▶│
     │                           │  {pub_key, sig, nonce,       │
     │                           │   launch_token, scope}       │
     │                           │                              ├─ Verify signature
     │                           │                              ├─ Validate launch token
     │                           │                              ├─ Check scope ⊆ ceiling
     │                           │                              ├─ Generate SPIFFE ID
     │                           │                              ├─ Issue JWT (EdDSA)
     │                           │                              ├─ Log to audit trail
     │                           │◀─ {agent_id, access_token} ──│
     │◀─ {access_token, scope} ──│                              │
     │                           │                              │
```

### Data Flow: Token Validation

```
Resource Server                                  Broker
     │                                             │
     ├─ POST /v1/token/validate ──────────────────▶│
     │  {token: "eyJ..."}                          │
     │                                             ├─ Parse JWT header
     │                                             ├─ Verify EdDSA signature
     │                                             ├─ Check expiration
     │                                             ├─ Check revocation list
     │                                             ├─ Extract claims
     │◀─ {valid: true, claims: {...}} ─────────────│
     │                                             │
```

---

## Security Design

### Token Signing

- **Algorithm:** EdDSA (Ed25519) — fast, compact, deterministic signatures
- **Key lifecycle:** Generated fresh on every broker startup (ephemeral by design)
- **JWT claims:** Standard (`iss`, `sub`, `exp`, `iat`, `jti`) plus custom (`scope`, `task_id`, `orch_id`, `chain_hash`)

### Scope Engine

Scopes follow `action:resource:identifier` format. The engine supports:

- **Wildcard matching:** `read:data:*` matches `read:data:customers`
- **Subset checking:** `read:data:customers` ⊆ `read:data:*`
- **Attenuation:** Delegated scopes must be ⊆ delegator's scopes

### Audit Trail

- **Hash-chained:** Each event includes SHA-256 of the previous event
- **Tamper-evident:** Modifying any event breaks the chain
- **Structured:** Machine-readable JSON with event type, agent ID, outcome, detail

### Rate Limiting

- `/v1/admin/auth`: 5 req/s per IP (burst 10)
- `/v1/sidecar/activate`: Same limits
- Other endpoints: No rate limiting (rely on token TTL for abuse prevention)

---

## Technology Choices

| Component | Technology | Why |
|-----------|-----------|-----|
| Language | Go 1.24+ | Performance, static binaries, crypto stdlib |
| HTTP | Standard library `net/http` | No framework dependencies |
| Crypto | Ed25519 (stdlib) | Fast signatures, small keys (32 bytes) |
| JWT | Custom EdDSA codec | No third-party JWT dependency |
| Storage | SQLite | Zero-config, single-file, embedded |
| CLI | Cobra | Standard Go CLI framework |
| Containers | Multi-stage Dockerfile | Small images (~20MB) |
| Error Format | RFC 7807 | Industry standard, machine-readable |
| Identity | SPIFFE | Industry standard for workload identity |

---

## Design Principles

1. **Standard library first:** Minimize external dependencies
2. **Zero-trust:** Validate everything on every request
3. **Ephemeral by default:** Keys and tokens are short-lived
4. **Defense in depth:** Multiple layers of security controls
5. **Immutable audit:** Every action is logged, hash-chained
6. **Scope attenuation:** Permissions only narrow, never expand
7. **Minimal state:** Broker can restart without persistent state

---

## Next Steps

- [[Contributing]] — How to contribute code
- [[Security]] — Security model and threat analysis
- [[API Reference]] — Endpoint documentation
