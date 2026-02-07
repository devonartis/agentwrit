# Token Module (M02)

## Purpose

The token module issues, verifies, and renews short-lived task-scoped tokens.

## Token format

AgentAuth currently uses a JWT-style compact format:

`base64url(header).base64url(payload).base64url(signature)`

- Header: `{"alg":"EdDSA","typ":"JWT"}`
- Signature: Ed25519 over `header.payload`
- Claims include: `sub`, `scope`, `task_id`, `orchestration_id`, `exp`, `nbf`, `iat`, `jti`

## Scope model

Scope syntax:

`action:resource:identifier`

Examples:
- `read:Customers:12345`
- `read:Customers:*`

Rules:
- `ScopeMatch(required, available)` checks whether available scope grants required access.
- wildcard identifier `*` grants all identifiers for same action/resource.
- `ScopeIsSubset(child, parent)` enforces delegation attenuation.

## Renewal behavior

`POST /v1/token/renew` verifies the current token first, then issues a new token with:
- same `sub`
- same `scope`
- same `task_id`
- same `orchestration_id`
- refreshed `exp` and `jti`

## Running token tests

```bash
go test ./internal/token ./internal/handler -v
./scripts/integration_test.sh
./scripts/live_test.sh
```

