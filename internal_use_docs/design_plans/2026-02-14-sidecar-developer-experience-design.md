# Sidecar Developer Experience Design

**Date:** 2026-02-14
**Status:** Approved
**Author:** Divine + Dai
**Pattern Reference:** `plans/archive/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md` (v1.2)
**ADR Reference:** `plans/archive/adrs/ADR-005-sidecar-first-developer-bootstrap.md`

---

## Problem

The AgentAuth broker is functional (14 endpoints, all tests pass, live smoketest exercises full flow). But a 3rd party developer cannot use it without understanding admin auth, launch tokens, Ed25519 challenge-response, scope attenuation, and token exchange. This violates ADR-005's core decision:

> "Developer apps do not use admin-secret workflows directly."
> "A local sidecar handles broker bootstrap/exchange and serves short-lived bearer tokens to the app."

The sidecar process — the thing that abstracts broker complexity from developers — was never built. Only the broker-side endpoints exist.

---

## Decision

Build a Go sidecar binary that auto-bootstraps with the broker and exposes a simple HTTP API to developers. Deploy via docker compose alongside the broker.

---

## What the Developer Sees

A 3rd party developer gets ONE thing: a sidecar URL (e.g. `http://localhost:8081`).

Their agent code:

```python
import requests

# One HTTP call. No crypto, no admin auth, no launch tokens.
resp = requests.post("http://localhost:8081/v1/token", json={
    "agent_name": "data-reader",
    "task_id": "task-789",
    "scope": ["read:data:*"]
})
token = resp.json()["access_token"]

# Use the token to access resources
headers = {"Authorization": f"Bearer {token}"}
data = requests.get("http://resource-server:9090/data/report-42", headers=headers)
```

---

## Sidecar API (Developer-Facing)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/v1/token` | Request a scoped token |
| `POST` | `/v1/token/renew` | Renew a token before expiry |
| `GET` | `/v1/health` | Sidecar readiness check |

### POST /v1/token

**Request:**
```json
{
  "agent_name": "data-reader",
  "task_id": "task-789",
  "scope": ["read:data:*"],
  "ttl": 300
}
```

**Response (200):**
```json
{
  "access_token": "<jwt>",
  "expires_in": 300,
  "scope": ["read:data:*"]
}
```

**Errors:**
- `400` — missing fields, invalid scope format
- `403` — requested scope exceeds sidecar's scope ceiling
- `502` — broker unavailable

### POST /v1/token/renew

**Request:**
```
Authorization: Bearer <current-jwt>
```
(empty body)

**Response (200):**
```json
{
  "access_token": "<new-jwt>",
  "expires_in": 300
}
```

### GET /v1/health

**Response (200):**
```json
{
  "status": "ok",
  "broker_connected": true,
  "scope_ceiling": ["read:data:*", "write:data:*"]
}
```

---

## Architecture

```
docker-compose.yml
┌──────────────────────────────────────────────────────────────┐
│  docker compose network                                       │
│                                                               │
│  ┌────────────┐          ┌────────────┐     ┌─────────────┐  │
│  │   Broker    │:8080     │  Sidecar   │:8081│ Developer's │  │
│  │    (Go)     │◄─────────│   (Go)     │◄────│   Agent     │  │
│  │             │  broker  │            │ dev │ (any lang)  │  │
│  │ Admin auth  │  API     │ Auto-      │ API │             │  │
│  │ Launch tkns │          │ bootstrap  │     │ POST        │  │
│  │ Challenge   │          │ Token      │     │ /v1/token   │  │
│  │ Register    │          │ exchange   │     │ → gets JWT  │  │
│  │ Exchange    │          │ Renewal    │     │             │  │
│  │ Revoke      │          │ Scope gate │     │             │  │
│  │ Audit       │          │            │     │             │  │
│  └────────────┘          └────────────┘     └─────────────┘  │
│       ▲                                                       │
│       │ validate token                                        │
│       │                   ┌─────────────┐                     │
│       └───────────────────│  Resource   │                     │
│                           │   Server    │                     │
│                           └─────────────┘                     │
└──────────────────────────────────────────────────────────────┘

Developer never touches:           Developer uses:
  - AA_ADMIN_SECRET                  - Sidecar URL
  - Admin auth flow                  - POST /v1/token
  - Launch tokens                    - Bearer JWT for resources
  - Challenge-response
  - Ed25519 keys
  - Token exchange
```

---

## Sidecar Auto-Bootstrap Flow

On `docker compose up`, the sidecar self-activates with zero human interaction:

