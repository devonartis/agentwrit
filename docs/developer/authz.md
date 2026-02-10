# Authorization Module (M03)

## Purpose

M03 adds zero-trust request authorization middleware (`ValMw`) that:
- fails closed when bearer token is missing/invalid
- enforces route-required scope against token scope claims
- writes why-denied structured logs for diagnostics

## Middleware flow

1. Read `Authorization: Bearer ...` header
2. Verify token signature and temporal claims via `TknSvc.Verify`
3. Read required scope from request context
4. Enforce scope match (`ScopeMatch`)
5. On success, inject `agent_id` into request context and continue
6. On failure, return RFC 7807 error (`401` or `403`)

## Design decisions

**Middleware over per-handler checks**: Authorization is enforced in a single `ValMw` middleware rather than duplicated in each handler. This ensures fail-closed behavior — new endpoints are protected by default when wrapped, and there is no risk of a handler forgetting to check the token.

**Context-injected scopes**: Required scopes are injected via `WithRequiredScope` rather than hard-coded in the middleware. This keeps the middleware generic and lets each route declare its own access requirements at the mux wiring level.

**Why-denied logging**: Every denial emits a structured `obs.Fail` with a machine-parseable `reason=` key. This supports zero-trust audit requirements without leaking sensitive claim data to the HTTP response.

## Protected endpoint example

`GET /v1/protected/customers/12345`

Required scope:
- `read:Customers:12345`

## Why-denied logging

Failure paths emit structured logs with machine-parseable reason keys:
- `reason=missing_bearer`
- `reason=invalid_token`
- `reason=scope_mismatch`

## Context helper for downstream handlers

`AgentIDFromContext(ctx)` returns the authenticated agent SPIFFE ID that `ValMw` stored in request context.
Use this in protected handlers to attribute actions to the authenticated principal.

## Validation

```bash
go test ./internal/authz ./tests/integration -tags=integration -v
./scripts/live_test.sh
```
