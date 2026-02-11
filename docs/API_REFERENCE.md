# AgentAuth API Reference

**Version:** 2.0.0
**Base URL:** `http://localhost:8080` (Docker Compose default host mapping; override with `AA_HOST_PORT`)
**Content-Type:** All requests and responses use `application/json` unless noted otherwise
**Errors:** All errors use [RFC 7807](https://tools.ietf.org/html/rfc7807) `application/problem+json`
**Request body limit:** All endpoints enforce a maximum request body size of 1 MB (1,048,576 bytes) via `MaxBytesBody`. Requests exceeding this limit receive a `413 Request Entity Too Large` error.

---

## Table of Contents

- [Authentication Model](#authentication-model)
- [Endpoint Summary](#endpoint-summary)
- [Public Endpoints](#public-endpoints)
  - [GET /v1/health](#get-v1health)
  - [GET /v1/metrics](#get-v1metrics)
  - [GET /v1/challenge](#get-v1challenge)
  - [POST /v1/token/validate](#post-v1tokenvalidate)
- [Admin Bootstrap](#admin-bootstrap)
  - [POST /v1/admin/auth](#post-v1adminauth)
  - [POST /v1/admin/launch-tokens](#post-v1adminlaunch-tokens)
  - [POST /v1/admin/sidecar-activations](#post-v1adminsidecar-activations)
  - [POST /v1/sidecar/activate](#post-v1sidecaractivate)
- [Agent Identity](#agent-identity)
  - [POST /v1/register](#post-v1register)
- [Token Management](#token-management)
  - [POST /v1/token/renew](#post-v1tokenrenew)
  - [POST /v1/token/exchange](#post-v1tokenexchange)
  - [POST /v1/revoke](#post-v1revoke)
- [Delegation](#delegation)
  - [POST /v1/delegate](#post-v1delegate)
- [Audit](#audit)
  - [GET /v1/audit/events](#get-v1auditevents)
- [Common Reference](#common-reference)
  - [Error Format (RFC 7807)](#error-format-rfc-7807)
  - [Scope Format](#scope-format)
  - [JWT Claims Structure](#jwt-claims-structure)
  - [SPIFFE ID Format](#spiffe-id-format)
  - [Prometheus Metrics](#prometheus-metrics)
- [End-to-End Walkthrough](#end-to-end-walkthrough)

---

## Authentication Model

AgentAuth uses three authentication levels:

| Level | Mechanism | Endpoints |
|-------|-----------|-----------|
| **Public** | No auth required | `GET /v1/challenge`, `GET /v1/health`, `GET /v1/metrics`, `POST /v1/token/validate` |
| **Launch token** | Opaque token in request body | `POST /v1/register` |
| **Bearer JWT** | `Authorization: Bearer <token>` header | All other endpoints |

Admin endpoints additionally require specific scopes in the JWT:

| Scope | Grants access to |
|-------|------------------|
| `admin:launch-tokens:*` | `POST /v1/admin/launch-tokens` |
| `admin:revoke:*` | `POST /v1/revoke` |
| `admin:audit:*` | `GET /v1/audit/events` |

The Bearer token middleware (`ValMw`) performs the following checks in order:

1. Extract `Authorization: Bearer <token>` header
2. Verify EdDSA signature against the broker's public key
3. Validate claims (issuer, expiry, not-before)
4. Check the revocation service (token, agent, task, and chain levels)
5. Set claims into request context for downstream handlers

If any check fails, the middleware returns an RFC 7807 error and the request never reaches the handler.

---

## Endpoint Summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/v1/health` | None | Broker health check |
| `GET` | `/v1/metrics` | None | Prometheus metrics |
| `GET` | `/v1/challenge` | None | Get cryptographic nonce |
| `POST` | `/v1/token/validate` | None | Validate a token and return claims |
| `POST` | `/v1/admin/auth` | Secret in body | Exchange admin secret for JWT |
| `POST` | `/v1/admin/launch-tokens` | Bearer + `admin:launch-tokens:*` | Create a launch token |
| `POST` | `/v1/admin/sidecar-activations` | Bearer + `admin:launch-tokens:*` | Create a sidecar activation token |
| `POST` | `/v1/sidecar/activate` | Token in body | Exchange activation token for sidecar Bearer token |
| `POST` | `/v1/register` | Launch token in body | Register an agent |
| `POST` | `/v1/token/renew` | Bearer | Renew a token |
| `POST` | `/v1/token/exchange` | Bearer + `sidecar:manage:*` | Sidecar-mediated token issuance |
| `POST` | `/v1/revoke` | Bearer + `admin:revoke:*` | Revoke tokens |
| `POST` | `/v1/delegate` | Bearer | Create delegation token |
| `GET` | `/v1/audit/events` | Bearer + `admin:audit:*` | Query audit trail |

---

## Public Endpoints

### GET /v1/health

Returns broker health status. Use this for liveness/readiness probes.

**Auth:** None

**Response `200 OK`:**

```json
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 142
}
```

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always `"ok"` when the broker is running |
| `version` | string | Broker version string |
| `uptime` | int64 | Seconds since broker started |

**Example:**

```bash
curl http://localhost:8080/v1/health
```

---

### GET /v1/metrics

Returns Prometheus metrics in text exposition format.

**Auth:** None

**Response `200 OK`:** Content-Type `text/plain; version=0.0.4; charset=utf-8`

The response is standard Prometheus exposition format, served by `promhttp.Handler()`. See [Prometheus Metrics](#prometheus-metrics) for the list of registered metrics.

**Example:**

```bash
curl http://localhost:8080/v1/metrics
```

---

### GET /v1/challenge

Get a cryptographic nonce for agent registration. The agent must sign this nonce with its Ed25519 private key to prove key possession during registration.

**Auth:** None

**Response `200 OK`:**

```json
{
  "nonce": "5c2cbff9a2ce36c4a72d18f4a711e08b4a9e3f12d5c8b7a6e9f0d1c2b3a4e5f6",
  "expires_in": 30
}
```

| Field | Type | Description |
|-------|------|-------------|
| `nonce` | string | 64-character hex string (32 random bytes from `crypto/rand`) |
| `expires_in` | int | Nonce validity in seconds (always `30`) |

Nonces are single-use. Once consumed during registration, they cannot be reused. Expired nonces are also rejected.

**Example:**

```bash
curl http://localhost:8080/v1/challenge
```

---

### POST /v1/token/validate

Validate a token and return its claims. Designed for resource servers that need to verify agent credentials without holding the broker's signing key.

**Auth:** None (the token to validate is in the request body)

**Request:**

```json
{
  "token": "eyJhbGciOiJFZERTQSIs..."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `token` | string | Yes | The JWT token string to validate |

**Response `200 OK` -- valid token:**

```json
{
  "valid": true,
  "claims": {
    "iss": "agentauth",
    "sub": "spiffe://agentauth.local/agent/orch-001/task-001/a1b2c3d4e5f6a7b8",
    "exp": 1770644202,
    "nbf": 1770643902,
    "iat": 1770643902,
    "jti": "e2b7777781a064686237079634888b11",
    "sid": "sidecar-123",
    "scope": ["read:Customers:*"],
    "task_id": "task-001",
    "orch_id": "orch-001",
    "delegation_chain": []
  }
}
```

**Response `200 OK` -- invalid token:**

```json
{
  "valid": false,
  "error": "signature verification failed"
}
```

**Design note:** This endpoint always returns `200 OK`. The `valid` boolean indicates whether the token is valid. This design lets resource servers distinguish between "I cannot reach the broker" (network error) and "the token is invalid" (`200` with `valid: false`).

**Error responses (non-200):**

| Status | Condition |
|--------|-----------|
| `400` | Malformed JSON body or missing `token` field |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}"
```

---

## Admin Bootstrap

### POST /v1/admin/auth

Exchange the pre-shared admin secret for a short-lived admin JWT. This is the bootstrap entry point for the entire system -- the first call you make after starting the broker.

**Auth:** None (the secret is in the request body)

**Request:**

```json
{
  "client_id": "admin",
  "client_secret": "<value of AA_ADMIN_SECRET env var>"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `client_id` | string | Yes | Identifier for the admin client |
| `client_secret` | string | Yes | Must match the `AA_ADMIN_SECRET` environment variable |

**Security note:** The secret comparison uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks.

**Rate limiting:** This endpoint is rate-limited to 5 requests per second per IP with a burst capacity of 10. Exceeding the limit returns `429 Too Many Requests` with a `Retry-After: 1` header and an RFC 7807 error body (`urn:agentauth:error:rate_limited`).

**Response `200 OK`:**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `access_token` | string | EdDSA-signed JWT |
| `expires_in` | int | Token TTL in seconds (always `300` for admin tokens) |
| `token_type` | string | Always `"Bearer"` |

The issued admin JWT has:
- `sub`: `"admin"`
- `scope`: `["admin:launch-tokens:*", "admin:revoke:*", "admin:audit:*"]`
- TTL: 300 seconds (5 minutes)

**Audit events:** On success, records `admin_auth`. On failure, records `admin_auth_failed`.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `400` | Missing `client_id` or `client_secret` |
| `400` | Malformed JSON body |
| `401` | Invalid credentials |
| `429` | Rate limit exceeded (5 req/s per IP, burst 10) |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id":"admin","client_secret":"my-secret"}'
```

---

### POST /v1/admin/launch-tokens

Create a per-agent launch token with policy binding. The launch token is an opaque hex credential that an agent presents during registration. It binds the agent to a maximum scope ceiling and TTL.

**Auth:** Bearer JWT with `admin:launch-tokens:*` scope

**Request:**

```json
{
  "agent_name": "data-reader",
  "allowed_scope": ["read:Customers:*"],
  "max_ttl": 300,
  "ttl": 60,
  "single_use": true
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `agent_name` | string | Yes | -- | Human-readable name for the agent |
| `allowed_scope` | string[] | Yes | -- | Maximum scope ceiling the agent can request at registration |
| `max_ttl` | int | No | `300` | Maximum token TTL in seconds for the agent's issued tokens |
| `ttl` | int | No | `30` | How long the launch token itself is valid (seconds) |
| `single_use` | bool | No | `true` | If `true`, the launch token is consumed on first successful registration |

**Response `201 Created`:**

```json
{
  "launch_token": "9ae89c0ae963f88a7d2b4c5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a",
  "expires_at": "2026-02-09T13:27:43Z",
  "policy": {
    "allowed_scope": ["read:Customers:*"],
    "max_ttl": 300
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `launch_token` | string | 64-character hex string (32 random bytes) |
| `expires_at` | string | RFC 3339 expiration timestamp |
| `policy.allowed_scope` | string[] | The scope ceiling bound to this token |
| `policy.max_ttl` | int | Maximum TTL for tokens issued to agents using this launch token |

**Audit events:** Records `launch_token_issued` on success.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `400` | Missing `agent_name` |
| `400` | Empty `allowed_scope` array |
| `400` | Malformed JSON body |
| `401` | Missing or invalid Bearer token |
| `403` | Token lacks `admin:launch-tokens:*` scope |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/admin/launch-tokens \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "agent_name": "data-reader",
    "allowed_scope": ["read:Customers:*"],
    "max_ttl": 300,
    "ttl": 60
  }'
```

---

### POST /v1/admin/sidecar-activations

Create a short-lived single-use sidecar activation token. This token is used to initialize a sidecar instance.

**Auth:** Bearer JWT with `admin:launch-tokens:*` scope

**Request:**

```json
{
  "allowed_scope_prefix": "read:data:*",
  "ttl": 900
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `allowed_scope_prefix` | string | Yes | -- | Scope prefix the sidecar is allowed to manage |
| `ttl` | int | No | `900` | Activation token TTL in seconds |

**Response `201 Created`:**

```json
{
  "activation_token": "eyJhbG...",
  "expires_at": "2026-02-09T13:45:00Z",
  "scope": "sidecar:activate:read:data:*"
}
```

**Audit events:** Records `sidecar_activation_issued` on success.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `400` | Missing `allowed_scope_prefix` or malformed JSON body |
| `401` | Missing or invalid Bearer token |
| `403` | Token lacks `admin:launch-tokens:*` scope |
| `500` | Internal error during activation token creation |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/admin/sidecar-activations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"allowed_scope_prefix":"read:data:*","ttl":900}'
```

---

### POST /v1/sidecar/activate

Exchange a Sidecar-Activation token for a functional sidecar Bearer token. Enforces single-use replay protection.

**Auth:** None (activation token in body)

**Rate limiting:** This endpoint is rate-limited to 5 requests per second per IP with a burst capacity of 10. Exceeding the limit returns `429 Too Many Requests` with a `Retry-After: 1` header.

**Request:**

```json
{
  "sidecar_activation_token": "eyJhbG..."
}
```

**Response `200 OK`:**

```json
{
  "access_token": "eyJhbG...",
  "expires_in": 900,
  "token_type": "Bearer",
  "sidecar_id": "e2b7777781a064686237079634888b11"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `access_token` | string | Functional sidecar Bearer token |
| `expires_in` | int | Token TTL in seconds |
| `token_type` | string | Always `"Bearer"` |
| `sidecar_id` | string | Stable sidecar identifier |

**Audit events:** Records `sidecar_activated` on success. Records `sidecar_activation_failed` on invalid or replayed token.

**Error responses:**

| Status | error_code | Condition |
|--------|------------|-----------|
| `400` | `invalid_request` | Malformed JSON body |
| `401` | `activation_token_replayed` | Token already used |
| `401` | `invalid_activation_token` | Token invalid or expired |
| `429` | `rate_limited` | Rate limit exceeded (5 req/s per IP, burst 10) |
| `500` | `internal_error` | Internal activation failure |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/sidecar/activate \
  -H "Content-Type: application/json" \
  -d "{\"sidecar_activation_token\":\"$ACTIVATION_TOKEN\"}"
```

---

## Agent Identity

### POST /v1/register

Register an agent with the broker. This is the core identity flow: the agent presents a launch token, a signed nonce, and its Ed25519 public key. On success, the broker assigns a SPIFFE ID and issues a scoped JWT.

**Auth:** Launch token in request body (not Bearer header)

**Request:**

```json
{
  "launch_token": "9ae89c0ae963f88a7d2b4c5e6f7a8b9c...",
  "nonce": "5c2cbff9a2ce36c4a72d18f4a711e08b...",
  "public_key": "<base64-encoded Ed25519 public key>",
  "signature": "<base64-encoded Ed25519 signature of the nonce hex bytes>",
  "orch_id": "orchestration-001",
  "task_id": "task-001",
  "requested_scope": ["read:Customers:*"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `launch_token` | string | Yes | Opaque hex token from `POST /v1/admin/launch-tokens` |
| `nonce` | string | Yes | Hex nonce from `GET /v1/challenge` |
| `public_key` | string | Yes | Standard base64-encoded Ed25519 public key (32 bytes decoded) |
| `signature` | string | Yes | Standard base64-encoded Ed25519 signature of the nonce bytes |
| `orch_id` | string | Yes | Orchestration identifier (becomes part of SPIFFE ID path) |
| `task_id` | string | Yes | Task identifier (becomes part of SPIFFE ID path) |
| `requested_scope` | string[] | Yes | Must be a subset of the launch token's `allowed_scope` |

**Signature computation:** The agent must sign the raw bytes of the nonce. The nonce is a hex string -- decode it to bytes with `hex.DecodeString`, then sign those bytes with `ed25519.Sign`. Base64-encode the resulting signature for the `signature` field. The `public_key` field is the raw 32-byte Ed25519 public key, standard base64-encoded.

**Response `200 OK`:**

```json
{
  "agent_id": "spiffe://agentauth.local/agent/orchestration-001/task-001/a1b2c3d4e5f6a7b8",
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 300
}
```

| Field | Type | Description |
|-------|------|-------------|
| `agent_id` | string | SPIFFE ID assigned to the agent |
| `access_token` | string | EdDSA-signed JWT for subsequent API calls |
| `expires_in` | int | Token TTL in seconds (capped by launch token's `max_ttl`) |

**Security invariants:**

1. **Scope check before consumption:** The broker verifies `requested_scope` is a subset of the launch token's `allowed_scope` *before* consuming the launch token. A scope violation does not waste a single-use token.
2. **Nonce consumed after scope check:** The nonce is consumed after the scope check but before signature verification. This prevents replay attacks.
3. **Launch token consumed last:** Only after all checks pass (scope, nonce, signature) is the launch token marked as consumed.

**Processing order:** validate fields -> validate launch token -> check scope subset -> consume nonce -> verify Ed25519 signature -> consume launch token -> generate SPIFFE ID -> issue JWT -> save agent record.

**Audit events:** Records `agent_registered` and `token_issued` on success. Records `registration_policy_violation` on scope violation.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `400` | Missing required fields (any of: `launch_token`, `nonce`, `public_key`, `signature`, `orch_id`, `task_id`, `requested_scope`) |
| `400` | Malformed JSON body |
| `401` | Launch token not found, expired, or already consumed |
| `401` | Nonce not found, expired, or already consumed |
| `401` | Invalid Ed25519 public key (wrong size or base64 decode failure) |
| `401` | Nonce signature verification failed |
| `403` | Requested scope exceeds launch token's allowed scope |
| `500` | Internal error (SPIFFE ID generation, token issuance, or agent save failure) |

**Example (curl + openssl):**

```bash
# 1. Get a nonce
NONCE=$(curl -s http://localhost:8080/v1/challenge | jq -r '.nonce')

# 2. Generate Ed25519 key pair (using openssl)
openssl genpkey -algorithm Ed25519 -out agent_key.pem
openssl pkey -in agent_key.pem -pubout -out agent_pub.pem

# 3. Sign the nonce bytes and register (see Python example below for a complete flow)
```

**Example (Python):**

```python
import base64, requests
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

# Generate key pair
private_key = Ed25519PrivateKey.generate()
public_key = private_key.public_key()
pub_bytes = public_key.public_bytes(Encoding.Raw, PublicFormat.Raw)

# Get challenge
nonce = requests.get("http://localhost:8080/v1/challenge").json()["nonce"]

# Sign nonce bytes (decode hex nonce to bytes, then sign)
nonce_bytes = bytes.fromhex(nonce)
signature = private_key.sign(nonce_bytes)

# Register
resp = requests.post("http://localhost:8080/v1/register", json={
    "launch_token": launch_token,
    "nonce": nonce,
    "public_key": base64.b64encode(pub_bytes).decode(),
    "signature": base64.b64encode(signature).decode(),
    "orch_id": "orch-001",
    "task_id": "task-001",
    "requested_scope": ["read:Customers:*"]
})
print(resp.json())
# {"agent_id": "spiffe://agentauth.local/agent/orch-001/task-001/...", "access_token": "...", "expires_in": 300}
```

---

## Token Management

### POST /v1/token/renew

Renew an existing token. Issues a new token with the same claims (`sub`, `sid`, `scope`, `task_id`, `orch_id`, `delegation_chain`) but fresh timestamps (`iat`, `nbf`, `exp`) and a new JTI.

**Auth:** Bearer JWT (the token being renewed)

**Request body:** None. The token to renew is taken from the `Authorization: Bearer` header.

**Response `200 OK`:**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 300
}
```

| Field | Type | Description |
|-------|------|-------------|
| `access_token` | string | New EdDSA-signed JWT |
| `expires_in` | int | TTL of the new token in seconds (uses `AA_DEFAULT_TTL`) |

**Note:** The old token remains valid until it expires or is explicitly revoked. Renewal does not invalidate the previous token.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `401` | Missing `Authorization` header |
| `401` | Invalid authorization scheme (not `Bearer`) |
| `401` | Token verification failed (expired, invalid signature, etc.) |
| `403` | Token has been revoked |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/token/renew \
  -H "Authorization: Bearer $AGENT_TOKEN"
```

---

### POST /v1/token/exchange

Exchange sidecar authority for an agent token. This endpoint is used by a sidecar to mint an agent token within the sidecar's permitted scope ceiling.

**Auth:** Bearer JWT with `sidecar:manage:*` scope and one or more scope-ceiling entries in claims like `sidecar:scope:read:data:*`.

**Request:**

```json
{
  "agent_id": "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
  "scope": ["read:data:*"],
  "ttl": 90,
  "sidecar_id": "client-supplied-value-ignored"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent_id` | string | Yes | SPIFFE agent ID receiving the token |
| `scope` | string[] | Yes | Requested scopes; must be subset of sidecar scope ceiling |
| `ttl` | int | No | Token TTL in seconds. Must be 0-900 (`maxExchangeTTL`). `0` or omitted clamps to 900s. Negative or >900 returns 400. |
| `sidecar_id` | string | No | Ignored by broker; lineage comes from caller token `sid` |

**Response `200 OK`:**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 90,
  "token_type": "Bearer",
  "agent_id": "spiffe://agentauth.local/agent/orch-1/task-1/abc123",
  "sidecar_id": "abc123"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `access_token` | string | EdDSA-signed JWT for the target agent |
| `expires_in` | int | Token TTL in seconds |
| `token_type` | string | Always `"Bearer"` |
| `agent_id` | string | Target SPIFFE ID |
| `sidecar_id` | string | Broker-derived sidecar lineage identifier |

**Security semantics:**

1. Requested scopes must be a subset of the sidecar scope ceiling (`scope_escalation_denied` on failure).
2. Each scope entry must be valid `action:resource:identifier` format (`invalid_scope_format` on failure).
3. Client-provided `sidecar_id` is ignored; broker injects lineage from authenticated sidecar token (`sid`, falling back to `sub`).
4. If no sidecar scope ceiling exists in the token, request is denied (`sidecar_scope_missing`).
5. TTL is capped at `maxExchangeTTL` (900 seconds). TTL=0 clamps to 900s; negative or >900 is rejected.

**Audit events:** Records `sidecar_exchange_success` on success. Records `sidecar_exchange_denied` on scope escalation, missing ceiling, or agent-not-found denial.

**Error responses:**

| Status | `error_code` | Condition |
|--------|-------------|-----------|
| `400` | `malformed_body` | Malformed JSON body |
| `400` | `invalid_content_type` | Missing `Content-Type: application/json` header |
| `400` | `missing_field` | Missing `agent_id` or empty `scope` |
| `400` | `invalid_scope_format` | Scope entry not in `action:resource:identifier` format |
| `400` | `invalid_ttl` | TTL negative or exceeds 900 seconds |
| `401` | `missing_credentials` | Missing or invalid Bearer token |
| `403` | `scope_escalation_denied` | Requested scope exceeds sidecar scope ceiling |
| `403` | `sidecar_scope_missing` | Caller token has no `sidecar:scope:*` entries |
| `404` | `agent_not_found` | Target `agent_id` not registered |
| `500` | `sidecar_derivation_failed` | Could not derive sidecar identity from caller token |
| `500` | `token_issuance_failed` | Internal token issuance error |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/token/exchange \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SIDECAR_TOKEN" \
  -d '{"agent_id":"spiffe://agentauth.local/agent/orch-1/task-1/abc123","scope":["read:data:*"],"ttl":90}'
```

---

### POST /v1/revoke

Revoke tokens at one of four levels. Revoked tokens are rejected by the validation middleware on all subsequent requests.

**Auth:** Bearer JWT with `admin:revoke:*` scope

**Request:**

```json
{
  "level": "token",
  "target": "e2b7777781a064686237079634888b11"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `level` | string | Yes | One of: `"token"`, `"agent"`, `"task"`, `"chain"` |
| `target` | string | Yes | The identifier to revoke (see table below) |

**Revocation levels:**

| Level | Target value | Effect |
|-------|-------------|--------|
| `token` | JTI (from token's `jti` claim) | Revokes a single token |
| `agent` | SPIFFE ID (from token's `sub` claim) | Revokes all tokens for that agent |
| `task` | Task ID (from token's `task_id` claim) | Revokes all tokens for that task |
| `chain` | Root delegator's agent ID (SPIFFE ID) | Revokes an entire delegation chain |

**Response `200 OK`:**

```json
{
  "revoked": true,
  "level": "token",
  "target": "e2b7777781a064686237079634888b11",
  "count": 1
}
```

| Field | Type | Description |
|-------|------|-------------|
| `revoked` | bool | Always `true` on success |
| `level` | string | The revocation level that was applied |
| `target` | string | The identifier that was revoked |
| `count` | int | Number of revocation entries added (always `1`) |

**How revocation is enforced:** The `ValMw` middleware checks every incoming Bearer token against the revocation service. It checks, in order: token JTI, agent SPIFFE ID (`sub`), task ID, and delegation chain root delegator's agent ID (`DelegChain[0].Agent`). If any match, the request is rejected with `403 Forbidden` and the message "token has been revoked".

**Audit events:** Records `token_revoked` on success.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `400` | Missing `level` or `target` |
| `400` | Invalid revocation level (not one of `token`, `agent`, `task`, `chain`) |
| `400` | Malformed JSON body |
| `401` | Missing or invalid Bearer token |
| `403` | Token lacks `admin:revoke:*` scope |

**Example:**

```bash
# Revoke a single token by JTI
curl -X POST http://localhost:8080/v1/revoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"level":"token","target":"e2b7777781a064686237079634888b11"}'

# Revoke all tokens for an agent
curl -X POST http://localhost:8080/v1/revoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"level":"agent","target":"spiffe://agentauth.local/agent/orch/task/inst"}'

# Revoke all tokens for a task
curl -X POST http://localhost:8080/v1/revoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"level":"task","target":"task-001"}'
```

---

## Delegation

### POST /v1/delegate

Create a scope-attenuated delegation token. An agent can delegate a subset of its own permissions to another registered agent. The delegated token carries a chain recording who delegated what.

**Auth:** Bearer JWT (the delegating agent's token)

**Request:**

```json
{
  "delegate_to": "spiffe://agentauth.local/agent/orch-001/task-001/b2c3d4e5f6a7b8c9",
  "scope": ["read:Customers:customer-123"],
  "ttl": 60
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `delegate_to` | string | Yes | -- | SPIFFE ID of the delegate agent (must already be registered) |
| `scope` | string[] | Yes | -- | Must be a subset of the delegator's current scope |
| `ttl` | int | No | `60` | TTL for the delegated token in seconds |

**Response `200 OK`:**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 60,
  "delegation_chain": [
    {
      "agent": "spiffe://agentauth.local/agent/orch-001/task-001/a1b2c3d4e5f6a7b8",
      "scope": ["read:Customers:*"],
      "delegated_at": "2026-02-09T13:30:00Z",
      "signature": "a1b2c3d4..."
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `access_token` | string | EdDSA-signed JWT for the delegate |
| `expires_in` | int | TTL of the delegated token in seconds |
| `delegation_chain` | object[] | Chain of delegations leading to this token |
| `delegation_chain[].agent` | string | SPIFFE ID of the delegator |
| `delegation_chain[].scope` | string[] | Scope the delegator held at delegation time |
| `delegation_chain[].delegated_at` | string | RFC 3339 timestamp of when delegation occurred |
| `delegation_chain[].signature` | string | Hex-encoded Ed25519 signature over canonical content (`agent|scope_csv|delegated_at`) |

**Constraints:**

- **Maximum delegation depth:** 5. The chain length (number of entries in `delegation_chain`) cannot exceed 5.
- **Scope attenuation only:** Delegated scope MUST be a subset of the delegator's scope. Scope can only narrow, never expand.
- **Delegate must exist:** The target agent (`delegate_to`) must already be registered in the broker.
- **Chain is inherited:** If the delegator's token already has a delegation chain, the new delegation appends to it.

**Audit events:** Records `delegation_created` on success.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `400` | Missing `delegate_to` or empty `scope` |
| `400` | Malformed JSON body |
| `401` | Missing or invalid Bearer token |
| `403` | Delegated scope exceeds delegator's scope |
| `403` | Delegation depth limit exceeded (chain length >= 5) |
| `403` | Token has been revoked |
| `404` | Delegate agent not found (not registered) |

**Example:**

```bash
curl -X POST http://localhost:8080/v1/delegate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AGENT_A_TOKEN" \
  -d '{
    "delegate_to": "spiffe://agentauth.local/agent/orch-001/task-001/b2c3d4e5",
    "scope": ["read:Customers:customer-123"],
    "ttl": 60
  }'
```

---

## Audit

### GET /v1/audit/events

Query the audit trail. Returns events with hash-chain integrity verification. Every event includes a SHA-256 hash chained to the previous event, providing tamper-evident integrity.

**Auth:** Bearer JWT with `admin:audit:*` scope

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `agent_id` | string | -- | Filter by agent SPIFFE ID (exact match) |
| `task_id` | string | -- | Filter by task ID (exact match) |
| `event_type` | string | -- | Filter by event type (exact match) |
| `since` | string | -- | RFC 3339 timestamp lower bound (inclusive) |
| `until` | string | -- | RFC 3339 timestamp upper bound (inclusive) |
| `limit` | int | `100` | Max results per page (capped at `1000`) |
| `offset` | int | `0` | Pagination offset |

**Response `200 OK`:**

```json
{
  "events": [
    {
      "id": "evt-000001",
      "timestamp": "2026-02-09T13:26:42Z",
      "event_type": "admin_auth",
      "detail": "admin authenticated as admin",
      "hash": "a3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1",
      "prev_hash": "0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "id": "evt-000002",
      "timestamp": "2026-02-09T13:26:43Z",
      "event_type": "launch_token_issued",
      "detail": "launch token issued for agent=data-reader scope=[read:Customers:*] max_ttl=300 created_by=admin",
      "hash": "b4c3d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3",
      "prev_hash": "a3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1"
    }
  ],
  "total": 2,
  "offset": 0,
  "limit": 100
}
```

**Event object fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Sequential event identifier (`evt-NNNNNN`) |
| `timestamp` | string | RFC 3339 UTC timestamp |
| `event_type` | string | Event type (see table below) |
| `agent_id` | string | Agent SPIFFE ID (omitted from JSON if empty) |
| `task_id` | string | Task ID (omitted from JSON if empty) |
| `orch_id` | string | Orchestration ID (omitted from JSON if empty) |
| `detail` | string | Human-readable event description (PII-sanitized) |
| `hash` | string | SHA-256 hash of this event chained to `prev_hash` |
| `prev_hash` | string | Hash of the preceding event (genesis: all zeros) |

**Event types:**

| Event Type | Recorded when |
|------------|---------------|
| `admin_auth` | Admin successfully authenticated via `POST /v1/admin/auth` |
| `admin_auth_failed` | Admin authentication attempt failed |
| `launch_token_issued` | Launch token created via `POST /v1/admin/launch-tokens` |
| `launch_token_denied` | Launch token request denied |
| `sidecar_activation_issued` | Sidecar activation token issued via `POST /v1/admin/sidecar-activations` |
| `sidecar_activated` | Sidecar activation token exchanged via `POST /v1/sidecar/activate` |
| `sidecar_activation_failed` | Sidecar activation failed (invalid/replayed token) |
| `sidecar_exchange_success` | Sidecar token exchange succeeded via `POST /v1/token/exchange` |
| `sidecar_exchange_denied` | Sidecar token exchange denied (scope escalation, missing ceiling, agent not found) |
| `agent_registered` | Agent registration succeeded via `POST /v1/register` |
| `registration_policy_violation` | Registration rejected due to scope violation |
| `token_issued` | Agent token issued (during registration) |
| `token_renewed` | Token renewed successfully via `POST /v1/token/renew` |
| `token_renewal_failed` | Token renewal attempt failed |
| `token_revoked` | Token/agent/task/chain revoked via `POST /v1/revoke` |
| `delegation_created` | Delegation token issued via `POST /v1/delegate` |
| `resource_accessed` | Resource access logged by a resource server |

**Hash chain integrity:** Each event's `hash` is computed as `SHA-256(prev_hash | id | timestamp | event_type | agent_id | task_id | orch_id | detail)`. To verify chain integrity, recompute each hash from the raw fields and compare.

**PII sanitization:** Detail strings containing keywords like "secret", "password", "private_key", or "token_value" have their values replaced with `***REDACTED***`.

**Error responses:**

| Status | Condition |
|--------|-----------|
| `401` | Missing or invalid Bearer token |
| `403` | Token lacks `admin:audit:*` scope |

**Examples:**

```bash
# Get all events
curl "http://localhost:8080/v1/audit/events" \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Filter by event type
curl "http://localhost:8080/v1/audit/events?event_type=agent_registered&limit=10" \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Filter by agent and time range
curl "http://localhost:8080/v1/audit/events?agent_id=spiffe://agentauth.local/agent/orch/task/inst&since=2026-02-09T00:00:00Z" \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Paginate
curl "http://localhost:8080/v1/audit/events?limit=20&offset=40" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Common Reference

### Error Format (RFC 7807)

All error responses use `Content-Type: application/problem+json` per [RFC 7807](https://tools.ietf.org/html/rfc7807):

```json
{
  "type": "urn:agentauth:error:unauthorized",
  "title": "Unauthorized",
  "status": 401,
  "detail": "token verification failed: signature verification failed",
  "instance": "/v1/admin/launch-tokens",
  "error_code": "unauthorized",
  "request_id": "c3b53ae48e64af95",
  "hint": "Ensure the Bearer token is valid and not expired"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | URN identifying the error class (e.g., `urn:agentauth:error:unauthorized`) |
| `title` | string | Human-readable error category (HTTP status text) |
| `status` | int | HTTP status code |
| `detail` | string | Specific error description |
| `instance` | string | The request path that triggered the error |
| `error_code` | string | Standardized internal error code for programmatic handling |
| `request_id` | string | Unique request identifier for log correlation |
| `hint` | string | Optional diagnostic hint for developers |

**Error type URNs:**

| URN | HTTP Status | Meaning |
|-----|-------------|---------|
| `urn:agentauth:error:invalid_request` | 400 | Malformed request, missing fields, or invalid parameters |
| `urn:agentauth:error:unauthorized` | 401 | Missing, expired, or invalid credentials |
| `urn:agentauth:error:scope_violation` | 403 | Requested scope exceeds allowed scope |
| `urn:agentauth:error:insufficient_scope` | 403 | Token lacks required admin scope, or token is revoked |
| `urn:agentauth:error:not_found` | 404 | Resource (e.g., delegate agent) not found |
| `urn:agentauth:error:internal_error` | 500 | Server-side error |

---

### Scope Format

Scopes follow the three-part pattern `action:resource:identifier`:

```
read:Customers:*              # Read any customer
read:Customers:customer-123   # Read a specific customer
write:Orders:*                # Write any order
admin:launch-tokens:*         # Admin: manage launch tokens
admin:revoke:*                # Admin: revoke credentials
admin:audit:*                 # Admin: query audit trail
```

**Scope matching rules:**

- A scope `A` is covered by scope `B` if all three parts match: `A.action == B.action AND A.resource == B.resource AND (A.identifier == B.identifier OR B.identifier == "*")`
- The wildcard `*` in the identifier position matches any identifier value
- `read:Customers:customer-123` is a subset of `read:Customers:*`
- `write:Customers:*` is NOT a subset of `read:Customers:*` (different action)
- `read:Orders:*` is NOT a subset of `read:Customers:*` (different resource)

**Subset checking:** `ScopeIsSubset(requested, allowed)` returns `true` if every scope in `requested` is covered by at least one scope in `allowed`. This is used during registration (requested vs. launch token allowed) and delegation (delegated vs. delegator's scope).

---

### JWT Claims Structure

All tokens are EdDSA-signed JWTs (algorithm: `EdDSA`, type: `JWT`). The header is:

```json
{"alg": "EdDSA", "typ": "JWT"}
```

The payload contains these claims:

```json
{
  "iss": "agentauth",
  "sub": "spiffe://agentauth.local/agent/orch-001/task-001/a1b2c3d4e5f6a7b8",
  "exp": 1770644202,
  "nbf": 1770643902,
  "iat": 1770643902,
  "jti": "e2b7777781a064686237079634888b11",
  "sid": "sidecar-123",
  "scope": ["read:Customers:*"],
  "task_id": "task-001",
  "orch_id": "orch-001",
  "delegation_chain": []
}
```

| Claim | JSON key | Type | Description |
|-------|----------|------|-------------|
| Issuer | `iss` | string | Always `"agentauth"` |
| Subject | `sub` | string | SPIFFE ID of the agent, or `"admin"` for admin tokens |
| Audience | `aud` | string[] | Optional audience (omitted if empty) |
| Expiration | `exp` | int64 | Expiration timestamp (Unix epoch seconds) |
| Not Before | `nbf` | int64 | Not-before timestamp (Unix epoch seconds) |
| Issued At | `iat` | int64 | Issued-at timestamp (Unix epoch seconds) |
| JWT ID | `jti` | string | Unique token identifier (32-character hex from 16 random bytes) |
| Sidecar/session ID | `sid` | string | Optional sidecar/session identifier (present for sidecar-mediated issuance) |
| Scope | `scope` | string[] | Array of granted scopes |
| Task ID | `task_id` | string | Task identifier (omitted for admin tokens) |
| Orchestration ID | `orch_id` | string | Orchestration identifier (omitted for admin tokens) |
| Delegation Chain | `delegation_chain` | object[] | Delegation chain (omitted if empty); see [POST /v1/delegate](#post-v1delegate) |
| Chain Hash | `chain_hash` | string | SHA-256 hex digest of the JSON-serialized delegation chain (present only on delegated tokens) |

**Token format:** Standard three-part JWT: `base64url(header).base64url(payload).base64url(signature)`. The signature is computed over the ASCII bytes of `base64url(header).base64url(payload)` using the broker's Ed25519 private key. Uses raw base64url encoding (no padding).

**Validation checks performed by `TknSvc.Verify`:**

1. Token has exactly 3 dot-separated parts
2. Signature is valid against the broker's Ed25519 public key
3. `iss` equals `"agentauth"`
4. `sub` is non-empty
5. `jti` is non-empty
6. `exp` has not passed (if set)
7. `nbf` has passed (if set)

---

### SPIFFE ID Format

Agent identifiers follow the [SPIFFE](https://spiffe.io/) format:

```
spiffe://{trust_domain}/agent/{orch_id}/{task_id}/{instance_id}
```

**Components:**

| Component | Source | Example |
|-----------|--------|---------|
| `trust_domain` | `AA_TRUST_DOMAIN` env var (default: `agentauth.local`) | `agentauth.local` |
| `orch_id` | From registration request `orch_id` field | `orchestration-001` |
| `task_id` | From registration request `task_id` field | `task-001` |
| `instance_id` | Broker-generated random hex (16 characters / 8 bytes) | `a1b2c3d4e5f6a7b8` |

**Full example:** `spiffe://agentauth.local/agent/orchestration-001/task-001/a1b2c3d4e5f6a7b8`

SPIFFE IDs are generated using the `go-spiffe/v2` library, which validates trust domain and path segment format.

---

### Prometheus Metrics

Available at `GET /v1/metrics` in Prometheus exposition format.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `agentauth_tokens_issued_total` | counter | `scope` | Total tokens issued |
| `agentauth_tokens_revoked_total` | counter | `level` | Total tokens revoked |
| `agentauth_registrations_total` | counter | `status` | Agent registration attempts (`success`/`failure`) |
| `agentauth_admin_auth_total` | counter | `status` | Admin auth attempts (`success`/`failure`) |
| `agentauth_launch_tokens_created_total` | counter | -- | Total launch tokens created |
| `agentauth_active_agents` | gauge | -- | Current number of active (registered) agents |
| `agentauth_request_duration_seconds` | histogram | `endpoint` | Request duration in seconds |
| `agentauth_clock_skew_total` | counter | -- | Clock skew events detected during token validation |

---

## End-to-End Walkthrough

This walkthrough demonstrates the complete agent lifecycle using `curl`.
For repository integration work, run the broker via Docker Compose.

```bash
# Start broker + sidecar stack
export AA_ADMIN_SECRET=my-secret
./scripts/stack_up.sh
```

### Step 1: Health check

```bash
curl -s http://localhost:8080/v1/health | jq .
# {"status":"ok","version":"2.0.0","uptime":1}
```

### Step 2: Authenticate as admin

```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id":"admin","client_secret":"my-secret"}' \
  | jq -r '.access_token')

echo "Admin token: ${ADMIN_TOKEN:0:20}..."
```

### Step 3: Create a launch token

```bash
LAUNCH_RESP=$(curl -s -X POST http://localhost:8080/v1/admin/launch-tokens \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "agent_name": "data-reader",
    "allowed_scope": ["read:Customers:*"],
    "max_ttl": 300,
    "ttl": 120
  }')

LAUNCH_TOKEN=$(echo "$LAUNCH_RESP" | jq -r '.launch_token')
echo "Launch token: ${LAUNCH_TOKEN:0:16}..."
echo "Expires at: $(echo "$LAUNCH_RESP" | jq -r '.expires_at')"
echo "Policy: $(echo "$LAUNCH_RESP" | jq '.policy')"
```

### Step 4: Get a challenge nonce

```bash
NONCE=$(curl -s http://localhost:8080/v1/challenge | jq -r '.nonce')
echo "Nonce: ${NONCE:0:16}..."
```

### Step 5: Register the agent (Python helper)

The registration step requires Ed25519 cryptography. Here is a complete Python script:

```python
#!/usr/bin/env python3
"""Register an agent with the AgentAuth broker."""
import base64, json, sys, requests
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

BROKER = "http://localhost:8080"

def register(launch_token: str) -> dict:
    # Generate Ed25519 key pair
    private_key = Ed25519PrivateKey.generate()
    pub_bytes = private_key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)

    # Get challenge nonce
    nonce = requests.get(f"{BROKER}/v1/challenge").json()["nonce"]

    # Sign the nonce bytes
    nonce_bytes = bytes.fromhex(nonce)
    signature = private_key.sign(nonce_bytes)

    # Register
    resp = requests.post(f"{BROKER}/v1/register", json={
        "launch_token": launch_token,
        "nonce": nonce,
        "public_key": base64.b64encode(pub_bytes).decode(),
        "signature": base64.b64encode(signature).decode(),
        "orch_id": "demo-orch",
        "task_id": "demo-task",
        "requested_scope": ["read:Customers:*"],
    })
    resp.raise_for_status()
    return resp.json()

if __name__ == "__main__":
    result = register(sys.argv[1])
    print(json.dumps(result, indent=2))
```

```bash
# Run the registration
REG_RESP=$(python3 register_agent.py "$LAUNCH_TOKEN")
AGENT_ID=$(echo "$REG_RESP" | jq -r '.agent_id')
AGENT_TOKEN=$(echo "$REG_RESP" | jq -r '.access_token')
echo "Agent ID: $AGENT_ID"
```

### Step 6: Validate the agent token

```bash
curl -s -X POST http://localhost:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}" | jq .
```

### Step 7: Renew the agent token

```bash
NEW_TOKEN=$(curl -s -X POST http://localhost:8080/v1/token/renew \
  -H "Authorization: Bearer $AGENT_TOKEN" \
  | jq -r '.access_token')

echo "Renewed token: ${NEW_TOKEN:0:20}..."
```

### Step 8: Delegate to another agent

First register a second agent (Agent B), then delegate from Agent A:

```bash
# Assume AGENT_B_ID was obtained by registering a second agent
curl -s -X POST http://localhost:8080/v1/delegate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AGENT_TOKEN" \
  -d "{
    \"delegate_to\": \"$AGENT_B_ID\",
    \"scope\": [\"read:Customers:customer-123\"],
    \"ttl\": 60
  }" | jq .
```

### Step 9: Revoke a token

```bash
# Get the JTI from the token claims
JTI=$(curl -s -X POST http://localhost:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}" \
  | jq -r '.claims.jti')

# Revoke it
curl -s -X POST http://localhost:8080/v1/revoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\":\"token\",\"target\":\"$JTI\"}" | jq .
# {"revoked":true,"level":"token","target":"...","count":1}

# Verify it's revoked -- any Bearer request with the old token now gets 403
curl -s -X POST http://localhost:8080/v1/token/renew \
  -H "Authorization: Bearer $AGENT_TOKEN"
# {"type":"urn:agentauth:error:insufficient_scope","title":"Forbidden","status":403,"detail":"token has been revoked",...}
```

### Step 10: Query the audit trail

```bash
curl -s "http://localhost:8080/v1/audit/events?limit=50" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.events[] | {id, event_type, detail}'
```
