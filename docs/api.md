# API Reference

> **Document Version:** 2.0 | **Last Updated:** February 2026 | **Status:** Current
>
> **Audience:** Developers and operators who need the definitive contract for every endpoint.
>
> **Prerequisites:** [Concepts](concepts.md) for background, [Getting Started: Developer](getting-started-developer.md) or [Getting Started: Operator](getting-started-operator.md) for walkthroughs.
>
> **Next steps:** [Troubleshooting](troubleshooting.md) for error resolution | [Common Tasks](common-tasks.md) for step-by-step workflows.

---

## Overview

AgentAuth exposes a JSON HTTP API. All request and response bodies use `Content-Type: application/json`. The broker listens on port 8080 by default (`AA_PORT`), and the sidecar on port 8081 (`AA_SIDECAR_PORT`).

All error responses use RFC 7807 `application/problem+json` format:

```json
{
  "type": "urn:agentauth:error:{errType}",
  "title": "HTTP Status Text",
  "status": 400,
  "detail": "human-readable description",
  "instance": "/v1/endpoint",
  "error_code": "specific_code",
  "request_id": "hex-id",
  "hint": "optional guidance"
}
```

The `error_code` field is always present. The `hint` field is only present on extended error responses (token exchange, sidecar activation).

All responses include an `X-Request-ID` header. If the client sends `X-Request-ID`, it is propagated; otherwise the broker generates one.

Request bodies are limited to 1 MB on POST endpoints.

---

## End-to-End Authentication Flow

This diagram shows the complete flow from operator bootstrap through developer token usage:

```mermaid
sequenceDiagram
    participant Admin as Operator
    participant BR as Broker
    participant SC as Sidecar
    participant DEV as Developer App

    Note over Admin,BR: 1. Operator Bootstrap
    Admin->>BR: POST /v1/admin/auth<br/>{"client_id", "client_secret"}
    BR-->>Admin: {"access_token": "admin-jwt"}

    Admin->>BR: POST /v1/admin/launch-tokens<br/>{"agent_name", "allowed_scope", ...}
    BR-->>Admin: {"launch_token": "64-hex-chars"}

    Note over SC,BR: 2. Sidecar Bootstrap
    SC->>BR: POST /v1/admin/auth
    BR-->>SC: admin JWT
    SC->>BR: POST /v1/admin/sidecar-activations
    BR-->>SC: activation JWT
    SC->>BR: POST /v1/sidecar/activate
    BR-->>SC: sidecar bearer token

    Note over DEV,BR: 3. Developer Token Request
    DEV->>SC: POST /v1/token<br/>{"agent_name", "scope", "task_id"}
    SC->>BR: (challenge, register, exchange)
    BR-->>SC: agent JWT
    SC-->>DEV: {"access_token", "agent_id"}

    Note over DEV: 4. Use Token
    DEV->>DEV: Authorization: Bearer <token>
```

---

## Authentication Mechanisms

Three mechanisms are used, depending on the endpoint:

1. **None** -- Public endpoints (health, metrics, challenge, validate, admin auth, sidecar activate)
2. **Bearer token** -- JWT in the `Authorization: Bearer <token>` header. The `ValMw` middleware verifies signature, checks revocation, and injects claims into context.
3. **Launch token** -- Passed in the request body field `launch_token` during agent registration. Not a Bearer token.

Some endpoints require specific scopes in addition to a valid Bearer token. These are noted per-endpoint below.

---

## Broker Endpoints

### Public Endpoints (no auth required)

---

#### GET /v1/challenge

Generate a cryptographic nonce for agent registration.

**Auth:** None

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `nonce` | string | 64-character hex nonce |
| `expires_in` | int | TTL in seconds (always 30) |

```bash
curl http://localhost:8080/v1/challenge
```

```json
{
  "nonce": "a1b2c3d4e5f6...64chars",
  "expires_in": 30
}
```

---

#### GET /v1/health

Broker health check.

**Auth:** None

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `status` | string | Always `"ok"` |
| `version` | string | Broker version (currently `"2.0.0"`) |
| `uptime` | int | Seconds since startup |
| `db_connected` | bool | Whether the SQLite audit database is connected and responsive. `false` if `AA_DB_PATH` is unset or the database is unreachable. |
| `audit_events_count` | int | Total number of audit events in the in-memory log. Useful for verifying persistence — this count should survive broker restarts when `AA_DB_PATH` is configured. |

```bash
curl http://localhost:8080/v1/health
```

```json
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 42,
  "db_connected": true,
  "audit_events_count": 56
}
```