```
                                    Sidecar                              Broker
                                      │                                    │
                              START   │                                    │
                                │     │                                    │
                                v     │                                    │
                        ┌─────────────┐                                    │
                        │ Read env:   │                                    │
                        │ BROKER_URL  │                                    │
                        │ ADMIN_SECRET│                                    │
                        │ SCOPE_CEIL  │                                    │
                        └──────┬──────┘                                    │
                               │                                           │
                               v                                           │
                        ┌─────────────┐    GET /v1/health                  │
                        │ Wait for    │──────────────────────────────────>│
                        │ broker ready│    200 OK                          │
                        │ (retry loop)│<──────────────────────────────────│
                        └──────┬──────┘                                    │
                               │                                           │
                               v                                           │
                        ┌─────────────┐    POST /v1/admin/auth             │
                        │ Step 1:     │    {client_secret: AA_ADMIN_SECRET}│
                        │ Admin auth  │──────────────────────────────────>│
                        │             │    200 {access_token: admin-jwt}    │
                        │             │<──────────────────────────────────│
                        └──────┬──────┘                                    │
                               │                                           │
                               v                                           │
                        ┌─────────────┐    POST /v1/admin/sidecar-activ.   │
                        │ Step 2:     │    Authorization: Bearer admin-jwt │
                        │ Get activ.  │    {scope_prefix, ttl}             │
                        │ token       │──────────────────────────────────>│
                        │             │    201 {activation_token}           │
                        │             │<──────────────────────────────────│
                        └──────┬──────┘                                    │
                               │                                           │
                               v                                           │
                        ┌─────────────┐    POST /v1/sidecar/activate       │
                        │ Step 3:     │    {sidecar_activation_token}      │
                        │ Activate    │──────────────────────────────────>│
                        │ (single-use)│    200 {access_token: sidecar-jwt} │
                        │             │<──────────────────────────────────│
                        └──────┬──────┘                                    │
                               │                                           │
                               v                                           │
                        ┌─────────────┐                                    │
                        │   READY     │                                    │
                        │ Serving on  │                                    │
                        │ :8081       │                                    │
                        │             │                                    │
                        │ Background: │    POST /v1/token/renew (periodic) │
                        │ auto-renew  │──────────────────────────────────>│
                        └─────────────┘                                    │
```

---

## Token Request Flow (Developer → Sidecar → Broker)

```
Developer Agent                    Sidecar (:8081)                      Broker (:8080)
     │                                  │                                    │
     │  POST /v1/token                  │                                    │
     │  {                               │                                    │
     │    "agent_name": "data-reader",  │                                    │
     │    "task_id": "task-789",        │                                    │
     │    "scope": ["read:data:*"]      │                                    │
     │  }                               │                                    │
     │─────────────────────────────────>│                                    │
     │                                  │                                    │
     │                                  │  1. Validate: scope within ceiling │
     │                                  │     (local check, instant)         │
     │                                  │                                    │
     │                                  │  2. POST /v1/token/exchange        │
     │                                  │     Auth: Bearer <sidecar-jwt>     │
     │                                  │     {agent_id, scope, ttl}         │
     │                                  │────────────────────────────────────>│
     │                                  │                                    │
     │                                  │          Validate sidecar token    │
     │                                  │          Enforce scope attenuation │
     │                                  │          Issue JWT with sidecar_id │
     │                                  │          Audit: exchange event     │
     │                                  │                                    │
     │                                  │  200 {access_token, expires_in}    │
     │                                  │<────────────────────────────────────│
     │                                  │                                    │
     │  200 OK                          │                                    │
     │  {access_token, expires_in,      │                                    │
     │   scope}                         │                                    │
     │<─────────────────────────────────│                                    │
```

---

## Docker Compose Configuration

```yaml
# docker-compose.yml
services:
  broker:
    build:
      context: .
      target: broker
    ports:
      - "8080:8080"
    environment:
      AA_ADMIN_SECRET: ${AA_ADMIN_SECRET}
      AA_PORT: "8080"
      AA_TRUST_DOMAIN: "agentauth.local"
      AA_DEFAULT_TTL: "300"
      AA_LOG_LEVEL: "verbose"

  sidecar:
    build:
      context: .
      target: sidecar
    ports:
      - "8081:8081"
    environment:
      AA_BROKER_URL: "http://broker:8080"
      AA_ADMIN_SECRET: ${AA_ADMIN_SECRET}
      AA_SIDECAR_SCOPE_CEILING: "read:data:*,write:data:*"
      AA_SIDECAR_PORT: "8081"
    depends_on:
      broker:
        condition: service_healthy
```

