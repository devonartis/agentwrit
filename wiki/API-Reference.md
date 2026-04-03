# API Reference

Complete reference for every AgentAuth HTTP endpoint. All examples shown in curl, Python, and TypeScript.

> **Base URLs:**
> - **Broker:** `http://localhost:8080` (default)
> - **Sidecar:** `http://localhost:8081` (default)
>
> **Content Type:** All requests and responses use `application/json`
>
> **Error Format:** All errors follow [RFC 7807](https://www.rfc-editor.org/rfc/rfc7807) (see [Error Responses](#error-responses))

---

## Table of Contents

- [Public Endpoints](#public-endpoints) (no auth required)
- [Agent Endpoints](#agent-endpoints) (Bearer token required)
- [Admin Endpoints](#admin-endpoints) (admin token required)
- [Sidecar Endpoints](#sidecar-endpoints) (called via sidecar)
- [Error Responses](#error-responses)

---

## Public Endpoints

These endpoints require **no authentication**. Anyone can call them.

### GET /v1/health

Check if the service is running.

```bash
curl http://localhost:8080/v1/health
```

**Response (200):**
```json
{
  "status": "ok"
}
```

Sidecar health includes the scope ceiling:
```bash
curl http://localhost:8081/v1/health
```
```json
{
  "status": "ok",
  "scope_ceiling": ["read:data:*", "write:data:*"]
}
```

---

### GET /v1/metrics

Prometheus-format metrics.

```bash
curl http://localhost:8081/v1/metrics
```

Returns Prometheus text format with counters for tokens issued, renewed, revoked, etc.

---

### GET /v1/challenge

Get a cryptographic nonce for agent registration (used in the BYOK flow).

```bash
curl http://localhost:8080/v1/challenge
```

**Response (200):**
```json
{
  "nonce": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"
}
```

> **Note:** Nonces expire after 30 seconds and are single-use. If you use the sidecar, you never call this directly.

---

### POST /v1/token/validate

Validate a token and inspect its claims. No auth required — anyone can validate.

**Request:**
```json
{
  "token": "eyJhbGciOi..."
}
```

**Python:**
```python
resp = requests.post("http://localhost:8080/v1/token/validate",
    json={"token": "eyJhbGciOi..."})
result = resp.json()
```

**TypeScript:**
```typescript
const resp = await fetch("http://localhost:8080/v1/token/validate", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ token: "eyJhbGciOi..." })
});
const result = await resp.json();
```

**Response (200, valid):**
```json
{
  "valid": true,
  "claims": {
    "iss": "agentauth",
    "sub": "spiffe://agentauth.local/agent/orch-001/task-42/proc-abc",
    "exp": 1745405630,
    "iat": 1745405330,
    "jti": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
    "scope": ["read:data:*"],
    "task_id": "task-42",
    "orch_id": "orch-001"
  }
}
```

**Response (200, invalid):**
```json
{
  "valid": false,
  "error": "token_expired",
  "detail": "Token expired at 2026-02-15T10:05:00Z"
}
```

---

## Agent Endpoints

These require a **Bearer token** in the `Authorization` header.

### POST /v1/register

Register a new agent (BYOK flow). The sidecar calls this automatically.

**Request:**
```json
{
  "agent_name": "my-agent",
  "public_key": "base64-encoded-32-byte-ed25519-public-key",
  "signature": "base64-encoded-ed25519-signature-of-nonce-bytes",
  "nonce": "hex-nonce-from-challenge",
  "launch_token": "64-char-hex-launch-token",
  "task_id": "task-001",
  "orch_id": "orch-001",
  "requested_scope": ["read:data:*"]
}
```

**Response (200):**
```json
{
  "agent_id": "spiffe://agentauth.local/agent/orch-001/task-001/proc-abc123",
  "access_token": "eyJhbGciOi...",
  "expires_in": 300
}
```

| Status | Meaning |
|--------|---------|
| 200 | Registered successfully |
| 400 | Invalid request (missing fields, bad format) |
| 401 | Invalid launch token, nonce, or signature |
| 403 | Scope exceeds launch token's allowed scope |

---

### POST /v1/token/renew

Renew a token before it expires.

**Request:** No body. Token in Authorization header.

```bash
curl -X POST http://localhost:8081/v1/token/renew \
  -H "Authorization: Bearer eyJhbGciOi..."
```

**Python:**
```python
resp = requests.post(f"{SIDECAR}/v1/token/renew",
    headers={"Authorization": f"Bearer {token}"})
```

**TypeScript:**
```typescript
const resp = await fetch(`${SIDECAR_URL}/v1/token/renew`, {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` }
});
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOi...(new token)...",
  "expires_in": 300
}
```

| Status | Meaning |
|--------|---------|
| 200 | Renewed successfully |
| 401 | Token expired |
| 403 | Token revoked |

---

### POST /v1/delegate

Delegate a narrower scope to another agent.

**Request:**
```json
{
  "delegate_to": "agent-b-name",
  "scope": ["read:data:reports"],
  "ttl": 60
}
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOi...(token for delegate)...",
  "delegation_chain": [
    {
      "agent": "spiffe://agentauth.local/agent/.../agent-a",
      "scope": ["read:data:*"],
      "signature": "base64-signature"
    }
  ],
  "chain_hash": "sha256-of-chain"
}
```

| Status | Meaning |
|--------|---------|
| 200 | Delegation successful |
| 400 | Invalid request |
| 403 | Scope not a subset of delegator's scope, or max depth (5) exceeded |

---

### POST /v1/token/release

Release a token (signal task completion).

```bash
curl -X POST http://localhost:8080/v1/token/release \
  -H "Authorization: Bearer eyJhbGciOi..."
```

**Response:** `204 No Content` (success)

---

## Admin Endpoints

These require an **admin token** from `/v1/admin/auth`.

### POST /v1/admin/auth

Authenticate as an administrator.

**Request:**
```json
{
  "client_id": "admin",
  "client_secret": "your-AA_ADMIN_SECRET-value"
}
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOi...(admin token)...",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

> **Rate limit:** 5 requests/second with burst of 10. Cache the admin token.

---

### POST /v1/admin/launch-tokens

Create a launch token for agent bootstrap.

**Request:**
```json
{
  "agent_name": "new-agent",
  "allowed_scope": ["read:data:*"],
  "max_ttl": 300,
  "single_use": true,
  "ttl": 30
}
```

**Response (200):**
```json
{
  "launch_token": "64-character-hex-string",
  "expires_at": "2026-02-27T15:32:40Z"
}
```

---

### POST /v1/revoke

Revoke tokens at various levels.

**Request:**
```json
{
  "level": "agent",
  "target": "spiffe://agentauth.local/agent/..."
}
```

| Level | Target | What Gets Revoked |
|-------|--------|-------------------|
| `token` | JTI string | One specific token |
| `agent` | SPIFFE ID | All tokens for that agent |
| `task` | Task ID | All tokens for that task |
| `chain` | SPIFFE ID | All tokens in delegation chain |

**Response (200):**
```json
{
  "revoked": true,
  "level": "agent",
  "target": "spiffe://...",
  "count": 3
}
```

---

### GET /v1/audit/events

Query the audit trail.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `agent_id` | string | Filter by agent SPIFFE ID |
| `task_id` | string | Filter by task ID |
| `event_type` | string | Filter by event type |
| `since` | string | Events after this time (RFC3339) |
| `until` | string | Events before this time (RFC3339) |
| `outcome` | string | `success` or `denied` |
| `limit` | int | Max results (default 100) |
| `offset` | int | Pagination offset |

**Response (200):**
```json
{
  "events": [
    {
      "id": "uuid",
      "timestamp": "2026-02-27T15:32:10Z",
      "event_type": "token_issued",
      "agent_id": "spiffe://...",
      "outcome": "success",
      "detail": "Token issued for scope read:data:*"
    }
  ],
  "total": 42,
  "offset": 0,
  "limit": 100
}
```

---

### POST /v1/sidecar/activate

Activate a sidecar (called by the sidecar during bootstrap).

---

### GET /v1/admin/sidecars

List all registered sidecars.

---

### GET /v1/admin/sidecars/{id}/ceiling

Get a sidecar's scope ceiling.

---

### PUT /v1/admin/sidecars/{id}/ceiling

Update a sidecar's scope ceiling.

---

## Sidecar Endpoints

These are called on the **sidecar** (port 8081 by default), not the broker.

### POST /v1/token

Request a scoped token (the sidecar handles registration automatically).

**Request:**
```json
{
  "agent_name": "my-agent",
  "scope": ["read:data:*"],
  "ttl": 300,
  "task_id": "task-001"
}
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOi...",
  "expires_in": 300,
  "scope": ["read:data:*"],
  "agent_id": "spiffe://agentauth.local/agent/.../proc-abc",
  "token_type": "Bearer"
}
```

---

## Error Responses

All broker errors use the RFC 7807 format:

```json
{
  "type": "urn:agentauth:error:scope_violation",
  "title": "Forbidden",
  "status": 403,
  "detail": "requested scope exceeds allowed scope",
  "instance": "/v1/register",
  "error_code": "scope_violation",
  "request_id": "a1b2c3d4e5f67890",
  "hint": "requested scope must be a subset of allowed scope"
}
```

### Error Codes

| Code | Status | Meaning |
|------|--------|---------|
| `invalid_request` | 400 | Malformed JSON, missing fields |
| `invalid_scope_format` | 400 | Scope not in `action:resource:identifier` format |
| `invalid_ttl` | 400 | TTL negative or exceeds maximum |
| `unauthorized` | 401 | Missing/invalid token or credentials |
| `scope_violation` | 403 | Requested scope exceeds allowed scope |
| `insufficient_scope` | 403 | Token lacks required scope |
| `not_found` | 404 | Agent or resource not found |
| `rate_limited` | 429 | Rate limit exceeded |
| `internal_error` | 500 | Unexpected server failure |

### Request ID

Every response includes a `request_id` (in JSON body and `X-Request-ID` header). Include this when reporting issues.

---

## Next Steps

- [[Common Tasks]] — Recipes using these endpoints
- [[Troubleshooting]] — Fix common errors by status code
- [[Developer Guide]] — Integration guide