---

#### GET /v1/metrics

Prometheus metrics endpoint.

**Auth:** None

Returns Prometheus text exposition format. See [Prometheus Metrics](#prometheus-metrics) for the full metric list.

```bash
curl http://localhost:8080/v1/metrics
```

---

#### POST /v1/token/validate

Verify a token and return its claims. Also checks revocation status.

**Auth:** None

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `token` | string | Yes | JWT string to validate |

**Response 200 (valid):**

```json
{
  "valid": true,
  "claims": {
    "iss": "agentauth",
    "sub": "spiffe://agentauth.local/agent/orch/task/instance",
    "exp": 1707600000,
    "nbf": 1707599700,
    "iat": 1707599700,
    "jti": "a1b2c3d4e5f67890...",
    "scope": ["read:data:*"],
    "task_id": "task-001",
    "orch_id": "my-orchestrator"
  }
}
```

**Response 200 (invalid or revoked):**

```json
{
  "valid": false,
  "error": "token expired"
}
```

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 400 | `invalid_request` | Missing `token` field or malformed JSON |

```bash
curl -X POST http://localhost:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d '{"token": "eyJ..."}'
```

---

#### POST /v1/admin/auth

Authenticate as an administrator using the shared secret.

**Auth:** None (rate-limited: 5 req/s, burst 10)

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `client_id` | string | Yes | Identifier for the admin client |
| `client_secret` | string | Yes | Value of `AA_ADMIN_SECRET` |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `access_token` | string | Admin JWT (TTL 300s) |
| `expires_in` | int | Always 300 |
| `token_type` | string | Always `"Bearer"` |

The admin JWT carries scopes: `admin:launch-tokens:*`, `admin:revoke:*`, `admin:audit:*`.

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 400 | `invalid_request` | Missing `client_id` or `client_secret` |
| 401 | `unauthorized` | Invalid credentials |
| 429 | `rate_limited` | Rate limit exceeded (`Retry-After: 1` header) |

```bash
curl -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id": "admin", "client_secret": "my-dev-secret"}'
```

```json
{
  "access_token": "eyJ...",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

---

#### POST /v1/sidecar/activate

Exchange a single-use sidecar activation token for a functional sidecar bearer token.

**Auth:** None (rate-limited: 5 req/s, burst 10; single-use JTI consumption)

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `sidecar_activation_token` | string | Yes | JWT from `POST /v1/admin/sidecar-activations` |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `access_token` | string | Sidecar bearer JWT (TTL up to 900s) |
| `expires_in` | int | TTL in seconds |
| `token_type` | string | Always `"Bearer"` |
| `sidecar_id` | string | JTI of the consumed activation token |

The sidecar bearer token has `sub="sidecar:{jti}"` and scopes including `sidecar:manage:*` plus `sidecar:scope:{each_allowed_scope}`.

**Error responses:**

| Status | Error Code | Condition |
|---|---|---|
| 400 | `invalid_request` | Malformed JSON |
| 401 | `invalid_activation_token` | Invalid, expired, or malformed activation token |
| 401 | `activation_token_replayed` | Activation token already used |
| 429 | `rate_limited` | Rate limit exceeded (`Retry-After: 1` header) |
| 500 | `internal_error` | Activation failed |

```bash
curl -X POST http://localhost:8080/v1/sidecar/activate \
  -H "Content-Type: application/json" \
  -d '{"sidecar_activation_token": "eyJ..."}'
```

```json
{
  "access_token": "eyJ...",
  "expires_in": 600,
  "token_type": "Bearer",
  "sidecar_id": "abc123..."
}
```

---

### Agent Endpoints (Bearer token required)

---

#### POST /v1/register

Register an agent via challenge-response. The agent must have obtained a nonce from `GET /v1/challenge` and signed it with its Ed25519 private key.

**Auth:** Launch token (in request body, not Bearer)

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `launch_token` | string | Yes | 64-char hex launch token from admin |
| `nonce` | string | Yes | Nonce from GET /v1/challenge |
| `public_key` | string | Yes | Base64-encoded Ed25519 public key (32 bytes) |
| `signature` | string | Yes | Base64-encoded Ed25519 signature of nonce bytes |
| `orch_id` | string | Yes | Orchestration identifier |
| `task_id` | string | Yes | Task identifier |
| `requested_scope` | string[] | Yes | Scopes to request (must be subset of launch token's allowed_scope) |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `agent_id` | string | SPIFFE ID: `spiffe://{domain}/agent/{orch}/{task}/{instance}` |
| `access_token` | string | EdDSA-signed JWT |
| `expires_in` | int | Token TTL in seconds |

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 400 | `invalid_request` | Malformed JSON or missing required fields |
| 401 | `unauthorized` | Invalid/expired/consumed launch token, invalid nonce, bad signature, bad public key |
| 403 | `scope_violation` | Requested scope exceeds launch token's allowed scope |
| 500 | `internal_error` | Unexpected failure |

```bash
curl -X POST http://localhost:8080/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "launch_token": "a1b2c3d4...64chars",
    "nonce": "deadbeef...64chars",
    "public_key": "base64EncodedEd25519PubKey==",
    "signature": "base64EncodedSignatureOfNonceBytes==",
    "orch_id": "my-orchestrator",
    "task_id": "task-001",
    "requested_scope": ["read:data:*"]
  }'
```

```json
{
  "agent_id": "spiffe://agentauth.local/agent/my-orchestrator/task-001/a1b2c3d4e5f6",
  "access_token": "eyJ...",
  "expires_in": 300
}
```

---

#### POST /v1/token/renew

Renew an existing token with fresh timestamps and a new JTI. The original token remains valid until its own expiry.

**Auth:** Bearer token (validated by `ValMw`)

**Request body:** None (token is read from Authorization header)

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `access_token` | string | New JWT with fresh timestamps |
| `expires_in` | int | TTL in seconds |

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 401 | `unauthorized` | Missing, invalid, expired, or revoked Bearer token |

```bash
curl -X POST http://localhost:8080/v1/token/renew \
  -H "Authorization: Bearer eyJ..."
```

```json
{
  "access_token": "eyJ...",
  "expires_in": 300
}
```

---

#### POST /v1/delegate

Create a scope-attenuated delegation token for another registered agent.

**Auth:** Bearer token (validated by `ValMw`)

```mermaid
sequenceDiagram
    participant A as Agent A (delegator)
    participant BR as Broker
    participant B as Agent B (delegate)

    A->>BR: POST /v1/delegate<br/>Bearer: agent-a-token<br/>{"delegate_to", "scope", "ttl"}
    BR->>BR: Verify Bearer (ValMw)
    BR->>BR: Check depth < 5
    BR->>BR: ScopeIsSubset(requested, delegator)
    BR->>BR: Verify delegate agent exists
    BR->>BR: Sign DelegRecord, compute chain_hash
    BR->>BR: Issue JWT (sub=agentB, attenuated scope)
    BR-->>A: {"access_token", "delegation_chain"}
    A->>B: Deliver delegated token
```

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `delegate_to` | string | Yes | SPIFFE ID of the delegate agent |
| `scope` | string[] | Yes | Scopes to grant (must be subset of delegator's scope) |
| `ttl` | int | No | TTL in seconds (default 60) |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `access_token` | string | JWT for the delegate agent |
| `expires_in` | int | TTL in seconds |
| `delegation_chain` | DelegRecord[] | Complete chain including new entry |

Each `DelegRecord`:

| Field | Type | Description |
|---|---|---|
| `agent` | string | SPIFFE ID of the delegating agent |
| `scope` | string[] | Scope held at time of delegation |
| `delegated_at` | string | RFC3339 timestamp |
| `signature` | string | Ed25519 hex signature of the record |

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 400 | `invalid_request` | Missing `delegate_to` or `scope` |
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `scope_violation` | Requested scope exceeds delegator's scope, or depth limit (5) exceeded |
| 404 | `not_found` | Delegate agent not registered |
| 500 | `internal_error` | Delegation failed |

```bash
curl -X POST http://localhost:8080/v1/delegate \
  -H "Authorization: Bearer <delegator-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "delegate_to": "spiffe://agentauth.local/agent/orch/task/instance2",
    "scope": ["read:data:*"],
    "ttl": 60
  }'
```

```json
{
  "access_token": "eyJ...",
  "expires_in": 60,
  "delegation_chain": [
    {
      "agent": "spiffe://agentauth.local/agent/orch/task/instance1",
      "scope": ["read:data:*", "write:data:*"],
      "delegated_at": "2026-02-15T12:00:00Z",
      "signature": "a1b2c3..."
    }
  ]
}
```

---

### Admin Endpoints (Bearer + admin scope required)

---

#### POST /v1/admin/launch-tokens

Create a launch token for agent registration.

**Auth:** Bearer token with `admin:launch-tokens:*` scope

**Request body:**

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `agent_name` | string | Yes | -- | Name of the agent this token is for |
| `allowed_scope` | string[] | Yes | -- | Scope ceiling for the agent |
| `max_ttl` | int | No | 300 | Maximum token TTL the agent can request |
| `single_use` | bool | No | true | Whether token can only be used once |
| `ttl` | int | No | 30 | Launch token validity period in seconds |

**Response 201:**

| Field | Type | Description |
|---|---|---|
| `launch_token` | string | 64-character hex token |
| `expires_at` | string | RFC3339 expiration timestamp |
| `policy.allowed_scope` | string[] | Scope ceiling bound to this token |
| `policy.max_ttl` | int | TTL ceiling for issued agent tokens |

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 400 | `invalid_request` | Missing `agent_name` or empty `allowed_scope` |
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `insufficient_scope` | Token lacks `admin:launch-tokens:*` scope |
| 500 | `internal_error` | Token creation failed |

```bash
curl -X POST http://localhost:8080/v1/admin/launch-tokens \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "my-agent",
    "allowed_scope": ["read:data:*"],
    "max_ttl": 600,
    "single_use": true,
    "ttl": 60
  }'
```

```json
{
  "launch_token": "a1b2c3d4e5f6...64chars",
  "expires_at": "2026-02-15T12:01:00Z",
  "policy": {
    "allowed_scope": ["read:data:*"],
    "max_ttl": 600
  }
}
```

---

#### POST /v1/admin/sidecar-activations

Create a sidecar activation token. This is a single-use JWT that the sidecar exchanges at `POST /v1/sidecar/activate` for a functional bearer token.

**Auth:** Bearer token with `admin:launch-tokens:*` scope

**Request body:**

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `allowed_scopes` | string[] | Yes | -- | Scope ceiling for the sidecar |
| `ttl` | int | No | 900 | Activation token TTL in seconds |

**Response 201:**

| Field | Type | Description |
|---|---|---|
| `activation_token` | string | JWT string |
| `expires_at` | string | RFC3339 expiration timestamp |
| `scope` | string | Space-separated `sidecar:activate:{scope}` entries |

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 400 | `invalid_request` | Empty `allowed_scopes` |
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `insufficient_scope` | Token lacks `admin:launch-tokens:*` scope |
| 500 | `internal_error` | Token creation failed |

```bash
curl -X POST http://localhost:8080/v1/admin/sidecar-activations \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"allowed_scopes": ["read:data:*", "write:data:*"], "ttl": 600}'
```

```json
{
  "activation_token": "eyJ...",
  "expires_at": "2026-02-15T12:10:00Z",
  "scope": "sidecar:activate:read:data:* sidecar:activate:write:data:*"
}
```

---

#### POST /v1/revoke

Revoke tokens at one of four levels.

**Auth:** Bearer token with `admin:revoke:*` scope

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `level` | string | Yes | One of: `token`, `agent`, `task`, `chain` |
| `target` | string | Yes | JTI, SPIFFE ID, task ID, or root delegator agent ID |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `revoked` | bool | Always `true` on success |
| `level` | string | The revocation level applied |
| `target` | string | The revocation target |
| `count` | int | Number of entries affected |

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 400 | `invalid_request` | Missing level/target, or invalid revocation level |
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `insufficient_scope` | Token lacks `admin:revoke:*` scope |
| 500 | `internal_error` | Revocation failed |

```bash
curl -X POST http://localhost:8080/v1/revoke \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"level": "token", "target": "a1b2c3d4e5f67890..."}'
```

```json
{
  "revoked": true,
  "level": "token",
  "target": "a1b2c3d4e5f67890...",
  "count": 1
}
```

---

#### GET /v1/audit/events

Query the hash-chained audit trail with filters and pagination.

**Auth:** Bearer token with `admin:audit:*` scope

**Query parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `agent_id` | string | -- | Filter by agent SPIFFE ID |
| `task_id` | string | -- | Filter by task ID |
| `event_type` | string | -- | Filter by event type |
| `outcome` | string | -- | Filter by outcome (e.g. `success`, `denied`) |
| `since` | string | -- | RFC3339 timestamp lower bound |
| `until` | string | -- | RFC3339 timestamp upper bound |
| `limit` | int | 100 | Max events to return (max 1000) |
| `offset` | int | 0 | Pagination offset |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `events` | AuditEvent[] | Array of audit events |
| `total` | int | Total matching events (before pagination) |
| `offset` | int | Applied offset |
| `limit` | int | Applied limit |

Each `AuditEvent`:

| Field | Type | Description |
|---|---|---|
| `id` | string | Sequential ID (`evt-000001`) |
| `timestamp` | string | RFC3339 timestamp |
| `event_type` | string | One of 23 event types |
| `agent_id` | string | Agent SPIFFE ID (if applicable) |
| `task_id` | string | Task ID (if applicable) |
| `orch_id` | string | Orchestration ID (if applicable) |
| `detail` | string | Human-readable description (PII-sanitized) |
| `resource` | string | Target resource path (e.g. API endpoint) |
| `outcome` | string | Event outcome: `success` or `denied` |
| `deleg_depth` | int | Delegation chain depth (0 = direct) |
| `deleg_chain_hash` | string | SHA-256 hash of the delegation chain |
| `bytes_transferred` | int | Bytes transferred (for metered operations) |
| `hash` | string | SHA-256 hex hash of this event |
| `prev_hash` | string | SHA-256 hex hash of the previous event |

The 23 event types include the original lifecycle events (`admin_auth`, `agent_registered`, `token_issued`, `token_revoked`, `token_renewed`, `delegation_created`, etc.) plus 6 enforcement audit events:

| Event Type | Description |
|---|---|
| `token_auth_failed` | Bad signature, expired, or malformed JWT presented |
| `token_revoked_access` | Revoked token used on any endpoint |
| `scope_violation` | Token lacks required scope for endpoint |
| `scope_ceiling_exceeded` | Sidecar scope ceiling exceeded |
| `delegation_attenuation_violation` | Delegation attempted to widen scope |
| `token_released` | Agent voluntarily surrendered its credential |

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `insufficient_scope` | Token lacks `admin:audit:*` scope |

```bash
curl "http://localhost:8080/v1/audit/events?event_type=agent_registered&limit=10" \
  -H "Authorization: Bearer <admin-token>"
```

```json
{
  "events": [
    {
      "id": "evt-000001",
      "timestamp": "2026-02-15T12:00:00Z",
      "event_type": "agent_registered",
      "agent_id": "spiffe://agentauth.local/agent/orch/task/instance",
      "task_id": "task-001",
      "orch_id": "my-orchestrator",
      "detail": "Agent registered with scope [read:data:*]",
      "hash": "abc123...",
      "prev_hash": "0000000000000000000000000000000000000000000000000000000000000000"
    }
  ],
  "total": 1,
  "offset": 0,
  "limit": 10
}
```

---

### Sidecar-Only Endpoint (Bearer + sidecar:manage:* scope)

---

#### POST /v1/token/exchange

Sidecar-mediated token issuance. The sidecar calls this endpoint with its own Bearer token to request a scoped token for an agent.

**Auth:** Bearer token with `sidecar:manage:*` scope (validated by `ValMw` + `ValMw.RequireScope`)

```mermaid
sequenceDiagram
    participant SC as Sidecar
    participant BR as Broker

    SC->>BR: POST /v1/token/exchange<br/>Bearer: sidecar-token<br/>{"agent_id", "scope", "ttl"}
    BR->>BR: ValMw: verify sidecar Bearer
    BR->>BR: ValMw.RequireScope: sidecar:manage:*
    BR->>BR: Derive sidecar_id from token sid claim
    BR->>BR: Extract sidecar:scope:* entries from token
    BR->>BR: Check requested scope within ceiling
    BR->>BR: Look up agent in store
    BR->>BR: TknSvc.Issue(sub=agent, scope, sidecar_id)
    BR-->>SC: {"access_token", "expires_in",<br/>"token_type", "agent_id", "sidecar_id"}
```

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `agent_id` | string | Yes | SPIFFE ID of the target agent |
| `scope` | string[] | Yes | Requested scopes (must be subset of sidecar ceiling) |
| `ttl` | int | No | TTL in seconds (0 = default 900, valid range 0-900; negative values rejected) |
| `sidecar_id` | string | No | Ignored; derived from caller token's `sid` claim |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `access_token` | string | JWT for the agent |
| `expires_in` | int | TTL in seconds |
| `token_type` | string | Always `"Bearer"` |
| `agent_id` | string | SPIFFE ID of the agent |
| `sidecar_id` | string | Derived from authenticated caller token |

**Error responses:**

| Status | Error Code | Condition |
|---|---|---|
| 400 | `missing_field` | Missing `agent_id` or `scope` |
| 400 | `invalid_ttl` | TTL < 0 or > 900 |
| 400 | `invalid_scope_format` | Scope not in `action:resource:identifier` format |
| 400 | `invalid_content_type` | Content-Type is not `application/json` |
| 401 | `missing_credentials` | Missing Bearer token |
| 403 | `scope_escalation_denied` | Requested scope exceeds sidecar ceiling |
| 403 | `sidecar_scope_missing` | Caller token has no `sidecar:scope:*` entries |
| 404 | `agent_not_found` | Agent not registered |
| 500 | `token_issuance_failed` | Token issuance failed |

```bash
curl -X POST http://localhost:8080/v1/token/exchange \
  -H "Authorization: Bearer <sidecar-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "spiffe://agentauth.local/agent/orch/task/instance",
    "scope": ["read:data:*"],
    "ttl": 300
  }'
```

```json
{
  "access_token": "eyJ...",
  "expires_in": 300,
  "token_type": "Bearer",
  "agent_id": "spiffe://agentauth.local/agent/orch/task/instance",
  "sidecar_id": "abc123..."
}
```

---

#### POST /v1/token/release

Agent self-revocation. An authenticated agent surrenders its credential by revoking its own token's JTI. This is a task-completion signal — the agent is done and no longer needs its token.

**Auth:** Bearer token (any valid token — no admin scope required)

**Request body:** None (the Bearer token in the Authorization header identifies the token to release)

**Response 204:** No Content (success)

**Error responses:**

| Status | Type | Condition |
|---|---|---|
| 401 | `unauthorized` | Missing or invalid Bearer token |
| 403 | `insufficient_scope` | Token already revoked |

**Idempotency:** Releasing an already-released token returns 403 (token already revoked via the ValMw middleware). The `aactl` CLI treats this as idempotent success.

**Audit event:** `token_released` with the agent's SPIFFE ID and JTI.

```bash
curl -X POST http://localhost:8080/v1/token/release \
  -H "Authorization: Bearer eyJ..."
```

---

## Sidecar Endpoints

---

### GET /v1/health (Sidecar)

Sidecar health and readiness.

**Auth:** None

**Response 200 (healthy):**

| Field | Type | Description |
|---|---|---|
| `status` | string | `"ok"` or `"degraded"` |
| `broker_connected` | bool | Whether sidecar has a valid broker token |
| `healthy` | bool | Overall health status |
| `sidecar_id` | string | Stable sidecar identifier (derived from activation). Use this value for runtime ceiling management via `PUT /v1/admin/sidecars/{id}/ceiling`. |
| `scope_ceiling` | string[] | Configured scope ceiling |
| `agents_registered` | int | Agents in sidecar memory |
| `last_renewal` | string | RFC3339 timestamp of last token renewal |
| `uptime_seconds` | float | Seconds since bootstrap |

**Response 503 (bootstrapping):**

```json
{
  "status": "bootstrapping",
  "healthy": false
}
```

```bash
curl http://localhost:8081/v1/health
```

---

### GET /v1/metrics (Sidecar)

Prometheus metrics for the sidecar.

**Auth:** None

See [Prometheus Metrics](#prometheus-metrics) for the full metric list.

```bash
curl http://localhost:8081/v1/metrics
```

---

### POST /v1/token (Sidecar)

Request a scoped agent token. The sidecar handles key generation, challenge-response registration, and token exchange with the broker transparently.

**Auth:** None

**Request body:**

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `agent_name` | string | Yes | -- | Agent identifier |
| `task_id` | string | No | `"default"` | Task identifier |
| `scope` | string[] | Yes | -- | Requested scopes (must be within sidecar ceiling) |
| `ttl` | int | No | 300 | Token TTL in seconds |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `access_token` | string | JWT for the agent |
| `expires_in` | int | TTL in seconds |
| `scope` | string[] | Granted scopes |
| `agent_id` | string | SPIFFE ID of the agent |

**Response 200 (circuit open, cached token):**

Same body as above, but with the header `X-AgentAuth-Cached: true`.

**Error responses:**

| Status | Detail | Condition |
|---|---|---|
| 400 | `scope is required` | Empty scope array |
| 400 | `agent_name is required` | Missing agent_name |
| 403 | `requested scope exceeds sidecar ceiling` | Scope not within ceiling |
| 502 | `agent registration failed: ...` | Broker registration failed |
| 502 | `broker token exchange failed: ...` | Broker exchange failed |
| 503 | `broker unavailable and no cached token` | Circuit open, no cache |

```bash
curl -X POST http://localhost:8081/v1/token \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "my-agent",
    "task_id": "task-001",
    "scope": ["read:data:*"],
    "ttl": 300
  }'
```

```json
{
  "access_token": "eyJ...",
  "expires_in": 300,
  "scope": ["read:data:*"],
  "agent_id": "spiffe://agentauth.local/agent/my-agent/task-001/a1b2c3d4"
}
```

---

### POST /v1/token/renew (Sidecar)

Renew an existing token by proxying to the broker's renew endpoint.

**Auth:** Bearer token (in Authorization header)

**Request body:** None

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `access_token` | string | New JWT |
| `expires_in` | int | TTL in seconds |

**Error responses:**

| Status | Detail | Condition |
|---|---|---|
| 401 | `missing or invalid Authorization bearer token` | No Bearer token |
| 502 | `broker token renew failed: ...` | Broker renew failed |

```bash
curl -X POST http://localhost:8081/v1/token/renew \
  -H "Authorization: Bearer eyJ..."
```

---

### GET /v1/challenge (Sidecar)

Proxy the broker's challenge endpoint for BYOK developers.

**Auth:** None

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `nonce` | string | 64-character hex nonce (from broker) |
| `expires_in` | int | Always 30 |

```bash
curl http://localhost:8081/v1/challenge
```

---

### POST /v1/register (Sidecar BYOK)

Register an agent using developer-provided Ed25519 keys (Bring Your Own Key).

**Auth:** None

**Request body:**

| Field | Type | Required | Description |
|---|---|---|---|
| `agent_name` | string | Yes | Agent identifier |
| `task_id` | string | No | Task identifier (default: `"default"`) |
| `public_key` | string | Yes | Base64-encoded Ed25519 public key (32 bytes) |
| `signature` | string | Yes | Base64-encoded Ed25519 signature of nonce bytes |
| `nonce` | string | Yes | Hex nonce from GET /v1/challenge |

**Response 200:**

| Field | Type | Description |
|---|---|---|
| `agent_id` | string | SPIFFE ID of the registered agent |

**Response 200 (already registered):**

```json
{
  "agent_id": "spiffe://...",
  "cached": true
}
```

**Error responses:**

| Status | Detail | Condition |
|---|---|---|
| 400 | `agent_name is required` | Missing agent_name |
| 400 | `public_key, signature, and nonce are required` | Missing crypto fields |
| 400 | `invalid public key: must be 32-byte Ed25519 key, base64-encoded` | Bad key |
| 502 | `admin auth failed: ...` | Broker admin auth failed |
| 502 | `broker registration failed: ...` | Broker registration failed |

```bash
curl -X POST http://localhost:8081/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "my-agent",
    "task_id": "task-001",
    "public_key": "base64Ed25519PubKey==",
    "signature": "base64SignatureOfNonceBytes==",
    "nonce": "deadbeef...64chars"
  }'
```

```json
{
  "agent_id": "spiffe://agentauth.local/agent/my-agent/task-001/a1b2c3d4"
}
```

---

## Scope System

### Format

Scopes follow a three-part colon-separated format:

```
action:resource:identifier
```

Examples:
- `read:data:*` -- Read any data resource
- `write:data:customer-123` -- Write to a specific data resource
- `admin:revoke:*` -- Admin revocation on any target
- `admin:launch-tokens:*` -- Admin launch token management
- `admin:audit:*` -- Admin audit access
- `sidecar:manage:*` -- Sidecar management scope
- `sidecar:scope:read:data:*` -- Sidecar scope ceiling entry

### Wildcard Rules

A `*` in the identifier position of an allowed scope covers any specific identifier in a requested scope:

- `read:data:*` covers `read:data:customer-123` (wildcard covers specific)
- `read:data:customer-123` does NOT cover `read:data:*` (specific does not cover wildcard)
- Action and resource parts must match exactly

### Attenuation

Scopes can only narrow, never expand. This is enforced at three points:

1. **Registration:** `requested_scope` must be a subset of `launch_token.allowed_scope`
2. **Delegation:** `delegated_scope` must be a subset of `delegator.scope`
3. **Token exchange:** `requested_scope` must be covered by sidecar's `sidecar:scope:*` entries

---

## JWT Claims

All tokens issued by AgentAuth use EdDSA (Ed25519) signing with compact JWT serialization.

### TknClaims Fields

| Field | JSON Key | Type | Description |
|---|---|---|---|
| `Iss` | `iss` | string | Always `"agentauth"` |
| `Sub` | `sub` | string | SPIFFE agent ID or `"admin"` or `"sidecar:{id}"` |
| `Aud` | `aud` | string[] | Audience (optional, used for sidecar activation) |
| `Exp` | `exp` | int64 | Expiration timestamp (Unix seconds) |
| `Nbf` | `nbf` | int64 | Not-before timestamp (Unix seconds) |
| `Iat` | `iat` | int64 | Issued-at timestamp (Unix seconds) |
| `Jti` | `jti` | string | Unique token ID (32 hex chars from 16 random bytes) |
| `Sid` | `sid` | string | Session/sidecar ID (optional) |
| `SidecarID` | `sidecar_id` | string | Sidecar identifier (optional) |
| `Scope` | `scope` | string[] | Granted scopes |
| `TaskId` | `task_id` | string | Task identifier (optional) |
| `OrchId` | `orch_id` | string | Orchestration identifier (optional) |
| `DelegChain` | `delegation_chain` | DelegRecord[] | Delegation provenance chain (optional) |
| `ChainHash` | `chain_hash` | string | SHA-256 hex hash of delegation chain (optional) |

### Token Format

```
base64url({"alg":"EdDSA","typ":"JWT"}).base64url(claims).base64url(ed25519_signature)
```

---

## Error Reference

### RFC 7807 Error Types

| Error Type | Status | Description |
|---|---|---|
| `invalid_request` | 400 | Malformed JSON, missing required fields, invalid scope format, invalid TTL |
| `unauthorized` | 401 | Bad credentials, invalid/expired/consumed token or launch token |
| `scope_violation` | 403 | Requested scope exceeds allowed scope |
| `insufficient_scope` | 403 | Bearer token lacks required scope for endpoint |
| `not_found` | 404 | Agent or resource not found |
| `internal_error` | 500 | Unexpected server failure |

### Extended Error Codes (Token Exchange)

| Error Code | Status | Description |
|---|---|---|
| `missing_credentials` | 401 | No Bearer token provided |
| `missing_field` | 400 | Required field missing (agent_id, scope) |
| `invalid_ttl` | 400 | TTL out of range (negative or > 900; 0 is valid and defaults to 900) |
| `invalid_scope_format` | 400 | Scope not in `action:resource:identifier` format |
| `invalid_content_type` | 400 | Content-Type must be `application/json` |
| `scope_escalation_denied` | 403 | Scope exceeds sidecar ceiling |
| `sidecar_scope_missing` | 403 | Caller token has no sidecar scope entries |
| `agent_not_found` | 404 | Agent not registered |
| `token_issuance_failed` | 500 | Token creation failed |
| `sidecar_derivation_failed` | 500 | Could not derive sidecar identity |

### Extended Error Codes (Sidecar Activation)

| Error Code | Status | Description |
|---|---|---|
| `invalid_activation_token` | 401 | Activation token is invalid or expired |
| `activation_token_replayed` | 401 | Activation token has already been used |

### Rate Limiting

Applied to `POST /v1/admin/auth` and `POST /v1/sidecar/activate`:
- Rate: 5 requests per second per IP
- Burst: 10
- Response: HTTP 429 with `Retry-After: 1` header
- IP extraction: `X-Forwarded-For` (first entry) or `RemoteAddr`

---

## Prometheus Metrics

### Broker Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `agentauth_tokens_issued_total` | CounterVec | `scope` | Tokens issued by primary scope |
| `agentauth_tokens_revoked_total` | CounterVec | `level` | Revocations by level |
| `agentauth_registrations_total` | CounterVec | `status` | Registration attempts (success/failure) |
| `agentauth_admin_auth_total` | CounterVec | `status` | Admin auth attempts (success/failure) |
| `agentauth_launch_tokens_created_total` | Counter | -- | Launch tokens created |
| `agentauth_active_agents` | Gauge | -- | Currently registered agents |
| `agentauth_request_duration_seconds` | HistogramVec | `endpoint` | Request latency |
| `agentauth_clock_skew_total` | Counter | -- | Clock skew events |

### Sidecar Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `agentauth_sidecar_bootstrap_total` | CounterVec | `status` | Bootstrap attempts |
| `agentauth_sidecar_renewals_total` | CounterVec | `status` | Token renewal attempts |
| `agentauth_sidecar_token_exchanges_total` | CounterVec | `status` | Agent token exchanges |
| `agentauth_sidecar_scope_denials_total` | Counter | -- | Scope ceiling denials |
| `agentauth_sidecar_agents_registered` | Gauge | -- | Agents in memory |
| `agentauth_sidecar_request_duration_seconds` | HistogramVec | `endpoint` | Request latency |
| `agentauth_sidecar_circuit_state` | Gauge | -- | Circuit breaker state (0=closed, 1=open, 2=probing) |
| `agentauth_sidecar_circuit_trips_total` | Counter | -- | Circuit trips |
| `agentauth_sidecar_cached_tokens_served_total` | Counter | -- | Cached tokens served during circuit open |
