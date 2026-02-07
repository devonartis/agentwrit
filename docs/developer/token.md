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

## Delegation chain

The `delegation_chain` claim tracks the provenance of delegated tokens. Each entry is a `DelegRecord`:

- `agent` — SPIFFE ID of the delegating agent
- `scope` — scopes the delegator held at delegation time
- `delegated_at` — ISO 8601 timestamp of the delegation
- `signature` — Ed25519 signature by the delegating agent

**Chain behavior**: When agent A delegates to agent B, B's token carries A's record in its chain. Scope attenuation is enforced — B cannot receive broader scope than A held. `ScopeIsSubset(requested, parent)` validates this.

**Chain hash**: For revocation purposes (M04), the delegation chain is hashed by JSON-serializing the `[]DelegRecord` array and computing SHA-256. This hash enables revoking all tokens sharing a common delegation lineage. Empty chains produce an empty hash string and skip chain-level revocation checks.

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

