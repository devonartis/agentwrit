# AgentAuth API Reference

## API contract source

OpenAPI document:
- `docs/api/openapi.yaml`

## Base URL

Local development:
- `http://127.0.0.1:8080`

## Endpoints currently implemented (M01-M04)

### GET /v1/health

Purpose:
- broker liveness/readiness check

Authentication:
- none

Success response:
- status: `200`
- body:

```json
{
  "status": "healthy"
}
```

## Error model (current)

Error responses follow RFC 7807 (`application/problem+json`) where implemented.
Common fields:
- `type`
- `title`
- `status`

Common categories currently used:
- bad request (`400`)
- authentication failure (`401`)
- authorization failure (`403`)
- internal error (`500`)

### GET /v1/challenge

Purpose:
- generate a nonce for anti-replay registration proof

Authentication:
- none

Success response:
- status: `200`
- body fields:
  - `nonce` (64-char hex)
  - `expires_at` (RFC3339 timestamp)

### POST /v1/register

Purpose:
- validate launch token + nonce signature and register agent identity

Authentication:
- launch token and proof-of-possession signature

Success response:
- status: `201`
- body fields:
  - `agent_instance_id`
  - `access_token`
  - `expires_in`
  - `refresh_after`

Error responses:
- `400` malformed request body
- `401` invalid/expired/reused launch token
- `403` nonce/signature verification failed

Errors are returned as RFC 7807 `application/problem+json`.

### POST /v1/token/validate

Purpose:
- verify token signature/expiry and optionally enforce required scope

Request body:
- `token` (required)
- `required_scope` (optional)

Success response:
- status: `200`
- fields:
  - `valid`
  - `agent_id`
  - `scope`
  - `expires_in`
  - `delegation_depth`

Error responses:
- `401` invalid token
- `403` scope mismatch

### POST /v1/token/renew

Purpose:
- rotate token for long-running agents

Request body:
- `token` (required)

Success response:
- status: `200`
- fields:
  - `access_token`
  - `expires_in`
  - `refresh_after`

Error responses:
- `401` invalid token

### POST /v1/revoke

Purpose:
- revoke tokens at 4 levels: token, agent, task, delegation_chain

Authentication:
- none (admin endpoint)

Request body:
- `level` (required) — one of: `token`, `agent`, `task`, `delegation_chain`
- `target_id` (required) — the identifier to revoke (JTI, agent SPIFFE ID, task ID, or chain hash)
- `reason` (optional) — human-readable reason for the revocation

Success response:
- status: `200`
- body fields:
  - `revoked` (boolean, always true)
  - `level` (echoed back)
  - `target_id` (echoed back)
  - `revoked_at` (RFC 3339 timestamp)

Error responses:
- `400` invalid level or missing target_id

Errors are returned as RFC 7807 `application/problem+json`.

### GET /v1/protected/customers/12345

Purpose:
- demonstrate zero-trust middleware enforcement on a protected resource

Authentication:
- required bearer token in `Authorization` header

Required scope:
- `read:Customers:12345`

Success response:
- status: `200`
- returns protected customer payload

Error responses:
- `401` missing/invalid bearer token
- `403` insufficient scope

## Versioning

- API version path prefix: `/v1`
- Spec version in `openapi.yaml` should be updated with endpoint additions.