**Developer's `.env` file:**
```
AA_ADMIN_SECRET=change-this-to-a-real-secret
```

**Developer runs:** `docker compose up` — done.

---

## Go Package Layout

```
cmd/
  broker/
    main.go                    ← EXISTING (no changes)
  sidecar/
    main.go                    ← NEW: sidecar HTTP server + bootstrap
    bootstrap.go               ← NEW: auto-activation sequence (3-step)
    handler.go                 ← NEW: /v1/token, /v1/token/renew, /v1/health
    config.go                  ← NEW: sidecar-specific env vars
```

The sidecar binary:
- Lives in the same repo as the broker
- Shares NO internal packages (talks to broker via HTTP only)
- Compiles to a separate static binary
- Multi-stage Dockerfile builds both `broker` and `sidecar` targets

---

## Environment Variables (Sidecar)

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_BROKER_URL` | `http://localhost:8080` | Broker base URL |
| `AA_ADMIN_SECRET` | (required) | Shared with broker for auto-activation |
| `AA_SIDECAR_SCOPE_CEILING` | (required) | Comma-separated max scopes the sidecar can issue |
| `AA_SIDECAR_PORT` | `8081` | Sidecar HTTP port |
| `AA_SIDECAR_LOG_LEVEL` | `standard` | Logging level |

---

## Pattern Compliance (Phased)

```
┌──────────────────────────────────────────────────────────────┐
│              SECURITY PATTERN COMPONENTS                       │
│                                                                │
│  Phase 1 (Token Exchange)           Phase 2 (Registration)     │
│  ─────────────────────────          ───────────────────────    │
│  [x] 1. Ephemeral Identity          [x] Full per-agent        │
│        (via sidecar_id +                 SPIFFE identity       │
│         agent metadata)                                        │
│  [x] 2. Short-Lived Scoped Tokens   [x] Same                  │
│  [x] 3. Zero-Trust Enforcement      [x] Same                  │
│  [x] 4. Expiration & Revocation     [x] + Per-agent revoke    │
│  [x] 5. Audit Logging               [x] Full per-agent audit  │
│  [ ] 6. Mutual Auth                 [x] Agent keypairs enable  │
│  [ ] 7. Delegation Chains           [x] Agent SPIFFE IDs      │
│                                          enable                │
│                                                                │
│  Phase 3 (Mutual Auth + Delegation)                            │
│  ──────────────────────────────────                            │
│  [x] 6. Mutual Auth — wire mutauth routes,                    │
│         sidecar-to-sidecar handshake                           │
│  [x] 7. Delegation — per-agent SPIFFE IDs                     │
│         enable full chain verification                         │
│                                                                │
│  Phase 4 (Demo + Attack Scenarios)                             │
│  ─────────────────────────────────                             │
│  3 agents, resource server, 5 attack scenarios                 │
│  Insecure vs secure side-by-side comparison                    │
│                                                                │
│  Phase 5 (Cloud IAM Federation)                                │
│  ──────────────────────────────                                │
│  OIDC endpoints, ES256 federation key                          │
│  (planning branch already exists)                              │
│                                                                │
│  Developer API stays the same across all phases:               │
│  POST /v1/token → JWT                                          │
└──────────────────────────────────────────────────────────────┘
```

---

## Scope of Work (Phase 1 Only)

**Build:**
1. `cmd/sidecar/` — Go sidecar binary (~4 files, ~400 lines)
2. Updated `docker-compose.yml` — adds sidecar service
3. Updated `Dockerfile` — multi-stage build for both binaries
4. This design doc

**Do NOT build yet:**
- Python SDK wrapper (Phase 1b)
- Per-agent registration through sidecar (Phase 2)
- Mutual auth routes (Phase 3)
- Demo with 3 agents + 5 attacks (Phase 4)
- Resource server (Phase 4)
- Cloud IAM federation (Phase 5, planning branch exists)

---

## Success Criteria

| Criterion | Measurement |
|-----------|-------------|
| Developer runs `docker compose up` and sidecar is ready | Health check returns `ok` within 10 seconds |
| Developer calls `POST /v1/token` with scope | Receives valid JWT |
| JWT validates against broker | `POST /v1/token/validate` returns `valid: true` |
| Scope ceiling enforced | Request exceeding ceiling returns `403` |
| Sidecar auto-renews its own token | No manual intervention needed |
| Developer sees zero admin/bootstrap concepts | No AA_ADMIN_SECRET, no launch tokens, no Ed25519 in their code |

---

**END OF DESIGN**
